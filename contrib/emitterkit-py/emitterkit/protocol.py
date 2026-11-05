"""Wire protocol types for the Foundation generation protocol (version 1).

The compiler sends a GenerationRequest on standard input.
A plugin returns a GenerationResponse on standard output.
Every field uses snake_case to match the wire JSON.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Optional

# PROTOCOL_VERSION is the protocol version this client speaks.
PROTOCOL_VERSION = 1


@dataclass
class Span:
    """Source location of a declaration or expression."""

    file: str = ""
    line: int = 0
    column: int = 0

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Span":
        return cls(file=data.get("file", ""), line=data.get("line", 0), column=data.get("column", 0))

    def to_dict(self) -> dict[str, Any]:
        return {"file": self.file, "line": self.line, "column": self.column}


@dataclass
class Diagnostic:
    """One compilation diagnostic emitted by a plugin."""

    code: str
    severity: str
    message: str
    span: Span = field(default_factory=Span)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Diagnostic":
        return cls(
            code=data.get("code", ""),
            severity=data.get("severity", "error"),
            message=data.get("message", ""),
            span=Span.from_dict(data.get("span") or {}),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "code": self.code,
            "severity": self.severity,
            "message": self.message,
            "span": self.span.to_dict(),
        }


@dataclass
class TargetInfo:
    """Target configuration the compiler sends to the plugin."""

    name: str = ""
    kind: str = ""
    language: str = ""
    out_dir: str = ""
    architecture: str = ""
    http: str = ""
    options: dict[str, str] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "TargetInfo":
        return cls(
            name=data.get("name", ""),
            kind=data.get("kind", ""),
            language=data.get("language", ""),
            out_dir=data.get("out_dir", ""),
            architecture=data.get("architecture", ""),
            http=data.get("http", ""),
            options=dict(data.get("options") or {}),
        )

    def to_dict(self) -> dict[str, Any]:
        out: dict[str, Any] = {
            "name": self.name,
            "kind": self.kind,
            "language": self.language,
            "out_dir": self.out_dir,
        }
        if self.architecture:
            out["architecture"] = self.architecture
        if self.http:
            out["http"] = self.http
        if self.options:
            out["options"] = dict(self.options)
        return out


@dataclass
class GenerationRequest:
    """Payload the compiler sends to an external generator plugin."""

    protocol_version: int = PROTOCOL_VERSION
    workspace_root: str = ""
    target: TargetInfo = field(default_factory=TargetInfo)
    semantic_ir: dict[str, Any] = field(default_factory=dict)
    architecture_ir: Optional[dict[str, Any]] = None
    platform: Optional[dict[str, Any]] = None
    files: dict[str, str] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "GenerationRequest":
        return cls(
            protocol_version=data.get("protocol_version", PROTOCOL_VERSION),
            workspace_root=data.get("workspace_root", ""),
            target=TargetInfo.from_dict(data.get("target") or {}),
            semantic_ir=dict(data.get("semantic_ir") or {}),
            architecture_ir=data.get("architecture_ir"),
            platform=data.get("platform"),
            files=dict(data.get("files") or {}),
        )

    def to_dict(self) -> dict[str, Any]:
        out: dict[str, Any] = {
            "protocol_version": self.protocol_version,
            "workspace_root": self.workspace_root,
            "target": self.target.to_dict(),
            "semantic_ir": self.semantic_ir,
        }
        if self.architecture_ir is not None:
            out["architecture_ir"] = self.architecture_ir
        if self.platform is not None:
            out["platform"] = self.platform
        if self.files:
            out["files"] = dict(self.files)
        return out


@dataclass
class GenerationResponse:
    """Payload a plugin returns to the compiler."""

    files: dict[str, str] = field(default_factory=dict)
    stubs: dict[str, str] = field(default_factory=dict)
    diagnostics: list[Diagnostic] = field(default_factory=list)
    test_command: str = ""
    protocol_version: int = PROTOCOL_VERSION

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "GenerationResponse":
        return cls(
            files=dict(data.get("files") or {}),
            stubs=dict(data.get("stubs") or {}),
            diagnostics=[Diagnostic.from_dict(d) for d in data.get("diagnostics") or []],
            test_command=data.get("test_command", ""),
            protocol_version=data.get("protocol_version", PROTOCOL_VERSION),
        )

    def to_dict(self) -> dict[str, Any]:
        out: dict[str, Any] = {"protocol_version": self.protocol_version}
        if self.files:
            out["files"] = dict(self.files)
        if self.stubs:
            out["stubs"] = dict(self.stubs)
        if self.diagnostics:
            out["diagnostics"] = [d.to_dict() for d in self.diagnostics]
        if self.test_command:
            out["test_command"] = self.test_command
        return out
