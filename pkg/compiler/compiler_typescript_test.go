package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuildAndTest_TypeScriptCleanArchitecture_CompilesTypeChecksAndPassesTests(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "commerce.fdn"), []byte(sharedMultiTargetBlueprint), 0644); err != nil {
		t.Fatalf("write commerce.fdn: %v", err)
	}

	buildProfile := `project:
  name: commerce
  module: "@commerce/backend"

storage:
  dialect: postgres
  mode: declarative

targets:
  - name: ts-backend
    kind: backend
    language: typescript
    architecture: clean
    http: express
    out_dir: ./dist/ts-backend
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	// 1. Initial Build
	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("expected TypeScript clean architecture build to succeed, got: %+v", diags)
	}
	if len(res.Architectures) != 1 || res.Architectures[0].Language != "typescript" || res.Architectures[0].Pattern != "clean" {
		t.Fatalf("expected lowered TypeScript clean ArchitectureIR, got: %+v", res.Architectures)
	}

	outDir := filepath.Join(dir, "dist", "ts-backend")
	expectedTSFiles := []string{
		"package.json",
		"tsconfig.json",
		"schema.sql",
		"src/entities/domain.ts",
		"src/usecases/ports.ts",
		"src/usecases/placeorder_usecase.ts",
		"src/usecases/cancelorder_usecase.ts",
		"src/controllers/express_router.ts",
		"src/gateways/memory_repositories.ts",
		"src/gateways/postgres_repositories.ts",
		"src/slots/pricingengine_stub.ts",
		"src/main/container.ts",
		"src/main/server.ts",
		"test/contracts.test.ts",
		"test/scenarios.test.ts",
	}
	for _, rel := range expectedTSFiles {
		if _, err := os.Stat(filepath.Join(outDir, rel)); err != nil {
			t.Errorf("expected TypeScript file %s to be emitted: %v", rel, err)
		}
	}

	// Verify create-if-missing Slot Stub is excluded from .foundation-manifest.json
	manifestBytes, err := os.ReadFile(filepath.Join(outDir, ".foundation-manifest.json"))
	if err != nil {
		t.Fatalf("read .foundation-manifest.json: %v", err)
	}
	if strings.Contains(string(manifestBytes), "src/slots/pricingengine_stub.ts") {
		t.Fatalf("expected src/slots/pricingengine_stub.ts to be excluded from .foundation-manifest.json, got:\n%s", string(manifestBytes))
	}

	// 2. Run compiler.Test (executes tsc --noEmit and node --test on generated TypeScript suite)
	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("expected TypeScript type-checking and scenario/contract tests to pass, got: %+v", testDiags)
	}

	// 3. Edit the TypeScript Slot Stub and verify rebuild preserves it and passes tests
	stubPath := filepath.Join(outDir, "src/slots/pricingengine_stub.ts")
	customTSStub := `import type { PricingEngineSlot } from "../usecases/ports.ts";
import type { QuoteRequest, QuoteResult } from "../entities/domain.ts";
import { ErrQuoteUnavailable } from "../entities/domain.ts";

// Handwritten TypeScript Slot implementation
export class PricingEngineStub implements PricingEngineSlot {
  async calculateQuote(req: QuoteRequest): Promise<QuoteResult> {
    if (req.quantity <= 0) {
      throw ErrQuoteUnavailable;
    }
    return {
      unit_price: 250,
      discount: 10,
    };
  }
}

export function createPricingEngineSlot(): PricingEngineSlot {
  return new PricingEngineStub();
}
`
	if err := os.WriteFile(stubPath, []byte(customTSStub), 0644); err != nil {
		t.Fatalf("write custom TS stub: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{Force: false}); len(diags) != 0 {
		t.Fatalf("expected rebuild after editing TS slot stub to succeed without conflict, got: %+v", diags)
	}

	afterStub, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("read TS stub after rebuild: %v", err)
	}
	if string(afterStub) != customTSStub {
		t.Fatalf("expected handwritten TS slot stub to be preserved untouched, got:\n%s", string(afterStub))
	}

	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("expected handwritten TS slot stub to pass tsc --noEmit and contract/scenario tests, got: %+v", testDiags)
	}
}
