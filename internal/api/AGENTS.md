# API

The API package is the transport boundary: Connect handlers, streaming endpoints, websocket fanout, and project routing.

## Owns

- request validation and error mapping
- proto conversion
- routing a request to the correct project/global backend
- streaming APIs for transcripts, events, and live state

## Does Not Own

- workflow execution policy
- SQL details
- provider-specific runtime behavior

## Rules

- Keep handlers thin. Delegate business logic to the right package.
- Project-scoped calls must resolve the correct backend; do not fall back silently.
- If a proto field is renamed or retyped, update handler code, generated types, web consumers, and tests together.
- Streaming endpoints must preserve contract semantics. Do not rename fields without updating every producer and consumer path.

## When Changing This Package

- For new API fields, verify storage, proto conversion, and frontend usage all line up.
- For project routing changes, test both explicit `project_id` and default-project behavior.
- For transcript/session changes, verify both REST/Connect responses and live streaming paths.

## Verification

```bash
go test ./internal/api/...
pnpm -C web exec tsc --noEmit
```
