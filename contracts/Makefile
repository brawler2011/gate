.PHONY: all go-gen ts-gen deps clean help list-specs

.DEFAULT_GOAL := all

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c

SPECS := $(shell find . -type f -path "*/v*/openapi.yaml" 2>/dev/null)

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
		out_dir="$(GO_OUT)/$$service/$$version"; \
		\
		mkdir -p "$$out_dir"; \
		\
		if [ "$$service" = "core" ]; then \
			go run $(OAPI_CODEGEN) \
				-generate std-http-server,models,strict-server,client \
				-package "$$pkg_name" \
				-o "$$out_dir/$$service.go" \
				"$$spec_path" || exit 1; \
			sed -i 's|/workshop/files/{path}"|/workshop/files/{path...}"|g' "$$out_dir/$$service.go"; \
		elif [ "$$service" = "gateway" ]; then \
			:; \
		else \
			go run $(OAPI_CODEGEN) \
				-generate std-http-server,models,strict-server \
				-package "$$pkg_name" \
				-o "$$out_dir/$$service.go" \
				"$$spec_path" || exit 1; \
		fi; \
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
		out_dir="$(TS_OUT)/$$service/$$version"; \
		\
		bunx openapi-typescript-codegen --input "$$spec_path" --output "$$out_dir" --client fetch --name "$$service" --useOptions || exit 1; \
	done

all: go-gen ts-gen

clean:
	@for spec in $(SPECS); do \
		spec_path="$$spec"; \
		dir="$${spec_path%/*}"; \
		parent="$${dir%/*}"; \
		service="$${parent##*/}"; \
		version="$${dir##*/}"; \
		rm -rf \
			"$(GO_OUT)/$$service/$$version/$$service.go" \
			"$(TS_OUT)/$$service/$$version"; \
	done
