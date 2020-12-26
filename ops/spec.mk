
 ####################
 # API Spec targets #
 ####################

SPEC_UI      ?= $(PROJECT)-spec
SPEC_UI_PORT ?= 8080

.PHONY: spec spec-run spec-open spec-stop spec-merge

# Launches a UI to view the API spec
spec: spec-run spec-open

# Runs the spec UI in a docker container
spec-run:
	@if [ $(call container_is_running,${SPEC_UI}) -eq 0 ]; then \
		docker run -d --rm \
		--name ${SPEC_UI} \
		-p ${SPEC_UI_PORT} \
		-v "${PWD}/api/spec:/usr/share/nginx/html/spec:ro" \
		-e URL=/spec/openapi.yml \
		swaggerapi/swagger-ui \
	;fi

# Opens the spec UI.
spec-open:
	@$(eval port := $(call container_get_port,${SPEC_UI},${SPEC_UI_PORT}))
	open http://${DOCKER_HOST}:$(port)

# Terminates the spec UI.
spec-stop:
	@if [ $(call container_is_running,${SPEC_UI}) -eq 1 ]; then \
		docker kill ${SPEC_UI} \
	;fi
