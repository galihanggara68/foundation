"""Reference Python client for the Foundation generation protocol."""

from .protocol import (
    PROTOCOL_VERSION,
    Diagnostic,
    GenerationRequest,
    GenerationResponse,
    Span,
    TargetInfo,
)
from .runner import decode_request, encode_response, run_plugin

__all__ = [
    "PROTOCOL_VERSION",
    "Diagnostic",
    "GenerationRequest",
    "GenerationResponse",
    "Span",
    "TargetInfo",
    "decode_request",
    "encode_response",
    "run_plugin",
]
