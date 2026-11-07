# Gate

## Installation / Prerequisites

- [Task Installation](https://taskfile.dev/docs/installation)

## API request limits

The API limits request bodies before decoding, including chunked requests.
Oversized bodies return HTTP 413. Ordinary requests, including login, allow
1 MiB. Upload limits depend on the operation, independently of Content-Type:

| Upload | Maximum body size |
| --- | --- |
| Workshop source files | 2 MiB |
| Workshop media | 50 MiB |
| Workshop test files | 256 MiB |
| Problem package imports | 500 MiB + 1 MiB multipart overhead |
| Avatars and post forms | 50 MiB + 1 MiB multipart overhead |

Imports also enforce the 500 MiB file limit. Protected operations check the
session and, where required, administrator role before decoding. Resource
permissions are checked using decoded operation parameters. A declared body
size above the limit is rejected before these access checks; unknown-length
unauthorized requests are rejected without reading their bodies.
