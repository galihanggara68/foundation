package main

import (
	"fmt"
	"io"
)

// pluginUsage documents the plugin command family.
const pluginUsage = "Usage: foundation plugin <new|doctor> [flags...]"

// runPlugin routes plugin subcommands to corresponding handlers.
func runPlugin(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, pluginUsage)
		return 1
	}

	switch args[0] {
	case "new":
		return runPluginNew(args[1:], stdout, stderr)
	case "doctor":
		return runPluginDoctor(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown plugin command %q\n", args[0])
		fmt.Fprintln(stderr, pluginUsage)
		return 1
	}
}
