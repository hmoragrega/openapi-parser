
 ################
 # TEST targets #
 ################

.PHONY: test

test:
	go test -v -race ./...
