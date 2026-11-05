# emitterkit-ts

A zero-dependency reference client for the Foundation generation protocol.
TypeScript plugin authors use the package to decode requests and to encode responses.

## Usage

```ts
import { runPlugin } from "@foundation-contrib/emitterkit";

await runPlugin((req) => ({
  files: { "generated.ts": "// generated\n" },
  test_command: "npm test",
}));
```

The compiler sends a `GenerationRequest` on standard input.
The plugin writes a `GenerationResponse` on standard output.
`runPlugin` keeps standard output pure for the wire protocol.

## API

- `decodeRequest(input)` parses a `GenerationRequest` from JSON text or bytes.
- `encodeResponse(resp)` serializes a `GenerationResponse`, omitting empty collections.
- `runPlugin(handler, streams?)` reads a request, runs the handler, and writes the response.

The published JSON Schema is at [`docs/schemas/generation-protocol-v1.json`](../../docs/schemas/generation-protocol-v1.json).

## Test

```bash
npm test
```
