package gosdk

import (
	"fmt"
	"path/filepath"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// Emitter generates a standalone, zero-external-dependency Go client SDK package.
type Emitter struct{}

func (Emitter) Emit(archIR *arch.ArchitectureIR) (emit.Emission, []ir.Diagnostic) {
	files := make(map[string][]byte)

	modName := archIR.Module
	if archIR.OutDir != "" && archIR.OutDir != "." {
		modName = filepath.ToSlash(filepath.Join(archIR.Module, archIR.OutDir))
	}

	files["go.mod"] = []byte(fmt.Sprintf(`module %s

go 1.23
`, modName))

	files["client.go"] = []byte(emitClient(archIR))
	files["types.go"] = []byte(emitTypes(archIR))
	files["actions.go"] = []byte(emitActions(archIR))
	files["errors.go"] = []byte(emitErrors(archIR))

	return emit.Emission{
		TrackedFiles: files,
		StubFiles:    make(map[string][]byte),
	}, nil
}
