# Gate frontend

Use `bun install`, `bun run dev`, `bun run test`, `bun run typecheck`,
`bun run lint`, `bun run knip` and `bun run build:local`.

Required application URLs are documented in `.env.example`. Browser and server
OpenTelemetry SDK registration/export are removed; no OTel environment is needed.
Ordinary Next.js server errors and safe API-operation console errors remain.
`ApiResult.requestId` links backend error responses to JSON logs.
`NEXT_TELEMETRY_DISABLED=1` in Docker disables Next.js's own telemetry.

Local frontend stdout is terminal output, not Docker logs.
`next.config.mjs` and `next-env.d.ts` are protected.
