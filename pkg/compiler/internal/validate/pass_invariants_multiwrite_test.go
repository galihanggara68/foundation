package validate_test

import (
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

const multiWriteBlueprintPrelude = `
entity Account {
  id: uuid @pk
  balance: int
}

entity TransferRecord {
  id: uuid @pk
  amount: int
}

slot PaymentGateway {
  Charge(amount: int) -> bool
}

enum Status {
  Active
  Inactive
}
`

// TestValidate_MultiWrite_SequentialWrites_EmitsFDN0227 tests that sequential writes emit FDN0227.
func TestValidate_MultiWrite_SequentialWrites_EmitsFDN0227(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action TransferFunds(acc_id: uuid, rec_id: uuid, amt: int) -> int {
  update Account where id == acc_id set {
    balance: amt
  }
  let rec = create TransferRecord {
    id: rec_id,
    amount: amt
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	diag := findCode(diags, "FDN0227")
	if diag == nil {
		t.Fatalf("expected FDN0227 warning for sequential writes, got none. All diags: %+v", diags)
	}
	if diag.Severity != ir.SeverityWarning {
		t.Errorf("expected FDN0227 severity to be warning, got %s", diag.Severity)
	}
	wantMsg := `action "TransferFunds" performs multiple write operations (update, create) along an execution path without @transactional; wrap the action in @transactional to guarantee atomicity`
	if diag.Message != wantMsg {
		t.Errorf("diagnostic message mismatch.\nwant: %s\ngot:  %s", wantMsg, diag.Message)
	}
}

// TestValidate_MultiWrite_SameEntity_EmitsFDN0227 tests multiple writes to the same entity.
func TestValidate_MultiWrite_SameEntity_EmitsFDN0227(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action DoubleUpdate(acc_id: uuid, amt: int) -> int {
  update Account where id == acc_id set {
    balance: amt
  }
  update Account where id == acc_id set {
    balance: amt
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	diag := findCode(diags, "FDN0227")
	if diag == nil {
		t.Fatalf("expected FDN0227 warning for multiple writes to same entity, got none. All diags: %+v", diags)
	}
	if diag.Severity != ir.SeverityWarning {
		t.Errorf("expected FDN0227 severity to be warning, got %s", diag.Severity)
	}
	if !strings.Contains(diag.Message, "(update, update)") {
		t.Errorf("expected diagnostic message to list (update, update), got: %s", diag.Message)
	}
}

// TestValidate_MultiWrite_WithTransactional_NoWarning tests that @transactional suppresses FDN0227.
func TestValidate_MultiWrite_WithTransactional_NoWarning(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action TransferFundsTx(acc_id: uuid, rec_id: uuid, amt: int) -> int @transactional {
  update Account where id == acc_id set {
    balance: amt
  }
  let rec = create TransferRecord {
    id: rec_id,
    amount: amt
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	if diag := findCode(diags, "FDN0227"); diag != nil {
		t.Errorf("expected no FDN0227 warning for @transactional action, got: %+v", diag)
	}
}

// TestValidate_MultiWrite_IfElse_SingleWriteEach_NoWarning tests single writes in if/else branches.
func TestValidate_MultiWrite_IfElse_SingleWriteEach_NoWarning(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action ProcessConditional(cond: bool, acc_id: uuid, rec_id: uuid, amt: int) -> int {
  if cond {
    update Account where id == acc_id set {
      balance: amt
    }
  } else {
    let rec = create TransferRecord {
      id: rec_id,
      amount: amt
    }
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	if diag := findCode(diags, "FDN0227"); diag != nil {
		t.Errorf("expected no FDN0227 warning for single write per branch, got: %+v", diag)
	}
}

// TestValidate_MultiWrite_If_MultipleWritesInThen_EmitsFDN0227 tests two writes in then-branch.
func TestValidate_MultiWrite_If_MultipleWritesInThen_EmitsFDN0227(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action ProcessConditionalMulti(cond: bool, acc_id: uuid, rec_id: uuid, amt: int) -> int {
  if cond {
    update Account where id == acc_id set {
      balance: amt
    }
    let rec = create TransferRecord {
      id: rec_id,
      amount: amt
    }
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	diag := findCode(diags, "FDN0227")
	if diag == nil {
		t.Fatalf("expected FDN0227 warning for two writes in then-branch, got none. All diags: %+v", diags)
	}
	if diag.Severity != ir.SeverityWarning {
		t.Errorf("expected FDN0227 severity to be warning, got %s", diag.Severity)
	}
}

// TestValidate_MultiWrite_SingleWrite_NoWarning tests an action with only one write.
func TestValidate_MultiWrite_SingleWrite_NoWarning(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action SingleWrite(acc_id: uuid, amt: int) -> int {
  update Account where id == acc_id set {
    balance: amt
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	if diag := findCode(diags, "FDN0227"); diag != nil {
		t.Errorf("expected no FDN0227 warning for single write, got: %+v", diag)
	}
}

// TestValidate_MultiWrite_SlotCallAndSingleWrite_NoWarning tests slot calls do not increment write count.
func TestValidate_MultiWrite_SlotCallAndSingleWrite_NoWarning(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action SlotAndWrite(acc_id: uuid, amt: int) -> int {
  let ok = slot PaymentGateway.Charge(amt)
  update Account where id == acc_id set {
    balance: amt
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	if diag := findCode(diags, "FDN0227"); diag != nil {
		t.Errorf("expected no FDN0227 warning for slot call and single write, got: %+v", diag)
	}
}

// TestValidate_MultiWrite_MatchArms_SingleWriteEach_NoWarning tests single writes in match arms.
func TestValidate_MultiWrite_MatchArms_SingleWriteEach_NoWarning(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action MatchSingle(status: Status, acc_id: uuid, rec_id: uuid, amt: int) -> int {
  match status {
    .Active => {
      update Account where id == acc_id set {
        balance: amt
      }
    }
    .Inactive => {
      let rec = create TransferRecord {
        id: rec_id,
        amount: amt
      }
    }
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	if diag := findCode(diags, "FDN0227"); diag != nil {
		t.Errorf("expected no FDN0227 warning for single write per match arm, got: %+v", diag)
	}
}

// TestValidate_MultiWrite_LoopWithWrite_EmitsFDN0227 tests that a loop with a write emits FDN0227.
func TestValidate_MultiWrite_LoopWithWrite_EmitsFDN0227(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action LoopWrite(ids: []uuid, amt: int) -> int {
  for acc_id in ids {
    update Account where id == acc_id set {
      balance: amt
    }
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	diag := findCode(diags, "FDN0227")
	if diag == nil {
		t.Fatalf("expected FDN0227 warning for loop with write step, got none. All diags: %+v", diags)
	}
	if diag.Severity != ir.SeverityWarning {
		t.Errorf("expected FDN0227 severity to be warning, got %s", diag.Severity)
	}
	if !strings.Contains(diag.Message, "(update)") || strings.Contains(diag.Message, "update, update") {
		t.Errorf("expected single write name in loop message '(update)', got: %s", diag.Message)
	}
}

// TestValidate_MultiWrite_ZeroIterationLoopPreservesSubsequentWrites tests that writes before and after a loop trigger FDN0227.
func TestValidate_MultiWrite_ZeroIterationLoopPreservesSubsequentWrites(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action LoopZeroPath(ids: []uuid, acc_id: uuid, rec_id: uuid, amt: int) -> int {
  update Account where id == acc_id set {
    balance: amt
  }
  for id in ids {
    let x = 1
  }
  let rec = create TransferRecord {
    id: rec_id,
    amount: amt
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	diag := findCode(diags, "FDN0227")
	if diag == nil {
		t.Fatalf("expected FDN0227 warning for writes before and after loop, got none. All diags: %+v", diags)
	}
}

// TestValidate_MultiWrite_Match_MultipleWritesInArm_EmitsFDN0227 tests two writes in a match arm.
func TestValidate_MultiWrite_Match_MultipleWritesInArm_EmitsFDN0227(t *testing.T) {
	src := multiWriteBlueprintPrelude + `
action MatchMulti(status: Status, acc_id: uuid, rec_id: uuid, amt: int) -> int {
  match status {
    .Active => {
      update Account where id == acc_id set {
        balance: amt
      }
      let rec = create TransferRecord {
        id: rec_id,
        amount: amt
      }
    }
    .Inactive => {
      return amt
    }
  }
  return amt
}
`
	_, diags := parseAndValidate(t, src)
	diag := findCode(diags, "FDN0227")
	if diag == nil {
		t.Fatalf("expected FDN0227 warning for two writes in match arm, got none. All diags: %+v", diags)
	}
	if diag.Severity != ir.SeverityWarning {
		t.Errorf("expected FDN0227 severity to be warning, got %s", diag.Severity)
	}
}

