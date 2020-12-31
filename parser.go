package openapiparser

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type parser struct {
	specFile string
	paths    map[string]Path

	schemasParsed   map[string]*Schema
	responsesParsed map[string]*Response
	pathsParsed     map[string]*Path
	parsedFilesMx   sync.RWMutex

	schemas        map[string]*Schema
	schemaNames    map[string]string
	schemaPromises map[string][]chan Schema
	schemasMx      sync.RWMutex

	parameters         map[string]*Parameter
	parametersPromises map[string][]chan Parameter
	parametersMx       sync.RWMutex
}

type reference struct {
	Ref string `yaml:"$ref,omitempty"`
}

func newParser(file string) *parser {
	return &parser{
		specFile: file,

		// maps that holds resources already parsed with by file as cache.
		schemasParsed:   make(map[string]*Schema),
		pathsParsed:     make(map[string]*Path),
		responsesParsed: make(map[string]*Response),

		// parameters holds the parameters parsed by the key in the root spec file.
		parameters:         make(map[string]*Parameter),
		parametersPromises: make(map[string][]chan Parameter),

		// schemas holds the schemas parsed by the key in the root spec file.
		schemas:        make(map[string]*Schema),
		schemaNames:    make(map[string]string), // file => name
		schemaPromises: make(map[string][]chan Schema),
	}
}

func (p *parser) parse() (spec *Spec) {
	spec = new(Spec)
	rootContent := fileContent(p.specFile)
	capture(rootContent, map[string]captureFunc{
		"openapi":    captureString(&spec.OpenAPI),
		"info":       captureNodeDecode(&spec.Info),
		"servers":    captureNodeDecode(&spec.Servers),
		"paths":      p.capturePaths(spec),
		"components": p.captureComponents(spec),
	})

	return spec
}

func (p *parser) addSchema(key string, x Schema) {
	p.schemasMx.Lock()
	defer p.schemasMx.Unlock()

	p.schemas[key] = &x
	if promises, ok := p.schemaPromises[key]; ok {
		for _, c := range promises {
			c <- x
			close(c)
		}
		delete(p.schemaPromises, key)
	}
}

func (p *parser) schemaByFile(currentFile, ref string) Schema {
	p.schemasMx.RLock()
	key, ok := p.schemaNames[joinPath(currentFile, ref)]
	p.schemasMx.RUnlock()
	if ok {
		return <-p.promiseSchemaByKey(key)
	}

	// is an anonymous schema, go solve it
	return p.parseSchemaRef(currentFile, ref)
}

func (p *parser) promiseSchemaByKey(key string) <-chan Schema {
	p.schemasMx.RLock()
	defer p.schemasMx.RUnlock()

	c := make(chan Schema, 1)
	if x, ok := p.schemas[key]; ok {
		c <- *x
		close(c)
		return c
	}
	p.schemaPromises[key] = append(p.schemaPromises[key], c)
	return c
}

func (p *parser) addParameter(key string, x Parameter) {
	p.parametersMx.Lock()
	defer p.parametersMx.Unlock()

	p.parameters[key] = &x
	if promises, ok := p.parametersPromises[key]; ok {
		for _, c := range promises {
			c <- x
			close(c)
		}
		delete(p.parametersPromises, key)
	}
}

func (p *parser) promiseParameter(key string) <-chan Parameter {
	p.parametersMx.Lock()
	defer p.parametersMx.Unlock()

	c := make(chan Parameter, 1)
	if param, ok := p.parameters[key]; ok {
		c <- *param
		close(c)
		return c
	}

	p.parametersPromises[key] = append(p.parametersPromises[key], c)
	return c
}

func (p *parser) capturePaths(spec *Spec) captureFunc {
	return func(node *yaml.Node) {
		for _, v := range mapPairs(node.Content) {
			key := v.key
			next := assertKind(v.node, yaml.MappingNode)
			var ref reference
			assertNodeDecode(next, &ref)
			assertRefNotEmpty(key, &ref)

			pp := p.parsePathRef(p.specFile, ref.Ref)
			pp.Key = key
			spec.Paths = append(spec.Paths, pp)
		}
	}
}

func (p *parser) captureComponents(spec *Spec) captureFunc {
	return func(node *yaml.Node) {
		capture(node.Content, map[string]captureFunc{
			"schemas":    p.captureSchemas(spec),
			"parameters": p.captureParameters(spec),
		})
	}
}

func (p *parser) captureParameters(spec *Spec) captureFunc {
	return func(node *yaml.Node) {
		for _, v := range mapPairs(node.Content) {
			next := assertKind(v.node, yaml.MappingNode)
			var param Parameter
			assertNodeDecode(next, &param)
			assertNotRefRecursively(&param.Schema)

			p.addParameter(v.key, param)
			spec.Parameters = append(spec.Parameters, param)
		}
	}
}

