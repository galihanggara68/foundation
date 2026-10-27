package emitterkit

import (
	"github.com/galihanggara68/foundation/pkg/compiler"
)

// EmitBuiltin invokes the Foundation compiler's built-in target emitter for archIR.
func EmitBuiltin(archIR *compiler.ArchitectureIR) (compiler.Emission, []compiler.Diagnostic) {
	return compiler.EmitBuiltin(archIR)
}

// EmitGoHexagonalScaffolding generates the standard Go Hexagonal file tree for archIR.
func EmitGoHexagonalScaffolding(archIR *compiler.ArchitectureIR) (compiler.Emission, []compiler.Diagnostic) {
	if archIR == nil {
		return compiler.Emission{}, []compiler.Diagnostic{{
			Code:     "FDN0300",
			Severity: compiler.SeverityError,
			Message:  "cannot emit Go hexagonal scaffolding for nil ArchitectureIR",
			Span:     compiler.Span{File: "foundation.build.yaml", Line: 1, Column: 1},
		}}
	}
	clone := *archIR
	clone.Kind = "backend"
	clone.Language = "go"
	clone.Pattern = "hexagonal"
	return compiler.EmitBuiltin(&clone)
}

// EmitGoDomain extracts and returns only the domain files from the Go hexagonal scaffolding for archIR.
func EmitGoDomain(archIR *compiler.ArchitectureIR) (map[string]string, []compiler.Diagnostic) {
	emission, diags := EmitGoHexagonalScaffolding(archIR)
	if len(diags) > 0 {
		return nil, diags
	}
	domainFiles := make(map[string]string)
	for path, content := range emission.TrackedFiles {
		if len(path) >= 16 && path[:16] == "internal/domain/" {
			domainFiles[path] = string(content)
		}
	}
	return domainFiles, nil
}

// ExtractFiles converts byte-slice TrackedFiles and StubFiles from compiler.Emission into string maps.
func ExtractFiles(emission compiler.Emission) (files map[string]string, stubs map[string]string) {
	files = make(map[string]string, len(emission.TrackedFiles))
	for path, content := range emission.TrackedFiles {
		files[path] = string(content)
	}
	stubs = make(map[string]string, len(emission.StubFiles))
	for path, content := range emission.StubFiles {
		stubs[path] = string(content)
	}
	return files, stubs
}

// ExportIdent converts an identifier to an exported Go identifier honoring Go initialisms (ID, SKU, URL).
func ExportIdent(s string) string {
	return compiler.ExportIdent(s)
}
