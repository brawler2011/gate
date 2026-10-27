# Contracts Generation

OpenAPI specifications and code generation for TypeScript client and Go server.

## Quick Start

### Using Task (go-task)

```bash
# Generate OpenAPI contracts (Go & TS)
task contracts:gen-openapi

# Or from repository root generate everything
task gen

# Clean generated files
task clean
```


## OpenAPI validation

Run `task contracts:lint` from the repository root. The same task runs in
`task precommit` and the mandatory CI codegen job. Vacuum is pinned to v0.33.0
in the contracts Taskfile; `vacuum.yaml` uses its recommended OpenAPI rules.
Correctness errors block changes. Existing documentation and style warnings
remain advisory, so API naming and documentation can be improved separately.
The task shows error details and totals for all severities.
