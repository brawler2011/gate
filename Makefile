.PHONY: all go-gen ts-gen dependencies go-dependencies ts-dependencies clean help list-specs

.DEFAULT_GOAL := all

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c

SPECS := $(shell find . -type f -path "*/v*/openapi.yaml" 2>/dev/null)

GO   ?= go
NPM  ?= npm
NPX  ?= npx
NODE ?= node

OAPI_CODEGEN = github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen

deps: go.sum go.mod package.json
	@command -v "$(GO)"   >/dev/null || { echo "Error: go is not installed";  exit 1; }
	@command -v "$(NPM)"  >/dev/null || { echo "Error: npm is not installed"; exit 1; }
	@command -v "$(NPX)"  >/dev/null || { echo "Error: npx is not installed"; exit 1; }
	@command -v "$(NODE)" >/dev/null || { echo "Error: node is not installed"; exit 1; }

merge-gateway: deps
	@node tools/merge-openapi.js

go-gen: deps
	@if [ -z "$(SPECS)" ]; then \
		echo "Error: No OpenAPI specs found matching */v*/openapi.yaml"; \
		exit 1; \
	fi
	@for spec in $(SPECS); do \
		spec_path="$$spec"; \
		dir="$${spec_path%/*}"; \
		parent="$${dir%/*}"; \
		service="$${parent##*/}"; \
		version="$${dir##*/}"; \
		pkg_name="$${service}$${version}"; \
		\
		mkdir -p "$$dir"; \
		\
		go run $(OAPI_CODEGEN) \
			-generate fiber-server,models \
			-package "$$pkg_name" \
			-o "$$dir/$$service.go" \
			"$$spec_path" || exit 1; \
		\
		mkdir -p "$$dir/client"; \
		go run $(OAPI_CODEGEN) \
			-generate client,models \
			-package "$$pkg_name" \
			-o "$$dir/client/$$service.go" \
			"$$spec_path" || exit 1; \
	done

ts-gen: deps
	@if [ -z "$(SPECS)" ]; then \
		echo "Error: No OpenAPI specs found matching */v*/openapi.yaml"; \
		exit 1; \
	fi
	@for spec in $(SPECS); do \
		spec_path="$$spec"; \
		dir="$${spec_path%/*}"; \
		parent="$${dir%/*}"; \
		service="$${parent##*/}"; \
		version="$${dir##*/}"; \
		npx openapi --input "$$spec_path" --output "$$dir" --client fetch --name "$$service" --useOptions || exit 1; \
	done

all: merge-gateway go-gen ts-gen

clean:
	@for spec in $(SPECS); do \
		spec_path="$$spec"; \
		dir="$${spec_path%/*}"; \
		parent="$${dir%/*}"; \
		service="$${parent##*/}"; \
		rm -rf "$$dir/$$service.go" \
			"$$dir/client/" \
			"$$dir"/*.ts \
			"$$dir/models" \
			"$$dir/services" \
			"$$dir/core"; \
	done