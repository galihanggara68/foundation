package emitterkit

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// ModelDumperFunc converts a GenerationRequest to an intermediate model.
type ModelDumperFunc func(req GenerationRequest) (any, error)

// PluginOption configures a plugin run.
type PluginOption func(*pluginConfig)

type pluginConfig struct {
	dumpModelFn ModelDumperFunc
}

// WithDumpModel configures a model dump function for the plugin.
func WithDumpModel(fn ModelDumperFunc) PluginOption {
	return func(c *pluginConfig) {
		c.dumpModelFn = fn
	}
}

// RunPluginWithFlags parses command-line flags and runs the plugin handler.
// The function stops the process with code 0 on success.
// The function logs errors to standard error and stops the process with code 1 on failure.
func RunPluginWithFlags(handler HandlerFunc, opts ...PluginOption) {
	if err := RunPluginWithFlagsStream(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, handler, opts...); err != nil {
		fmt.Fprintf(os.Stderr, "plugin execution failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// RunPluginWithFlagsStream parses flags and processes plugin input and output streams.
func RunPluginWithFlagsStream(
	args []string,
	in io.Reader,
	out io.Writer,
	errOut io.Writer,
	handler HandlerFunc,
	opts ...PluginOption,
) error {
	var cfg pluginConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	flags := flag.NewFlagSet("plugin", flag.ContinueOnError)
	if errOut != nil {
		flags.SetOutput(errOut)
	} else {
		flags.SetOutput(io.Discard)
	}

	dumpModel := flags.Bool("dump-model", false, "dump intermediate model as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if *dumpModel {
		if cfg.dumpModelFn == nil {
			return errors.New("model dumper is not configured")
		}

		req, err := compiler.DecodeGenerationRequest(in)
		if err != nil {
			return fmt.Errorf("failed to decode generation request: %w", err)
		}

		model, err := cfg.dumpModelFn(req)
		if err != nil {
			return fmt.Errorf("failed to dump model: %w", err)
		}

		data, err := json.MarshalIndent(model, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to encode model: %w", err)
		}
		data = append(data, '\n')
		if _, err := out.Write(data); err != nil {
			return err
		}
		return nil
	}

	return RunPluginStream(in, out, handler)
}
