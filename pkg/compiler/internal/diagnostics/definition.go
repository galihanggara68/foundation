package diagnostics

import "github.com/galihanggara68/foundation/pkg/compiler/internal/ir"

// Definition holds metadata for a compiler diagnostic code.
type Definition struct {
	Code            string
	Severity        ir.Severity
	Category        string
	MessageTemplate string
	Description     string
	DocAnchor       string
}
