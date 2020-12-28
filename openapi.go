package openapiparser

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Spec struct {
	Meta       `yaml:"meta,inline"`
	Paths      *Paths      `yaml:"paths,omitempty"`
	Components *Components `yaml:"components,omitempty"`
}

type Meta struct {
	OpenAPI string   `yaml:"openapi,omitempty"`
	Info    Info     `yaml:"info,omitempty"`
	Servers []Server `yaml:"servers,omitempty"`
}

type Path struct {
	Key   string               `yaml:"key,omitempty"`
	File  string               `yaml:"file,omitempty"`
	Ref   string               `yaml:"$ref,omitempty"`
	Map   map[string]*Endpoint `yaml:"map"`
	Slice []*Endpoint          `yaml:"slice"`
}

type Endpoint struct {
	Method      string       `yaml:"method,omitempty"`
	Summary     string       `yaml:"summary,omitempty"`
	Description string       `yaml:"description,omitempty"`
	Tags        []string     `yaml:"tags,omitempty"`
	OperationID string       `yaml:"operationId,omitempty"`
	Parameters  []*Parameter `yaml:"parameters,omitempty"`
	RequestBody *RequestBody `yaml:"requestBody,omitempty"`
	Responses   *Responses   `yaml:"responses,omitempty"`
}

type Parameter struct {
	// Key mapping key
	Key string `yaml:"key,omitempty"`

	Ref         string  `yaml:"$ref,omitempty"`
	In          string  `yaml:"in,omitempty"`
	Name        string  `yaml:"name,omitempty"`
	Required    bool    `yaml:"required,omitempty"`
	Description string  `yaml:"description,omitempty"`
	Schema      *Schema `yaml:"schema,omitempty"`
}

type Contact struct {
	Name  string `yaml:"name,omitempty"`
	URL   string `yaml:"url,omitempty"`
	Email string `yaml:"email,omitempty"`
}

