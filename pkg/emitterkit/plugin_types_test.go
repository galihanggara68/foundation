package emitterkit_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
)

// TestExportedTypesInEmitterkit verifies that emitterkit re-exports all compiler types.
func TestExportedTypesInEmitterkit(t *testing.T) {
	sem := emitterkit.SemanticIR{
		Actors: []emitterkit.ActorDecl{{Name: "User"}},
		Slots: []emitterkit.SlotDecl{{
			Name: "Notifier",
			Methods: []emitterkit.SlotMethodDecl{{
				Name:       "Send",
				Params:     []emitterkit.ParamDecl{{Name: "to", Type: "string"}},
				ReturnType: "bool",
			}},
		}},
		Enums:  []emitterkit.EnumDecl{{Name: "Status", Variants: []string{"Active"}}},
		Events: []emitterkit.EventDecl{{Name: "UserCreated"}},
		Policies: []emitterkit.PolicyDecl{{
			Name:   "OwnerOnly",
			Params: []emitterkit.ParamDecl{{Name: "actor", Type: "string"}},
			Body:   emitterkit.Expr{Kind: emitterkit.ExprKind("ident"), Value: "true"},
		}},
		Values: []emitterkit.ValueDecl{{
			Name:   "Money",
			Fields: []emitterkit.FieldDecl{{Name: "amount", Type: "decimal"}},
		}},
		Entities: []emitterkit.EntityDecl{{
			Name: "Account",
			Fields: []emitterkit.FieldDecl{{
				Name: "id",
				Type: "uuid",
				Kind: emitterkit.FieldKind("uuid"),
				Traits: []emitterkit.TraitDecl{{
					Name: "pk",
					Args: map[string]string{"_": "true"},
				}},
			}},
		}},
		Errors: []emitterkit.ErrorDecl{{
			Name:    "AccountNotFound",
			Kind:    emitterkit.ErrorKind("not_found"),
			Posture: emitterkit.ErrorPosture("operational"),
			Message: "account not found",
		}},
		Actions: []emitterkit.ActionDecl{{
			Name:   "CreateAccount",
			Params: []emitterkit.ParamDecl{{Name: "name", Type: "string"}},
			Steps: []emitterkit.StepDecl{{
				Kind: emitterkit.StepKind("let"),
				Expr: &emitterkit.Expr{
					Kind: emitterkit.ExprKind("ident"),
					Span: emitterkit.Span{File: "main.fdn", Line: 10, Column: 4},
				},
				Fields: []emitterkit.FieldInitDecl{{
					Name:  "balance",
					Value: emitterkit.Expr{Kind: emitterkit.ExprKind("literal")},
				}},
				MatchArms: []emitterkit.MatchArmDecl{{
					Variant: "ok",
					Steps:   []emitterkit.StepDecl{{Kind: emitterkit.StepKind("return")}},
				}},
				WherePredicates: []emitterkit.WherePredicate{{
					Field: "status",
					Op:    "=",
					Value: emitterkit.Expr{Kind: emitterkit.ExprKind("ident")},
				}},
			}},
		}},
		Scenarios: []emitterkit.ScenarioDecl{{
			Name: "SuccessPath",
			Givens: []emitterkit.GivenSeedDecl{{
				EntityName: "Account",
				Fields: []emitterkit.FieldInitDecl{{
					Name:  "name",
					Value: emitterkit.Expr{Kind: emitterkit.ExprKind("literal"), Value: "test"},
				}},
			}},
			GivenSlots: []emitterkit.GivenSlotDecl{{
				SlotName:   "Notifier",
				MethodName: "Send",
			}},
			Expects: []emitterkit.ExpectDecl{{
				Left:  emitterkit.Expr{Kind: emitterkit.ExprKind("ident"), Value: "status"},
				Op:    "==",
				Right: emitterkit.Expr{Kind: emitterkit.ExprKind("literal"), Value: "200"},
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

	diag := emitterkit.Diagnostic{
		Code:     "FDN0001",
		Severity: emitterkit.SeverityError,
		Message:  "fatal test error",
		Span: emitterkit.Span{
			File:   "test.fdn",
			Line:   1,
			Column: 1,
		},
	}
	if diag.Severity != emitterkit.SeverityError {
		t.Errorf("expected SeverityError, got %q", diag.Severity)
	}
	if emitterkit.SeverityWarning != "warning" {
		t.Errorf("expected SeverityWarning 'warning', got %q", emitterkit.SeverityWarning)
	}
	if emitterkit.SeverityInfo != "info" {
		t.Errorf("expected SeverityInfo 'info', got %q", emitterkit.SeverityInfo)
	}

	plan := emitterkit.StoragePlanIR{
		Dialect:   "postgres",
		Mode:      "declarative",
		SchemaSQL: "CREATE TABLE test ();",
		Migrations: []emitterkit.MigrationPlan{{
			Number:  1,
			Name:    "0001_initial",
			UpSQL:   "CREATE TABLE test ();",
			DownSQL: "DROP TABLE test;",
		}},
		Tables: []emitterkit.TableSchema{{
			Name: "users",
			Columns: []emitterkit.ColumnSchema{{
				Name:       "id",
				Type:       "UUID",
				IsPK:       true,
				IsNullable: false,
			}},
			PrimaryKeys: []string{"id"},
			ForeignKeys: []emitterkit.ForeignKey{{
				Column:        "org_id",
				ForeignTable:  "organizations",
				ForeignColumn: "id",
			}},
			Indexes: []emitterkit.IndexSchema{{
				Name:     "idx_users_email",
				Table:    "users",
				Columns:  []string{"email"},
				IsUnique: true,
			}},
			Uniques: []emitterkit.UniqueConstraint{{
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

// TestExportedPlatformTypesInEmitterkit confirms that emitterkit exposes platform types and defaults.
func TestExportedPlatformTypesInEmitterkit(t *testing.T) {
	req := emitterkit.GenerationRequest{
		Platform: &emitterkit.PlatformInfo{
			Pagination: emitterkit.PaginationDefaults{
				DefaultLimit: 20,
				MaxLimit:     100,
			},
			ReservedErrors: []emitterkit.ReservedError{
				{
					Name:    "NotFound",
					Kind:    emitterkit.ErrorKind("not_found"),
					Status:  404,
					Posture: emitterkit.ErrorPosture("business"),
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

// TestExportedDiagnosticRegistryInEmitterkit confirms that emitterkit exposes diagnostic registry helpers.
func TestExportedDiagnosticRegistryInEmitterkit(t *testing.T) {
	if len(emitterkit.DiagnosticRegistry) == 0 {
		t.Fatal("expected non-empty DiagnosticRegistry in emitterkit")
	}

	def, ok := emitterkit.LookupDiagnostic("FDN0001")
	if !ok || def.Code != "FDN0001" {
		t.Fatalf("expected FDN0001 definition, got: %+v", def)
	}

	diag := emitterkit.NewDiagnostic("FDN0001", emitterkit.Span{File: "test.fdn", Line: 1, Column: 1}, "disk error")
	if diag.Code != "FDN0001" || diag.Severity != emitterkit.SeverityError {
		t.Fatalf("unexpected diagnostic: %+v", diag)
	}

	detailDiag := emitterkit.NewDiagnosticWithDetail("FDN0227", emitterkit.Span{File: "test.fdn", Line: 1, Column: 1}, "custom warning")
	if detailDiag.Code != "FDN0227" || detailDiag.Severity != emitterkit.SeverityWarning || detailDiag.Message != "custom warning" {
		t.Fatalf("unexpected detail diagnostic: %+v", detailDiag)
	}
}
