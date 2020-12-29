package doc

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hmoragrega/openapi-parser"
)

const DefaultJSONIndent = "  "

var (
	spacesRx = regexp.MustCompile(`\s+`)
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
	// Key a flatten key for the resource filed
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
		js, err := x.JSONIndent(indent)
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
	if len(keys) > 0 {
		key = strings.Join(keys, ".") + "." + key
	}
	fields = append(fields, ResourceField{
		Key:         key,
		Type:        xType,
		Link:        link,
		Description: strings.TrimRight(desc, "\n "),
	})

	return append(fields, child...), nil
}

/*
---
weight: 50
title: Enumerations
---
# Enumerations

A number of resource fields will only accept a list of known values, as outlined in this section.
{{range $enum := .}}
## {{ $enum.Title }}

References [{{ $enum.Resource}}](#{{ $enum.ResourceAnchor }})

Mode | Description
-----|-------------
{{range $option := $enum.Options}}{{$option.Name}} | {{$option.Description}}
{{end}}
{{end}}
*/

type Enum struct {
	// Title of the enum, includes the field name an the resource name.
	Title string
	// Resource resource name the enum belongs to.
	Resource string
	// Options all the available options for the enumeration.
	Options []Option
}

type Option struct {
	// Key holds the option value.
	Value string
	// Description is only filled if there was a custom "x-enum" entry in the spec file.
	Description string
}

// Enumerations returns a list of enumerations in a spec file.
func Enumerations(spec *openapiparser.Spec) (enums []Enum, err error) {
	for _, x := range spec.Schemas {
		enums = append(enums, schemaEnums(&x, []string{x.Name})...)
	}
	return enums, nil
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
		Title:    toTitle(strings.Join(keys, " ")),
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
		Title:    toTitle(strings.Join(keys, " ")),
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
