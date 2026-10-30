package compiler

import (
	"context"
	"fmt"
	"sort"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/plugin"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

type pipelineEmission struct {
	Emission    emit.Emission
	TestCommand string
}

// executeGeneratorPipeline runs a chain of external generator plugins in sequence,
// passing virtual file maps forward and tracking emitter provenance for collision detection.
func executeGeneratorPipeline(
	ctx context.Context,
	workspaceDir string,
	target arch.TargetConfig,
	sem *ir.SemanticIR,
	archIR *arch.ArchitectureIR,
	builtinEmission *emit.Emission,
) (*pipelineEmission, []Diagnostic) {
	virtualFiles := make(map[string]string)
	virtualStubs := make(map[string]string)
	fileProvenance := make(map[string]string)

	if builtinEmission != nil {
		for p, content := range builtinEmission.TrackedFiles {
			virtualFiles[p] = string(content)
			fileProvenance[p] = "builtin"
		}
		for p, content := range builtinEmission.StubFiles {
			virtualStubs[p] = string(content)
		}
	}

	var allDiags []Diagnostic
	var lastTestCommand string

	runner := plugin.NewRunner(plugin.RunnerConfig{
		Timeout: plugin.DefaultTimeout,
	})

	for _, genName := range target.EffectiveGenerators() {
		pluginPath, err := plugin.Resolve(workspaceDir, genName)
		if err != nil {
			allDiags = append(allDiags, Diagnostic{
				Code:     "FDN0701",
				Severity: SeverityError,
				Message:  fmt.Sprintf("failed to resolve generator plugin %q: %v", genName, err),
				Span:     Span{File: "foundation.build.yaml", Line: 1, Column: 1},
			})
			return nil, allDiags
		}

		// Snapshot current virtual file state for plugin input
		reqFiles := make(map[string]string, len(virtualFiles))
		for k, v := range virtualFiles {
			reqFiles[k] = v
		}

		platformInfo := platform.DefaultInfo()
		req := protocol.GenerationRequest{
			ProtocolVersion: protocol.ProtocolVersion,
			WorkspaceRoot:   workspaceDir,
			Target:          protocol.TargetInfoFromConfig(target, target.Options),
			SemanticIR:      sem,
			ArchitectureIR:  archIR,
			Platform:        &platformInfo,
			Files:           reqFiles,
		}

		resp, execDiags := runner.Execute(ctx, pluginPath, req)
		allDiags = append(allDiags, execDiags...)
		if hasErrorDiagnostic(execDiags) {
			return nil, allDiags
		}

		// Surface plugin-emitted diagnostics
		allDiags = append(allDiags, resp.Diagnostics...)
		if resp.HasErrors() {
			return nil, allDiags
		}

		if resp.TestCommand != "" {
			lastTestCommand = resp.TestCommand
		}

		// Deterministic iteration for merging files with collision detection (FDN0702)
		fileKeys := make([]string, 0, len(resp.Files))
		for k := range resp.Files {
			fileKeys = append(fileKeys, k)
		}
		sort.Strings(fileKeys)

		for _, relPath := range fileKeys {
			content := resp.Files[relPath]
			prevOwner, exists := fileProvenance[relPath]
			// Warn on collision only when a downstream plugin mutates a file previously emitted
			// by an upstream plugin (not intentional builtin scaffolding override or unchanged pass-through)
			if exists && prevOwner != "builtin" && prevOwner != genName && content != virtualFiles[relPath] {
				allDiags = append(allDiags, Diagnostic{
					Code:     "FDN0702",
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("generator %q overwrote file %q previously emitted by %q", genName, relPath, prevOwner),
					Span:     Span{File: "foundation.build.yaml", Line: 1, Column: 1},
				})
			}
			if !exists || content != virtualFiles[relPath] {
				fileProvenance[relPath] = genName
			}
			virtualFiles[relPath] = content
		}

		// Deterministic iteration for merging create-if-missing stub files
		stubKeys := make([]string, 0, len(resp.Stubs))
		for k := range resp.Stubs {
			stubKeys = append(stubKeys, k)
		}
		sort.Strings(stubKeys)

		for _, relPath := range stubKeys {
			virtualStubs[relPath] = resp.Stubs[relPath]
		}
	}

	tracked := make(map[string][]byte, len(virtualFiles))
	for p, content := range virtualFiles {
		tracked[p] = []byte(content)
	}

	stubs := make(map[string][]byte, len(virtualStubs))
	for p, content := range virtualStubs {
		stubs[p] = []byte(content)
	}

	return &pipelineEmission{
		Emission: emit.Emission{
			TrackedFiles: tracked,
			StubFiles:    stubs,
			TestCommand:  lastTestCommand,
		},
		TestCommand: lastTestCommand,
	}, allDiags
}

func hasErrorDiagnostic(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}
