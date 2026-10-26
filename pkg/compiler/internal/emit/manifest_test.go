package emit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
)

func TestReconcileManifest_ProtectsLockfilesFromConflictAndPruning(t *testing.T) {
	dir := t.TempDir()

	initialTracked := map[string][]byte{
		"go.mod": []byte("module example.com/test\n\ngo 1.23\n"),
		"go.sum": []byte("some-dep v1.0.0 h1:xyz=\n"),
	}

	diags := emit.ReconcileManifest(dir, initialTracked, nil, "", false)
	if len(diags) > 0 {
		t.Fatalf("unexpected diags on initial reconcile: %v", diags)
	}

	// External package manager modifies go.sum
	updatedSum := []byte("some-dep v1.0.0 h1:xyz=\nnew-dep v2.0.0 h1:abc=\n")
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), updatedSum, 0644); err != nil {
		t.Fatalf("failed to update go.sum: %v", err)
	}

	// Next emission without go.sum in tracked files (or with older content) should NOT trigger conflict
	nextTracked := map[string][]byte{
		"go.mod": []byte("module example.com/test\n\ngo 1.23\n"),
	}

	diags2 := emit.ReconcileManifest(dir, nextTracked, nil, "", false)
	if len(diags2) > 0 {
		t.Fatalf("expected lockfile to be exempt from conflict, but got diags: %v", diags2)
	}

	// Next emission WITH a stub go.sum should NOT overwrite the updated go.sum
	nextTrackedWithStub := map[string][]byte{
		"go.mod": []byte("module example.com/test\n\ngo 1.23\n"),
		"go.sum": []byte("stub\n"),
	}

	diags3 := emit.ReconcileManifest(dir, nextTrackedWithStub, nil, "", false)
	if len(diags3) > 0 {
		t.Fatalf("expected stub lockfile to not error, got: %v", diags3)
	}

	contentAfterRebuild, err := os.ReadFile(filepath.Join(dir, "go.sum"))
	if err != nil {
		t.Fatalf("expected go.sum to exist: %v", err)
	}
	if string(contentAfterRebuild) != string(updatedSum) {
		t.Fatalf("expected go.sum to retain updatedSum, but got %s", string(contentAfterRebuild))
	}
}
