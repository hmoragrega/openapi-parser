package openapiparser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
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
	Paths      []Path            `yaml:"paths,omitempty"`
	Schemas    []Schema          `yaml:"schemas,omitempty"`
	SchemaMap  map[string]Schema `yaml:"-"`
	Parameters []Parameter       `yaml:"parameters,omitempty"`
}

type Meta struct {
	OpenAPI string   `yaml:"openapi,omitempty"`
	Info    Info     `yaml:"info,omitempty"`
	Servers []Server `yaml:"servers,omitempty"`
}

type Path struct {
	Key       string     `yaml:"key,omitempty"`
	File      string     `yaml:"file,omitempty"`
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
	// File holds the name of the file where the schema is defined
	// components schemas section
	File string `yaml:"file,omitempty"`
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

	solve string
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
		panic("unexpected type building schema JSON " + s.Type + " at " + s.File)
	}
	return nil
}

type Response struct {
	Ref          string               `yaml:"$ref,omitempty"`
	File         string               `yaml:"file,omitempty"`
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
	File     string `yaml:"file,omitempty"`
	Schema   Schema `yaml:"schema,omitempty"`
	Example  string `yaml:"example,omitempty"`
	Examples string `yaml:"examples,omitempty"`
}

type reference struct {
	Ref string `yaml:"$ref,omitempty"`
}

type parser struct {
	specFile   string
	parameters map[string]Parameter
	schemas    map[string]Schema
	paths      map[string]Path

	schemasParsed   map[string]Schema
	responsesParsed map[string]Response
	pathsParsed     map[string]Path
	schemaPromises  map[string]struct{}
}

func (p *parser) completeSchemas(keys []string) []Schema {
	var (
		list  = make([]Schema, len(keys))
		names = make(map[string]string, len(keys))
	)
	for _, x := range p.schemas {
		names[x.File] = x.Name
	}
	for i, k := range keys {
		x := p.schemas[k]
		p.completeSchema(&x, names)
		p.schemas[k] = x
		p.schemasParsed[x.File] = x // overwrite with the updated values.
		list[i] = x
	}
	return list
}

func (p *parser) completeSchema(x *Schema, names map[string]string) {
	if x == nil {
		return
	}
	for i, prop := range x.Properties {
		p.completeSchema(&prop, names)
		x.Properties[i] = prop
	}
	p.completeSchema(x.Items, names)
	if x.solve != "" {
		xx, ok := p.schemas[x.solve]
		if !ok {
			panic(fmt.Errorf("cannot find schema named %s", x.solve))
		}
		key := x.Key // keep the key
		*x = xx
		(*x).Key = key
	}
	if x.Name == "" {
		if name, ok := names[x.File]; ok {
			// if not found means the schema imported a file
			// but is in the list of #/components/schemas
			(*x).Name = name
		}
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
		schemasParsed:   make(map[string]Schema),
		pathsParsed:     make(map[string]Path),
		responsesParsed: make(map[string]Response),
		schemaPromises:  make(map[string]struct{}),
	}
}

type captureFunc func(node *yaml.Node)

type captureMap map[string]captureFunc

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

func (p *parser) parse() *Spec {
	rootContent := fileContent(p.specFile)

	// we can directly decode the meta as it's
	// schemas is fixed and known in advance.
	var meta Meta
	captureContent2(rootContent, map[string]captureFunc{
		"openapi": captureString(&meta.OpenAPI),
		"info":    captureNodeDecode(&meta.Info),
		"servers": captureNodeDecode(&meta.Servers),
	}, nil)

	content := captureContent(rootContent, map[string]yaml.Kind{
		"paths":      yaml.MappingNode,
		"components": yaml.MappingNode,
	})
	components := captureContent(content["components"], map[string]yaml.Kind{
		"schemas":    yaml.MappingNode,
		"parameters": yaml.MappingNode,
	})
	if x, ok := components["parameters"]; ok {
		params := p.parseParameters(x)
		p.parameters = params.Map
	}
	keys, schemas := p.parseSchemas(components["schemas"])
	p.schemas = schemas
	list := p.completeSchemas(keys)

	return &Spec{
		Meta:      meta,
		Paths:     p.parsePaths(content["paths"]),
		Schemas:   list,
		SchemaMap: p.schemas,
	}
}

// ParametersMap hold the parameters in a map
// but also as an slice with the same order
// they were found during parsing.
type Parameters struct {
	Map  map[string]Parameter `yaml:"map"`
	Keys []string             `yaml:"keys"`
}

