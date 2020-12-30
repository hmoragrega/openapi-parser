package doc

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hmoragrega/openapi-parser"
)

const (
	DefaultJSONIndent  = "  "
	DefaultContentType = "application/json"
)

var (
	spacesRx  = regexp.MustCompile(`\s+`)
	pathParam = regexp.MustCompile("{(\\w+)}")
)

// Resource is a different representation of a defined schema
type Resource struct {
	// Name name of the resource.
	Name string
	// JSON a rendered example of the resource.
	JSON string
	// Description the description of the resource.
	Description string
	// Fields the fields of the resource.
	Fields []ResourceField
}

type ResourceField struct {
	// Status a flatten key for the resource filed
	// Examples:
	//  - foo
	//  - foo.bar
	//  - foo.#.bar
	Key string
	// Type JSON type of the field.
	Type string
	// Link if not empty holds the name of a named resource
	// that was hold the information for this field.
	Link string
	// Enumeration holds the title of the enumeration.
	Enumeration string
	// Description the resource description.
	Description string
}

// Resources returns a list of resource defined in a spec file.
func Resources(spec *openapiparser.Spec) (resources []Resource, err error) {
	return ResourcesWithJSONIndent(spec, DefaultJSONIndent)
}

// ResourcesWithJSONIndent returns a list of resource defined in a spec file with
// custom indentation for the JSON examples.
func ResourcesWithJSONIndent(spec *openapiparser.Spec, indent string) (resources []Resource, err error) {
	for _, x := range spec.Schemas {
		js, err := x.JSONIndent(openapiparser.ReadOp, indent)
		if err != nil {
			return nil, fmt.Errorf("could not build JSON for schema %s: %v", x.Key, err)
		}
		fields, err := buildSchemaFields(&x, x.Name, nil)
		if err != nil {
			return nil, fmt.Errorf("could not build Resource fields for schema %s: %v", x.Key, err)
		}
		resources = append(resources, Resource{
			Name:        x.Name,
			Description: x.Description,
			JSON:        js,
			Fields:      fields,
		})
	}
	return resources, nil
}

func buildSchemaFields(x *openapiparser.Schema, name string, keys []string) (fields []ResourceField, err error) {
	for _, p := range x.Properties {
		child, err := buildResourceFields(&p, name, keys)
		if err != nil {
			return nil, err
		}
		fields = append(fields, child...)
	}
	return fields, nil
}

func buildResourceFields(x *openapiparser.Schema, name string, keys []string) (fields []ResourceField, err error) {
	var lines []string
	for _, l := range strings.Split(x.Description, "\n") {
		line := strings.TrimSpace(l)
		if line != "" { // remove empty lines
			lines = append(lines, line)
		}
	}
	var (
		desc  = strings.Join(lines, "\n")
		key   = strings.Trim(x.Key, "\n ")
		child []ResourceField
		link  string
		xType string
		enum  string
	)
	if key == "" {
		// is an array, use a hash to separate
		// the elements like `array.#.object_key`
		key = "#"
	}
	switch x.Type {
	case "bool", "boolean":
		xType = "Boolean"
	case "string":
		xType = "String"
	case "object":
		xType = "Object"
		if x.Name != "" && x.Name != name {
			// Is a named Resource, add a link and move on
			link = x.Name
			break
		}
		child, err = buildSchemaFields(x, name, append(keys, key))
		if err != nil {
			return nil, err
		}
	case "number", "integer":
		xType = "Number"
	case "array":
		xType = "Array"
		if x.Items.Type == "array" || x.Items.Type == "object" {
			// see if there's some fields to collapse
			child, err = buildResourceFields(x.Items, name, append(keys, key))
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("cannot determine JSON type %q", x.Type)
	}
	if len(x.Enum) > 0 || len(x.EnumX.Options()) > 0 {
		enumKeys := []string{name}
		for _, k := range keys {
			if k != "#" {
				enumKeys = append(enumKeys, k)
			}
		}
		enum = enumTitle(append(enumKeys, key))
	}
	if len(keys) > 0 {
		key = strings.Join(keys, ".") + "." + key
	}
	fields = append(fields, ResourceField{
		Key:         key,
		Type:        xType,
		Link:        link,
		Enumeration: enum,
		Description: strings.TrimRight(desc, "\n "),
	})

	return append(fields, child...), nil
}

type Enum struct {
	// Title of the enum, includes the field name an the resource name.
	Title string
	// Resource resource name the enum belongs to.
	Resource string
	// Options all the available options for the enumeration.
	Options []Option
}

type Option struct {
	// Status holds the option value.
	Value string
	// Description is only filled if there was a custom "x-enum" entry in the spec file.
	Description string
}

// Enumerations returns a list of enumerations in a spec file.
func Enumerations(spec *openapiparser.Spec) (enums []Enum) {
	for _, x := range spec.Schemas {
		enums = append(enums, schemaEnums(&x, []string{x.Name})...)
	}
	return enums
}

func schemaEnums(x *openapiparser.Schema, keys []string) (enums []Enum) {
	if x == nil {
		return enums
	}
	if opts := x.EnumX.Options(); len(opts) > 0 {
		enums = append(enums, enumXToDocEnum(x.EnumX, keys))
	} else if len(x.Enum) > 0 {
		enums = append(enums, enumToDocEnum(x.Enum, keys))
	}
	for _, p := range x.Properties {
		enums = append(enums, schemaEnums(&p, append(keys, p.Key))...)
	}
	if x.Items != nil {
		enums = append(enums, schemaEnums(x.Items, append(keys, x.Items.Key))...)
	}
	return enums
}

func enumXToDocEnum(x openapiparser.EnumX, keys []string) Enum {
	e := Enum{
		Title:    enumTitle(keys),
		Resource: keys[0],
	}
	for _, o := range x.Options() {
		e.Options = append(e.Options, Option{
			Value:       o.Key,
			Description: o.Description,
		})
	}
	return e
}

func enumToDocEnum(opts []string, keys []string) Enum {
	e := Enum{
		Title:    enumTitle(keys),
		Resource: keys[0],
	}
	for _, o := range opts {
		e.Options = append(e.Options, Option{
			Value:       o,
			Description: "",
		})
	}
	return e
}

func toTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.Title(s)
	s = spacesRx.ReplaceAllString(s, " ")

	return s
}

