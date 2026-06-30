.PHONY: all go-gen ts-gen deps clean

.DEFAULT_GOAL := all

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c

GO   ?= go
NPM  ?= npm
NPX  ?= npx
NODE ?= node

OAPI_CODEGEN = github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen

GO_OUT := ../backend/contracts
TS_OUT := ../frontend/contracts

deps: go.sum go.mod package.json
	@command -v "$(GO)"   >/dev/null || { echo "Error: go is not installed";  exit 1; }
	@command -v "$(NPM)"  >/dev/null || { echo "Error: npm is not installed"; exit 1; }
	@command -v "$(NPX)"  >/dev/null || { echo "Error: npx is not installed"; exit 1; }
	@command -v "$(NODE)" >/dev/null || { echo "Error: node is not installed"; exit 1; }

go-gen: deps
	mkdir -p $(GO_OUT)/core/v1
	go run $(OAPI_CODEGEN) \
		-generate std-http-server,models,strict-server,client \
		-package corev1 \
		-o $(GO_OUT)/core/v1/core.go \
		core/v1/openapi.yaml
	sed -i 's|/workshop/files/{path}"|/workshop/files/{path...}"|g' $(GO_OUT)/core/v1/core.go

	mkdir -p $(GO_OUT)/observer/v1
	go run $(OAPI_CODEGEN) \
		-generate std-http-server,models,strict-server,skip-prune \
		-package observerv1 \
		-o $(GO_OUT)/observer/v1/observer.go \
		observer/v1/openapi.yaml

ts-gen: deps
	rm -rf $(TS_OUT)/core/v1
	mkdir -p $(TS_OUT)/core/v1
	bunx openapi-typescript-codegen --input core/v1/openapi.yaml --output $(TS_OUT)/core/v1 --client fetch --name core --useOptions

	rm -rf $(TS_OUT)/observer/v1
	mkdir -p $(TS_OUT)/observer/v1
	bunx openapi-typescript-codegen --input observer/v1/openapi.yaml --output $(TS_OUT)/observer/v1 --client fetch --name observer --useOptions

all: go-gen ts-gen

clean:
	rm -rf $(GO_OUT)/core/v1 $(GO_OUT)/observer/v1
	rm -rf $(TS_OUT)/core/v1 $(TS_OUT)/observer/v1

