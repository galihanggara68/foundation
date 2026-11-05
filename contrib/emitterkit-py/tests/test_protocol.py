"""Unit tests for the emitterkit Python client."""

import io
import json
import unittest

import emitterkit
from emitterkit import (
    GenerationRequest,
    GenerationResponse,
    TargetInfo,
    decode_request,
    encode_response,
    run_plugin,
)

SAMPLE_REQUEST = {
    "protocol_version": 1,
    "workspace_root": ".",
    "target": {"name": "sample", "kind": "backend", "language": "go", "out_dir": "gen"},
    "semantic_ir": {"entities": [{"name": "Item"}]},
}


class ProtocolTests(unittest.TestCase):
    def test_decode_request(self):
        req = decode_request(json.dumps(SAMPLE_REQUEST))
        self.assertEqual(req.protocol_version, 1)
        self.assertEqual(req.target.name, "sample")
        self.assertEqual(req.semantic_ir["entities"][0]["name"], "Item")

    def test_decode_request_accepts_bytes(self):
        req = decode_request(json.dumps(SAMPLE_REQUEST).encode("utf-8"))
        self.assertEqual(req.workspace_root, ".")

    def test_response_omits_empty_collections(self):
        payload = json.loads(encode_response(GenerationResponse()))
        self.assertEqual(payload["protocol_version"], emitterkit.PROTOCOL_VERSION)
        self.assertNotIn("files", payload)
        self.assertNotIn("stubs", payload)
        self.assertNotIn("diagnostics", payload)

    def test_target_roundtrip_preserves_options(self):
        info = TargetInfo.from_dict({"name": "t", "kind": "backend", "language": "go", "out_dir": "gen", "options": {"k": "v"}})
        self.assertEqual(info.to_dict()["options"], {"k": "v"})

    def test_run_plugin_writes_response(self):
        def handler(req: GenerationRequest) -> GenerationResponse:
            return GenerationResponse(files={"out.go": "package out"}, test_command="go test ./...")

        stdout = io.StringIO()
        code = run_plugin(handler, stdin=io.StringIO(json.dumps(SAMPLE_REQUEST)), stdout=stdout)
        self.assertEqual(code, 0)
        payload = json.loads(stdout.getvalue())
        self.assertEqual(payload["files"]["out.go"], "package out")
        self.assertEqual(payload["test_command"], "go test ./...")

    def test_run_plugin_reports_decode_error(self):
        stdout = io.StringIO()
        run_plugin(lambda req: GenerationResponse(), stdin=io.StringIO("{malformed"), stdout=stdout)
        payload = json.loads(stdout.getvalue())
        self.assertEqual(payload["diagnostics"][0]["code"], "FDN0701")

    def test_run_plugin_reports_handler_error(self):
        def handler(req: GenerationRequest) -> GenerationResponse:
            raise RuntimeError("boom")

        stdout = io.StringIO()
        run_plugin(handler, stdin=io.StringIO(json.dumps(SAMPLE_REQUEST)), stdout=stdout)
        payload = json.loads(stdout.getvalue())
        self.assertEqual(payload["diagnostics"][0]["code"], "FDN0701")
        self.assertIn("boom", payload["diagnostics"][0]["message"])


if __name__ == "__main__":
    unittest.main()
