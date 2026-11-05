package compiler_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// TestExportedSemanticIRTypes verifies that the compiler package instantiates all semantic IR types.
func TestExportedSemanticIRTypes(t *testing.T) {
	sem := compiler.SemanticIR{
		Actors: []compiler.ActorDecl{{Name: "User"}},
		Slots: []compiler.SlotDecl{{
			Name: "Notifier",
			Methods: []compiler.SlotMethodDecl{{
				Name:       "Send",
				Params:     []compiler.ParamDecl{{Name: "to", Type: "string"}},
				ReturnType: "bool",
			}},
		}},
		Enums:  []compiler.EnumDecl{{Name: "Status", Variants: []string{"Active"}}},
		Events: []compiler.EventDecl{{Name: "UserCreated"}},
		Policies: []compiler.PolicyDecl{{
			Name:   "OwnerOnly",
			Params: []compiler.ParamDecl{{Name: "actor", Type: "string"}},
			Body:   compiler.Expr{Kind: compiler.ExprKind("ident"), Value: "true"},
		}},
		Values: []compiler.ValueDecl{{
			Name:   "Money",
			Fields: []compiler.FieldDecl{{Name: "amount", Type: "decimal"}},
		}},
		Entities: []compiler.EntityDecl{{
			Name: "Account",
			Fields: []compiler.FieldDecl{{
				Name: "id",
				Type: "uuid",
				Kind: compiler.FieldKind("uuid"),
				Traits: []compiler.TraitDecl{{
					Name: "pk",
					Args: map[string]string{"_": "true"},
				}},
			}},
		}},
		Errors: []compiler.ErrorDecl{{
			Name:    "AccountNotFound",
			Kind:    compiler.ErrorKind("not_found"),
			Posture: compiler.ErrorPosture("operational"),
			Message: "account not found",
		}},
		Actions: []compiler.ActionDecl{{
			Name:   "CreateAccount",
			Params: []compiler.ParamDecl{{Name: "name", Type: "string"}},
			Steps: []compiler.StepDecl{{
				Kind: compiler.StepKind("let"),
				Expr: &compiler.Expr{
					Kind: compiler.ExprKind("ident"),
					Span: compiler.Span{File: "main.fdn", Line: 10, Column: 4},
				},
				Fields: []compiler.FieldInitDecl{{
					Name:  "balance",
					Value: compiler.Expr{Kind: compiler.ExprKind("literal")},
				}},
				MatchArms: []compiler.MatchArmDecl{{
					Variant: "ok",
					Steps:   []compiler.StepDecl{{Kind: compiler.StepKind("return")}},
				}},
				WherePredicates: []compiler.WherePredicate{{
					Field: "status",
					Op:    "=",
					Value: compiler.Expr{Kind: compiler.ExprKind("ident")},
				}},
			}},
		}},
		Scenarios: []compiler.ScenarioDecl{{
			Name: "SuccessPath",
			Givens: []compiler.GivenSeedDecl{{
				EntityName: "Account",
				Fields: []compiler.FieldInitDecl{{
					Name:  "name",
					Value: compiler.Expr{Kind: compiler.ExprKind("literal"), Value: "test"},
				}},
			}},
			GivenSlots: []compiler.GivenSlotDecl{{
				SlotName:   "Notifier",
				MethodName: "Send",
			}},
			Expects: []compiler.ExpectDecl{{
				Left:  compiler.Expr{Kind: compiler.ExprKind("ident"), Value: "status"},
				Op:    "==",
				Right: compiler.Expr{Kind: compiler.ExprKind("literal"), Value: "200"},
			}},
		}},
	}

	if len(sem.Actors) != 1 || sem.Actors[0].Name != "User" {
		t.Fatalf("expected actor User, got %+v", sem.Actors)
	}
	if len(sem.Slots) != 1 || sem.Slots[0].Name != "Notifier" {
		t.Fatalf("expected slot Notifier, got %+v", sem.Slots)
	}
	if len(sem.Entities) != 1 || sem.Entities[0].Name != "Account" {
		t.Fatalf("expected entity Account, got %+v", sem.Entities)
	}
}

