# Contracts Generation

OpenAPI specifications and code generation for TypeScript client and Go server.

## Quick Start

### Linux/macOS (Using Make)

```bash
# Generate all code
make all

# Or use specific targets
make ts-gen          # TypeScript client only
make go-gen          # Go server only
make merge-gateway   # Merge gateway specifications
make deps            # Check dependencies
make clean           # Remove generated files
```

