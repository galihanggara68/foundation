package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// runCapture runs the capture subcommand.
func runCapture(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", ".", "workspace directory containing .fdn Blueprints and foundation.build.yaml")
	target := fs.String("target", "", "name of target in foundation.build.yaml")
	out := fs.String("out", "", "output file path for captured request envelope")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	payload, diags := compiler.CaptureRequest(*dir, *target)
	if compiler.HasErrors(diags) {
		printDiagnostics(stderr, diags)
		return 1
	}
	if len(diags) > 0 {
		printDiagnostics(stderr, diags)
	}

	if *out != "" {
		outDir := filepath.Dir(*out)
		if outDir != "" && outDir != "." {
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				fmt.Fprintf(stderr, "failed to create directory: %v\n", err)
				return 1
			}
		}
		if err := os.WriteFile(*out, payload, 0o644); err != nil {
			fmt.Fprintf(stderr, "failed to write output file: %v\n", err)
			return 1
		}
		return 0
	}

	if _, err := stdout.Write(payload); err != nil {
		fmt.Fprintf(stderr, "failed to write payload: %v\n", err)
		return 1
	}
	return 0
}