// captureSchemas will capture the schemas, it has to do it concurrently
// since a schemas may include another that will be waiting patiently
// until it is parsed to continue.
func (p *parser) captureSchemas(spec *Spec) captureFunc {
	type schemaDef struct {
		name string
		file string
		ref  string
	}
	return func(node *yaml.Node) {
		var (
			wg    sync.WaitGroup
			pairs = mapPairs(node.Content)
			defs  = make([]schemaDef, len(pairs))
		)
		spec.Schemas = make([]Schema, len(pairs))

		// first pass to get key and file.
		p.schemasMx.Lock()
		p.schemaNames = make(map[string]string)
		for i, v := range pairs {
			key := v.key
			var ref reference
			assertKind(v.node, yaml.MappingNode)
			assertNodeDecode(v.node, &ref)
			assertRefNotEmpty(key, &ref)
			file := joinPath(p.specFile, ref.Ref)
			p.schemaNames[file] = v.key
			defs[i] = schemaDef{
				ref:  ref.Ref,
				file: file,
			}
		}
		p.schemasMx.Unlock()
		// parse concurrently.
		for i, v := range pairs {
			wg.Add(1)
			go func(i int, v pair) {
				defer wg.Done()
				file := defs[i].file
				x := p.parseSchemaContent(file, fileNode(file))
				x.Ref = defs[i].ref
				x.Name = v.key

				p.addSchema(v.key, x)
				spec.Schemas[i] = x
			}(i, v)
		}
		wg.Wait()
	}
}

func (p *parser) captureResponseRef(currentFile string, res *Response, isRef *bool) captureFunc {
	return func(node *yaml.Node) {
		*isRef = true
		ref := assertScalar(node).Value

		// Check if we have parsed it already
		responseFile := joinPath(currentFile, ref)
		p.parsedFilesMx.RLock()
		cache, ok := p.responsesParsed[responseFile]
		p.parsedFilesMx.RUnlock()
		if ok {
			*res = *cache
			return
		}

		p.captureResponse(responseFile, res)(fileNode(responseFile))
		res.Ref = ref

		p.parsedFilesMx.Lock()
		p.responsesParsed[responseFile] = res
		p.parsedFilesMx.Unlock()
	}
}

func (p *parser) captureMediaTypes(currentFile string, mediaTypes *ContentTypes) captureFunc {
	return func(node *yaml.Node) {
		assertKind(node, yaml.MappingNode)
		*mediaTypes = make(ContentTypes)
		mt := *mediaTypes
		for _, v := range mapPairs(node.Content) {
			assertKind(v.node, yaml.MappingNode)
			var mediaType MediaType
			capture(v.node.Content, captureMap{
				"schema":   p.captureSchema(currentFile, &mediaType.Schema),
				"example":  captureRawFunc(&mediaType.Example),
				"examples": captureRawFunc(&mediaType.Examples),
			})
			mt[v.key] = mediaType
		}
	}
}

func (p *parser) parseProperties(currentFile string, node *yaml.Node) (props []Schema) {
	for _, v := range mapPairs(node.Content) {
		schema := p.parseSchemaContent(currentFile, v.node)
		schema.Key = v.key
		props = append(props, schema)
	}
	return props
}

func (p *parser) parseSchemaRef(currentFile string, ref string) (schema Schema) {
	// Check if we have parsed it already
	schemaFile := joinPath(currentFile, ref)
	p.parsedFilesMx.RLock()
	cache, ok := p.schemasParsed[schemaFile]
	p.parsedFilesMx.RUnlock()
	if ok {
		return *cache
	}

	schema = p.parseSchemaContent(schemaFile, fileNode(schemaFile))
	schema.Ref = ref

	p.parsedFilesMx.Lock()
	p.schemasParsed[schemaFile] = &schema
	p.parsedFilesMx.Unlock()

	return schema
}

func (p *parser) parsePathRef(currentFile string, ref string) (path Path) {
	pathFile := joinPath(currentFile, ref)
	p.parsedFilesMx.RLock()
	cache, ok := p.pathsParsed[pathFile]
	p.parsedFilesMx.RUnlock()
	if ok {
		return *cache
	}

	path = p.parsePathFileContent(pathFile, fileContent(pathFile))
	path.Ref = ref

	p.parsedFilesMx.Lock()
	p.pathsParsed[pathFile] = &path
	p.parsedFilesMx.Unlock()

	return path
}

