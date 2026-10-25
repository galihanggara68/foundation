package tsclean

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func emitTSContractsTest(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`import { test } from "node:test";
import * as assert from "node:assert/strict";
import * as domain from "../src/entities/domain.ts";
`)
	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("import { create%sSlot } from \"../src/slots/%s_stub.ts\";\n", sl.Name, strings.ToLower(sl.Name)))
	}
	sb.WriteString("\nvoid domain;\nvoid assert;\n\n")

	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("test(\"Contract: %sSlot\", async () => {\n", sl.Name))
		sb.WriteString(fmt.Sprintf("  const impl = create%sSlot();\n  assert.ok(impl, \"expected non-null %sSlot\");\n", sl.Name, sl.Name))
		for _, m := range sl.Methods {
			var args []string
			for _, p := range m.Params {
				args = append(args, tsZeroVal(p.Type, archIR, false))
			}
			sb.WriteString(fmt.Sprintf("  try {\n    await impl.%s(%s);\n  } catch (err) {\n",
				toCamelCase(m.Name), strings.Join(args, ", ")))
			if len(m.Throws) > 0 {
				var checks []string
				for _, th := range m.Throws {
					checks = append(checks, fmt.Sprintf("err === domain.Err%s", th))
				}
				sb.WriteString(fmt.Sprintf("    assert.ok(%s, `undeclared error thrown by %s.%s: ${String(err)}`);\n",
					strings.Join(checks, " || "), sl.Name, m.Name))
			} else {
				sb.WriteString(fmt.Sprintf("    assert.fail(`slot %s.%s declares no throws errors, but threw: ${String(err)}`);\n",
					sl.Name, m.Name))
			}
			sb.WriteString("  }\n")
		}
		sb.WriteString("});\n\n")
	}

	return sb.String()
}

func emitTSScenariosTest(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`import { test } from "node:test";
import * as assert from "node:assert/strict";
import * as domain from "../src/entities/domain.ts";
import { createTestContainer } from "../src/main/container.ts";

void domain;
void assert;

`)

	for _, sc := range archIR.Scenarios {
		sb.WriteString(fmt.Sprintf("test(%q, async () => {\n", sc.Name))
		sb.WriteString("  const c = createTestContainer();\n")
		sb.WriteString("  let actor: domain.Actor = { id: \"\", role: \"\", verified: false, is_verified: false };\n")

		for _, g := range sc.Givens {
			if g.EntityName == "actor" {
				sb.WriteString("  actor = {\n    ...actor,\n")
				emitTSFieldInits(&sb, g.Fields, "    ")
				sb.WriteString("  };\n")
				continue
			}
			sb.WriteString(fmt.Sprintf("  await c.%sRepo.create({\n", toCamelCase(g.EntityName)))
			emitTSFieldInits(&sb, g.Fields, "    ")
			sb.WriteString("  });\n")
		}

		for _, gs := range sc.GivenSlots {
			var stubArgs []string
			for _, arg := range gs.Args {
				stubArgs = append(stubArgs, emitTSExpr(arg, false))
			}
			if gs.ReturnExpr != nil {
				retExpr := emitTSExpr(*gs.ReturnExpr, false)
				if len(stubArgs) > 0 {
					stubArgs = append(stubArgs, retExpr)
					sb.WriteString(fmt.Sprintf("  c.%sSlot.stub%sReturn(%s);\n",
						toCamelCase(gs.SlotName), gs.MethodName, strings.Join(stubArgs, ", ")))
				} else {
					sb.WriteString(fmt.Sprintf("  c.%sSlot.stub%sReturnAny(%s);\n",
						toCamelCase(gs.SlotName), gs.MethodName, retExpr))
				}
			} else if gs.ThrowsError != "" {
				errExpr := "domain.Err" + gs.ThrowsError
				if len(stubArgs) > 0 {
					stubArgs = append(stubArgs, errExpr)
					sb.WriteString(fmt.Sprintf("  c.%sSlot.stub%sError(%s);\n",
						toCamelCase(gs.SlotName), gs.MethodName, strings.Join(stubArgs, ", ")))
				} else {
					sb.WriteString(fmt.Sprintf("  c.%sSlot.stub%sErrorAny(%s);\n",
						toCamelCase(gs.SlotName), gs.MethodName, errExpr))
				}
			}
		}

		var callArgs []string
		callArgs = append(callArgs, "actor")
		for _, arg := range sc.WhenArgs {
			callArgs = append(callArgs, emitTSExpr(arg, false))
		}

		expectsError := ""
		for _, ex := range sc.Expects {
			if ex.ErrorRaised != "" {
				expectsError = ex.ErrorRaised
			}
		}

		if expectsError != "" {
			sb.WriteString(fmt.Sprintf("  let caughtErr: unknown;\n  try {\n    await c.%s.execute(%s);\n  } catch (err) {\n    caughtErr = err;\n  }\n",
				toCamelCase(sc.WhenAction), strings.Join(callArgs, ", ")))
			sb.WriteString(fmt.Sprintf("  assert.strictEqual(caughtErr, domain.Err%s);\n", expectsError))
		} else {
			sb.WriteString(fmt.Sprintf("  const result = await c.%s.execute(%s);\n  void result;\n",
				toCamelCase(sc.WhenAction), strings.Join(callArgs, ", ")))
			for idx, ex := range sc.Expects {
				emitTSScenarioExpect(&sb, idx, ex)
			}
		}

		sb.WriteString("});\n\n")
	}

	return sb.String()
}