type Info struct {
	Title       string   `yaml:"title,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Contact     *Contact `yaml:"contact"`
	APIVersion  string   `yaml:"version,omitempty"`
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
	// Options has the raw options
	Options []string
	// Map holds all the options and the extended descriptions
	Map map[string]*EnumOptionX
	// Slice holds the options in the same order has defined in the spec file.
	Slice []*EnumOptionX
}

type Components struct {
	Schemas    *Schemas    `yaml:"schemas,omitempty"`
	Parameters *Parameters `yaml:"parameters,omitempty"`
}

type Schema struct {
	File             string   `yaml:"file,omitempty"`
	Ref              string   `yaml:"ref,omitempty"`
	Key              string   `yaml:"key,omitempty"`
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
	EnumX            *EnumX   `yaml:"x-enum,omitempty"`
	Example          string   `yaml:"example,omitempty"`
	Default          string   `yaml:"default,omitempty"`
	Properties       *Schemas `yaml:"properties,omitempty"`
	Items            *Schema  `yaml:"items,omitempty"`
}

func (s *Schema) isRef() bool {
	return s.Ref != ""
}

type Response struct {
	Ref          string                `yaml:"$ref,omitempty"`
	File         string                `yaml:"file,omitempty"`
	Key          string                `yaml:"key,omitempty"`
	Description  string                `yaml:"description,omitempty"`
	ContentTypes map[string]*MediaType `yaml:"content,omitempty"`
	Headers      map[string]*Header    `yaml:"headers,omitempty"`
}

type Header struct {
	Schema      *Schema `yaml:"schema,omitempty"`
	Description string  `yaml:"description,omitempty"`
}

type RequestBody struct {
	Required    bool                  `yaml:"required,omitempty"`
	Description string                `yaml:"description,omitempty"`
	Content     map[string]*MediaType `yaml:"content,omitempty"`
}

type MediaType struct {
	File     string  `yaml:"file,omitempty"`
	Schema   *Schema `yaml:"schema,omitempty"`
	Example  string  `yaml:"example,omitempty"`
	Examples string  `yaml:"examples,omitempty"`
}

type reference struct {
	Ref string `yaml:"$ref,omitempty"`
}

type parser struct {
	specFile   string
	parameters *Parameters
	schemas    *Schemas
	paths      *Paths

	schemaCallbacks map[string][]func(*Schema)
	schemasParsed   map[string]*Schema
	responsesParsed map[string]*Response
	pathsParsed     map[string]*Path
}

func (p *parser) promiseSchema(name string, schema *Schema) *Schema {
	p.schemaCallbacks[name] = append(p.schemaCallbacks[name], func(final *Schema) {
		*schema = *final
	})
	return schema
}

func (p *parser) resolveSchemaCallbacks() {
	for name, schema := range p.schemas.Map {
		for _, f := range p.schemaCallbacks[name] {
			f(schema)
		}
		delete(p.schemaCallbacks, name)
	}
	var missing []string
	for name := range p.schemaCallbacks {
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		panic(fmt.Errorf("missing component schemas: %s", strings.Join(missing, ",")))
	}
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
		specFile:        absolutePath(file),
		schemasParsed:   make(map[string]*Schema),
		schemaCallbacks: make(map[string][]func(schema *Schema)),
		pathsParsed:     make(map[string]*Path),
		responsesParsed: make(map[string]*Response),
	}
}

func (p *parser) parse() *Spec {
	var root yaml.Node
	unmarshalFile(p.specFile, &root)
	rootContent := rootNodeContent(&root)

	// we can directly decode the meta as it's
	// content is known in advance.
	var meta Meta
	assertNodeDecode(&root, &meta)

	content := captureContent(rootContent, map[string]yaml.Kind{
		"paths":      yaml.MappingNode,
		"components": yaml.MappingNode,
	})
	components := captureContent(content["components"], map[string]yaml.Kind{
		"schemas":    yaml.MappingNode,
		"parameters": yaml.MappingNode,
	})
	if x, ok := components["parameters"]; ok {
		p.parameters = p.parseParameters(x)
	}
	p.schemas = p.parseSchemas(components["schemas"])
	p.resolveSchemaCallbacks()

	p.paths = p.parsePaths(content["paths"])

	return &Spec{
		Meta:  meta,
		Paths: p.paths,
		Components: &Components{
			Schemas:    p.schemas,
			Parameters: p.parameters,
		},
	}
}

// ParametersMap hold the parameters in a map
// but also as an slice with the same order
// they were found during parsing.
type Parameters struct {
	Map   map[string]*Parameter `yaml:"map"`
	Slice []*Parameter          `yaml:"slice"`
}

func (p *parser) parseParameters(nodes []*yaml.Node) *Parameters {
	params := &Parameters{
		Map: make(map[string]*Parameter),
	}
	for i := 0; i < len(nodes); i++ {
		n := nodes[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		key := n.Value
		next := assertNextKind(nodes, &i, yaml.MappingNode)
		var param Parameter
		assertNodeDecode(next, &param)
		assertNotRef(param.Schema)
		param.Key = key

		params.Map[key] = &param
		params.Slice = append(params.Slice, &param)
	}
	return params
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
	for _, p := range schema.Properties.Map {
		assertNotRef(p)
	}
}

func assertRef(key string, ref *reference) {
	if ref == nil || ref.Ref == "" {
		panic(fmt.Errorf("missing expected reference: %q", key))
	}
}

// rootNodeContent a root node has a file unmarshalled
// on it, we have to unwrap the first layer to get
// to the content.
func rootNodeContent(root *yaml.Node) []*yaml.Node {
	assertKind(root, yaml.DocumentNode)
	c := assertContent(root, 1)
	assertKind(c[0], yaml.MappingNode)
	return assertContent(c[0], -1)
}

type Schemas struct {
	Map   map[string]*Schema `yaml:"map"`
	Slice []*Schema          `yaml:"slice"`
}

type Responses struct {
	Map   map[string]*Response `yaml:"map"`
	Slice []*Response          `yaml:"slice"`
}

type Paths struct {
	Map   map[string]*Path `yaml:"map"`
	Slice []*Path          `yaml:"slice"`
}

func (p *parser) parseSchemas(nodes []*yaml.Node) *Schemas {
	schemas := &Schemas{
		Map: make(map[string]*Schema),
	}
	for i := 0; i < len(nodes); i++ {
		n := nodes[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		key := n.Value
		next := assertNextKind(nodes, &i, yaml.MappingNode)
		var ref reference
		assertNodeDecode(next, &ref)
		assertRef(key, &ref)

		// parse schemas recursively
		s := p.parseSchemaRef(p.specFile, ref.Ref, nil)
		s.Key = key
		schemas.Map[key] = s
		schemas.Slice = append(schemas.Slice, s)
	}
	return schemas
}

func (p *parser) parsePaths(nodes []*yaml.Node) *Paths {
	paths := &Paths{
		Map: make(map[string]*Path),
	}
	for i := 0; i < len(nodes); i++ {
		n := nodes[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		key := n.Value
		next := assertNextKind(nodes, &i, yaml.MappingNode)
		var ref reference
		assertNodeDecode(next, &ref)
		assertRef(key, &ref)

		path := p.parsePathRef(p.specFile, ref.Ref, nil)
		path.Key = key
		paths.Map[key] = path
		paths.Slice = append(paths.Slice, path)
	}
	return paths
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

func (p *parser) parseResponseRef(currentFile string, ref string, files []string) (response *Response) {
	// Check if we have parsed it already
	dir := filepath.Dir(currentFile)
	responseFile := filepath.Join(dir, ref)
	if x, ok := p.responsesParsed[responseFile]; ok {
		return x
	}
	for _, f := range files {
		if f == responseFile {
			panic(fmt.Errorf("circular dependency detected for file %q", responseFile))
		}
	}

	var root yaml.Node
	data := readFile(responseFile)
	unmarshal(data, &root)
	dir = fileDir(responseFile)
	content := rootNodeContent(&root)

	response = p.parseResponseContent(responseFile, content, append(files, responseFile))
	response.File = responseFile
	response.Ref = ref
	p.responsesParsed[responseFile] = response

	return response
}

func (p *parser) parseSchemaRef(currentFile string, ref string, files []string) (schema *Schema) {
	// Check if we have parsed it already
	dir := filepath.Dir(currentFile)
	schemaFile := filepath.Join(dir, ref)
	if x, ok := p.schemasParsed[schemaFile]; ok {
		return x
	}
	for _, f := range files {
		if f == schemaFile {
			panic(fmt.Errorf("circular dependency detected for file %q", schemaFile))
		}
	}

	var root yaml.Node
	data := readFile(schemaFile)
	unmarshal(data, &root)
	dir = fileDir(schemaFile)
	content := rootNodeContent(&root)

	schema = p.parseSchemaContent(schemaFile, content, append(files, schemaFile))
	schema.File = schemaFile
	schema.Ref = ref
	p.schemasParsed[schemaFile] = schema

	return schema
}

func (p *parser) parsePathRef(currentFile string, ref string, files []string) (path *Path) {
	// Check if we have parsed it already
	dir := filepath.Dir(currentFile)
	pathFile := filepath.Join(dir, ref)
	if x, ok := p.pathsParsed[pathFile]; ok {
		return x
	}
	for _, f := range files {
		if f == pathFile {
			panic(fmt.Errorf("circular dependency detected for file %q", pathFile))
		}
	}

	var root yaml.Node
	data := readFile(pathFile)
	unmarshal(data, &root)
	dir = fileDir(pathFile)
	content := rootNodeContent(&root)

	path = p.parsePathFileContent(pathFile, content, append(files, pathFile))
	path.File = pathFile
	path.Ref = ref
	p.pathsParsed[pathFile] = path

	return path
}

func (p *parser) solveSchemaRef(currentFile string, files []string, schema *Schema) *Schema {
	ref := schema.Ref
	dir := filepath.Dir(currentFile)
	idx := strings.Index(ref, "#")
	if idx == -1 {
		// easy case, there's no hash part, we can go read the file directly.
		return p.parseSchemaRef(currentFile, ref, files)
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

	// we may have a reference to a component that has not been
	// parsed yet... but we don't have the file, we'll have to make
	// to register a promise.
	return p.promiseSchema(ref[21:], schema)
}

// parseSchemaContent must contain the
func (p *parser) parseSchemaContent(currentFile string, content []*yaml.Node, files []string) (schema *Schema) {
	schema = new(Schema)
	for i := 0; i < len(content); i++ {
		n := content[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		switch v := n.Value; v {
		case "$ref":
			schema.Ref = strings.TrimSpace(assertNextKind(content, &i, yaml.ScalarNode).Value)
			return p.solveSchemaRef(currentFile, files, schema)
		case "type":
			schema.Type = assertNextKind(content, &i, yaml.ScalarNode).Value
		case "description":
			schema.Description = assertNextKind(content, &i, yaml.ScalarNode).Value
		case "title":
			schema.Title = assertNextKind(content, &i, yaml.ScalarNode).Value
		case "required":
			schema.Required = captureStringSlice(assertNextKind(content, &i, yaml.SequenceNode).Content)
		case "nullable":
			schema.Nullable = assertBool(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "format":
			schema.Format = assertNextKind(content, &i, yaml.ScalarNode).Value
		case "pattern":
			schema.Pattern = assertNextKind(content, &i, yaml.ScalarNode).Value
		case "readOnly":
			schema.ReadOnly = assertBool(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "writeOnly":
			schema.WriteOnly = assertBool(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "minimum":
			schema.Minimum = assertFloat(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "exclusiveMinimum":
			schema.ExclusiveMinimum = assertFloat(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "maximum":
			schema.Maximum = assertFloat(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "exclusiveMaximum":
			schema.ExclusiveMaximum = assertFloat(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "minItems":
			schema.MinItems = assertInt(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "maxItems":
			schema.MaxItems = assertInt(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "uniqueItems":
			schema.UniqueItems = assertBool(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "minLength":
			schema.MinLength = assertInt(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "maxLength":
			schema.MaxLength = assertInt(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "minProperties":
			schema.MinProperties = assertInt(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "maxProperties":
			schema.MaxProperties = assertInt(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "enum":
			schema.Enum = captureStringSlice(assertNextKind(content, &i, yaml.SequenceNode).Content)
		case "x-enum":
			schema.EnumX = p.parseEnumX(assertNextKind(content, &i, yaml.MappingNode).Content)
		case "default":
			schema.Default = captureRaw("default", assertNext(content, &i))
		case "example":
			schema.Example = captureRaw("example", assertNext(content, &i))
		case "properties":
			schema.Properties = p.parseProperties(currentFile, assertNextKind(content, &i, yaml.MappingNode).Content, files)
		case "items":
			schema.Items = p.parseSchemaContent(currentFile, assertNextKind(content, &i, yaml.MappingNode).Content, files)
		default:
			panic(fmt.Errorf("unsupported shcema definfintion key %q at %s", v, schema.File))
		}
	}
	return schema
}

func (p *parser) parseEnumX(nodes []*yaml.Node) (x *EnumX) {
	x = &EnumX{
		Map: make(map[string]*EnumOptionX),
	}
	for i := 0; i < len(nodes); i++ {
		n := nodes[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		key := n.Value
		next := assertNextKind(nodes, &i, yaml.ScalarNode)
		opt := &EnumOptionX{
			Key:         key,
			Description: next.Value,
		}
		x.Map[key] = opt
		x.Slice = append(x.Slice, opt)
		x.Options = append(x.Options, key)
	}
	return x
}

func (p *parser) parsePathFileContent(currentFile string, content []*yaml.Node, files []string) (path *Path) {
	path = &Path{
		Map: make(map[string]*Endpoint),
	}
	for i := 0; i < len(content); i++ {
		n := content[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		method := n.Value
		next := assertNextKind(content, &i, yaml.MappingNode)
		e := p.parsePathEndpoint(currentFile, assertContent(next, -1), files)
		path.Map[method] = e
		path.Slice = append(path.Slice, e)
	}
	return path
}

func (p *parser) parseEndpointParameters(currentFile string, nodes []*yaml.Node) (params []*Parameter) {
	for i := 0; i < len(nodes); i++ {
		n := nodes[i]
		if n.Kind != yaml.MappingNode {
			panic("endpoint parameter should be a map")
		}

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
			if x, ok := p.parameters.Map[paramKey]; ok {
				params = append(params, x)
				continue
			}
		}
		if param.Schema == nil {
			panic(fmt.Errorf("no schema defined for path parameter at: %q", currentFile))
		}
		assertNotRef(param.Schema)
		params = append(params, &param)
	}

	return params
}

func (p *parser) parsePathEndpoint(currentFile string, content []*yaml.Node, files []string) (endpoint *Endpoint) {
	endpoint = new(Endpoint)
	for i := 0; i < len(content); i++ {
		n := content[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		switch v := n.Value; v {
		case "summary":
			endpoint.Summary = assertNextKind(content, &i, yaml.ScalarNode).Value
		case "description":
			endpoint.Description = assertNextKind(content, &i, yaml.ScalarNode).Value
		case "tags":
			endpoint.Tags = captureStringSlice(assertNextKind(content, &i, yaml.SequenceNode).Content)
		case "operationId":
			endpoint.OperationID = assertNextKind(content, &i, yaml.ScalarNode).Value
		case "parameters":
			next := assertNextKind(content, &i, yaml.SequenceNode)
			endpoint.Parameters = p.parseEndpointParameters(currentFile, assertContent(next, -1))
		case "responses":
			next := assertNextKind(content, &i, yaml.MappingNode)
			endpoint.Responses = p.parsePathResponses(currentFile, assertContent(next, -1), files)
		case "requestBody":
			next := assertNextKind(content, &i, yaml.MappingNode)
			endpoint.RequestBody = p.parseRequestBody(currentFile, assertContent(next, -1), files)
		default:
			panic(fmt.Errorf("unsupported endpoint key %q at %s", v, currentFile))
		}
	}

	return endpoint
}

func (p *parser) parsePathResponses(currentFile string, content []*yaml.Node, files []string) (responses *Responses) {
	responses = &Responses{
		Map: make(map[string]*Response),
	}
	for i := 0; i < len(content); i++ {
		n := content[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		status := n.Value
		next := assertNextKind(content, &i, yaml.MappingNode)
		var ref reference
		if err := next.Decode(&ref); err != nil {
			panic(fmt.Errorf("cannot decode path response %w at %s (%s)", err, currentFile, status))
		}
		var res *Response
		if ref.Ref != "" {
			res = p.parseResponseRef(currentFile, ref.Ref, files)
			res.Ref = ref.Ref

		} else {
			res = p.parseResponseContent(currentFile, assertContent(next, -1), files)
		}
		responses.Map[status] = res
		responses.Slice = append(responses.Slice, res)
	}

	return responses
}

func (p *parser) parseRequestBody(currentFile string, content []*yaml.Node, files []string) (body *RequestBody) {
	body = new(RequestBody)
	for i := 0; i < len(content); i++ {
		n := content[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		switch v := n.Value; v {
		case "$ref":
			panic(fmt.Errorf("request body full object reference is not supported at %q", currentFile))
		case "content":
			next := assertNextKind(content, &i, yaml.MappingNode)
			body.Content = p.parseMediaTypes(currentFile, assertContent(next, -1), files)
		case "required":
			body.Required = assertBool(assertNextKind(content, &i, yaml.ScalarNode).Value)
		case "description":
			body.Description = assertNextKind(content, &i, yaml.ScalarNode).Value
		case "example":
			body.Description = assertNextKind(content, &i, yaml.ScalarNode).Value
		default:
			panic(fmt.Errorf("unsupported endpoint key %q at %s", v, currentFile))
		}
	}
	return body
}

func (p *parser) parseMediaTypes(currentFile string, content []*yaml.Node, files []string) (mediaTypes map[string]*MediaType) {
	mediaTypes = make(map[string]*MediaType)
	for i := 0; i < len(content); i++ {
		n := content[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		contentType := n.Value
		next := assertNextKind(content, &i, yaml.MappingNode)
		mediaType := p.parseMediaType(currentFile, assertContent(next, -1), files)
		mediaType.File = currentFile
		mediaTypes[contentType] = mediaType
	}

	return mediaTypes
}

func (p *parser) parseMediaType(currentFile string, content []*yaml.Node, files []string) (mediaType *MediaType) {
	mediaType = new(MediaType)
	for i := 0; i < len(content); i++ {
		n := content[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		switch v := n.Value; v {
		case "schema":
			next := assertNextKind(content, &i, yaml.MappingNode)
			mediaType.Schema = p.parseSchemaContent(currentFile, assertContent(next, -1), files)
		case "example":
			mediaType.Example = captureRaw("example", assertNext(content, &i))
		case "examples":
			mediaType.Examples = captureRaw("examples", assertNext(content, &i))
		default:
			panic(fmt.Errorf("unexpected key in a media type map: %s at %s", v, currentFile))
		}
	}

	return mediaType
}

func (p *parser) parseResponseContent(currentFile string, content []*yaml.Node, files []string) (response *Response) {
	response = new(Response)
	for i := 0; i < len(content); i++ {
		n := content[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		switch v := n.Value; v {
		case "description":
			response.Description = assertNextKind(content, &i, yaml.ScalarNode).Value
		case "headers":
			panic("headers in responses are not supported yet")
		case "content":
			next := assertNextKind(content, &i, yaml.MappingNode)
			response.ContentTypes = p.parseMediaTypes(currentFile, assertContent(next, -1), files)
		default:
			panic(fmt.Errorf("unsupported response definfintion key %q at %s", v, currentFile))
		}
	}
	return response
}

func (p *parser) parseResponseContentTypes(currentFile string, content []*yaml.Node, files []string) map[string]*MediaType {
	ct := make(map[string]*MediaType)
	for i := 0; i < len(content); i++ {
		n := content[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		contentType := n.Value // application/json
		next := assertNextKind(content, &i, yaml.MappingNode)
		nodes := assertContent(next, -1)

		ct[contentType] = &MediaType{}
		for j := 0; j < len(nodes); j++ {
			nn := nodes[j]
			if nn.Kind != yaml.ScalarNode {
				continue
			}
			switch v := nn.Value; v {
			case "schema":
				next = assertNextKind(nodes, &j, yaml.MappingNode)
				p.parseSchemaContent(currentFile, assertContent(next, -1), files)
			case "example":
				ct[contentType].Example = captureRaw("example", assertNext(nodes, &j))
			case "examples":
				ct[contentType].Examples = captureRaw("examples", assertNext(nodes, &j))
			}
		}
	}
	return ct
}

func (p *parser) parseProperties(currentFile string, content []*yaml.Node, files []string) *Schemas {
	props := &Schemas{
		Map: make(map[string]*Schema),
	}
	for i := 0; i < len(content); i++ {
		n := content[i]
		if n.Kind != yaml.ScalarNode {
			continue
		}
		key := n.Value
		next := assertNextKind(content, &i, yaml.MappingNode)

		schema := p.parseSchemaContent(currentFile, next.Content, files)
		schema.Key = key
		schema.File = currentFile
		props.Map[key] = schema
		props.Slice = append(props.Slice, schema)
	}
	return props
}

func captureContent(content []*yaml.Node, capture map[string]yaml.Kind) map[string][]*yaml.Node {
	length := len(content)
	found := make(map[string][]*yaml.Node)

	for i := 0; i < length; i++ {
		c := content[i]
		if c.Kind != yaml.ScalarNode {
			continue
		}
		if wantKind, ok := capture[c.Value]; ok {
			next := assertNextKind(content, &i, wantKind)
			found[c.Value] = next.Content
			delete(capture, c.Value)
		}
	}
	return found
}

func assertNext(nodes []*yaml.Node, cursor *int) (next *yaml.Node) {
	*cursor++
	if *cursor >= len(nodes) {
		panic(errors.New("expected next node"))
	}
	next = nodes[*cursor]
	return next
}

func assertNextKind(nodes []*yaml.Node, cursor *int, want yaml.Kind) (next *yaml.Node) {
	next = assertNext(nodes, cursor)
	assertKind(next, want)
	return next
}

func assertNodeDecode(node *yaml.Node, v interface{}) {
	if err := node.Decode(v); err != nil {
		panic(fmt.Errorf("cannot decoded node into struct type %s: %v", reflect.TypeOf(v), err))
	}
}

func assertKind(node *yaml.Node, want yaml.Kind) {
	if got := node.Kind; got != want {
		panic(fmt.Errorf("expected node kind %q, got %q", want, got))
	}
}

func assertContent(node *yaml.Node, want int) []*yaml.Node {
	if got := len(node.Content); want != -1 && got != want {
		panic(fmt.Errorf("expected node content length of %q, got %q", want, got))
	}
	return node.Content
}

func unmarshalFile(file string, v interface{}) {
	unmarshal(readFile(file), v)
}

func fileDir(file string) string {
	abs, err := filepath.Abs(file)
	if err != nil {
		panic(fmt.Errorf("cannot detect absolute path %q: %w", file, err))
	}
	dir, _ := filepath.Split(abs)
	return dir
}

func unmarshal(data []byte, v interface{}) {
	if err := yaml.Unmarshal(data, v); err != nil {
		panic(fmt.Errorf("cannot unmarshal yaml data to %q: %w", reflect.TypeOf(v), err))
	}
}

func readFile(file string) []byte {
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
	return b
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

func absolutePath(file string) string {
	abs, err := filepath.Abs(file)
	if err != nil {
		panic(fmt.Errorf("cannot detect file absolute path %q: %v", file, err))
	}
	return abs
}
