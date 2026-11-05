import test from "node:test";
import assert from "node:assert";
import { PROTOCOL_VERSION, decodeRequest, encodeResponse, runPlugin } from "../src/index.js";

const SAMPLE_REQUEST = {
  protocol_version: 1,
  workspace_root: ".",
  target: { name: "sample", kind: "backend", language: "go", out_dir: "gen" },
  semantic_ir: { entities: [{ name: "Item" }] },
};

test("decodeRequest parses an envelope", () => {
  const req = decodeRequest(JSON.stringify(SAMPLE_REQUEST));
  assert.strictEqual(req.protocol_version, PROTOCOL_VERSION);
  assert.strictEqual(req.target.name, "sample");
  assert.strictEqual(req.semantic_ir.entities[0].name, "Item");
});

test("decodeRequest accepts bytes", () => {
  const req = decodeRequest(new TextEncoder().encode(JSON.stringify(SAMPLE_REQUEST)));
  assert.strictEqual(req.workspace_root, ".");
});

test("encodeResponse omits empty collections", () => {
  const payload = JSON.parse(encodeResponse({}));
  assert.strictEqual(payload.protocol_version, PROTOCOL_VERSION);
  assert.strictEqual("files" in payload, false);
  assert.strictEqual("diagnostics" in payload, false);
});

test("runPlugin writes the handler response", async () => {
  let out = "";
  const code = await runPlugin(
    () => ({ files: { "out.go": "package out" }, test_command: "go test ./..." }),
    { read: () => JSON.stringify(SAMPLE_REQUEST), write: (chunk) => { out += chunk; } },
  );
  assert.strictEqual(code, 0);
  const payload = JSON.parse(out);
  assert.strictEqual(payload.files["out.go"], "package out");
  assert.strictEqual(payload.test_command, "go test ./...");
});

test("runPlugin reports a decode error", async () => {
  let out = "";
  await runPlugin(() => ({}), { read: () => "{malformed", write: (chunk) => { out += chunk; } });
  const payload = JSON.parse(out);
  assert.strictEqual(payload.diagnostics[0].code, "FDN0701");
});

test("runPlugin reports a handler error", async () => {
  let out = "";
  await runPlugin(
    () => { throw new Error("boom"); },
    { read: () => JSON.stringify(SAMPLE_REQUEST), write: (chunk) => { out += chunk; } },
  );
  const payload = JSON.parse(out);
  assert.strictEqual(payload.diagnostics[0].code, "FDN0701");
  assert.match(payload.diagnostics[0].message, /boom/);
});