func (p *parser) parseParameters(content []*yaml.Node) Parameters {
	params := Parameters{
		Map: make(map[string]Parameter),
	}
	for _, v := range mapPairs(content) {
		next := assertKind(v.node, yaml.MappingNode)
		var param Parameter
		assertNodeDecode(next, &param)
		assertNotRef(&param.Schema)

		key := v.key
		params.Map[key] = param
		params.Keys = append(params.Keys, key)
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
	for _, p := range schema.Properties {
		assertNotRef(&p)
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
	Map  map[string]Schema `yaml:"map"`
	Keys []string          `yaml:"keys"`
}

type Responses struct {
	Map  map[string]Response `yaml:"map"`
	Keys []string            `yaml:"keys"`
}

type Paths struct {
	Map  map[string]Path `yaml:"map"`
	Keys []string        `yaml:"key"`
}

func (p *parser) parseSchemas(content []*yaml.Node) (keys []string, schemas map[string]Schema) {
	schemas = make(map[string]Schema)
	for _, v := range mapPairs(content) {
		key := v.key
		var ref reference
		assertKind(v.node, yaml.MappingNode)
		assertNodeDecode(v.node, &ref)
		assertRef(key, &ref)

		// parse schemas recursively
		s := p.parseSchemaRef(p.specFile, ref.Ref, nil)
		s.Name = key
		schemas[key] = s
		keys = append(keys, key)
	}
	return keys, schemas
}

func (p *parser) parsePaths(content []*yaml.Node) []Path {
	var paths []Path
	for _, v := range mapPairs(content) {
		key := v.key
		next := assertKind(v.node, yaml.MappingNode)
		var ref reference
		assertNodeDecode(next, &ref)
		assertRef(key, &ref)

		path := p.parsePathRef(p.specFile, ref.Ref, nil)
		path.Key = key
		paths = append(paths, path)
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

func (p *parser) parseResponseRef(currentFile string, ref string, files []string) (response Response) {
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

	content := fileContent(responseFile)

	response = p.parseResponseContent(responseFile, content, append(files, responseFile))
	response.File = responseFile
	response.Ref = ref
	p.responsesParsed[responseFile] = response

	return response
}

func (p *parser) parseSchemaRef(currentFile string, ref string, files []string) Schema {
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

	content := fileContent(schemaFile)
	schema := p.parseSchemaContent(schemaFile, content, append(files, schemaFile))
	schema.File = schemaFile
	schema.Ref = ref
	p.schemasParsed[schemaFile] = schema

	return schema
}

func (p *parser) parsePathRef(currentFile string, ref string, files []string) (path Path) {
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

	content := fileContent(pathFile)
	path = p.parsePathFileContent(pathFile, content, append(files, pathFile))
	path.File = pathFile
	path.Ref = ref
	p.pathsParsed[pathFile] = path

	return path
}

func (p *parser) solveSchemaRef(currentFile string, ref string, files []string) Schema {
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

	name := ref[21:]
	if x, ok := p.schemas[name]; ok {
		return x
	}

	// we may have a reference to a component that has not been
	// parsed yet...
	return Schema{solve: name}
}

// parseSchemaContent must contain the
func (p *parser) parseSchemaContent(currentFile string, content []*yaml.Node, files []string) (schema Schema) {
	for _, v := range mapPairs(content) {
		switch k := v.key; k {
		case "$ref":
			// TODO trims space on values
			ref := strings.TrimSpace(assertKind(v.node, yaml.ScalarNode).Value)
			return p.solveSchemaRef(currentFile, ref, files)
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
			schema.Properties = p.parseProperties(currentFile, assertKind(v.node, yaml.MappingNode).Content, files)
		case "items":
			x := p.parseSchemaContent(currentFile, assertKind(v.node, yaml.MappingNode).Content, files)
			schema.Items = &x
		default:
			panic(fmt.Errorf("unsupported shcema definfintion key %q at %s", k, schema.File))
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

func (p *parser) parsePathFileContent(currentFile string, content []*yaml.Node, files []string) (path Path) {
	for _, v := range mapPairs(content) {
		assertKind(v.node, yaml.MappingNode)
		e := p.parsePathEndpoint(currentFile, v.node.Content, files)
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
			x, ok := p.parameters[paramKey]
			if !ok {
				panic(fmt.Errorf("undefined parameter %q at: %q", ref, currentFile))

			}
			params = append(params, x)
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

func (p *parser) parsePathEndpoint(currentFile string, content []*yaml.Node, files []string) (endpoint Endpoint) {
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
			endpoint.Responses = p.parsePathResponses(currentFile, v.node.Content, files)
		case "requestBody":
			assertKind(v.node, yaml.MappingNode)
			endpoint.RequestBody = p.parseRequestBody(currentFile, v.node.Content, files)
		default:
			panic(fmt.Errorf("unsupported endpoint key %q at %s", k, currentFile))
		}
	}

	return endpoint
}

func (p *parser) parsePathResponses(currentFile string, content []*yaml.Node, files []string) (responses []Response) {
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
			res = p.parseResponseRef(currentFile, ref.Ref, files)
			res.Ref = ref.Ref
		} else {
			res = p.parseResponseContent(currentFile, v.node.Content, files)
		}
		res.Status = status
		responses = append(responses, res)
	}

	return responses
}

func (p *parser) parseRequestBody(currentFile string, content []*yaml.Node, files []string) (body RequestBody) {
	for _, v := range mapPairs(content) {
		switch k := v.key; k {
		case "$ref":
			panic(fmt.Errorf("request body full object reference is not supported at %q", currentFile))
		case "content":
			assertKind(v.node, yaml.MappingNode)
			body.Content = p.parseMediaTypes(currentFile, v.node.Content, files)
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

func (p *parser) parseMediaTypes(currentFile string, content []*yaml.Node, files []string) (mediaTypes map[string]MediaType) {
	mediaTypes = make(map[string]MediaType)
	for _, v := range mapPairs(content) {
		contentType := v.key
		assertKind(v.node, yaml.MappingNode)
		mediaType := p.parseMediaType(currentFile, v.node.Content, files)
		mediaType.File = currentFile
		mediaTypes[contentType] = mediaType
	}

	return mediaTypes
}

func (p *parser) parseMediaType(currentFile string, content []*yaml.Node, files []string) (mediaType MediaType) {
	for _, v := range mapPairs(content) {
		switch k := v.key; k {
		case "schema":
			assertKind(v.node, yaml.MappingNode)
			mediaType.Schema = p.parseSchemaContent(currentFile, v.node.Content, files)
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

func (p *parser) parseResponseContent(currentFile string, content []*yaml.Node, files []string) (response Response) {
	for _, v := range mapPairs(content) {
		switch k := v.key; k {
		case "description":
			response.Description = assertKind(v.node, yaml.ScalarNode).Value
		case "headers":
			panic("headers in responses are not supported yet")
		case "content":
			assertKind(v.node, yaml.MappingNode)
			response.ContentTypes = p.parseMediaTypes(currentFile, v.node.Content, files)
		default:
			panic(fmt.Errorf("unsupported response definfintion key %q at %s", k, currentFile))
		}
	}
	return response
}

func (p *parser) parseResponseContentTypes(currentFile string, content []*yaml.Node, files []string) map[string]*MediaType {
	ct := make(map[string]*MediaType)
	for _, v := range mapPairs(content) {
		contentType := v.key // application/json
		assertKind(v.node, yaml.MappingNode)
		mt := p.parseMediaType(currentFile, v.node.Content, files)
		ct[contentType] = &mt
	}
	return ct
}

func (p *parser) parseProperties(currentFile string, content []*yaml.Node, files []string) (props []Schema) {
	for _, v := range mapPairs(content) {
		assertKind(v.node, yaml.MappingNode)
		schema := p.parseSchemaContent(currentFile, v.node.Content, files)
		schema.File = currentFile
		schema.Key = v.key
		props = append(props, schema)
	}
	return props
}

func captureContent(content []*yaml.Node, capture map[string]yaml.Kind) map[string][]*yaml.Node {
	found := make(map[string][]*yaml.Node)
	for _, v := range mapPairs(content) {
		if wantKind, ok := capture[v.key]; ok {
			assertKind(v.node, wantKind)
			found[v.key] = v.node.Content
			delete(capture, v.key)
		}
	}
	return found
}

func captureContent2(content []*yaml.Node, required captureMap, optional captureMap) {
	for _, v := range mapPairs(content) {
		if fx, ok := required[v.key]; ok {
			fx(v.node)
			delete(required, v.key)
			continue
		}
		if fx, ok := optional[v.key]; ok {
			fx(v.node)
			delete(optional, v.key)
			continue
		}
	}
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

func assertContent(node *yaml.Node, want int) []*yaml.Node {
	if got := len(node.Content); want != -1 && got != want {
		panic(fmt.Errorf("expected node content length of %q, got %q", want, got))
	}
	return node.Content
}

func unmarshalFile(file string, v interface{}) {
	unmarshal(readFile(file), v)
}

func fileContent(file string) []*yaml.Node {
	var root yaml.Node
	unmarshal(readFile(file), &root)
	return rootNodeContent(&root)
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