// TestExportedDiagnosticTypes verifies that the compiler package instantiates diagnostic types and constants.
func TestExportedDiagnosticTypes(t *testing.T) {
	diag := compiler.Diagnostic{
		Code:     "FDN0001",
		Severity: compiler.SeverityError,
		Message:  "fatal test error",
		Span: compiler.Span{
			File:   "test.fdn",
			Line:   1,
			Column: 1,
		},
	}
	if diag.Severity != compiler.SeverityError {
		t.Errorf("expected SeverityError, got %q", diag.Severity)
	}

	warnDiag := compiler.Diagnostic{
		Code:     "FDN0002",
		Severity: compiler.SeverityWarning,
		Message:  "test warning",
	}
	if warnDiag.Severity != compiler.SeverityWarning {
		t.Errorf("expected SeverityWarning, got %q", warnDiag.Severity)
	}

	infoDiag := compiler.Diagnostic{
		Code:     "FDN0003",
		Severity: compiler.SeverityInfo,
		Message:  "test info",
	}
	if infoDiag.Severity != compiler.SeverityInfo {
		t.Errorf("expected SeverityInfo, got %q", infoDiag.Severity)
	}
}

// TestExportedStoragePlanTypes verifies that the compiler package instantiates storage plan types.
func TestExportedStoragePlanTypes(t *testing.T) {
	plan := compiler.StoragePlanIR{
		Dialect:   "postgres",
		Mode:      "declarative",
		SchemaSQL: "CREATE TABLE test ();",
		Migrations: []compiler.MigrationPlan{{
			Number:  1,
			Name:    "0001_initial",
			UpSQL:   "CREATE TABLE test ();",
			DownSQL: "DROP TABLE test;",
		}},
		Tables: []compiler.TableSchema{{
			Name: "users",
			Columns: []compiler.ColumnSchema{{
				Name:       "id",
				Type:       "UUID",
				IsPK:       true,
				IsNullable: false,
			}},
			PrimaryKeys: []string{"id"},
			ForeignKeys: []compiler.ForeignKey{{
				Column:        "org_id",
				ForeignTable:  "organizations",
				ForeignColumn: "id",
			}},
			Indexes: []compiler.IndexSchema{{
				Name:     "idx_users_email",
				Table:    "users",
				Columns:  []string{"email"},
				IsUnique: true,
			}},
			Uniques: []compiler.UniqueConstraint{{
				Name:    "uq_users_org_email",
				Table:   "users",
				Columns: []string{"org_id", "email"},
			}},
		}},
	}

	if plan.Dialect != "postgres" {
		t.Errorf("expected Dialect postgres, got %q", plan.Dialect)
	}
	if len(plan.Tables) != 1 || plan.Tables[0].Name != "users" {
		t.Fatalf("expected table users, got %+v", plan.Tables)
	}
}

// TestExportedPlatformTypes confirms that the compiler package exposes platform types and defaults.
func TestExportedPlatformTypes(t *testing.T) {
	req := compiler.GenerationRequest{
		Platform: &compiler.PlatformInfo{
			Pagination: compiler.PaginationDefaults{
				DefaultLimit: 20,
				MaxLimit:     100,
			},
			ReservedErrors: []compiler.ReservedError{
				{
					Name:    "NotFound",
					Kind:    compiler.ErrorKind("not_found"),
					Status:  404,
					Posture: compiler.ErrorPosture("business"),
				},
			},
		},
	}

	if req.Platform == nil {
		t.Fatal("expected platform info to be present")
	}
	if req.Platform.Pagination.DefaultLimit != 20 {
		t.Errorf("expected DefaultLimit 20, got %d", req.Platform.Pagination.DefaultLimit)
	}
	if len(req.Platform.ReservedErrors) != 1 {
		t.Fatalf("expected 1 reserved error, got %d", len(req.Platform.ReservedErrors))
	}
	if req.Platform.ReservedErrors[0].Name != "NotFound" {
		t.Errorf("expected error name NotFound, got %q", req.Platform.ReservedErrors[0].Name)
	}
}

