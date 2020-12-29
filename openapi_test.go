package openapiparser_test

import (
	"bytes"
	"io/ioutil"
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
		generated, err := yaml.Marshal(spec)
		if err != nil {
			t.Fatalf("cannot marshal generated spec: %v", err)
		}
		golden, err := ioutil.ReadFile("test/golden.yml")
		if err != nil {
			t.Fatalf("cannot read golden test file: %v", err)
		}
		generated = bytes.TrimSpace(generated)
		golden = bytes.TrimSpace(golden)

		if bytes.Compare(golden, generated) != 0 {
			t.Errorf("generated file is different")
		}
	})

	t.Run("JSON example output", func(t *testing.T) {
		s := spec.SchemaMap["School"]
		got, err := s.JSONIndent("\t")
		if err != nil {
			t.Fatalf("error generatiing indented JSON: %v", err)
		}
		if got != schoolJSON {
			t.Fatalf("generated JSON is different. Got:\n%s\n Want:\n%s\n", got, schoolJSON)
		}
	})
}

var (
	schoolJSON = `{
	"id": "e017d029-a459-4cfc-bf35-dd774ddf50e7",
	"code": "ru-moscow-101",
	"name": "Moscow's International Business School - \"Center\"",
	"contacts": [
		{
			"email": "email@example.com",
			"position": "Main office secretary.",
			"name": "John doe",
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
	"foundation_year": 1983,
	"modality": "Private",
	"open": true,
	"createdAt": "2015-12-13T10:05:48+01:00",
	"updatedAt": "2015-12-13T10:05:48+01:00"
}`
)