type Tag struct {
	// Tag name
	Name string
	// Endpoint list of endpoints with the same tag.
	Endpoint []Endpoint
}

type Endpoint struct {
	// Summary of the endpoint.
	Summary string
	// Description of the endpoint.
	Description string
	// Method HTTP method.
	Method string
	// Status HTTP status of the first response.
	Status string
	// Path with param tags as defined.
	Path string
	// Path with param tags replaced by examples.
	PathWithParams string
	// RequestBody request body example, if any.
	RequestBody string
	// Response JSON example of the first defined response.
	Response string
	// ResponseDescription description of the first response.
	ResponseDescription string
}

type endpointsConfig struct {
	contentType string
	indent      string
}

func WithContentType(contentType string) func(*endpointsConfig) {
	return func(config *endpointsConfig) {
		config.contentType = contentType
	}
}

func WithIndent(indent string) func(*endpointsConfig) {
	return func(config *endpointsConfig) {
		config.indent = indent
	}
}

// PathTags builds a lis of tags that contain the paths belonging to
// them respecting the order in which they were defined in the spec file.
//
// The last tag will be "Default" and contain all the endpoints that have no
// tags (if any).
//
// If an endpoint has more than one tag defined only the first one will be
// used.
func EndpointsByTag(spec *openapiparser.Spec, opts ...func(*endpointsConfig)) (tags []Tag, err error) {
	c := endpointsConfig{
		contentType: DefaultContentType,
		indent:      DefaultJSONIndent,
	}
	for _, o := range opts {
		o(&c)
	}
	found := make(map[string]Tag)
	var order []string
	for _, p := range spec.Paths {
		for _, ep := range p.Endpoints {
			name := "Default"
			if len(ep.Tags[0]) > 0 {
				name = ep.Tags[0]
			}
			tag, ok := found[name]
			if !ok {
				found[name] = Tag{
					Name: name,
				}
				order = append(order, name)
			}

			var res *openapiparser.Response
			for _, r := range ep.Responses {
				if r.Status[0] < '3' {
					res = &r
					break
				}
			}
			if res == nil {
				return nil, fmt.Errorf("no success response available for endpoint: %s %s", ep.Method, p.Key)
			}

			ex, err := responseExample(res, c.indent, c.contentType)
			if err != nil {
				return nil, fmt.Errorf("cannot group endpoints by tag: %v", err)
			}
			rb, err := requestBody(ep.RequestBody, c.indent, c.contentType)
			if err != nil {
				return nil, fmt.Errorf("cannot group endpoints by tag: %v", err)
			}
			tag.Name = name
			tag.Endpoint = append(tag.Endpoint, Endpoint{
				Summary:             ep.Summary,
				Description:         ep.Description,
				Method:              ep.Method,
				Path:                p.Key,
				PathWithParams:      replacePathParams(p.Key, ep),
				RequestBody:         rb,
				Status:              res.Status,
				Response:            ex,
				ResponseDescription: res.Description,
			})
			found[name] = tag
		}
	}
	for _, tag := range order {
		tags = append(tags, found[tag])
	}
	return tags, nil
}

func replacePathParams(path string, ep openapiparser.Endpoint) string {
	for _, m := range pathParam.FindAllStringSubmatch(path, -1) {
		var (
			tag         = m[0]
			placeholder = m[1]
		)
		for _, param := range ep.Parameters {
			if param.In != "path" || param.Name != placeholder {
				continue
			}
			var (
				x = placeholder
				s = param.Schema
			)
			if s.Example != "" {
				x = cleanExample(s.Example)
			} else {
				switch s.Type {
				case "string":
					if s.Format == "uuid" {
						x = "e017d029-a459-4cfc-bf35-dd774ddf50e7"
						break
					}
				case "integer", "number":
					if s.Format == "double" || s.Format == "float" {
						x = "100.50"
						break
					}
					x = "100"
				}
			}
			path = strings.ReplaceAll(path, tag, x)
		}
	}
	return path
}

func responseExample(res *openapiparser.Response, indent string, contentType string) (string, error) {
	for ct, m := range res.ContentTypes {
		if ct == contentType {
			return m.Schema.JSONIndent(openapiparser.ReadOp, indent)
		}
	}
	return "", fmt.Errorf("no content type defined for response: %s", res.Description)
}

func requestBody(rb openapiparser.RequestBody, indent string, contentType string) (string, error) {
	for ct, m := range rb.Content {
		if ct == contentType {
			return m.Schema.JSONIndent(openapiparser.WriteOp, indent)
		}
	}
	return "", nil
}

func cleanExample(s string) string {
	return strings.TrimSpace(strings.Trim(s, `"`))
}

func enumTitle(keys []string) string {
	return toTitle(strings.Join(keys, " "))
}
