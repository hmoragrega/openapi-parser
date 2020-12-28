package openapiparser_test

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	openapiparser "github.com/hmoragrega/openapi-parser"
	"gopkg.in/yaml.v3"
)

func TestParse(t *testing.T) {
	spec, err := openapiparser.Parse("api/spec/openapi.yml")
	if err != nil {
		t.Fatalf("cannot parse example file: %v", err)
	}
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

	s := spec.Components.Schemas.Map["School"]
	got, err := s.JSON()
	if err != nil {
		t.Fatalf("error generatiing JSON: %v", err)
	}
	if want := strings.ReplaceAll(schoolJSON, "\n", ""); got != want {
		t.Fatalf("generated json is different. Got:\n%s\n Want:\n%s\n", got, want)
	}
	got, err = s.JSONIndent("  ")
	if err != nil {
		t.Fatalf("error generatiing indented JSON: %v", err)
	}
	if got != schoolJSONIndent {
		t.Fatalf("generated indented JSON is different. Got:\n%s\n Want:\n%s\n", got, schoolJSONIndent)
	}
}

var (
	schoolJSON = `{"id":"e017d029-a459-4cfc-bf35-dd774ddf50e7",
"code":"ru-moscow-101","name":"Moscow's International Business School - \"Center\"",
"contact":{"email":"email@example.com","name":"John doe","phone":["(555)-1234566789"]},
"campus":[{"street":"Krylatskaya Ulitsa","number":22,"area-code":"FRS12-188",
"city":"Moscow","country":"RUS"}],"foundation_year":1983,"modality":"Private","open":true,
"createdAt":"2015-12-13T10:05:48+01:00","updatedAt":"2015-12-13T10:05:48+01:00"}`

	schoolJSONIndent = `{
  "id": "e017d029-a459-4cfc-bf35-dd774ddf50e7",
  "code": "ru-moscow-101",
  "name": "Moscow's International Business School - \"Center\"",
  "contact": {
    "email": "email@example.com",
    "name": "John doe",
    "phone": [
      "(555)-1234566789"
    ]
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
