package openapiparser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Spec struct {
	OpenAPI    string      `yaml:"openapi,omitempty"`
	Info       Info        `yaml:"info,omitempty"`
	Servers    []Server    `yaml:"servers,omitempty"`
	Paths      []Path      `yaml:"paths,omitempty"`
	Schemas    []Schema    `yaml:"schemas,omitempty"`
	Parameters []Parameter `yaml:"parameters,omitempty"`
}

type Path struct {
	Key       string     `yaml:"key,omitempty"`
	Ref       string     `yaml:"$ref,omitempty"`
	Endpoints []Endpoint `yaml:"endpoints"`
}

type Endpoint struct {
	Method      string      `yaml:"method,omitempty"`
	Summary     string      `yaml:"summary,omitempty"`
	Description string      `yaml:"description,omitempty"`
	Tags        []string    `yaml:"tags,omitempty"`
	OperationID string      `yaml:"operationId,omitempty"`
	Parameters  []Parameter `yaml:"parameters,omitempty"`
	RequestBody RequestBody `yaml:"requestBody,omitempty"`
	Responses   []Response  `yaml:"responses,omitempty"`
}

type Parameter struct {
	Ref         string `yaml:"$ref,omitempty"`
	In          string `yaml:"in,omitempty"`
	Name        string `yaml:"name,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
	Description string `yaml:"description,omitempty"`
	Schema      Schema `yaml:"schema,omitempty"`
}

type Contact struct {
	Name  string `yaml:"name,omitempty"`
	URL   string `yaml:"url,omitempty"`
	Email string `yaml:"email,omitempty"`
}

type Info struct {
	Title       string  `yaml:"title,omitempty"`
	Description string  `yaml:"description,omitempty"`
	Contact     Contact `yaml:"contact"`
	APIVersion  string  `yaml:"version,omitempty"`
}

type Server struct {
	URL         string `yaml:"url,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type EnumOptionX struct {
	Key         string `yaml:"title,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type EnumX struct {
	Options []EnumOptionX
}

type Schema struct {
	// Key is dependent on where the schema was found. It may represent the name
	// of the Schema if it's the root spec components map, but it's
	// the key of the property is it was found as a nested schema.
	Key string `yaml:"key,omitempty"`
	// Name is the name of the schema if it's present in the
	// components schemas section
	Name string `yaml:"name,omitempty"`

	Ref              string   `yaml:"$ref,omitempty"`
	Type             string   `yaml:"type,omitempty"`
	Description      string   `yaml:"description,omitempty"`
	Title            string   `yaml:"title,omitempty"`
	Required         []string `yaml:"required,omitempty"`
	Nullable         bool     `yaml:"Nullable,omitempty"`
	Format           string   `yaml:"format,omitempty"`
	Pattern          string   `yaml:"pattern,omitempty"`
	ReadOnly         bool     `yaml:"readOnly,omitempty"`
	WriteOnly        bool     `yaml:"writeOnly,omitempty"`
	Minimum          float64  `yaml:"minimum,omitempty"`
	Maximum          float64  `yaml:"maximum,omitempty"`
	ExclusiveMinimum float64  `yaml:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum float64  `yaml:"exclusiveMaximum,omitempty"`
	MinItems         int      `yaml:"minItems,omitempty"`
	MaxItems         int      `yaml:"maxItems,omitempty"`
	UniqueItems      bool     `yaml:"uniqueItems,omitempty"`
	MinLength        int      `yaml:"minLength,omitempty"`
	MaxLength        int      `yaml:"maxLength,omitempty"`
	MinProperties    int      `yaml:"minProperties,omitempty"`
	MaxProperties    int      `yaml:"maxProperties,omitempty"`
	Enum             []string `yaml:"enum,omitempty"`
	EnumX            EnumX    `yaml:"x-enum,omitempty"`
	Example          string   `yaml:"example,omitempty"`
	Default          string   `yaml:"default,omitempty"`
	Properties       []Schema `yaml:"properties,omitempty"`
	Items            *Schema  `yaml:"items,omitempty"`
}

func (s Schema) isRef() bool {
	return s.Ref != ""
}

type SchemaOp int

const (
	ReadOp SchemaOp = iota << 1
	WriteOp
)

