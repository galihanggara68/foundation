package validate_test

import (
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/validate"
)

func parseAndValidateBlueprint(t *testing.T, src string) []ir.Diagnostic {
	t.Helper()
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}
	return validate.Validate(sem)
}

func filterDiagnosticsByCode(diags []ir.Diagnostic, code string) []ir.Diagnostic {
	var out []ir.Diagnostic
	for _, d := range diags {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

func TestValidate_UnusedLetVariable_EmitsWarningFDN0228(t *testing.T) {
	src := `
error NotFound {
  kind: not_found
  message: "not found"
}

action Compute() -> int {
  let unused = 42
  return 0
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 1 {
		t.Fatalf("expected 1 FDN0228 diagnostic, got %d: %+v", len(unusedDiags), unusedDiags)
	}

	diag := unusedDiags[0]
	if diag.Severity != ir.SeverityWarning {
		t.Errorf("expected warning severity, got %v", diag.Severity)
	}

	want := `variable "unused" declared in action "Compute" is never read; remove the declaration or prefix with '_' to ignore`
	if diag.Message != want {
		t.Errorf("expected message %q, got %q", want, diag.Message)
	}
}

func TestValidate_UnusedQueryOneVariable_EmitsWarningFDN0228(t *testing.T) {
	src := `
entity Dataset {
  id: uuid @pk
  name: string
}

error NotFound {
  kind: not_found
  message: "not found"
}

action FetchData(id: uuid) -> int {
  let d = query one Dataset where id == id else NotFound
  return 0
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 1 {
		t.Fatalf("expected 1 FDN0228 diagnostic, got %d: %+v", len(unusedDiags), unusedDiags)
	}

	diag := unusedDiags[0]
	if diag.Severity != ir.SeverityWarning {
		t.Errorf("expected warning severity, got %v", diag.Severity)
	}

	want := `variable "d" declared in action "FetchData" is never read; remove the declaration or prefix with '_' to ignore`
	if diag.Message != want {
		t.Errorf("expected message %q, got %q", want, diag.Message)
	}
}

func TestValidate_UnusedActionParameter_EmitsWarningFDN0228(t *testing.T) {
	src := `
action Process(active: bool, unusedParam: string) -> bool {
  return active
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 1 {
		t.Fatalf("expected 1 FDN0228 diagnostic, got %d: %+v", len(unusedDiags), unusedDiags)
	}

	diag := unusedDiags[0]
	if diag.Severity != ir.SeverityWarning {
		t.Errorf("expected warning severity, got %v", diag.Severity)
	}

	want := `action parameter "unusedParam" in action "Process" is never read; prefix with '_' if intentionally unused`
	if diag.Message != want {
		t.Errorf("expected message %q, got %q", want, diag.Message)
	}
}

func TestValidate_UnderscoreSuppression_EmitsNoWarning(t *testing.T) {
	src := `
entity Dataset {
  id: uuid @pk
}

error NotFound {
  kind: not_found
  message: "not found"
}

action RunAudit(_ctx: string, id: uuid) -> int {
  let _temp = 100
  let _audit = query one Dataset where id == id else NotFound
  return 0
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 0 {
		t.Errorf("expected 0 FDN0228 diagnostics with underscore prefix, got %d: %+v", len(unusedDiags), unusedDiags)
	}
}

func TestValidate_SideEffectingStep_Create_EmitsSideEffectWarning(t *testing.T) {
	src := `
entity User {
  id: uuid @pk
  name: string
}

action Register(name: string) -> int {
  let u = create User {
    name: name
  }
  return 0
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 1 {
		t.Fatalf("expected 1 FDN0228 diagnostic, got %d: %+v", len(unusedDiags), unusedDiags)
	}

	diag := unusedDiags[0]
	if diag.Severity != ir.SeverityWarning {
		t.Errorf("expected warning severity, got %v", diag.Severity)
	}

	want := `variable "u" declared in action "Register" is never read, but its initializer performs a side effect (create); discard the binding with '_' or remove the let prefix`
	if diag.Message != want {
		t.Errorf("expected message %q, got %q", want, diag.Message)
	}
}

func TestValidate_SideEffectingStep_Slot_EmitsSideEffectWarning(t *testing.T) {
	src := `
slot Payment {
  @external
  Charge(amount: int) -> bool throws ChargeFailed
}

error ChargeFailed {
  kind: conflict
  message: "failed"
}

action Pay(amount: int) -> int {
  let res = slot Payment.Charge(amount) else ChargeFailed
  return 0
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 1 {
		t.Fatalf("expected 1 FDN0228 diagnostic, got %d: %+v", len(unusedDiags), unusedDiags)
	}

	diag := unusedDiags[0]
	if diag.Severity != ir.SeverityWarning {
		t.Errorf("expected warning severity, got %v", diag.Severity)
	}

	want := `variable "res" declared in action "Pay" is never read, but its initializer performs a side effect (slot); discard the binding with '_' or remove the let prefix`
	if diag.Message != want {
		t.Errorf("expected message %q, got %q", want, diag.Message)
	}
}

func TestValidate_MemberAccess_MarksVariableUsed(t *testing.T) {
	src := `
entity User {
  id: uuid @pk
  name: string
}

error NotFound {
  kind: not_found
  message: "not found"
}

action GetUserName(user_id: uuid) -> string {
  let user = query one User where id == user_id else NotFound
  return user.name
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 0 {
		t.Errorf("expected 0 FDN0228 diagnostics for referenced member, got %d: %+v", len(unusedDiags), unusedDiags)
	}
}

func TestValidate_LoopIterVar_UnusedAndSuppressed(t *testing.T) {
	src := `
event Logged {
  msg: string
}

action ProcessItems(items: []string) -> int {
  for item in items {
    emit Logged {
      msg: "constant"
    }
  }
  for _ignored in items {
    emit Logged {
      msg: "other"
    }
  }
  return 0
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 1 {
		t.Fatalf("expected 1 FDN0228 diagnostic for loop variable, got %d: %+v", len(unusedDiags), unusedDiags)
	}

	diag := unusedDiags[0]
	want := `variable "item" declared in action "ProcessItems" is never read; remove the declaration or prefix with '_' to ignore`
	if diag.Message != want {
		t.Errorf("expected message %q, got %q", want, diag.Message)
	}
	if strings.Contains(diag.Message, "_ignored") {
		t.Errorf("wildcard variable _ignored should be suppressed")
	}
}

func TestValidate_WhereClauseEntityField_DoesNotMarkParameterUsed(t *testing.T) {
	src := `
entity User {
  id: uuid @pk
  status: string
}

error UserNotFound {
  kind: not_found
  message: "user not found"
}

action FindUser(id: uuid, status: string) -> User {
  let u = query one User where id == id else UserNotFound
  return u
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 1 {
		t.Fatalf("expected 1 FDN0228 diagnostic for unused parameter 'status', got %d: %+v", len(unusedDiags), unusedDiags)
	}
	if !strings.Contains(unusedDiags[0].Message, `action parameter "status" in action "FindUser" is never read`) {
		t.Errorf("expected warning for status parameter, got %s", unusedDiags[0].Message)
	}
}

func TestValidate_InnerScopeUnusedLocalVariable_EmitsWarning(t *testing.T) {
	src := `
action Compute(flag: bool) -> int {
  let result = 10
  if flag {
    let localUnused = 99
  }
  return result
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 1 {
		t.Fatalf("expected 1 FDN0228 diagnostic for 'localUnused', got %d: %+v", len(unusedDiags), unusedDiags)
	}
	if !strings.Contains(unusedDiags[0].Message, `variable "localUnused" declared in action "Compute" is never read`) {
		t.Errorf("expected warning for localUnused, got %s", unusedDiags[0].Message)
	}
}

func TestValidate_ActionTraitKey_MarksParameterUsed(t *testing.T) {
	src := `
entity Account {
  id: uuid @pk
  balance: int
}

action Deposit(id: uuid, amount: int, request_id: string) -> bool
  @idempotent(key: request_id)
{
  update Account where id == id set {
    balance: amount
  }
  return true
}
`
	diags := parseAndValidateBlueprint(t, src)
	unusedDiags := filterDiagnosticsByCode(diags, "FDN0228")

	if len(unusedDiags) != 0 {
		t.Fatalf("expected 0 FDN0228 diagnostics because request_id is used in @idempotent trait, got: %+v", unusedDiags)
	}
}
