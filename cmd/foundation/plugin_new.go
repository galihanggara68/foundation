package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/galihanggara68/foundation/cmd/foundation/internal/pluginnew"
)

// pluginNewUsage documents the plugin new command line.
const pluginNewUsage = "Usage: foundation plugin new <plugin-name> [--dir <output-dir>] [--module <go-module-path>] [--replace <foundation-path>]"

// runPluginNew handles the plugin new CLI subcommand.
func runPluginNew(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("plugin new", flag.ContinueOnError)
	fs.SetOutput(stderr)

	dir := fs.String("dir", ".", "output directory for generated plugin")
	module := fs.String("module", "", "Go module path")
	replace := fs.String("replace", "", "local replacement path for foundation module")

	var flagArgs []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		return 1
	}

	if len(positional) == 0 {
		fmt.Fprintln(stderr, "error: plugin name is required")
		fmt.Fprintln(stderr, pluginNewUsage)
		return 1
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "error: unexpected argument %q\n", positional[1])
		fmt.Fprintln(stderr, pluginNewUsage)
		return 1
	}

	pluginName := positional[0]
	modulePath := *module
	if modulePath == "" {
		modulePath = pluginName
	}

	targetDir := resolveTargetDir(*dir, pluginName)

	opts := pluginnew.ScaffoldOptions{
		PluginName:        pluginName,
		TargetDir:         targetDir,
		ModulePath:        modulePath,
		FoundationVersion: "v0.1.0",
		ReplacePath:       *replace,
	}

	if err := pluginnew.Scaffold(opts); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Created plugin %s in %s\n", pluginName, targetDir)
	return 0
}

// resolveTargetDir determines the destination path for the plugin.
func resolveTargetDir(dirFlag, pluginName string) string {
	if dirFlag == "" || dirFlag == "." {
		return pluginName
	}
	if filepath.Base(dirFlag) == pluginName {
		return dirFlag
	}
	return filepath.Join(dirFlag, pluginName)
}
