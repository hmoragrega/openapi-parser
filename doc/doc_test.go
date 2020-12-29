package doc_test

import (
	"bytes"
	"testing"

	openapiparser "github.com/hmoragrega/openapi-parser"
	"github.com/hmoragrega/openapi-parser/doc"
	"gopkg.in/yaml.v3"
)

func TestResources(t *testing.T) {
	spec, err := openapiparser.Parse("../api/spec/openapi.yml")
	if err != nil {
		t.Fatalf("cannot parse example file: %v", err)
	}

	t.Run("build resources", func(t *testing.T) {
		res, err := doc.Resources(spec)
		if err != nil {
			t.Fatalf("failed to build the spec resources: %v", err)
		}
		got, err := yaml.Marshal(res[0].Fields)
		if err != nil {
			t.Fatalf("failed to marshall the resources as yaml: %v", err)
		}
		if bytes.Compare(got, wantResources) != 0 {
			t.Fatalf("generated yaml is different. Got:\n'%s'\n Want:\n'%s'\n", got, wantResources)
		}
	})

	t.Run("build enumerations", func(t *testing.T) {
		got, err := yaml.Marshal(doc.Enumerations(spec))
		if err != nil {
			t.Fatalf("failed to marshall the resources as yaml: %v", err)
		}
		if bytes.Compare(got, wantEnumerations) != 0 {
			t.Fatalf("generated yaml is different. Got:\n'%s'\n Want:\n'%s'\n", got, wantEnumerations)
		}
	})
}

var (
	wantResources = []byte(`- key: id
  type: String
  link: ""
  description: Unique ID of the office (UUID v4).
- key: code
  type: String
  link: ""
  description: Unique code of the school (URL safe).
- key: name
  type: String
  link: ""
  description: |-
    Name of the school
    Constraints:
    - Minimum length: 3.
    - Maximum length: 100.
    - Accepts alphanumeric characters, dashes, dots and spaces.
- key: contacts
  type: Array
  link: ""
  description: Contact persons list.
- key: contacts.#
  type: Object
  link: ""
  description: Person contact information.
- key: contacts.#.email
  type: String
  link: ""
  description: Contact email.
- key: contacts.#.position
  type: String
  link: ""
  description: Contact person's position.
- key: contacts.#.name
  type: String
  link: ""
  description: Contact person's name.
- key: contacts.#.priority
  type: String
  link: ""
  description: ""
- key: contacts.#.phone
  type: Array
  link: ""
  description: Phone numbers.
- key: main_office
  type: Object
  link: Address
  description: Physical address
- key: campus
  type: Array
  link: ""
  description: Campus locations.
- key: campus.#
  type: Object
  link: Address
  description: Physical address
- key: foundation_year
  type: Number
  link: ""
  description: Year of foundation.
- key: modality
  type: String
  link: ""
  description: The school education mode.
- key: open
  type: Boolean
  link: ""
  description: Indicates whether the schools is accepting new schoolars.
- key: createdAt
  type: String
  link: ""
  description: The date where the record was created.
- key: updatedAt
  type: String
  link: ""
  description: The last time the record was updated.
`)
	wantEnumerations = []byte(`- title: School Contacts Priority
  resource: School
  options:
    - value: Primary
      description: ""
    - value: Secondary
      description: ""
- title: School Modality
  resource: School
  options:
    - value: Private
      description: private school.
    - value: Public
      description: public school.
    - value: Mixed
      description: public and private.
`)
)