func (p *parser) captureSchemaRef(currentFile string, schema *Schema, isRef *bool) captureFunc {
	return func(node *yaml.Node) {
		*isRef = true
		ref := assertScalar(node).Value
		idx := strings.Index(ref, "#")
		if idx == -1 {
			// this may be a direct reference to a component schema, we need to wait
			// until all of them are solved.
			*schema = p.schemaByFile(currentFile, ref)
			return
		}
		file := currentFile
		if idx > 0 {
			// the ref contains both a file path and the hashed reference, separate them
			file = joinPath(currentFile, ref[:idx])
			ref = ref[idx:]
		}
		if file != p.specFile {
			// hashed references are only allowed if they point to the root file.
			panic(fmt.Errorf("only local references to the root spec file are valid: %q", file))
		}
		if strings.Index(ref, "#/components/schemas/") != 0 {
			panic(fmt.Errorf("only local references to root component schemas are supported, got: %q", ref))
		}
		// we may have a reference to a component
		// that has not been parsed yet...
		*schema = <-p.promiseSchemaByKey(ref[21:])
	}
}

func (p *parser) captureProperties(currentFile string, props *[]Schema) captureFunc {
	return func(node *yaml.Node) {
		*props = p.parseProperties(currentFile, node)
	}
}

func (p *parser) captureSchema(currentFile string, x *Schema) captureFunc {
	return func(node *yaml.Node) {
		*x = p.parseSchemaContent(currentFile, node)
	}
}

func (p *parser) capturePointerSchema(currentFile string, x **Schema) captureFunc {
	return func(node *yaml.Node) {
		if x == nil {
			x = new(*Schema)
		}
		if *x == nil {
			*x = new(Schema)
		}
		**x = p.parseSchemaContent(currentFile, node)
	}
}

// parseSchemaContent must contain the
func (p *parser) parseSchemaContent(currentFile string, node *yaml.Node) (schema Schema) {
	var (
		refSchema Schema
		isRef     bool
	)
	capture(node.Content, captureMap{
		"$ref":             p.captureSchemaRef(currentFile, &refSchema, &isRef),
		"type":             captureString(&schema.Type),
		"format":           captureString(&schema.Format),
		"description":      captureString(&schema.Description),
		"title":            captureString(&schema.Title),
		"required":         captureStringSlice(&schema.Required),
		"nullable":         captureBool(&schema.Nullable),
		"pattern":          captureString(&schema.Pattern),
		"readOnly":         captureBool(&schema.ReadOnly),
		"writeOnly":        captureBool(&schema.WriteOnly),
		"minimum":          captureFloat(&schema.Minimum),
		"maximum":          captureFloat(&schema.Maximum),
		"exclusiveMinimum": captureFloat(&schema.ExclusiveMinimum),
		"exclusiveMaximum": captureFloat(&schema.ExclusiveMaximum),
		"minItems":         captureInt(&schema.MinItems),
		"maxItems":         captureInt(&schema.MaxItems),
		"uniqueItems":      captureBool(&schema.UniqueItems),
		"minLength":        captureInt(&schema.MinLength),
		"maxLength":        captureInt(&schema.MaxLength),
		"minProperties":    captureInt(&schema.MinProperties),
		"maxProperties":    captureInt(&schema.MaxProperties),
		"enum":             captureStringSlice(&schema.Enum),
		"x-enum":           captureEnumX(&schema.EnumX),
		"default":          captureRawFunc(&schema.Default),
		"example":          captureRawFunc(&schema.Example),
		"properties":       p.captureProperties(currentFile, &schema.Properties),
		"items":            p.capturePointerSchema(currentFile, &schema.Items),
	})
	if isRef {
		return refSchema
	}
	return schema
}

func (p *parser) parsePathFileContent(currentFile string, content []*yaml.Node) (path Path) {
	for _, v := range mapPairs(content) {
		assertKind(v.node, yaml.MappingNode)

		var endpoint Endpoint
		capture(v.node.Content, captureMap{
			"responses":   p.captureResponses(currentFile, &endpoint.Responses),
			"tags":        captureStringSlice(&endpoint.Tags),
			"summary":     captureString(&endpoint.Summary),
			"description": captureString(&endpoint.Description),
			"operationId": captureString(&endpoint.OperationID),
			"requestBody": p.captureRequestBody(currentFile, &endpoint.RequestBody),
			"parameters":  p.captureEndpointParameters(currentFile, &endpoint.Parameters),
		})

		endpoint.Method = v.key
		path.Endpoints = append(path.Endpoints, endpoint)
	}
	return path
}

func (p *parser) captureResponse(currentFile string, res *Response) captureFunc {
	return func(node *yaml.Node) {
		var (
			refRes Response
			isRef  bool
		)
		assertKind(node, yaml.MappingNode)
		capture(node.Content, captureMap{
			"$ref":        p.captureResponseRef(currentFile, &refRes, &isRef),
			"description": captureString(&res.Description),
			"content":     p.captureMediaTypes(currentFile, &res.ContentTypes),
		})
		if isRef {
			*res = refRes
		}
	}
}

