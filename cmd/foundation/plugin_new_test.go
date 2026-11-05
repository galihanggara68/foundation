package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPluginNew_MissingArguments_Fails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPluginNew([]string{}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "error: plugin name is required") {
		t.Errorf("expected missing name error, got: %s", stderr.String())
	}
}

func TestRunPluginNew_UnexpectedArguments_Fails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPluginNew([]string{"plugin-one", "plugin-two"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "error: unexpected argument") {
		t.Errorf("expected unexpected argument error, got: %s", stderr.String())
	}
}

func TestRunPluginNew_FlagParseError_Fails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPluginNew([]string{"--unknown-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestRunPluginNew_Success(t *testing.T) {
	tmpDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := runPluginNew([]string{"my-plugin", "--dir", tmpDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", code, stderr.String())
	}

	expectedOutput := "Created plugin my-plugin in " + filepath.Join(tmpDir, "my-plugin")
	if !strings.Contains(stdout.String(), expectedOutput) {
		t.Errorf("expected stdout to contain %q, got: %s", expectedOutput, stdout.String())
	}

	mainFile := filepath.Join(tmpDir, "my-plugin", "main.go")
	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		t.Fatalf("expected main.go in generated directory")
	}
}
