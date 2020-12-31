package openapiparser

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

type parser struct {
	specFile string
	paths    map[string]Path

	schemasParsed   map[string]*Schema
	responsesParsed map[string]*Response
	pathsParsed     map[string]*Path
	parsedFilesMx   sync.RWMutex

	schemas          map[string]*Schema
	schemaNames      map[string]string
	schemaNamesReady bool
	schemaPromises   map[string][]chan schemaResult
	schemasMx        sync.RWMutex

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
		schemaPromises: make(map[string][]chan schemaResult),
	}
}

func (p *parser) parse() (spec *Spec, err error) {
	rootContent, err := fileContent(p.specFile)
	if err != nil {
		return nil, err
	}

	spec = new(Spec)
	err = capture(rootContent, map[string]captureFunc{
		"openapi":    captureString(&spec.OpenAPI),
		"info":       captureNodeDecode(&spec.Info),
		"servers":    captureNodeDecode(&spec.Servers),
		"paths":      p.capturePaths(spec),
		"components": p.captureComponents(spec),
		//"tags": TODO
		//"security": TODO
		//"externalDocs": TODO
	})

	return spec, err
}

func (p *parser) addSchema(key string, x Schema, err error) {
	p.schemasMx.Lock()
	defer p.schemasMx.Unlock()

	if err == nil {
		p.schemas[key] = &x
	}
	if promises, ok := p.schemaPromises[key]; ok {
		for _, c := range promises {
			c <- schemaResult{schema: x, err: err}
			close(c)
		}
		delete(p.schemaPromises, key)
	}
}

func (p *parser) schemaByFile(currentFile, ref string) (Schema, error) {
	for {
		p.schemasMx.RLock()
		ready := p.schemaNamesReady
		p.schemasMx.RUnlock()
		if ready {
			break
		}
		<-time.NewTimer(time.Millisecond * 25).C
	}

	p.schemasMx.RLock()
	key, ok := p.schemaNames[joinPath(currentFile, ref)]
	p.schemasMx.RUnlock()
	if ok {
		x := <-p.promiseSchemaByKey(key)
		return x.schema, x.err
	}

	// is an anonymous schema, go solve it
	return p.parseSchemaRef(currentFile, ref)
}

type schemaResult struct {
	schema Schema
	err    error
}

