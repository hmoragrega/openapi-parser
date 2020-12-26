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
}
