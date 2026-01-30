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

### Windows (Using PowerShell)

```powershell
# Generate all code
.\generate.ps1 all

# Or use specific targets
.\generate.ps1 ts-gen          # TypeScript client only
.\generate.ps1 go-gen          # Go server only
.\generate.ps1 merge-gateway   # Merge gateway specifications
.\generate.ps1 deps            # Check dependencies
.\generate.ps1 clean           # Remove generated files
.\generate.ps1 help            # Show help
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

## Contracts Structure

### OpenAPI Schema (`core/v1/openapi.yaml`)

Contains all API and WebSocket event type definitions:

#### REST API Models

- Standard REST endpoints for problems, contests, submissions, users
- Request/Response models with full validation

#### WebSocket Event Models

- `TestingStartedEventModel` - Emitted when testing starts
- `TestCompletedEventModel` - Emitted after each test completes
- `TestingCompletedEventModel` - Emitted when testing finishes
- `TestProgressEventType` - Enum for event type discrimination

See [WEBSOCKET_API documentation](../docs/WEBSOCKET_API.md) for complete WebSocket API reference.

## Usage

### TypeScript Client

```typescript
import type { Problem, GetProblemResponse } from "../../contracts/core/v1";

const response: GetProblemResponse = await client.default.getProblem({
  id: problemId,
});
```

### WebSocket Event Types

After generation, WebSocket events can be imported and used:

```typescript
import type {
  TestingStartedEventModel,
  TestCompletedEventModel,
  TestingCompletedEventModel,
} from "../../contracts/core/v1";

const ws = new WebSocket("/ws/submissions?ids=uuid1,uuid2");

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);

  switch (data.type) {
    case "testing_started": {
      const event: TestingStartedEventModel = data;
      // Handle testing started
      break;
    }
    case "test_completed": {
      const event: TestCompletedEventModel = data;
      // Handle test completed
      break;
    }
    case "testing_completed": {
      const event: TestingCompletedEventModel = data;
      // Handle testing completed
      break;
    }
  }
};
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
