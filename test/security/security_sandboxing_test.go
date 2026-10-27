package security_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// TestSecurity_PluginDirectoryTraversal_EscapeOutDir probes whether a generator plugin
// emitting relative paths with "../" can escape outDir and overwrite sibling files.
func TestSecurity_PluginDirectoryTraversal_EscapeOutDir(t *testing.T) {
	dir := t.TempDir()

	outDir := filepath.Join(dir, "out")
	canaryPath := filepath.Join(dir, "canary_victim.txt")
	if err := os.WriteFile(canaryPath, []byte("SAFE_UNMODIFIED"), 0644); err != nil {
		t.Fatalf("failed to write canary file: %v", err)
	}

	mockPluginPath := filepath.Join(dir, "foundation-gen-malicious")
	scriptContent := `#!/usr/bin/env python3
import sys, json

req = json.load(sys.stdin)
resp = {
    "protocol_version": 1,
    "diagnostics": [],
    "files": {
        "../canary_victim.txt": "MALICIOUS_PWNED_CONTENT",
        "nested/../../canary_victim2.txt": "PWNED2"
    },
    "stubs": {}
}
sys.stdout.write(json.dumps(resp))
sys.stdout.flush()
`
	if err := os.WriteFile(mockPluginPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write mock plugin: %v", err)
	}

	blueprint := "entity Item { id: uuid @pk \n name: string }\n"
	if err := os.WriteFile(filepath.Join(dir, "model.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}

	buildProfile := fmt.Sprintf(`
version: "1.0"
project:
  name: "security-test"
  module: "example.com/sectest"
targets:
  - name: "exploit-test"
    kind: "backend"
    language: "go"
    architecture: "hexagonal"
    out_dir: "out"
    generator: %q
    scaffold: "none"
`, mockPluginPath)
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	// Execute compiler build
	res, diags := compiler.Build(dir, compiler.BuildOptions{Force: true})
	_ = res

	// Check if the canary outside outDir was overwritten
	canaryBytes, err := os.ReadFile(canaryPath)
	if err != nil {
		t.Fatalf("failed to read canary: %v", err)
	}

	canary2Path := filepath.Join(dir, "canary_victim2.txt")
	_, err2 := os.Stat(canary2Path)

	t.Logf("Compiler diagnostics returned: %d", len(diags))
	for _, d := range diags {
		t.Logf("  [%s] %s: %s", d.Severity, d.Code, d.Message)
	}

	if string(canaryBytes) == "MALICIOUS_PWNED_CONTENT" || err2 == nil {
		t.Errorf("SECURITY VULNERABILITY EXPOSED [Directory Traversal]: generator plugin successfully escaped outDir %q and wrote files outside: canary=%q, canary2_exists=%v",
			outDir, string(canaryBytes), err2 == nil)
	} else {
		t.Logf("PASS: Sandboxing prevented directory traversal outside outDir.")
	}
}

// TestSecurity_ManifestPoisoning_ArbitraryFileDeletion probes whether a poisoned
// .foundation-manifest.json containing "../file" paths triggers arbitrary deletion
// during stale file pruning.
func TestSecurity_ManifestPoisoning_ArbitraryFileDeletion(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("failed to create outDir: %v", err)
	}

	victimPath := filepath.Join(dir, "critical_project_file.go")
	if err := os.WriteFile(victimPath, []byte("package critical"), 0644); err != nil {
		t.Fatalf("failed to write victim file: %v", err)
	}

	poisonedManifest := `{
  "version": 1,
  "files": {
    "../critical_project_file.go": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  }
}`
	manifestPath := filepath.Join(outDir, ".foundation-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(poisonedManifest), 0644); err != nil {
		t.Fatalf("failed to write poisoned manifest: %v", err)
	}

	blueprint := "entity User { id: uuid @pk }\n"
	if err := os.WriteFile(filepath.Join(dir, "user.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}

	buildProfile := `
version: "1.0"
project:
  name: "security-test"
  module: "example.com/sectest"
targets:
  - name: "server"
    kind: "backend"
    language: "go"
    architecture: "hexagonal"
    out_dir: "out"
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{Force: true})
	for _, d := range diags {
		t.Logf("  [%s] %s: %s", d.Severity, d.Code, d.Message)
	}

	if _, err := os.Stat(victimPath); os.IsNotExist(err) {
		t.Errorf("SECURITY VULNERABILITY EXPOSED [Manifest Poisoning / Arbitrary File Deletion]: stale file pruning deleted file outside outDir: %s", victimPath)
	} else {
		t.Logf("PASS: Sandboxing protected outside files from manifest deletion.")
	}
}

// TestSecurity_RoguePlugin_CrashExitCodeHandled verifies that an unhandled crash
// or non-zero exit in a plugin produces structured diagnostics without panics.
func TestSecurity_RoguePlugin_CrashExitCodeHandled(t *testing.T) {
	dir := t.TempDir()

	mockCrashPlugin := filepath.Join(dir, "foundation-gen-crash")
	script := `#!/bin/sh
echo "FATAL INTERNAL ERROR: segfault in generator" >&2
exit 139
`
	if err := os.WriteFile(mockCrashPlugin, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write crash plugin: %v", err)
	}

	blueprint := "entity Item { id: uuid @pk }\n"
	if err := os.WriteFile(filepath.Join(dir, "model.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}

	buildProfile := fmt.Sprintf(`
version: "1.0"
project:
  name: "security-test"
  module: "example.com/sectest"
targets:
  - name: "crash-test"
    kind: "backend"
    language: "go"
    architecture: "hexagonal"
    out_dir: "out"
    generator: %q
    scaffold: "none"
`, mockCrashPlugin)
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{Force: true})
	foundCrashDiag := false
	for _, d := range diags {
		if d.Code == "FDN0701" && strings.Contains(d.Message, "code 139") {
			foundCrashDiag = true
		}
	}

	if !foundCrashDiag {
		t.Errorf("Expected FDN0701 exit code 139 diagnostic from compiler.Build, got: %v", diags)
	} else {
		t.Logf("PASS: Rogue crash handled cleanly with structured diagnostic.")
	}
}

// TestSecurity_RoguePlugin_MalformedOutputBomb verifies that corrupted output
// on stdout produces structured diagnostics without unhandled errors.
func TestSecurity_RoguePlugin_MalformedOutputBomb(t *testing.T) {
	dir := t.TempDir()

	mockGarbagePlugin := filepath.Join(dir, "foundation-gen-garbage")
	script := `#!/bin/sh
echo "NOT_JSON_GARBAGE_PAYLOAD_<<>>{}[]"
`
	if err := os.WriteFile(mockGarbagePlugin, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write garbage plugin: %v", err)
	}

	blueprint := "entity Item { id: uuid @pk }\n"
	if err := os.WriteFile(filepath.Join(dir, "model.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}

	buildProfile := fmt.Sprintf(`
version: "1.0"
project:
  name: "security-test"
  module: "example.com/sectest"
targets:
  - name: "garbage-test"
    kind: "backend"
    language: "go"
    architecture: "hexagonal"
    out_dir: "out"
    generator: %q
    scaffold: "none"
`, mockGarbagePlugin)
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{Force: true})
	foundDecodeDiag := false
	for _, d := range diags {
		if d.Code == "FDN0701" && strings.Contains(d.Message, "failed to decode") {
			foundDecodeDiag = true
		}
	}

	if !foundDecodeDiag {
		t.Errorf("Expected FDN0701 decode failure diagnostic, got: %v", diags)
	} else {
		t.Logf("PASS: Corrupted stdout handled cleanly with structured diagnostic.")
	}
}
