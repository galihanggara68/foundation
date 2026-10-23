package gohex

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitizeTestFuncName(name string) string {
	parts := nonAlphaNum.Split(name, -1)
	var sb strings.Builder
	sb.WriteString("TestScenario_")
	for _, p := range parts {
		if p == "" {
			continue
		}
		sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return sb.String()
}

func (lay goLayout) emitGoScenariosTest(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package scenarios_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"%s/internal/bootstrap"
	"%s/%s"
)

`, archIR.Module, archIR.Module, lay.DomainDir))

	for _, sc := range archIR.Scenarios {
		fnName := sanitizeTestFuncName(sc.Name)
		sb.WriteString(fmt.Sprintf("func %s(t *testing.T) {\n", fnName))
		sb.WriteString("\tctx := context.Background()\n")
		sb.WriteString("\tc := bootstrap.NewTestContainer()\n")

		for _, g := range sc.Givens {
			if g.EntityName == "actor" {
				sb.WriteString(fmt.Sprintf("\tctx = %s.WithActor(ctx, %s.Actor{\n", lay.DomainPkg, lay.DomainPkg))
				lay.emitGoFieldInits(&sb, g.Fields, "\t\t")
				sb.WriteString("\t})\n")
				continue
			}
			sb.WriteString(fmt.Sprintf("\tc.%sRepo.Seed(%s.%s{\n", g.EntityName, lay.DomainPkg, g.EntityName))
			lay.emitGoFieldInits(&sb, g.Fields, "\t\t")
			sb.WriteString("\t})\n")
		}

		for _, gs := range sc.GivenSlots {
			var stubArgs []string
			for _, arg := range gs.Args {
				stubArgs = append(stubArgs, lay.emitGoExpr(arg, false))
			}
			if gs.ReturnExpr != nil {
				retExpr := lay.emitGoExpr(*gs.ReturnExpr, false)
				if len(stubArgs) > 0 {
					stubArgs = append(stubArgs, retExpr)
					sb.WriteString(fmt.Sprintf("\tc.%sSlot.Stub%sReturn(%s)\n", gs.SlotName, gs.MethodName, strings.Join(stubArgs, ", ")))
				} else {
					sb.WriteString(fmt.Sprintf("\tc.%sSlot.Stub%sReturnAny(%s)\n", gs.SlotName, gs.MethodName, retExpr))
				}
			} else if gs.ThrowsError != "" {
				errExpr := lay.DomainPkg + ".Err" + gs.ThrowsError
				if len(stubArgs) > 0 {
					stubArgs = append(stubArgs, errExpr)
					sb.WriteString(fmt.Sprintf("\tc.%sSlot.Stub%sError(%s)\n", gs.SlotName, gs.MethodName, strings.Join(stubArgs, ", ")))
				} else {
					sb.WriteString(fmt.Sprintf("\tc.%sSlot.Stub%sErrorAny(%s)\n", gs.SlotName, gs.MethodName, errExpr))
				}
			}
		}

		var callArgs []string
		callArgs = append(callArgs, "ctx")
		for _, arg := range sc.WhenArgs {
			callArgs = append(callArgs, lay.emitGoExpr(arg, false))
		}
		sb.WriteString(fmt.Sprintf("\tresult, err := c.%s.Execute(%s)\n", sc.WhenAction, strings.Join(callArgs, ", ")))

		expectsError := false
		for _, ex := range sc.Expects {
			if ex.ErrorRaised != "" {
				expectsError = true
				sb.WriteString("\t_ = result\n")
				sb.WriteString(fmt.Sprintf("\tif !errors.Is(err, %s.Err%s) {\n\t\tt.Fatalf(\"expected error %s, got %%v\", err)\n\t}\n",
					lay.DomainPkg, ex.ErrorRaised, ex.ErrorRaised))
			}
		}

		if !expectsError {
			sb.WriteString("\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n")
			sb.WriteString("\t_ = result\n")
			for idx, ex := range sc.Expects {
				lay.emitGoScenarioExpect(&sb, idx, ex)
			}
		}

		sb.WriteString("}\n\n")
	}

	sb.WriteString(fmt.Sprintf(`func TestHTTPAdapter_Integration(t *testing.T) {
	c := bootstrap.NewTestContainer()
	if c.HTTPServer == nil {
		t.Fatal("expected non-nil HTTPServer")
	}
	req := httptest.NewRequest(http.MethodGet, "/__healthz_not_registered", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	c.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown route, got %%d", rec.Code)
	}
	_ = json.Marshal
	_ = errors.Is
	_ = %s.DomainError{}
}
`, lay.DomainPkg))

	return sb.String()
}

func (lay goLayout) emitGoScenarioExpect(sb *strings.Builder, idx int, ex ir.ExpectDecl) {
	if ex.EmittedEvent != "" {
		evVar := fmt.Sprintf("emitted%d", idx)
		sb.WriteString(fmt.Sprintf("\t%s := c.Events.ByType(%q)\n", evVar, ex.EmittedEvent))
		sb.WriteString(fmt.Sprintf("\tif len(%s) == 0 {\n\t\tt.Fatalf(\"expected event %s to be emitted, but none were published\")\n\t}\n",
			evVar, ex.EmittedEvent))
		if len(ex.EmittedFields) > 0 {
			sb.WriteString(fmt.Sprintf("\tevPayload%d, ok := %s[len(%s)-1].(%s.%s)\n", idx, evVar, evVar, lay.DomainPkg, ex.EmittedEvent))
			sb.WriteString(fmt.Sprintf("\tif !ok {\n\t\tt.Fatalf(\"unexpected event payload type for %s\")\n\t}\n", ex.EmittedEvent))
			for _, fi := range ex.EmittedFields {
				fieldAccess := fmt.Sprintf("evPayload%d.%s", idx, arch.ExportIdent(fi.Name))
				expectedVal := lay.emitGoExpr(fi.Value, false)
				sb.WriteString(fmt.Sprintf("\tif %s != %s {\n\t\tt.Fatalf(\"event %s.%s mismatch: got %%v, want %%v\", %s, %s)\n\t}\n",
					fieldAccess, expectedVal, ex.EmittedEvent, fi.Name, fieldAccess, expectedVal))
			}
		}
		return
	}

	if ex.StoredEntity != "" && ex.WhereValue != nil {
		findMethod := "FindBy" + arch.ExportIdent(ex.WhereField)
		whereExpr := lay.emitGoExpr(*ex.WhereValue, false)
		storedVar := fmt.Sprintf("stored%d", idx)
		sb.WriteString(fmt.Sprintf("\t%s, err := c.%sRepo.%s(ctx, %s)\n", storedVar, ex.StoredEntity, findMethod, whereExpr))
		sb.WriteString("\tif err != nil {\n\t\tt.Fatalf(\"unexpected repo error: %v\", err)\n\t}\n")
		if ex.NotStored {
			sb.WriteString(fmt.Sprintf("\tif %s != nil {\n\t\tt.Fatalf(\"expected %s where %s == %%v to be deleted, but found %%+v\", %s, *%s)\n\t}\n",
				storedVar, ex.StoredEntity, ex.WhereField, whereExpr, storedVar))
			return
		}
		sb.WriteString(fmt.Sprintf("\tif %s == nil {\n\t\tt.Fatalf(\"expected %s where %s == %%v to be stored, but found nil\", %s)\n\t}\n",
			storedVar, ex.StoredEntity, ex.WhereField, whereExpr))
		for _, fi := range ex.StoredFields {
			fieldAccess := fmt.Sprintf("%s.%s", storedVar, arch.ExportIdent(fi.Name))
			expectedVal := lay.emitGoExpr(fi.Value, false)
			sb.WriteString(fmt.Sprintf("\tif %s != %s {\n\t\tt.Fatalf(\"stored %s.%s mismatch: got %%v, want %%v\", %s, %s)\n\t}\n",
				fieldAccess, expectedVal, ex.StoredEntity, fi.Name, fieldAccess, expectedVal))
		}
		return
	}

	leftStr := lay.emitGoExpr(ex.Left, false)
	rightStr := lay.emitGoExpr(ex.Right, false)
	sb.WriteString(fmt.Sprintf("\tif !(%s %s %s) {\n\t\tt.Fatalf(\"assertion failed: expected %%v %s %%v\", %s, %s)\n\t}\n",
		leftStr, ex.Op, rightStr, ex.Op, leftStr, rightStr))
}
