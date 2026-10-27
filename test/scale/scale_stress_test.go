package scale_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// generateMegaBlueprint creates a synthetic valid Foundation blueprint with n entities,
// each having 10 fields and foreign key dependencies forming an acyclic DAG.
func generateMegaBlueprint(n int) string {
	var sb strings.Builder
	sb.WriteString("// Synthesized Mega-Blueprint for Scale Testing\n\n")

	sb.WriteString("value CommonMetadata {\n")
	sb.WriteString("  tag: string\n")
	sb.WriteString("  priority: int\n")
	sb.WriteString("}\n\n")

	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "entity Entity_%04d {\n", i)
		fmt.Fprintf(&sb, "  id: uuid @pk\n")
		fmt.Fprintf(&sb, "  name: string\n")
		fmt.Fprintf(&sb, "  code: string @unique\n")
		fmt.Fprintf(&sb, "  count: int\n")
		fmt.Fprintf(&sb, "  amount: decimal\n")
		fmt.Fprintf(&sb, "  is_active: bool\n")
		fmt.Fprintf(&sb, "  created_at: timestamp\n")
		fmt.Fprintf(&sb, "  updated_at: timestamp\n")
		fmt.Fprintf(&sb, "  version: int @version\n")

		// Add FK to previous entity to build a deep dependency chain
		if i > 1 {
			fmt.Fprintf(&sb, "  parent_id: uuid @fk(Entity_%04d.id)\n", i-1)
		} else {
			fmt.Fprintf(&sb, "  parent_id: uuid\n")
		}
		sb.WriteString("}\n\n")

		// Add an action for each entity
		fmt.Fprintf(&sb, "action Action_%04d(val: int) -> bool {\n", i)
		fmt.Fprintf(&sb, "  return true\n")
		sb.WriteString("}\n\n")
	}

	return sb.String()
}

// TestScale_CompilationThroughput benchmarks parsing, validation, lowering, and code generation
// across varying schema sizes (50, 100, 250, and 500 entities).
func TestScale_CompilationThroughput(t *testing.T) {
	sizes := []int{50, 100, 250, 500}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("%d_Entities", n), func(t *testing.T) {
			blueprint := generateMegaBlueprint(n)
			dir := t.TempDir()

			fdnPath := filepath.Join(dir, "mega.fdn")
			if err := os.WriteFile(fdnPath, []byte(blueprint), 0644); err != nil {
				t.Fatalf("failed to write mega blueprint: %v", err)
			}

			buildProfile := fmt.Sprintf(`
version: "1.0"
project:
  name: "scale-test-%d"
  module: "example.com/scaletest"
targets:
  - name: "backend-go"
    kind: "backend"
    language: "go"
    architecture: "hexagonal"
    out_dir: "out/go"
  - name: "sdk-go"
    kind: "sdk"
    language: "go"
    out_dir: "out/gosdk"
`, n)
			if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
				t.Fatalf("failed to write build profile: %v", err)
			}

			// Force GC before measuring memory
			runtime.GC()
			var mStart runtime.MemStats
			runtime.ReadMemStats(&mStart)

			start := time.Now()
			res, diags := compiler.Build(dir, compiler.BuildOptions{Force: true})
			elapsed := time.Since(start)

			var mEnd runtime.MemStats
			runtime.ReadMemStats(&mEnd)
			heapAllocMB := float64(mEnd.TotalAlloc-mStart.TotalAlloc) / (1024 * 1024)

			if len(diags) > 0 {
				for _, d := range diags {
					t.Errorf("Unexpected diagnostic: [%s] %s: %s", d.Severity, d.Code, d.Message)
				}
				t.Fatalf("Compilation failed with %d diagnostics for %d entities", len(diags), n)
			}

			if res == nil || len(res.Semantic.Entities) != n {
				t.Fatalf("Expected %d entities, got %v", n, len(res.Semantic.Entities))
			}

			// Verify generated files on disk
			manifestFile := filepath.Join(dir, "out/go", ".foundation-manifest.json")
			if _, err := os.Stat(manifestFile); err != nil {
				t.Fatalf("Expected .foundation-manifest.json to exist for %d entities: %v", n, err)
			}

			entitiesPerSec := float64(n) / elapsed.Seconds()
			t.Logf("SCALE REPORT [%d Entities | %d Fields | %d Actions]:", n, n*10, n)
			t.Logf("  Elapsed Time   : %v", elapsed)
			t.Logf("  Throughput     : %.1f entities/sec", entitiesPerSec)
			t.Logf("  Total Heap Alloc: %.2f MB", heapAllocMB)

			if n == 500 && elapsed > 5*time.Second {
				t.Logf("PERFORMANCE WARNING: 500 entities took %v (> 5.0s)", elapsed)
			}
		})
	}
}

// TestScale_SingleEntityMassiveFields tests compiler robustness against an entity with 1,000 fields.
func TestScale_SingleEntityMassiveFields(t *testing.T) {
	fieldCount := 1000
	var sb strings.Builder
	sb.WriteString("entity MonsterEntity {\n")
	sb.WriteString("  id: uuid @pk\n")
	for i := 1; i <= fieldCount; i++ {
		fmt.Fprintf(&sb, "  field_%04d: string\n", i)
	}
	sb.WriteString("}\n")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "monster.fdn"), []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write monster fdn: %v", err)
	}

	buildProfile := `
version: "1.0"
project:
  name: "monster-test"
  module: "example.com/monstertest"
targets:
  - name: "server"
    kind: "backend"
    language: "go"
    architecture: "hexagonal"
    out_dir: "out"
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write profile: %v", err)
	}

	start := time.Now()
	res, diags := compiler.Build(dir, compiler.BuildOptions{Force: true})
	elapsed := time.Since(start)

	if len(diags) > 0 {
		t.Fatalf("Failed to compile monster entity with 1,000 fields: %v", diags)
	}
	if len(res.Semantic.Entities[0].Fields) != fieldCount+1 {
		t.Fatalf("Expected %d fields, got %d", fieldCount+1, len(res.Semantic.Entities[0].Fields))
	}
	t.Logf("PASS: Monster entity with 1,000 fields compiled successfully in %v.", elapsed)
}

// TestScale_CyclicEntityRelations verifies that topological sort in the compiler
// handles cyclical foreign key dependencies without an infinite loop or panic.
func TestScale_CyclicEntityRelations(t *testing.T) {
	blueprint := `
entity Alpha {
  id: uuid @pk
  beta_id: uuid @fk(Beta.id)
}

entity Beta {
  id: uuid @pk
  gamma_id: uuid @fk(Gamma.id)
}

entity Gamma {
  id: uuid @pk
  alpha_id: uuid @fk(Alpha.id)
}
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cycle.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write cycle fdn: %v", err)
	}

	buildProfile := `
version: "1.0"
project:
  name: "cycle-test"
  module: "example.com/cycletest"
targets:
  - name: "server"
    kind: "backend"
    language: "go"
    architecture: "hexagonal"
    out_dir: "out"
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write profile: %v", err)
	}

	start := time.Now()
	res, diags := compiler.Build(dir, compiler.BuildOptions{Force: true})
	elapsed := time.Since(start)

	if len(diags) > 0 {
		t.Logf("Compiler diagnostics on cyclic relations: %d", len(diags))
		for _, d := range diags {
			t.Logf("  [%s] %s: %s", d.Severity, d.Code, d.Message)
		}
	}

	if res != nil {
		t.Logf("PASS: Cyclic entity graph resolved without hang in %v. Entities sorted: %d",
			elapsed, len(res.Semantic.Entities))
	}
}
