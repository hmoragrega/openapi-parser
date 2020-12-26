# OpenAPI Parser

**Disclaimer**: Work in progress.

Sounds good, but this is not probably what you are looking for,
this library parses **only** the `openapi` spec files that
_I_ write, not the full specification. *sorry not sorry*

## How to use
```
spec, err := openapi.Parse("api/spec")
```

### Spec
The spec object contains everything this information from the spec  
- info metadata
- paths
- schemas
- responses
- parameters

## Limitations

### Only YAML
It does only parse file in YAML format.

### Composition and inheritance.
Both are not supported, so these keys in schemas will trigger errors 
- discriminator
- allOf
- oneOf
- anyOf
- not

### Circular dependencies
Circular dependencies between schema files will trigger an error

### Path definitions 
It only supports file references, not inlined path definitions

Supported
```yaml
paths:
  /foo:
    $ref: "paths/foo.yml"
```
Unsupported
```yaml
paths:
  /foo:
    get:
      summary: Get Foo
      # ... continues path definition
```
### Parameters definitions,
Does not support path references in the root spec file.  

Supported
```yaml
components:
  parameters:
    Foo:
      in: query
      type: string
      # ... continues parameter
```
Unsupported
```yaml
components:
  parameters:
    Foo:
      $ref: "parameters/foo.yml"
```

### Local schemas
Does not support local definitions for other files other than the root spec file

For example, given this definition in the root file `openapi.yml`
```yaml
components:
  schemas:
    Foo:
      $ref: "schemas/foo.yml"
```
Supported
```
$ref: "../path/openapi.yml#/components/schemas/Foo"
``` 
Unsupported
```
$ref: "../path/anotherfile.yml#/components/schemas/Foo"
``` 

### Additional Properties.
The parser will trigger an error is key `additionalProperties` is defined.

### Response headers references
Does not support reference is response headers.

## Caveats

### Example and default
Does not parse `default` or `example` keys in the object schema, bot remain "as is" in a string value.  

Examples: 
```
default: foo => "foo"
```
```
default: 1 => "1"
```
With this example
```
example:
 foo: bar
 num: 1
```
You'll get it as a string, including newline characters:
```
foo: var
num: 1
```
