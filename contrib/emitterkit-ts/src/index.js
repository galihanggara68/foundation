import { readFileSync } from "node:fs";

export const PROTOCOL_VERSION = 1;

// decodeRequest parses a GenerationRequest from JSON text or bytes.
export function decodeRequest(input) {
  const text = typeof input === "string" ? input : new TextDecoder().decode(input);
  const data = JSON.parse(text);
  return {
    ...data,
    protocol_version: data.protocol_version ?? PROTOCOL_VERSION,
    target: data.target ?? {},
    semantic_ir: data.semantic_ir ?? {},
  };
}

// encodeResponse serializes a GenerationResponse, omitting empty collections.
export function encodeResponse(resp) {
  const out = { protocol_version: resp.protocol_version ?? PROTOCOL_VERSION };
  if (resp.files && Object.keys(resp.files).length > 0) {
    out.files = resp.files;
  }
  if (resp.stubs && Object.keys(resp.stubs).length > 0) {
    out.stubs = resp.stubs;
  }
  if (resp.diagnostics && resp.diagnostics.length > 0) {
    out.diagnostics = resp.diagnostics;
  }
  if (resp.test_command) {
    out.test_command = resp.test_command;
  }
  return JSON.stringify(out);
}

// errorResponse builds a GenerationResponse that carries one FDN0701 diagnostic.
function errorResponse(message) {
  return {
    diagnostics: [
      { code: "FDN0701", severity: "error", message, span: { file: "protocol", line: 1, column: 1 } },
    ],
  };
}

// runPlugin reads a request, runs the handler, and writes the response.
// The function writes only the response document to standard output.
export async function runPlugin(handler, streams = {}) {
  const read = streams.read ?? (() => readFileSync(0, "utf8"));
  const write = streams.write ?? ((chunk) => { process.stdout.write(chunk); });

  let req;
  try {
    req = decodeRequest(read());
  } catch (err) {
    write(encodeResponse(errorResponse(`failed to decode generation request: ${err}`)));
    return 0;
  }

  let resp;
  try {
    resp = await handler(req);
  } catch (err) {
    resp = errorResponse(String(err));
  }

  write(encodeResponse(resp));
  return 0;
}
