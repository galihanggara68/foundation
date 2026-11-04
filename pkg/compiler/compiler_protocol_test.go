package compiler_test

import (
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
			Name: "Money",
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
}