func (p *parser) captureResponses(currentFile string, responses *[]Response) captureFunc {
	return func(node *yaml.Node) {
		for _, v := range mapPairs(node.Content) {
			var res Response
			p.captureResponse(currentFile, &res)(v.node)
			res.Status = v.key
			*responses = append(*responses, res)
		}
	}
}

func (p *parser) captureRequestBody(currentFile string, body *RequestBody) captureFunc {
	return func(node *yaml.Node) {
		capture(node.Content, captureMap{
			// "$ref":     unsupported, will panic
			"content":     p.captureMediaTypes(currentFile, &body.ContentTypes),
			"required":    captureBool(&body.Required),
			"description": captureString(&body.Description),
			"example":     captureRawFunc(&body.Example),
		})
	}
}

func (p *parser) captureEndpointParameters(currentFile string, parameters *[]Parameter) captureFunc {
	return func(node *yaml.Node) {
		assertKind(node, yaml.SequenceNode)
		for _, n := range node.Content {
			assertKind(n, yaml.MappingNode)
			var param Parameter
			if err := n.Decode(&param); err != nil {
				panic(fmt.Errorf("cannot decode path parameter: %v", err))
			}
			assertNotRefRecursively(&param.Schema)
			if param.Ref != "" {
				ref := param.Ref
				dir := filepath.Dir(currentFile)
				idx := strings.Index(ref, "#")
				if idx == -1 {
					// easy case, there's no hash part, we can go read the file directly.
					panic(fmt.Errorf("parameters references can only be defined in the root spec file: %q", ref))
				}
				file := currentFile
				if idx > 0 {
					// the ref contains both a file path and the hashed reference, separate them
					file = filepath.Join(dir, ref[:idx])
					ref = ref[idx:]
				}
				if file != p.specFile {
					// hashed references are only allowed if they point to the root file.
					panic(fmt.Errorf("only local references to the root spec file are valid: %q", file))
				}
				idx = strings.Index(ref, "#/components/parameters/")
				if idx != 0 {
					panic(fmt.Errorf("only local references to root component schemas are supported, got: %q", ref))
				}
				paramKey := ref[24:]

				// We may not have it yet, but we will (or die trying with a deadlock...)
				*parameters = append(*parameters, <-p.promiseParameter(paramKey))
				continue
			}
			if param.Schema.Type == "" {
				panic(fmt.Errorf("no schema defined for path parameter at: %q", currentFile))
			}
			*parameters = append(*parameters, param)
		}
	}
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

func assertNodeDecode(node *yaml.Node, v interface{}) {
	if err := node.Decode(v); err != nil {
		panic(fmt.Errorf("cannot decoded node into struct type %s: %v", reflect.TypeOf(v), err))
	}
}

func assertKind(node *yaml.Node, want yaml.Kind) *yaml.Node {
	if got := node.Kind; got != want {
		panic(fmt.Errorf("expected node kind %q, got %q", want, got))
	}
	return node
}

func assertScalar(node *yaml.Node) *yaml.Node {
	if got := node.Kind; got != yaml.ScalarNode {
		panic(fmt.Errorf("expected node kind %q, got %q", yaml.ScalarNode, got))
	}
	return node
}

func fileNode(file string) *yaml.Node {
	f, err := os.Open(file)
	if err != nil {
		panic(fmt.Errorf("cannot open file %q: %w", file, err))
	}
	defer func() {
		_ = f.Close()
	}()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		panic(fmt.Errorf("cannot read file %q: %w", file, err))
	}

	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil {
		panic(fmt.Errorf("cannot unmarshal yaml data: %w", err))
	}

	assertKind(&root, yaml.DocumentNode)
	assertKind(root.Content[0], yaml.MappingNode)

	return root.Content[0]
}

func fileContent(file string) []*yaml.Node {
	return fileNode(file).Content
}

// assertNotRefRecursively asserts there is no reference in a schema recursively.
func assertNotRefRecursively(schema *Schema) {
	if schema == nil {
		return
	}
	if schema.Ref != "" {
		panic(fmt.Errorf("unsupported schema reference: %q", schema.Ref))
	}
	assertNotRefRecursively(schema.Items)
	if schema.Properties == nil {
		return
	}
	for _, p := range schema.Properties {
		assertNotRefRecursively(&p)
	}
}

func assertRefNotEmpty(key string, ref *reference) {
	if ref == nil || ref.Ref == "" {
		panic(fmt.Errorf("missing expected reference: %q", key))
	}
}
