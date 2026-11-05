"""Standard input and output runner for Foundation generator plugins."""

from __future__ import annotations

import json
import sys
from typing import Any, Callable, Optional, TextIO, Union

from .protocol import Diagnostic, GenerationRequest, GenerationResponse, Span

# Readable accepts JSON bytes, JSON text, or a readable stream.
Readable = Union[bytes, bytearray, str, TextIO]
Handler = Callable[[GenerationRequest], GenerationResponse]


def decode_request(source: Readable) -> GenerationRequest:
    """Decode a GenerationRequest from JSON bytes, JSON text, or a readable stream."""
    raw: Any = source.read() if hasattr(source, "read") else source
    if isinstance(raw, (bytes, bytearray)):
        raw = raw.decode("utf-8")
    return GenerationRequest.from_dict(json.loads(raw))


def encode_response(resp: GenerationResponse) -> str:
    """Encode a GenerationResponse to a JSON string."""
    return json.dumps(resp.to_dict())


def _error_response(message: str) -> GenerationResponse:
    """Build a GenerationResponse that carries one FDN0701 diagnostic."""
    return GenerationResponse(
        diagnostics=[
            Diagnostic(
                code="FDN0701",
                severity="error",
                message=message,
                span=Span(file="protocol", line=1, column=1),
            )
        ]
    )


def run_plugin(
    handler: Handler,
    stdin: Optional[TextIO] = None,
    stdout: Optional[TextIO] = None,
) -> int:
    """Read a request, run the handler, and write the response.

    The function writes only the response document to standard output.
    Standard output stays pure for the wire protocol.
    """
    in_stream = stdin if stdin is not None else sys.stdin
    out_stream = stdout if stdout is not None else sys.stdout

    try:
        req = decode_request(in_stream)
    except (ValueError, TypeError) as err:
        out_stream.write(encode_response(_error_response(f"failed to decode generation request: {err}")))
        return 0

    try:
        resp = handler(req)
    except Exception as err:  # noqa: BLE001 - report handler failures as diagnostics
        resp = GenerationResponse()
        resp.diagnostics.append(
            Diagnostic(
                code="FDN0701",
                severity="error",
                message=str(err),
                span=Span(file="plugin", line=1, column=1),
            )
        )

    out_stream.write(encode_response(resp))
    return 0
