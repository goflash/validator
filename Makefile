PKGS := $(shell go list ./... | grep -v "/examples/")

.PHONY: test
test:
	go test $(PKGS) -v -race

.PHONY: cover
cover:
	COVERPKG := $(shell echo $(PKGS) | tr ' ' ',')
	go test $(PKGS) -v -race -covermode=atomic -coverpkg=$(COVERPKG) -coverprofile=coverage.out
	@echo "\nCoverage summary:" && go tool cover -func=coverage.out | tail -n 1
