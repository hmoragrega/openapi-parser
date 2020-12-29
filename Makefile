
PROJECT     ?= openapi-parser
DOCKER_HOST ?= 127.0.0.1

include ops/test.mk
include ops/docker.mk
include ops/spec.mk