func emitTSScenarioExpect(sb *strings.Builder, idx int, ex ir.ExpectDecl) {
	if ex.EmittedEvent != "" {
		evVar := fmt.Sprintf("emitted%d", idx)
		sb.WriteString(fmt.Sprintf("  const %s = c.events.byType<domain.%s>(%q);\n", evVar, ex.EmittedEvent, ex.EmittedEvent))
		sb.WriteString(fmt.Sprintf("  assert.ok(%s.length > 0, \"expected event %s to be emitted\");\n", evVar, ex.EmittedEvent))
		if len(ex.EmittedFields) > 0 {
			sb.WriteString(fmt.Sprintf("  const evPayload%d = %s[%s.length - 1];\n", idx, evVar, evVar))
			for _, fi := range ex.EmittedFields {
				sb.WriteString(fmt.Sprintf("  assert.deepStrictEqual(evPayload%d.%s, %s);\n",
					idx, fi.Name, emitTSExpr(fi.Value, false)))
			}
		}
		return
	}

	if ex.StoredEntity != "" && (len(ex.WherePredicates) > 0 || ex.WhereValue != nil) {
		findMethod := toCamelCase(arch.StepRepoMethodName(arch.RepoMethodFind, ex.WherePredicates, ex.WhereField))
		var argExprs []string
		if len(ex.WherePredicates) > 0 {
			for _, wp := range ex.WherePredicates {
				argExprs = append(argExprs, emitTSExpr(wp.Value, false))
			}
		} else if ex.WhereValue != nil {
			argExprs = append(argExprs, emitTSExpr(*ex.WhereValue, false))
		}
		whereExpr := strings.Join(argExprs, ", ")
		storedVar := fmt.Sprintf("stored%d", idx)
		sb.WriteString(fmt.Sprintf("  const %s = await c.%sRepo.%s(%s);\n",
			storedVar, toCamelCase(ex.StoredEntity), findMethod, whereExpr))
		if ex.NotStored {
			sb.WriteString(fmt.Sprintf("  assert.strictEqual(%s, null);\n", storedVar))
			return
		}
		sb.WriteString(fmt.Sprintf("  assert.ok(%s !== null, \"expected %s to be stored\");\n", storedVar, ex.StoredEntity))
		for _, fi := range ex.StoredFields {
			sb.WriteString(fmt.Sprintf("  assert.deepStrictEqual(%s!.%s, %s);\n",
				storedVar, fi.Name, emitTSExpr(fi.Value, false)))
		}
		return
	}

	leftStr := emitTSExpr(ex.Left, false)
	rightStr := emitTSExpr(ex.Right, false)
	op := ex.Op
	if op == "==" {
		sb.WriteString(fmt.Sprintf("  assert.deepStrictEqual(%s, %s);\n", leftStr, rightStr))
		return
	}
	if op == "!=" {
		op = "!=="
	}
	sb.WriteString(fmt.Sprintf("  assert.ok(%s %s %s);\n", leftStr, op, rightStr))
}
