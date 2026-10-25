package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCheck_QueryManyMissingOrderBy_ReturnsFDN0215(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Order {
  id: uuid @pk
  status: string
}

action ListOrders() -> []Order {
  let orders = query many Order limit 20
  return orders
}
`
	if err := os.WriteFile(filepath.Join(dir, "orders.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	semIR, diags := compiler.Check(dir)
	if semIR != nil {
		t.Fatalf("expected nil SemanticIR on missing order_by, got %+v", semIR)
	}

	foundFDN0215 := false
	for _, d := range diags {
		if d.Code == "FDN0215" {
			foundFDN0215 = true
			if !strings.Contains(d.Message, "order_by") {
				t.Errorf("expected diagnostic message to mention order_by, got: %q", d.Message)
			}
		}
	}
	if !foundFDN0215 {
		t.Fatalf("expected diagnostic FDN0215 for missing order_by, got diagnostics: %+v", diags)
	}
}

func TestCheck_QueryManyNonUniqueOrderBy_ReturnsFDN0215(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Order {
  id: uuid @pk
  status: string
}

action ListOrders() -> []Order {
  let orders = query many Order order_by status asc limit 20
  return orders
}
`
	if err := os.WriteFile(filepath.Join(dir, "orders.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	semIR, diags := compiler.Check(dir)
	if semIR != nil {
		t.Fatalf("expected nil SemanticIR on non-unique order_by, got %+v", semIR)
	}

	foundFDN0215 := false
	for _, d := range diags {
		if d.Code == "FDN0215" {
			foundFDN0215 = true
			if !strings.Contains(d.Message, "status") || !strings.Contains(d.Message, "unique") {
				t.Errorf("expected diagnostic message to mention non-unique field, got: %q", d.Message)
			}
		}
	}
	if !foundFDN0215 {
		t.Fatalf("expected diagnostic FDN0215 for non-unique order_by, got diagnostics: %+v", diags)
	}
}

func TestCheck_QueryManyMissingLimit_ReturnsFDN0216(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Order {
  id: uuid @pk
  status: string
}

action ListOrders() -> []Order {
  let orders = query many Order order_by id asc
  return orders
}
`
	if err := os.WriteFile(filepath.Join(dir, "orders.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	semIR, diags := compiler.Check(dir)
	if semIR != nil {
		t.Fatalf("expected nil SemanticIR on missing limit, got %+v", semIR)
	}

	foundFDN0216 := false
	for _, d := range diags {
		if d.Code == "FDN0216" {
			foundFDN0216 = true
			if !strings.Contains(d.Message, "limit") {
				t.Errorf("expected diagnostic message to mention limit, got: %q", d.Message)
			}
		}
	}
	if !foundFDN0216 {
		t.Fatalf("expected diagnostic FDN0216 for missing limit, got diagnostics: %+v", diags)
	}
}

func TestCheck_QueryManyLimitCapExceeded_ReturnsFDN0216(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Order {
  id: uuid @pk
  status: string
}

action ListOrders() -> []Order {
  let orders = query many Order order_by id asc limit 101
  return orders
}
`
	if err := os.WriteFile(filepath.Join(dir, "orders.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	semIR, diags := compiler.Check(dir)
	if semIR != nil {
		t.Fatalf("expected nil SemanticIR on limit exceeding 100, got %+v", semIR)
	}

	foundFDN0216 := false
	for _, d := range diags {
		if d.Code == "FDN0216" {
			foundFDN0216 = true
			if !strings.Contains(d.Message, "100") {
				t.Errorf("expected diagnostic message to mention 100 cap, got: %q", d.Message)
			}
		}
	}
	if !foundFDN0216 {
		t.Fatalf("expected diagnostic FDN0216 for limit exceeding 100, got diagnostics: %+v", diags)
	}
}

func TestCheck_QueryManyDisjunction_ReturnsFDN0217(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Order {
  id: uuid @pk
  status: string
  total: decimal
}

action ListOrders() -> []Order {
  let orders = query many Order where status == "active" or total > 10.0 order_by id asc limit 20
  return orders
}
`
	if err := os.WriteFile(filepath.Join(dir, "orders.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	semIR, diags := compiler.Check(dir)
	if semIR != nil {
		t.Fatalf("expected nil SemanticIR on disjunction in query many, got %+v", semIR)
	}

	foundFDN0217 := false
	for _, d := range diags {
		if d.Code == "FDN0217" {
			foundFDN0217 = true
		}
	}
	if !foundFDN0217 {
		t.Fatalf("expected diagnostic FDN0217 for disjunction in query many, got diagnostics: %+v", diags)
	}
}

func TestCheck_QueryManyValid_PassesValidation(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Order {
  id: uuid @pk
  customer_id: uuid
  status: string
}

action ListOrders(customer_id: uuid, cur: string) -> []Order {
  let orders = query many Order where customer_id == customer_id and status == "active" order_by id asc limit 20 cursor cur
  return orders
}
`
	if err := os.WriteFile(filepath.Join(dir, "orders.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	semIR, diags := compiler.Check(dir)
	if len(diags) > 0 {
		t.Fatalf("unexpected validation diagnostics: %+v", diags)
	}
	if semIR == nil {
		t.Fatal("expected non-nil SemanticIR")
	}
}
