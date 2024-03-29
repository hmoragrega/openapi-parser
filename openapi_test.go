package openapiparser_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	openapiparser "github.com/hmoragrega/openapi-parser"
	"gopkg.in/yaml.v3"
)

func TestParse(t *testing.T) {
	spec, err := openapiparser.Parse("api/spec/openapi.yml")
	if err != nil {
		t.Fatalf("cannot parse example file: %v", err)
	}

	t.Run("compare golden file", func(t *testing.T) {
		got, err := yaml.Marshal(spec)
		if err != nil {
			t.Fatalf("cannot marshal generated spec: %v", err)
		}
		golden, err := ioutil.ReadFile("test/golden.yml")
		if err != nil {
			t.Fatalf("cannot read golden test file: %v", err)
		}
		got = bytes.TrimSpace(got)
		golden = bytes.TrimSpace(golden)

		if bytes.Compare(golden, got) != 0 {
			t.Errorf("generated file is different. Got:\n'%s'\n Want:\n'%s'\n", got, golden)

			f, err := os.Create("test/got.yml")
			if err != nil {
				t.Fatalf("cannot create generated file: %v", err)
			}
			if _, err = f.Write(got); err != nil {
				t.Fatalf("cannot write generated file: %v", err)
			}
			if err = f.Close(); err != nil {
				t.Fatalf("cannot close generated file: %v", err)
			}
		}
	})

	t.Run("JSON read example output", func(t *testing.T) {
		s := spec.Schemas[0] // School
		got, err := s.JSONIndent(openapiparser.ReadOp, "\t")
		if err != nil {
			t.Fatalf("error generatiing indented JSON: %v", err)
		}
		if got != schoolReadJSON {
			t.Fatalf("generated JSON is different. Got:\n%s\n Want:\n%s\n", got, schoolReadJSON)
		}
	})

	t.Run("JSON write example output", func(t *testing.T) {
		s := spec.Schemas[0] // School
		got, err := s.JSONIndent(openapiparser.WriteOp, "\t")
		if err != nil {
			t.Fatalf("error generatiing indented JSON: %v", err)
		}
		if got != schoolWriteJSON {
			t.Fatalf("generated JSON is different. Got:\n%s\n Want:\n%s\n", got, schoolWriteJSON)
		}
	})
}

var (
	schoolReadJSON = `{
	"id": "e017d029-a459-4cfc-bf35-dd774ddf50e7",
	"code": "ru-moscow-101",
	"name": "Moscow's International Business School - \"Center\"",
	"contacts": [
		{
			"email": "email@example.com",
			"position": "Main office secretary.",
			"name": "John doe",
			"priority": "Primary",
			"phone": [
				"(555)-1234566789"
			]
		}
	],
	"main_office": {
		"street": "Krylatskaya Ulitsa",
		"number": 22,
		"area-code": "FRS12-188",
		"city": "Moscow",
		"country": "RUS"
	},
	"campus": [
		{
			"street": "Krylatskaya Ulitsa",
			"number": 22,
			"area-code": "FRS12-188",
			"city": "Moscow",
			"country": "RUS"
		}
	],
	"embed": "embedded",
	"foundation_year": 1983,
	"modality": "Private",
	"open": true,
	"createdAt": "2015-12-13T10:05:48+01:00",
	"updatedAt": "2015-12-13T10:05:48+01:00"
}`

	schoolWriteJSON = `{
	"code": "ru-moscow-101",
	"name": "Moscow's International Business School - \"Center\"",
	"contacts": [
		{
			"email": "email@example.com",
			"position": "Main office secretary.",
			"name": "John doe",
			"priority": "Primary",
			"phone": [
				"(555)-1234566789"
			]
		}
	],
	"main_office": {
		"street": "Krylatskaya Ulitsa",
		"number": 22,
		"area-code": "FRS12-188",
		"city": "Moscow",
		"country": "RUS"
	},
	"campus": [
		{
			"street": "Krylatskaya Ulitsa",
			"number": 22,
			"area-code": "FRS12-188",
			"city": "Moscow",
			"country": "RUS"
		}
	],
	"embed": "embedded",
	"foundation_year": 1983,
	"modality": "Private",
	"open": true
}`
)
