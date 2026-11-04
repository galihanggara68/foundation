package testing_test

import (
	"os"
	"path/filepath"
	"strings"
	stdtesting "testing"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
	emittertesting "github.com/galihanggara68/foundation/pkg/emitterkit/testing"
)

func TestAssertGoldenTree_MatchingTree_Passes(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	goldenDir := t.TempDir()
	writeFile(t, filepath.Join(goldenDir, "domain/user.go"), "package domain\n\ntype User struct{}\n")
	writeFile(t, filepath.Join(goldenDir, "domain/stub.go"), "// stub\n")

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"domain/user.go": "package domain\n\ntype User struct{}\n",
		},
		Stubs: map[string]string{
			"domain/stub.go": "// stub\n",
		},
	}

	h.AssertGoldenTree(resp, goldenDir, emittertesting.TreeGoldenOptions{})

	if mock.failed {
		t.Fatalf("expected matching golden tree to pass, got errors: %v", mock.errors)
	}
}

func TestAssertGoldenTree_MissingFileInResponse_Fails(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	goldenDir := t.TempDir()
	writeFile(t, filepath.Join(goldenDir, "domain/user.go"), "package domain\n")
	writeFile(t, filepath.Join(goldenDir, "domain/order.go"), "package domain\n")

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"domain/user.go": "package domain\n",
		},
	}

	h.AssertGoldenTree(resp, goldenDir, emittertesting.TreeGoldenOptions{})

	if !mock.failed {
		t.Fatalf("expected missing file to fail")
	}
	assertContains(t, mock.errors, "missing expected golden file \"domain/order.go\" in response")
}

func TestAssertGoldenTree_UnexpectedFileInResponse_Fails(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	goldenDir := t.TempDir()
	writeFile(t, filepath.Join(goldenDir, "domain/user.go"), "package domain\n")

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"domain/user.go":  "package domain\n",
			"domain/extra.go": "package domain\n",
		},
	}

	h.AssertGoldenTree(resp, goldenDir, emittertesting.TreeGoldenOptions{})

	if !mock.failed {
		t.Fatalf("expected extra file to fail")
	}
	assertContains(t, mock.errors, "unexpected emitted file \"domain/extra.go\" in response")
}

func TestAssertGoldenTree_ContentMismatch_FailsWithDiff(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	goldenDir := t.TempDir()
	writeFile(t, filepath.Join(goldenDir, "config.yaml"), "host: localhost\nport: 8080\n")

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"config.yaml": "host: localhost\nport: 9090\n",
		},
	}

	h.AssertGoldenTree(resp, goldenDir, emittertesting.TreeGoldenOptions{})

	if !mock.failed {
		t.Fatalf("expected content mismatch to fail")
	}
	assertContains(t, mock.errors, "file \"config.yaml\" content mismatch")
	assertContains(t, mock.errors, "-port: 8080")
	assertContains(t, mock.errors, "+port: 9090")
}

func TestAssertGoldenTree_IgnorePaths_SkipsIgnoredFiles(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	goldenDir := t.TempDir()
	writeFile(t, filepath.Join(goldenDir, "domain/user.go"), "package domain\n")
	writeFile(t, filepath.Join(goldenDir, "debug.log"), "trace information\n")

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"domain/user.go": "package domain\n",
			"temp.tmp":       "scratch data\n",
		},
	}

	opts := emittertesting.TreeGoldenOptions{
		IgnorePaths: []string{"debug.log", "*.tmp"},
	}

	h.AssertGoldenTree(resp, goldenDir, opts)

	if mock.failed {
		t.Fatalf("expected ignored files to pass, got errors: %v", mock.errors)
	}
}

func TestAssertGoldenTree_UpdateGoldens_OverwritesAndDeletes(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	goldenDir := t.TempDir()
	oldPath := filepath.Join(goldenDir, "obsolete.txt")
	keepPath := filepath.Join(goldenDir, "service/handler.go")
	writeFile(t, oldPath, "obsolete content")
	writeFile(t, keepPath, "package service\n// v1\n")

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"service/handler.go": "package service\n// v2\n",
			"service/client.go":  "package service\n// client\n",
		},
	}

	opts := emittertesting.TreeGoldenOptions{
		UpdateGoldens: true,
	}

	h.AssertGoldenTree(resp, goldenDir, opts)

	if mock.failed {
		t.Fatalf("update goldens reported error: %v", mock.errors)
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected obsolete file to be deleted")
	}

	keepData, err := os.ReadFile(keepPath)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(keepData) != "package service\n// v2\n" {
		t.Fatalf("expected updated content, got %q", string(keepData))
	}

	clientPath := filepath.Join(goldenDir, "service/client.go")
	clientData, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if string(clientData) != "package service\n// client\n" {
		t.Fatalf("expected new file content, got %q", string(clientData))
	}

	checkMock := &recordTB{}
	checkHarness := emittertesting.NewHarnessTB(checkMock)
	checkHarness.AssertGoldenTree(resp, goldenDir, emittertesting.TreeGoldenOptions{})
	if checkMock.failed {
		t.Fatalf("expected verification after update to pass, got: %v", checkMock.errors)
	}
}

func TestAssertGoldenTree_NonExistentGoldenDir_RecordsFatal(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	missingDir := filepath.Join(t.TempDir(), "does-not-exist")
	resp := emitterkit.GenerationResponse{
		Files: map[string]string{"main.go": "package main\n"},
	}

	h.AssertGoldenTree(resp, missingDir, emittertesting.TreeGoldenOptions{})

	if !mock.failed {
		t.Fatalf("expected missing golden directory to fail")
	}
	if len(mock.fatals) == 0 {
		t.Fatalf("expected fatal error recorded for missing golden directory")
	}
}

func TestAssertGoldenTree_FilesOverrideStubs(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	goldenDir := t.TempDir()
	writeFile(t, filepath.Join(goldenDir, "main.go"), "package main\n// real\n")

	resp := emitterkit.GenerationResponse{
		Stubs: map[string]string{
			"main.go": "package main\n// stub\n",
		},
		Files: map[string]string{
			"main.go": "package main\n// real\n",
		},
	}

	h.AssertGoldenTree(resp, goldenDir, emittertesting.TreeGoldenOptions{})

	if mock.failed {
		t.Fatalf("expected files to override stubs, got errors: %v", mock.errors)
	}
}

func writeFile(t *stdtesting.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func assertContains(t *stdtesting.T, entries []string, substr string) {
	t.Helper()
	for _, entry := range entries {
		if strings.Contains(entry, substr) {
			return
		}
	}
	t.Fatalf("expected substring %q in recorded entries: %v", substr, entries)
}
