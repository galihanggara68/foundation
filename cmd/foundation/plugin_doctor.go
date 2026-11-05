package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/galihanggara68/foundation/cmd/foundation/internal/doctor"
)

// pluginDoctorUsage documents the plugin doctor command line.
const pluginDoctorUsage = "Usage: foundation plugin doctor <binary-path> [--verbose]"

// runPluginDoctor runs the plugin doctor command line tool.
func runPluginDoctor(args []string, stdout, stderr io.Writer) int {
	var binaryPath string
	var verbose bool

	for _, arg := range args {
		switch arg {
		case "-v", "-verbose", "--verbose":
			verbose = true
		case "-h", "-help", "--help":
			fmt.Fprintln(stderr, pluginDoctorUsage)
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "unknown flag %q\n", arg)
				fmt.Fprintln(stderr, pluginDoctorUsage)
				return 1
			}
			if binaryPath == "" {
				binaryPath = arg
			} else {
				fmt.Fprintf(stderr, "unexpected argument %q\n", arg)
				fmt.Fprintln(stderr, pluginDoctorUsage)
				return 1
			}
		}
	}

	if binaryPath == "" {
		fmt.Fprintln(stderr, pluginDoctorUsage)
		return 1
	}

	opts := doctor.DoctorOptions{
		Verbose: verbose,
	}

	ctx := context.Background()
	report, err := doctor.Run(ctx, binaryPath, opts)
	if err != nil {
		fmt.Fprintf(stderr, "doctor error: %v\n", err)
		return 1
	}

	for _, probe := range report.Probes {
		if probe.Passed {
			if verbose && probe.Message != "" {
				fmt.Fprintf(stdout, "[PASS] %s: %s\n", probe.Name, probe.Message)
			} else {
				fmt.Fprintf(stdout, "[PASS] %s\n", probe.Name)
			}
		} else {
			if probe.DiagnosticCode != "" {
				fmt.Fprintf(stdout, "[FAIL] %s: [%s] %s\n", probe.Name, probe.DiagnosticCode, probe.Message)
			} else {
				fmt.Fprintf(stdout, "[FAIL] %s: %s\n", probe.Name, probe.Message)
			}
		}
	}

	if report.AllPassed {
		return 0
	}
	return 1
}
