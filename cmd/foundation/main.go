package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func main() {
	os.Exit(runWithStdin(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithStdin(args, os.Stdin, stdout, stderr)
}

func runWithStdin(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}

	subcmd := args[0]
	fs := flag.NewFlagSet(subcmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", ".", "workspace directory containing .fdn Blueprints and foundation.build.yaml")
	force := fs.Bool("force", false, "overwrite manually modified manifest-tracked files")
	debug := fs.Bool("debug", false, "enable verbose diagnostic trace for language server")

	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}

	switch subcmd {
	case "lsp":
		_ = debug // Reserved for verbose trace options
		if err := compiler.LSP(stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "lsp error: %v\n", err)
			return 1
		}
		return 0

	case "check":
		_, diags := compiler.Check(*dir)
		if len(diags) > 0 {
			printDiagnostics(stderr, diags)
			return 1
		}
		fmt.Fprintln(stdout, "check passed")
		return 0

	case "build":
		_, diags := compiler.Build(*dir, compiler.BuildOptions{Force: *force})
		if len(diags) > 0 {
			printDiagnostics(stderr, diags)
			return 1
		}
		fmt.Fprintln(stdout, "build succeeded")
		return 0

	case "ir":
		data, diags := compiler.IR(*dir)
		if len(diags) > 0 {
			printDiagnostics(stderr, diags)
			return 1
		}
		_, _ = stdout.Write(data)
		return 0

	case "test":
		diags := compiler.Test(*dir)
		if len(diags) > 0 {
			printDiagnostics(stderr, diags)
			return 1
		}
		fmt.Fprintln(stdout, "tests passed")
		return 0

	default:
		fmt.Fprintf(stderr, "unknown command %q\n", subcmd)
		printUsage(stderr)
		return 1
	}
}

func printDiagnostics(w io.Writer, diags []compiler.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintf(w, "%s: [%s] %s: %s\n", d.Span.String(), d.Code, d.Severity, d.Message)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: foundation <check|build|ir|test|lsp> [--dir <path>] [--force]")
}