func (s Schema) JSON(op SchemaOp) (string, error) {
	var b strings.Builder
	err := jsonSchema(op, &s, &b, false)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func (s Schema) JSONIndent(op SchemaOp, indent string) (string, error) {
	var buf bytes.Buffer
	src, err := s.JSON(op)
	if err != nil {
		return "", err
	}
	if err := json.Indent(&buf, []byte(src), "", indent); err != nil {
		fmt.Println(src)
		return "", err
	}
	return buf.String(), nil
}

func jsonSchema(op SchemaOp, s *Schema, b *strings.Builder, withKey bool) error {
	clean := func(s string) string {
		return strings.Trim(s, "\n ")
	}
	key := clean(s.Key)
	ex := clean(s.Example)
	if withKey && key != "" {
		b.WriteByte('"')
		b.WriteString(key)
		b.WriteString(`":`)
	}
	switch s.Type {
	case "object":
		b.WriteByte('{')
		first := true
		for _, p := range s.Properties {
			if p.ReadOnly && op == WriteOp {
				continue
			}
			if p.WriteOnly && op == ReadOp {
				continue
			}
			if !first {
				b.WriteByte(',')
			}
			first = false
			if err := jsonSchema(op, &p, b, true); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case "array":
		b.WriteByte('[')
		if err := jsonSchema(op, s.Items, b, false); err != nil {
			return err
		}
		b.WriteByte(']')
	case "string":
		if ex != "" {
			b.WriteString(strconv.Quote(ex))
			return nil
		}
		if len(s.Enum) > 0 {
			b.WriteString(strconv.Quote(clean(s.Enum[0])))
			return nil
		}
		b.WriteByte('"')
		switch s.Format {
		case "uuid":
			b.WriteString("e017d029-a459-4cfc-bf35-dd774ddf50e7")
		case "date-time":
			b.WriteString("2015-12-13T10:05:48+01:00")
		case "date":
			b.WriteString("2015-12-13")
		case "time":
			b.WriteString("10:05:48+01:00")
		case "email":
			b.WriteString("email@example.com")
		default:
			b.WriteString("string")
		}
		b.WriteByte('"')
	case "integer", "number":
		if ex != "" {
			b.WriteString(ex)
			return nil
		}
		switch s.Format {
		case "double", "float":
			b.WriteString("100.00")
		default:
			b.WriteString("100")
		}
	case "boolean":
		if ex != "" {
			b.WriteString(ex)
			return nil
		}
		b.WriteString("true")
	default:
		panic("unexpected type building schema JSON " + s.Type)
	}
	return nil
}

type Response struct {
	Ref          string               `yaml:"$ref,omitempty"`
	Status       string               `yaml:"status,omitempty"`
	Description  string               `yaml:"description,omitempty"`
	ContentTypes map[string]MediaType `yaml:"content,omitempty"`
}

type RequestBody struct {
	Required    bool                 `yaml:"required,omitempty"`
	Description string               `yaml:"description,omitempty"`
	Content     map[string]MediaType `yaml:"content,omitempty"`
}

type MediaType struct {
	Schema   Schema `yaml:"schema,omitempty"`
	Example  string `yaml:"example,omitempty"`
	Examples string `yaml:"examples,omitempty"`
}

type reference struct {
	Ref string `yaml:"$ref,omitempty"`
}

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

func (p *parser) schemaByFile(file string) Schema {
	p.schemasMx.RLock()
	key, ok := p.schemaNames[file]
	p.schemasMx.RUnlock()
	if ok {
		return <-p.promiseSchemaByKey(key)
	}

	// is an anonymous schema, go solve it
	return p.parseSchemaContent(file, fileContent(file))
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

func Parse(file string) (spec *Spec, err error) {
	defer func() {
		if r := recover(); r != nil {
			triggeredAt := identifyPanic()
			if rErr, ok := r.(error); ok {
				err = fmt.Errorf("%w (%s)", rErr, triggeredAt)
				return
			}
			err = fmt.Errorf("unexpected error parsing file: %v (%s)", r, triggeredAt)
		}
	}()

	return newParser(file).parse(), nil
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

type captureFunc func(node *yaml.Node)

type captureMap map[string]captureFunc

func (p *parser) parse() (spec *Spec) {
	spec = new(Spec)
	rootContent := fileContent(p.specFile)
	capture(rootContent, map[string]captureFunc{
		"openapi": captureString(&spec.OpenAPI),
		"info":    captureNodeDecode(&spec.Info),
		"servers": captureNodeDecode(&spec.Servers),
		"paths":   p.capturePaths(spec),
	}, map[string]captureFunc{
		"components": p.captureComponents(spec),
	})

	return spec
}

func captureString(s *string) captureFunc {
	return func(node *yaml.Node) {
		*s = assertString(node)
	}
}
func captureNodeDecode(v interface{}) captureFunc {
	return func(node *yaml.Node) {
		panicIfErr(node.Decode(v))
	}
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

func (p *parser) capturePaths(spec *Spec) captureFunc {
	return func(node *yaml.Node) {
		for _, v := range mapPairs(node.Content) {
			key := v.key
			next := assertKind(v.node, yaml.MappingNode)
			var ref reference
			assertNodeDecode(next, &ref)
			assertRef(key, &ref)

			pp := p.parsePathRef(p.specFile, ref.Ref)
			pp.Key = key
			spec.Paths = append(spec.Paths, pp)
		}
	}
}

func (p *parser) captureComponents(spec *Spec) captureFunc {
	return func(node *yaml.Node) {
		capture(node.Content, map[string]captureFunc{
			"schemas": p.captureSchemas(spec),
		}, map[string]captureFunc{
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
			assertNotRef(&param.Schema)

			p.addParameter(v.key, param)
			spec.Parameters = append(spec.Parameters, param)
		}
	}
}

func joinPath(currentFile, ref string) string {
	return path.Join(filepath.Dir(currentFile), ref)
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
		p.schemaNames = make(map[string]string)
		for i, v := range pairs {
			key := v.key
			var ref reference
			assertKind(v.node, yaml.MappingNode)
			assertNodeDecode(v.node, &ref)
			assertRef(key, &ref)
			file := joinPath(p.specFile, ref.Ref)
			p.schemaNames[file] = v.key
			defs[i] = schemaDef{
				ref:  ref.Ref,
				file: file,
			}
		}
		// parse concurrently.
		for i, v := range pairs {
			wg.Add(1)
			go func(i int, v mapPair) {
				defer wg.Done()
				content := fileContent(defs[i].file)
				x := p.parseSchemaContent(defs[i].file, content)
				x.Ref = defs[i].ref
				x.Name = v.key

				p.addSchema(v.key, x)
				spec.Schemas[i] = x
			}(i, v)
		}
		wg.Wait()
	}
}

// assertNotRef asserts there is no reference in a schema recursively.
func assertNotRef(schema *Schema) {
	if schema == nil {
		return
	}
	if schema.isRef() {
		panic(fmt.Errorf("unsupported schema reference: %q", schema.Ref))
	}
	assertNotRef(schema.Items)
	if schema.Properties == nil {
		return
	}
	for _, p := range schema.Properties {
		assertNotRef(&p)
	}
}

func assertRef(key string, ref *reference) {
	if ref == nil || ref.Ref == "" {
		panic(fmt.Errorf("missing expected reference: %q", key))
	}
}

func assertBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		panic(fmt.Errorf("expected boolean string representation but got %q", s))
	}
	return b
}

func assertFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(fmt.Errorf("expected number but cannot parse as float %q: %v", s, err))
	}
	return f
}

func assertInt(s string) int {
	i, err := strconv.ParseInt(s, 10, 0)
	if err != nil {
		panic(fmt.Errorf("expected number but cannot parse as float %q: %v", s, err))
	}
	return int(i)
}

func captureStringSlice(content []*yaml.Node) []string {
	var ss []string
	for _, n := range content {
		assertKind(n, yaml.ScalarNode)
		ss = append(ss, n.Value)
	}
	return ss
}

func captureRaw(key string, node *yaml.Node) string {
	raw, err := yaml.Marshal(node)
	if err != nil {
		panic(fmt.Errorf("cannot capture raw node %q: %v", key, err))
	}
	return string(raw)
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

	schema = p.parseSchemaContent(schemaFile, fileContent(schemaFile))
	schema.Ref = ref

	p.parsedFilesMx.Lock()
	p.schemasParsed[schemaFile] = &schema
	p.parsedFilesMx.Unlock()

	return schema
}

func (p *parser) parseResponseRef(currentFile string, ref string) (response Response) {
	// Check if we have parsed it already
	responseFile := joinPath(currentFile, ref)
	p.parsedFilesMx.RLock()
	cache, ok := p.responsesParsed[responseFile]
	p.parsedFilesMx.RUnlock()
	if ok {
		return *cache
	}

	response = p.parseResponseContent(responseFile, fileContent(responseFile))
	response.Ref = ref

	p.parsedFilesMx.Lock()
	p.responsesParsed[responseFile] = &response
	p.parsedFilesMx.Unlock()

	return response
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

func (p *parser) solveSchemaRef(currentFile string, ref string) Schema {
	dir := filepath.Dir(currentFile)
	idx := strings.Index(ref, "#")
	if idx == -1 {
		// this may be a direct reference to a component schema, we need to wait
		// until all of them are solved.
		return p.schemaByFile(filepath.Join(dir, ref))
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
	idx = strings.Index(ref, "#/components/schemas/")
	if idx != 0 {
		panic(fmt.Errorf("only local references to root component schemas are supported, got: %q", ref))
	}

	// we may have a reference to a component
	// that has not been parsed yet...
	return <-p.promiseSchemaByKey(ref[21:])
}

// parseSchemaContent must contain the
func (p *parser) parseSchemaContent(currentFile string, content []*yaml.Node) (schema Schema) {
	for _, v := range mapPairs(content) {
		switch k := v.key; k {
		case "$ref":
			// TODO trims space on values
			ref := strings.TrimSpace(assertKind(v.node, yaml.ScalarNode).Value)
			return p.solveSchemaRef(currentFile, ref)
		case "type":
			schema.Type = assertKind(v.node, yaml.ScalarNode).Value
		case "description":
			schema.Description = assertKind(v.node, yaml.ScalarNode).Value
		case "title":
			schema.Title = assertKind(v.node, yaml.ScalarNode).Value
		case "required":
			schema.Required = captureStringSlice(assertKind(v.node, yaml.SequenceNode).Content)
		case "nullable":
			schema.Nullable = assertBool(assertKind(v.node, yaml.ScalarNode).Value)
		case "format":
			schema.Format = assertKind(v.node, yaml.ScalarNode).Value
		case "pattern":
			schema.Pattern = assertKind(v.node, yaml.ScalarNode).Value
		case "readOnly":
			schema.ReadOnly = assertBool(assertKind(v.node, yaml.ScalarNode).Value)
		case "writeOnly":
			schema.WriteOnly = assertBool(assertKind(v.node, yaml.ScalarNode).Value)
		case "minimum":
			schema.Minimum = assertFloat(assertKind(v.node, yaml.ScalarNode).Value)
		case "exclusiveMinimum":
			schema.ExclusiveMinimum = assertFloat(assertKind(v.node, yaml.ScalarNode).Value)
		case "maximum":
			schema.Maximum = assertFloat(assertKind(v.node, yaml.ScalarNode).Value)
		case "exclusiveMaximum":
			schema.ExclusiveMaximum = assertFloat(assertKind(v.node, yaml.ScalarNode).Value)
		case "minItems":
			schema.MinItems = assertInt(assertKind(v.node, yaml.ScalarNode).Value)
		case "maxItems":
			schema.MaxItems = assertInt(assertKind(v.node, yaml.ScalarNode).Value)
		case "uniqueItems":
			schema.UniqueItems = assertBool(assertKind(v.node, yaml.ScalarNode).Value)
		case "minLength":
			schema.MinLength = assertInt(assertKind(v.node, yaml.ScalarNode).Value)
		case "maxLength":
			schema.MaxLength = assertInt(assertKind(v.node, yaml.ScalarNode).Value)
		case "minProperties":
			schema.MinProperties = assertInt(assertKind(v.node, yaml.ScalarNode).Value)
		case "maxProperties":
			schema.MaxProperties = assertInt(assertKind(v.node, yaml.ScalarNode).Value)
		case "enum":
			schema.Enum = captureStringSlice(assertKind(v.node, yaml.SequenceNode).Content)
		case "x-enum":
			schema.EnumX = p.parseEnumX(assertKind(v.node, yaml.MappingNode).Content)
		case "default":
			schema.Default = captureRaw("default", v.node)
		case "example":
			schema.Example = captureRaw("example", v.node)
		case "properties":
			schema.Properties = p.parseProperties(currentFile, assertKind(v.node, yaml.MappingNode).Content)
		case "items":
			x := p.parseSchemaContent(currentFile, assertKind(v.node, yaml.MappingNode).Content)
			schema.Items = &x
		default:
			panic(fmt.Errorf("unsupported shcema definfintion key %q at %s", k, currentFile))
		}
	}
	return schema
}

func (p *parser) parseEnumX(content []*yaml.Node) (x EnumX) {
	for _, v := range mapPairs(content) {
		assertKind(v.node, yaml.ScalarNode)
		x.Options = append(x.Options, EnumOptionX{
			Key:         v.key,
			Description: v.node.Value,
		})
	}
	return x
}

func (p *parser) parsePathFileContent(currentFile string, content []*yaml.Node) (path Path) {
	for _, v := range mapPairs(content) {
		assertKind(v.node, yaml.MappingNode)
		e := p.parsePathEndpoint(currentFile, v.node.Content)
		e.Method = v.key
		path.Endpoints = append(path.Endpoints, e)
	}
	return path
}

func (p *parser) parseEndpointParameters(currentFile string, content []*yaml.Node) (params []Parameter) {
	for _, n := range content {
		assertKind(n, yaml.MappingNode)
		var param Parameter
		if err := n.Decode(&param); err != nil {
			panic(fmt.Errorf("cannot decode path parameter: %v", err))
		}
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
			params = append(params, <-p.promiseParameter(paramKey))
			continue
		}
		if param.Schema.Type == "" {
			panic(fmt.Errorf("no schema defined for path parameter at: %q", currentFile))
		}
		assertNotRef(&param.Schema)
		params = append(params, param)
	}

	return params
}

func (p *parser) parsePathEndpoint(currentFile string, content []*yaml.Node) (endpoint Endpoint) {
	for _, v := range mapPairs(content) {
		switch k := v.key; k {
		case "summary":
			endpoint.Summary = assertKind(v.node, yaml.ScalarNode).Value
		case "description":
			endpoint.Description = assertKind(v.node, yaml.ScalarNode).Value
		case "tags":
			endpoint.Tags = captureStringSlice(assertKind(v.node, yaml.SequenceNode).Content)
		case "operationId":
			endpoint.OperationID = assertKind(v.node, yaml.ScalarNode).Value
		case "parameters":
			assertKind(v.node, yaml.SequenceNode)
			endpoint.Parameters = p.parseEndpointParameters(currentFile, v.node.Content)
		case "responses":
			assertKind(v.node, yaml.MappingNode)
			endpoint.Responses = p.parsePathResponses(currentFile, v.node.Content)
		case "requestBody":
			assertKind(v.node, yaml.MappingNode)
			endpoint.RequestBody = p.parseRequestBody(currentFile, v.node.Content)
		default:
			panic(fmt.Errorf("unsupported endpoint key %q at %s", k, currentFile))
		}
	}

	return endpoint
}

func (p *parser) parsePathResponses(currentFile string, content []*yaml.Node) (responses []Response) {
	for _, v := range mapPairs(content) {
		status := v.key
		assertKind(v.node, yaml.MappingNode) // TODO create assertMap (helpers in mapPair?)
		var (
			ref reference
			res Response
		)
		if err := v.node.Decode(&ref); err != nil {
			panic(fmt.Errorf("cannot decode path response %w at %s (%s)", err, currentFile, status))
		}
		if ref.Ref != "" {
			res = p.parseResponseRef(currentFile, ref.Ref)
			res.Ref = ref.Ref
		} else {
			res = p.parseResponseContent(currentFile, v.node.Content)
		}
		res.Status = status
		responses = append(responses, res)
	}

	return responses
}

func (p *parser) parseRequestBody(currentFile string, content []*yaml.Node) (body RequestBody) {
	for _, v := range mapPairs(content) {
		switch k := v.key; k {
		case "$ref":
			panic(fmt.Errorf("request body full object reference is not supported at %q", currentFile))
		case "content":
			assertKind(v.node, yaml.MappingNode)
			body.Content = p.parseMediaTypes(currentFile, v.node.Content)
		case "required":
			body.Required = assertBool(assertKind(v.node, yaml.ScalarNode).Value) // TODO make assertX include type check
		case "description":
			body.Description = assertKind(v.node, yaml.ScalarNode).Value
		case "example":
			body.Description = assertKind(v.node, yaml.ScalarNode).Value
		default:
			panic(fmt.Errorf("unsupported endpoint key %q at %s", k, currentFile))
		}
	}
	return body
}

func (p *parser) parseMediaTypes(currentFile string, content []*yaml.Node) (mediaTypes map[string]MediaType) {
	mediaTypes = make(map[string]MediaType)
	for _, v := range mapPairs(content) {
		contentType := v.key
		assertKind(v.node, yaml.MappingNode)
		mediaType := p.parseMediaType(currentFile, v.node.Content)
		mediaTypes[contentType] = mediaType
	}

	return mediaTypes
}

func (p *parser) parseMediaType(currentFile string, content []*yaml.Node) (mediaType MediaType) {
	for _, v := range mapPairs(content) {
		switch k := v.key; k {
		case "schema":
			assertKind(v.node, yaml.MappingNode)
			mediaType.Schema = p.parseSchemaContent(currentFile, v.node.Content)
		case "example":
			mediaType.Example = captureRaw("example", v.node)
		case "examples":
			mediaType.Examples = captureRaw("examples", v.node)
		default:
			panic(fmt.Errorf("unexpected key in a media type map: %s at %s", k, currentFile))
		}
	}

	return mediaType
}

func (p *parser) parseResponseContent(currentFile string, content []*yaml.Node) (response Response) {
	for _, v := range mapPairs(content) {
		switch k := v.key; k {
		case "description":
			response.Description = assertKind(v.node, yaml.ScalarNode).Value
		case "headers":
			panic("headers in responses are not supported yet")
		case "content":
			assertKind(v.node, yaml.MappingNode)
			response.ContentTypes = p.parseMediaTypes(currentFile, v.node.Content)
		default:
			panic(fmt.Errorf("unsupported response definfintion key %q at %s", k, currentFile))
		}
	}
	return response
}

func (p *parser) parseResponseContentTypes(currentFile string, content []*yaml.Node) map[string]*MediaType {
	ct := make(map[string]*MediaType)
	for _, v := range mapPairs(content) {
		contentType := v.key // application/json
		assertKind(v.node, yaml.MappingNode)
		mt := p.parseMediaType(currentFile, v.node.Content)
		ct[contentType] = &mt
	}
	return ct
}

func (p *parser) parseProperties(currentFile string, content []*yaml.Node) (props []Schema) {
	for _, v := range mapPairs(content) {
		assertKind(v.node, yaml.MappingNode)
		schema := p.parseSchemaContent(currentFile, v.node.Content)
		schema.Key = v.key
		props = append(props, schema)
	}
	return props
}

// Capture runs capture functions concurrently on the given keys.
func capture(content []*yaml.Node, required captureMap, optional captureMap) {
	var wg sync.WaitGroup
	async := func(fx captureFunc, node *yaml.Node) {
		wg.Add(1)
		go func() {
			fx(node)
			wg.Done()
		}()
	}
	for _, v := range mapPairs(content) {
		if fx, ok := required[v.key]; ok {
			async(fx, v.node)
			delete(required, v.key)
			continue
		}
		if fx, ok := optional[v.key]; ok {
			async(fx, v.node)
			continue
		}
		panic(fmt.Errorf("unsupported key capturing content: %s", v.key))
	}
	wg.Wait()
	var missing []string
	for key := range required {
		missing = append(missing, key)
	}
	if len(missing) > 0 {
		panic(fmt.Errorf("missing required keys: %v", missing))
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

func assertString(node *yaml.Node) string {
	return assertScalar(node).Value
}

func fileContent(file string) []*yaml.Node {
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

	return root.Content[0].Content
}

func identifyPanic() string {
	var name, file string
	var line int
	var pc [16]uintptr

	n := runtime.Callers(3, pc[:])
	for _, pc := range pc[:n] {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		file, line = fn.FileLine(pc)
		name = fn.Name()
		if !strings.HasPrefix(name, "runtime.") {
			break
		}
	}

	switch {
	case name != "":
		return fmt.Sprintf("%v:%v", name, line)
	case file != "":
		return fmt.Sprintf("%v:%v", file, line)
	}

	return fmt.Sprintf("pc:%x", pc)
}

type mapPair struct {
	key  string
	node *yaml.Node
}

func mapPairs(nodes []*yaml.Node) []mapPair {
	length := len(nodes)
	if math.Mod(float64(length), 2) != 0 {
		panic(fmt.Errorf("asked for pairs on an off number of nodes: %d", length))
	}
	pairs := make([]mapPair, length/2)
	for i, j := 0, 1; j < length; i, j = i+2, j+2 {
		var (
			key   = nodes[i]
			value = nodes[j]
		)
		if key.Kind != yaml.ScalarNode {
			panic(fmt.Errorf("first element of a map pair should be an scalar, got: %v", key.Kind))
		}
		pairs[i/2] = mapPair{
			key:  key.Value,
			node: value,
		}
	}
	return pairs
}
