package compiler

import (
	"io"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/lsp"
)

// LSP runs the Foundation Language Server listening on in and writing to out.
// It serves as the top-level driver seam for IDE and editor integrations.
func LSP(in io.Reader, out io.Writer) error {
	srv := lsp.NewServer(in, out, func(sources []lsp.SourceDoc) ([]ir.Diagnostic, error) {
		var compilerSources []SourceFile
		for _, s := range sources {
			compilerSources = append(compilerSources, SourceFile{
				Path:    s.Path,
				Content: s.Content,
			})
		}
		_, diags := Compile(compilerSources)
		return diags, nil
	})
	return srv.Run()
}

// ServeLSP is an alias to LSP preserving semantic compatibility.
func ServeLSP(in io.Reader, out io.Writer) error {
	return LSP(in, out)
}
