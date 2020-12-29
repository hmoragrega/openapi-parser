package doc_test

import (
	"bytes"
	"testing"

	openapiparser "github.com/hmoragrega/openapi-parser"
	"github.com/hmoragrega/openapi-parser/doc"
	"gopkg.in/yaml.v3"
)

func TestDoc(t *testing.T) {
	spec, err := openapiparser.Parse("../api/spec/openapi.yml")
	if err != nil {
		t.Fatalf("cannot parse example file: %v", err)
	}

	t.Run("resources", func(t *testing.T) {
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

	t.Run("enumerations", func(t *testing.T) {
		got, err := yaml.Marshal(doc.Enumerations(spec))
		if err != nil {
			t.Fatalf("failed to marshall the resources as yaml: %v", err)
		}
		if bytes.Compare(got, wantEnumerations) != 0 {
			t.Fatalf("generated yaml is different. Got:\n'%s'\n Want:\n'%s'\n", got, wantEnumerations)
		}
	})

	t.Run("endpoints by tag", func(t *testing.T) {
		tags, err := doc.EndpointsByTag(spec)
		if err != nil {
			t.Fatalf("failed to build the spec endpoints by tag: %v", err)
		}
		got, err := yaml.Marshal(tags)
		if err != nil {
			t.Fatalf("failed to marshall the resources as yaml: %v", err)
		}
		if bytes.Compare(got, wantTags) != 0 {
			t.Fatalf("generated yaml is different. Got:\n'%s'\n Want:\n'%s'\n", got, wantTags)
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

	wantTags = []byte(`- name: Schools
  endpoint:
    - summary: List schools
      description: |
        Lists the schools.
      method: get
      status: "200"
      path: /v1/schools
      requestbody: ""
      pathwithparams: /v1/schools
      response: |-
        {
          "data": [
            {
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
              "foundation_year": 1983,
              "modality": "Private",
              "open": true,
              "createdAt": "2015-12-13T10:05:48+01:00",
              "updatedAt": "2015-12-13T10:05:48+01:00"
            }
          ]
        }
    - summary: Create school
      description: |
        Create new schools.
      method: post
      status: "200"
      path: /v1/schools
      requestbody: |-
        {
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
          "foundation_year": 1983,
          "modality": "Private",
          "open": true
        }
      pathwithparams: /v1/schools
      response: |-
        {
          "data": [
            {
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
              "foundation_year": 1983,
              "modality": "Private",
              "open": true,
              "createdAt": "2015-12-13T10:05:48+01:00",
              "updatedAt": "2015-12-13T10:05:48+01:00"
            }
          ]
        }
    - summary: Get School Method
      description: |
        Retrieves a school resource using its ID.
      method: get
      status: "200"
      path: /v1/schools/{uuid}
      requestbody: ""
      pathwithparams: /v1/schools/e017d029-a459-4cfc-bf35-dd774ddf50e7
      response: |-
        {
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
          "foundation_year": 1983,
          "modality": "Private",
          "open": true,
          "createdAt": "2015-12-13T10:05:48+01:00",
          "updatedAt": "2015-12-13T10:05:48+01:00"
        }
`)
)
