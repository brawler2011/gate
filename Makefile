.PHONY: all go-gen ts-gen dependencies go-dependencies ts-dependencies clean help list-specs

.DEFAULT_GOAL := help

# Find all OpenAPI specifications matching */v*/openapi.yaml
SPECS := $(shell find . -type f -path "*/v*/openapi.yaml" 2>/dev/null)

check-go:
	@which go > /dev/null || (echo "Error: go is not installed" && exit 1)

check-npm:
	@which npm > /dev/null || (echo "Error: npm is not installed" && exit 1)

go-dependencies: check-go go.sum go.mod
	@echo "Installing Go dependencies..."
	@go mod download

ts-dependencies: check-npm package.json
	@echo "Installing Node dependencies..."
	@npm install

dependencies: go-dependencies ts-dependencies
	@echo "Success: All dependencies installed"

# Merge blogs and core OpenAPI specs into gateway
merge-gateway: ts-dependencies
	@echo "Merging OpenAPI specifications into gateway..."
	@node tools/merge-openapi.js
	@echo "Success: Gateway OpenAPI created"

# Generate Go code for all services
go-gen: go-dependencies
	@if [ -z "$(SPECS)" ]; then \
		echo "Error: No OpenAPI specs found matching */v*/openapi.yaml"; \
		exit 1; \
	fi
	@echo "Generating Go code for all services..."
	@for spec in $(SPECS); do \
		spec_path="$$spec"; \
		dir="$${spec_path%/*}"; \
		parent="$${dir%/*}"; \
		service="$${parent##*/}"; \
		version="$${dir##*/}"; \
		pkg_name="$${service}$${version}"; \
		echo "  - Generating $$service/$$version (package: $$pkg_name)..."; \
		\
		mkdir -p "$$dir"; \
		\
		cfg_server="/tmp/cfg-server-$$service-$$version.yaml"; \
		echo "package: $$pkg_name" > "$$cfg_server"; \
		echo "generate:" >> "$$cfg_server"; \
		echo "  fiber-server: true" >> "$$cfg_server"; \
		echo "  models: true" >> "$$cfg_server"; \
		echo "output: $$dir/$$service.go" >> "$$cfg_server"; \
		go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config "$$cfg_server" "$$spec_path" || exit 1; \
		\
		cfg_client="/tmp/cfg-client-$$service-$$version.yaml"; \
		echo "package: $$pkg_name" > "$$cfg_client"; \
		echo "generate:" >> "$$cfg_client"; \
		echo "  client: true" >> "$$cfg_client"; \
		echo "  models: true" >> "$$cfg_client"; \
		echo "output: $$dir/client/$$service.go" >> "$$cfg_client"; \
		mkdir -p "$$dir/client"; \
		go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config "$$cfg_client" "$$spec_path" || exit 1; \
	done
	@echo "Success: Go code generated for all services"

# Generate TypeScript code for all services
ts-gen: ts-dependencies
	@if [ -z "$(SPECS)" ]; then \
		echo "Error: No OpenAPI specs found matching */v*/openapi.yaml"; \
		exit 1; \
	fi
	@echo "Generating TypeScript code for all services..."
	@for spec in $(SPECS); do \
		spec_path="$$spec"; \
		dir="$${spec_path%/*}"; \
		parent="$${dir%/*}"; \
		service="$${parent##*/}"; \
		version="$${dir##*/}"; \
		echo "  - Generating $$service/$$version..."; \
		npx openapi --input "$$spec_path" --output "$$dir" --client fetch --name "$$service" --useOptions || exit 1; \
	done
	@echo "Success: TypeScript code generated for all services"

all: merge-gateway go-gen ts-gen
	@echo "Success: All code generated successfully"

clean:
	@echo "Cleaning generated files..."
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
	@rm -rf gateway/
	@rm -f /tmp/cfg-server-*.yaml /tmp/cfg-client-*.yaml
	@echo "Success: Cleaned"

list-specs:
	@echo "Found OpenAPI specifications:"
	@for spec in $(SPECS); do \
		echo "  - $$spec"; \
	done

help:
	@echo "Available targets:"
	@echo ""
	@echo "  make all              - Generate both Go and TypeScript code for all services"
	@echo "  make go-gen           - Generate Go server code for all services"
	@echo "  make ts-gen           - Generate TypeScript client code for all services"
	@echo "  make dependencies     - Install all dependencies (Go + Node)"
	@echo "  make go-dependencies  - Install Go dependencies only"
	@echo "  make ts-dependencies  - Install Node dependencies only"
	@echo "  make clean            - Remove all generated files"
	@echo "  make list-specs       - List all found OpenAPI specifications"
	@echo "  make help             - Show this help message"
	@echo ""
	@echo "Generators:"
	@echo "  Go:         oapi-codegen v2.5.0"
	@echo "  TypeScript: openapi-typescript-codegen v0.29.0"
	@echo ""
	@echo "Specification format: service/version/openapi.yaml"
	@echo ""
	@echo "For more information, see README.md"