func (p *parser) promiseSchemaByKey(key string) <-chan schemaResult {
	p.schemasMx.Lock()
	defer p.schemasMx.Unlock()

	c := make(chan schemaResult, 1)
	x, ok := p.schemas[key]
	if ok {
		c <- schemaResult{schema: *x, err: nil}
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
	return func(node *yaml.Node) error {
		pairs, err := mapPairs(node.Content)
		if err != nil {
			return err
		}
		for _, v := range pairs {
			key := v.key
			if err := assertKind(v.node, yaml.MappingNode); err != nil {
				return fmt.Errorf("path %q is not a reference: %v", key, err)
			}
			var ref reference
			if err := v.node.Decode(&ref); err != nil {
				return err
			}
			if err := assertRefNotEmpty(&ref); err != nil {
				return fmt.Errorf("path %q is not a reference: %v", key, err)
			}
			pp, err := p.parsePathRef(p.specFile, ref.Ref)
			if err != nil {
				return fmt.Errorf("cannot parse path %q: %v", key, err)
			}
			pp.Key = key
			spec.Paths = append(spec.Paths, pp)
		}
		return nil
	}
}

func (p *parser) captureComponents(spec *Spec) captureFunc {
	return func(node *yaml.Node) error {
		return capture(node.Content, map[string]captureFunc{
			"schemas":    p.captureSchemas(spec),
			"parameters": p.captureParameters(spec),
			//"responses"
		})
	}
}

func (p *parser) captureParameters(spec *Spec) captureFunc {
	return func(node *yaml.Node) error {
		pairs, err := mapPairs(node.Content)
		if err != nil {
			return err
		}
		for _, v := range pairs {
			if !isMap(node) {
				return fmt.Errorf("parameter %q defintion is not a map", v.key)
			}
			var param Parameter
			if err := v.node.Decode(&param); err != nil {
				return fmt.Errorf("cannot decode parameter %q: %v", v.key, err)
			}
			if containsReferences(&param.Schema) {
				return fmt.Errorf("parameter %q contains a referecnes", v.key)
			}
			p.addParameter(v.key, param)
			spec.Parameters = append(spec.Parameters, param)
		}
		return nil
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
	return func(node *yaml.Node) error {
		pairs, err := mapPairs(node.Content)
		if err != nil {
			return err
		}
		defs := make([]schemaDef, len(pairs))
		spec.Schemas = make([]Schema, len(pairs))

		// first pass to get key and file.
		p.schemasMx.Lock()
		p.schemaNames = make(map[string]string)
		for i, v := range pairs {
			key := v.key
			var ref reference
			if !isMap(v.node) {
				p.schemasMx.Unlock()
				return fmt.Errorf("root schema defintion %q is not a map", key)
			}
			if err := v.node.Decode(&ref); err != nil {
				p.schemasMx.Unlock()
				return fmt.Errorf("cannot decode schema defintion %q as reference: %v", key, err)
			}
			if ref.Ref == "" {
				p.schemasMx.Unlock()
				return fmt.Errorf("schema %q reference is empty", key)
			}
			file := joinPath(p.specFile, ref.Ref)
			p.schemaNames[file] = v.key
			defs[i] = schemaDef{
				ref:  ref.Ref,
				file: file,
			}
		}
		p.schemaNamesReady = true
		p.schemasMx.Unlock()

		// parse concurrently.
		var g errgroup.Group // TODO context
		for i, v := range pairs {
			i := i
			v := v
			g.Go(func() error {
				file := defs[i].file
				root, err := fileNode(file)
				if err != nil {
					p.addSchema(v.key, Schema{}, err)
					return err
				}
				x, err := p.parseSchemaContent(file, root)
				if err != nil {
					p.addSchema(v.key, Schema{}, err)
					return err
				}
				x.Ref = defs[i].ref
				x.Name = v.key

				p.addSchema(v.key, x, nil)
				spec.Schemas[i] = x
				return nil
			})
		}
		return g.Wait()
	}
}

func (p *parser) captureResponseRef(currentFile string, res *Response, isRef *bool) captureFunc {
	return func(node *yaml.Node) error {
		if !isScalar(node) {
			return fmt.Errorf("response reference is not a string")
		}
		ref := node.Value
		*isRef = true

		// Check if we have parsed it already
		responseFile := joinPath(currentFile, ref)
		p.parsedFilesMx.RLock()
		cache, ok := p.responsesParsed[responseFile]
		p.parsedFilesMx.RUnlock()
		if ok {
			*res = *cache
			return nil
		}

		root, err := fileNode(responseFile)
		if err != nil {
			return err
		}
		err = p.captureResponse(responseFile, res)(root)
		if err != nil {
			return err
		}

		res.Ref = ref
		p.parsedFilesMx.Lock()
		p.responsesParsed[responseFile] = res
		p.parsedFilesMx.Unlock()
		return nil
	}
}

func (p *parser) captureMediaTypes(currentFile string, mediaTypes *ContentTypes) captureFunc {
	return func(node *yaml.Node) error {
		if !isMap(node) {
			return fmt.Errorf("media type node is not a map at %q:%d", currentFile, node.Line)
		}
		*mediaTypes = make(ContentTypes)
		mt := *mediaTypes
		pairs, err := mapPairs(node.Content)
		if err != nil {
			return err
		}
		for _, v := range pairs {
			if !isMap(v.node) {
				return fmt.Errorf("content type node is not a map at %q:%d", currentFile, v.node.Line)
			}
			var mediaType MediaType
			err := capture(v.node.Content, captureMap{
				"schema":   p.captureSchema(currentFile, &mediaType.Schema),
				"example":  captureRawFunc(&mediaType.Example),
				"examples": captureRawFunc(&mediaType.Examples),
			})
			if err != nil {
				return fmt.Errorf("cannot parse media type at %q:%d: %v", currentFile, v.node.Line, err)
			}
			mt[v.key] = mediaType
		}
		return nil
	}
}

func (p *parser) parseSchemaRef(currentFile string, ref string) (schema Schema, err error) {
	// Check if we have parsed it already
	schemaFile := joinPath(currentFile, ref)
	p.parsedFilesMx.RLock()
	cache, ok := p.schemasParsed[schemaFile]
	p.parsedFilesMx.RUnlock()
	if ok {
		return *cache, nil
	}

	root, err := fileNode(schemaFile)
	if err != nil {
		return schema, err
	}
	schema, err = p.parseSchemaContent(schemaFile, root)
	if err != nil {
		return schema, fmt.Errorf("cannot parse schema reference %s at %q: %v", ref, currentFile, err)
	}
	schema.Ref = ref

	p.parsedFilesMx.Lock()
	p.schemasParsed[schemaFile] = &schema
	p.parsedFilesMx.Unlock()

	return schema, nil
}

func (p *parser) parsePathRef(currentFile string, ref string) (path Path, err error) {
	pathFile := joinPath(currentFile, ref)
	p.parsedFilesMx.RLock()
	cache, ok := p.pathsParsed[pathFile]
	p.parsedFilesMx.RUnlock()
	if ok {
		return *cache, nil
	}

	root, err := fileContent(pathFile)
	if err != nil {
		return path, err
	}
	path, err = p.parsePathFileContent(pathFile, root)
	if err != nil {
		return path, err
	}
	path.Ref = ref

	p.parsedFilesMx.Lock()
	p.pathsParsed[pathFile] = &path
	p.parsedFilesMx.Unlock()

	return path, nil
}

func (p *parser) captureSchemaRef(currentFile string, schema *Schema, isRef *bool) captureFunc {
	return func(node *yaml.Node) error {
		*isRef = true
		if !isScalar(node) {
			return fmt.Errorf("schema reference is not a string at %s:%d", currentFile, node.Line)
		}
		ref := node.Value
		idx := strings.Index(ref, "#")
		if idx == -1 {
			// this may be a direct reference to a component schema, we need to wait
			// until all of them are solved.
			s, err := p.schemaByFile(currentFile, ref)
			if err != nil {
				return err
			}
			*schema = s
			return nil
		}

		file := currentFile
		if idx > 0 {
			// the ref contains both a file path and the hashed reference, separate them
			file = joinPath(currentFile, ref[:idx])
			ref = ref[idx:]
		}
		if file != p.specFile || strings.Index(ref, "#/components/schemas/") != 0 {
			// hashed references are only allowed if they point to the root file.
			return fmt.Errorf("only local references to root spec are allowed, got: %q at %s:%d", ref, currentFile, node.Line)
		}

		// we may have a reference to a component
		// that has not been parsed yet...
		k := ref[21:]
		x := <-p.promiseSchemaByKey(k)
		if x.err != nil {
			return fmt.Errorf("error processing schema dependency %q", k)
		}
		*schema = x.schema
		return nil
	}
}

func (p *parser) captureProperties(currentFile string, props *[]Schema) captureFunc {
	return func(node *yaml.Node) error {
		pairs, err := mapPairs(node.Content)
		if err != nil {
			return err
		}
		for _, v := range pairs {
			schema, err := p.parseSchemaContent(currentFile, v.node)
			if err != nil {
				return fmt.Errorf("cannot parse properties at %s:%d: %v", currentFile, v.node.Line, err)
			}
			schema.Key = v.key
			*props = append(*props, schema)
		}
		return nil
	}
}

func (p *parser) captureSchema(currentFile string, x *Schema) captureFunc {
	return func(node *yaml.Node) error {
		s, err := p.parseSchemaContent(currentFile, node)
		if err != nil {
			return err
		}
		*x = s
		return nil
	}
}

func (p *parser) capturePointerSchema(currentFile string, x **Schema) captureFunc {
	return func(node *yaml.Node) error {
		if x == nil {
			x = new(*Schema)
		}
		if *x == nil {
			*x = new(Schema)
		}
		s, err := p.parseSchemaContent(currentFile, node)
		if err != nil {
			return err
		}
		**x = s
		return nil
	}
}

// parseSchemaContent must contain the
func (p *parser) parseSchemaContent(currentFile string, node *yaml.Node) (schema Schema, err error) {
	var (
		refSchema Schema
		isRef     bool
	)
	err = capture(node.Content, captureMap{
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
	if err != nil {
		return schema, fmt.Errorf("cannot parse schema at %q: %v", currentFile, err)
	}
	if isRef {
		return refSchema, nil
	}
	return schema, nil
}

func (p *parser) parsePathFileContent(currentFile string, content []*yaml.Node) (path Path, err error) {
	pairs, err := mapPairs(content)
	if err != nil {
		return path, err
	}
	for _, v := range pairs {
		if !isMap(v.node) {
			return path, fmt.Errorf("endpoint definition is not a map at %q", currentFile)
		}
		var endpoint Endpoint
		err := capture(v.node.Content, captureMap{
			"responses":   p.captureResponses(currentFile, &endpoint.Responses),
			"tags":        captureStringSlice(&endpoint.Tags),
			"summary":     captureString(&endpoint.Summary),
			"description": captureString(&endpoint.Description),
			"operationId": captureString(&endpoint.OperationID),
			"requestBody": p.captureRequestBody(currentFile, &endpoint.RequestBody),
			"parameters":  p.captureEndpointParameters(currentFile, &endpoint.Parameters),
		})
		if err != nil {
			return path, fmt.Errorf("cannot parse endpoint %q at %q: %v", v.key, currentFile, err)
		}
		endpoint.Method = v.key
		path.Endpoints = append(path.Endpoints, endpoint)
	}
	return path, nil
}

func (p *parser) captureResponse(currentFile string, res *Response) captureFunc {
	return func(node *yaml.Node) error {
		var (
			refRes Response
			isRef  bool
		)
		if !isMap(node) {
			return fmt.Errorf("response definition is not a map at %s:%d", currentFile, node.Line)
		}
		err := capture(node.Content, captureMap{
			"$ref":        p.captureResponseRef(currentFile, &refRes, &isRef),
			"description": captureString(&res.Description),
			"content":     p.captureMediaTypes(currentFile, &res.ContentTypes),
		})
		if err != nil {
			return fmt.Errorf("cannot parse repsonse at %s: %v", currentFile, err)
		}
		if isRef {
			*res = refRes
		}
		return nil
	}
}

func (p *parser) captureResponses(currentFile string, responses *[]Response) captureFunc {
	return func(node *yaml.Node) error {
		pairs, err := mapPairs(node.Content)
		if err != nil {
			return err
		}
		for _, v := range pairs {
			var res Response
			if err := p.captureResponse(currentFile, &res)(v.node); err != nil {
				return err
			}
			res.Status = v.key
			*responses = append(*responses, res)
		}
		return nil
	}
}

func (p *parser) captureRequestBody(currentFile string, body *RequestBody) captureFunc {
	return func(node *yaml.Node) error {
		err := capture(node.Content, captureMap{
			// "$ref":     unsupported, will panic
			"content":     p.captureMediaTypes(currentFile, &body.ContentTypes),
			"required":    captureBool(&body.Required),
			"description": captureString(&body.Description),
			"example":     captureRawFunc(&body.Example),
		})
		if err != nil {
			return fmt.Errorf("cannot parse request body at %s:%d: %v", currentFile, node.Line, err)
		}
		return nil
	}
}

func (p *parser) captureEndpointParameters(currentFile string, parameters *[]Parameter) captureFunc {
	return func(node *yaml.Node) error {
		if !isSequence(node) {
			return fmt.Errorf("endpoint parameters should be an array at %s:%d", currentFile, node.Line)
		}
		for _, n := range node.Content {
			if !isMap(n) {
				return fmt.Errorf("endpoint parameter should be a map at %s:%d", currentFile, n.Line)
			}
			var param Parameter
			if err := n.Decode(&param); err != nil {
				return fmt.Errorf("cannot decode path parameter at %s:%d: %v", currentFile, n.Line, err)
			}
			if containsReferences(&param.Schema) {
				return fmt.Errorf("references are not allowed in parameters at %s:%d", currentFile, n.Line)
			}
			if param.Ref != "" {
				ref := param.Ref
				dir := filepath.Dir(currentFile)
				idx := strings.Index(ref, "#")
				if idx == -1 {
					// easy case, there's no hash part, we can go read the file directly.
					return fmt.Errorf("parameters references can only be defined in the root spec file: %q", ref)
				}
				file := currentFile
				if idx > 0 {
					// the ref contains both a file path and the hashed reference, separate them
					file = filepath.Join(dir, ref[:idx])
					ref = ref[idx:]
				}
				if file != p.specFile {
					// hashed references are only allowed if they point to the root file.
					return fmt.Errorf("only local references to the root spec file are valid, found %q at %s:%d", file, currentFile, n.Line)
				}
				idx = strings.Index(ref, "#/components/parameters/")
				if idx != 0 {
					return fmt.Errorf("only local references to root component schemas are supported, found %q at %s:%d", ref, currentFile, n.Line)
				}
				paramKey := ref[24:]

				// We may not have it yet, but we will (or die trying with a deadlock...)
				*parameters = append(*parameters, <-p.promiseParameter(paramKey))
				continue
			}
			*parameters = append(*parameters, param)
		}
		return nil
	}
}

func assertKind(node *yaml.Node, want yaml.Kind) error {
	if got := node.Kind; got != want {
		// TODO map kinds
		return fmt.Errorf("expected node kind %q, got %q", want, got)
	}
	return nil
}

func isMap(node *yaml.Node) bool {
	return node.Kind == yaml.MappingNode
}

func isSequence(node *yaml.Node) bool {
	return node.Kind == yaml.SequenceNode
}

func isScalar(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode
}

func assertScalar(node *yaml.Node) error {
	if got := node.Kind; got != yaml.ScalarNode {
		return fmt.Errorf("expected scalar node, got %q", got)
	}
	return nil
}

func fileNode(file string) (*yaml.Node, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %q: %w", file, err)
	}
	defer func() {
		_ = f.Close()
	}()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("cannot read file %q: %w", file, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("cannot unmarshal yaml data: %w", err)
	}
	if err := assertKind(&root, yaml.DocumentNode); err != nil {
		return nil, fmt.Errorf("file %q is not a document node: %v", file, err)
	}
	if err := assertKind(root.Content[0], yaml.MappingNode); err != nil {
		return nil, fmt.Errorf("file %q content is not a mapping node: %v", file, err)
	}

	return root.Content[0], nil
}

func fileContent(file string) ([]*yaml.Node, error) {
	root, err := fileNode(file)
	if err != nil {
		return nil, err
	}
	return root.Content, nil
}

// containsReferences checks if there is any reference in a schema recursively.
func containsReferences(schema *Schema) bool {
	if schema == nil {
		return false
	}
	if schema.Ref != "" {
		return true
	}
	if containsReferences(schema.Items) {
		return true
	}
	if schema.Properties == nil {
		return false
	}
	for _, p := range schema.Properties {
		if containsReferences(&p) {
			return true
		}
	}
	return false
}

func assertRefNotEmpty(ref *reference) error {
	if ref.Ref == "" {
		return fmt.Errorf("expected reference")
	}
	return nil
}

func joinPath(currentFile, ref string) string {
	return path.Join(filepath.Dir(currentFile), ref)
}
