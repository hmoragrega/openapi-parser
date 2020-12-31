package openapiparser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

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

type Response struct {
	Ref          string       `yaml:"$ref,omitempty"`
	Status       string       `yaml:"status,omitempty"`
	Description  string       `yaml:"description,omitempty"`
	ContentTypes ContentTypes `yaml:"content,omitempty"`
}

type RequestBody struct {
	Required     bool         `yaml:"required,omitempty"`
	Description  string       `yaml:"description,omitempty"`
	ContentTypes ContentTypes `yaml:"content,omitempty"`
	Example      string       `yaml:"example,omitempty"`
}

type ContentTypes map[string]MediaType

type MediaType struct {
	Schema   Schema `yaml:"schema,omitempty"`
	Example  string `yaml:"example,omitempty"`
	Examples string `yaml:"examples,omitempty"`
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

func joinPath(currentFile, ref string) string {
	return path.Join(filepath.Dir(currentFile), ref)
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