// TestExportedDiagnosticRegistry confirms that the compiler package exposes diagnostic registry helpers.
func TestExportedDiagnosticRegistry(t *testing.T) {
	if len(compiler.DiagnosticRegistry) == 0 {
		t.Fatal("expected non-empty DiagnosticRegistry")
	}

	def, ok := compiler.LookupDiagnostic("FDN0001")
	if !ok || def.Code != "FDN0001" {
		t.Fatalf("expected FDN0001 definition, got: %+v", def)
	}

	diag := compiler.NewDiagnostic("FDN0001", compiler.Span{File: "test.fdn", Line: 1, Column: 1}, "disk error")
	if diag.Code != "FDN0001" || diag.Severity != compiler.SeverityError {
		t.Fatalf("unexpected diagnostic: %+v", diag)
	}

	detailDiag := compiler.NewDiagnosticWithDetail("FDN0227", compiler.Span{File: "test.fdn", Line: 1, Column: 1}, "custom warning")
	if detailDiag.Code != "FDN0227" || detailDiag.Severity != compiler.SeverityWarning || detailDiag.Message != "custom warning" {
		t.Fatalf("unexpected detail diagnostic: %+v", detailDiag)
	}
}

// TestCaptureRequest confirms that CaptureRequest creates valid generation request envelopes.
func TestCaptureRequest(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Product {
  id: uuid @pk
  name: string
  price: decimal
}

action CreateProduct(name: string, price: decimal) -> Product
  @http(method: POST, path: "/v1/products")
{
  let p = create Product { name: name, price: price }
  return p
}
`
	buildProfile := `
project:
  name: store
  module: example.com/store

storage:
  dialect: postgres
  mode: declarative

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: ./dist/backend
  - name: client
    kind: sdk
    language: go
    architecture: sdk
    out_dir: ./dist/sdk
`
	if err := os.WriteFile(filepath.Join(dir, "store.fdn"), []byte(blueprint), 0o644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0o644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	t.Run("default target", func(t *testing.T) {
		data, diags := compiler.CaptureRequest(dir, "")
		if compiler.HasErrors(diags) {
			t.Fatalf("unexpected capture error: %+v", diags)
		}

		req, err := compiler.DecodeGenerationRequest(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("failed to decode generation request: %v", err)
		}

		if req.ProtocolVersion != compiler.ProtocolVersion {
			t.Errorf("expected protocol version %d, got %d", compiler.ProtocolVersion, req.ProtocolVersion)
		}
		if req.WorkspaceRoot != dir {
			t.Errorf("expected workspace root %q, got %q", dir, req.WorkspaceRoot)
		}
		if req.Target.Name != "backend" {
			t.Errorf("expected target backend, got %q", req.Target.Name)
		}
		if req.Target.Kind != "backend" {
			t.Errorf("expected kind backend, got %q", req.Target.Kind)
		}
		if req.SemanticIR == nil || len(req.SemanticIR.Entities) != 1 {
			t.Fatalf("expected semantic IR with 1 entity, got: %+v", req.SemanticIR)
		}
		if req.Platform == nil || req.Platform.Pagination.DefaultLimit == 0 {
			t.Fatalf("expected platform facts, got: %+v", req.Platform)
		}
		if req.ArchitectureIR == nil {
			t.Fatal("expected architecture IR to be populated")
		}
		if req.ArchitectureIR.Storage.Dialect != "postgres" {
			t.Errorf("expected storage dialect postgres, got %q", req.ArchitectureIR.Storage.Dialect)
		}
		if len(req.ArchitectureIR.Storage.Tables) == 0 {
			t.Error("expected lowered storage tables")
		}
		if len(req.Files) == 0 {
			t.Error("expected files map from builtin emitter")
		}
	})

	t.Run("explicit sdk target", func(t *testing.T) {
		data, diags := compiler.CaptureRequest(dir, "client")
		if compiler.HasErrors(diags) {
			t.Fatalf("unexpected capture error: %+v", diags)
		}

		req, err := compiler.DecodeGenerationRequest(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Target.Name != "client" || req.Target.Kind != "sdk" {
			t.Errorf("expected client sdk target, got %+v", req.Target)
		}
		if len(req.Files) == 0 {
			t.Error("expected sdk emitted files in files map")
		}
	})

	t.Run("missing target", func(t *testing.T) {
		_, diags := compiler.CaptureRequest(dir, "unknown")
		if !compiler.HasErrors(diags) {
			t.Fatal("expected error diagnostic for unknown target")
		}
		if diags[0].Code != "FDN0302" {
			t.Errorf("expected diagnostic FDN0302, got %s", diags[0].Code)
		}
	})

	t.Run("invalid blueprint dir", func(t *testing.T) {
		emptyDir := t.TempDir()
		_, diags := compiler.CaptureRequest(emptyDir, "")
		if !compiler.HasErrors(diags) {
			t.Fatal("expected error diagnostic for empty directory")
		}
	})
}
