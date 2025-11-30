# Contracts Generation

OpenAPI specifications and code generation for TypeScript client and Go server.

## Quick Start

### Using Make (recommended)

```bash
# Generate all code
make all

# Or use specific targets
make ts-gen          # TypeScript client only
make go-gen          # Go server only
make dependencies    # Install all dependencies
make clean          # Remove generated files
```

### Using npm

```bash
npm install          # Install dependencies
npm run gen          # Generate TypeScript client only
```

## Generators

### Go Server

- Tool: [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) v2.5.0
- Output: Fiber server interfaces and models
- Config: `cfg.yaml`

### TypeScript Client

- Tool: [openapi-typescript-codegen](https://github.com/ferdikoomen/openapi-typescript-codegen) v0.29.0
- Output: Fetch-based client with TypeScript types
- Compatible with Next.js API routes

## Usage

### TypeScript Client

```typescript
import type { Problem, GetProblemResponse } from "../../contracts/core/v1";

const response: GetProblemResponse = await client.default.getProblem({
  id: problemId,
});
```

### Go Server

```go
import corev1 "github.com/gate149/contracts/core/v1"

// Implement ServerInterface
type Handlers struct {
    // your dependencies
}

func (h *Handlers) UpdateProblem(c *fiber.Ctx, id openapi_types.UUID) error {
    var req corev1.UpdateProblemRequest
    if err := c.BodyParser(&req); err != nil {
        return err
    }
    // your logic
    return c.SendStatus(fiber.StatusOK)
}
// Register handlers 
corev1.RegisterHandlers(app, handlers)
```