package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func (lay goLayout) slotNeedsDomainPkg(sl ir.SlotDecl) bool {
	for _, m := range sl.Methods {
		for _, p := range m.Params {
			if strings.Contains(lay.goType(p.Type, false), lay.DomainPkg+".") {
				return true
			}
		}
		if strings.Contains(lay.goType(m.ReturnType, false), lay.DomainPkg+".") {
			return true
		}
	}
	return false
}

func (lay goLayout) emitGoSlotTypeAliases(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("package slots\n\nimport \"%s/%s\"\n\n", archIR.Module, lay.DomainDir))
	sb.WriteString(fmt.Sprintf("type DomainError = %s.DomainError\n", lay.DomainPkg))
	for _, en := range archIR.DomainEnums {
		sb.WriteString(fmt.Sprintf("type %s = %s.%s\n", en.Name, lay.DomainPkg, en.Name))
	}
	for _, v := range archIR.DomainValues {
		sb.WriteString(fmt.Sprintf("type %s = %s.%s\n", v.Name, lay.DomainPkg, v.Name))
	}
	for _, e := range archIR.DomainEntities {
		sb.WriteString(fmt.Sprintf("type %s = %s.%s\n", e.Name, lay.DomainPkg, e.Name))
	}
	for _, errDecl := range archIR.DomainErrors {
		sb.WriteString(fmt.Sprintf("var Err%s = %s.Err%s\n", errDecl.Name, lay.DomainPkg, errDecl.Name))
	}
	return sb.String()
}

func (lay goLayout) emitGoSlotPort(module string, sl ir.SlotDecl) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("package %s\n\nimport (\n\t\"context\"\n", lay.PortsPkg))
	if lay.slotNeedsDomainPkg(sl) {
		sb.WriteString(fmt.Sprintf("\t\"%s/%s\"\n", module, lay.DomainDir))
	}
	sb.WriteString(")\n\n")

	sb.WriteString(fmt.Sprintf("type %sSlot interface {\n", sl.Name))
	for _, m := range sl.Methods {
		var params []string
		params = append(params, "ctx context.Context")
		for _, p := range m.Params {
			params = append(params, fmt.Sprintf("%s %s", p.Name, lay.goType(p.Type, false)))
		}
		if m.ReturnType != "" {
			sb.WriteString(fmt.Sprintf("\t%s(%s) (%s, error)\n", m.Name, strings.Join(params, ", "), lay.goType(m.ReturnType, false)))
		} else {
			sb.WriteString(fmt.Sprintf("\t%s(%s) error\n", m.Name, strings.Join(params, ", ")))
		}
	}
	sb.WriteString("}\n")
	return sb.String()
}

func (lay goLayout) emitGoSlotStub(sl ir.SlotDecl) string {
	var sb strings.Builder
	sb.WriteString("package slots\n\nimport \"context\"\n\n")

	stubType := sl.Name + "Stub"
	sb.WriteString(fmt.Sprintf("// %s is a create-if-missing starter implementation of %sSlot.\n", stubType, sl.Name))
	sb.WriteString("// Developers may freely edit this file; Foundation never overwrites existing Slot Stubs.\n")
	sb.WriteString(fmt.Sprintf("type %s struct{}\n\n", stubType))
	sb.WriteString(fmt.Sprintf("func New%sSlot() *%s {\n\treturn &%s{}\n}\n\n", sl.Name, stubType, stubType))

	for _, m := range sl.Methods {
		var params []string
		params = append(params, "ctx context.Context")
		for _, p := range m.Params {
			params = append(params, fmt.Sprintf("%s %s", p.Name, lay.goType(p.Type, true)))
		}
		if m.ReturnType != "" {
			retGo := lay.goType(m.ReturnType, true)
			sb.WriteString(fmt.Sprintf("func (s *%s) %s(%s) (%s, error) {\n\t_ = ctx\n", stubType, m.Name, strings.Join(params, ", "), retGo))
			for _, p := range m.Params {
				sb.WriteString(fmt.Sprintf("\t_ = %s\n", p.Name))
			}
			sb.WriteString(fmt.Sprintf("\tvar out %s\n\treturn out, nil\n}\n\n", retGo))
		} else {
			sb.WriteString(fmt.Sprintf("func (s *%s) %s(%s) error {\n\t_ = ctx\n", stubType, m.Name, strings.Join(params, ", ")))
			for _, p := range m.Params {
				sb.WriteString(fmt.Sprintf("\t_ = %s\n", p.Name))
			}
			sb.WriteString("\treturn nil\n}\n\n")
		}
	}
	return sb.String()
}

func (lay goLayout) emitGoMemorySlotFake(module string, sl ir.SlotDecl) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("package %s\n\nimport (\n\t\"context\"\n\t\"reflect\"\n\t\"sync\"\n", lay.MemoryPkg))
	if lay.slotNeedsDomainPkg(sl) {
		sb.WriteString(fmt.Sprintf("\t\"%s/%s\"\n", module, lay.DomainDir))
	}
	if !lay.UnifiedRepositories {
		sb.WriteString(fmt.Sprintf("\t\"%s/%s\"\n", module, lay.PortsDir))
	}
	sb.WriteString(")\n\n")

	portInterface := sl.Name + "Slot"
	if !lay.UnifiedRepositories {
		portInterface = lay.PortsPkg + "." + portInterface
	}

	fakeType := "Fake" + sl.Name + "Slot"
	for _, m := range sl.Methods {
		ruleType := fmt.Sprintf("fake%s%sRule", sl.Name, m.Name)
		sb.WriteString(fmt.Sprintf("type %s struct {\n\thasArgs bool\n", ruleType))
		for _, p := range m.Params {
			sb.WriteString(fmt.Sprintf("\targ_%s %s\n", p.Name, lay.goType(p.Type, false)))
		}
		if m.ReturnType != "" {
			sb.WriteString(fmt.Sprintf("\tret %s\n", lay.goType(m.ReturnType, false)))
		}
		sb.WriteString("\terr error\n}\n\n")
	}

	sb.WriteString(fmt.Sprintf("type %s struct {\n\tmu sync.RWMutex\n\tfallback %s\n", fakeType, portInterface))
	for _, m := range sl.Methods {
		sb.WriteString(fmt.Sprintf("\trules%s []fake%s%sRule\n", m.Name, sl.Name, m.Name))
	}
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("func New%s(fallback %s) *%s {\n\t_ = reflect.DeepEqual\n\treturn &%s{fallback: fallback}\n}\n\n",
		fakeType, portInterface, fakeType, fakeType))

	for _, m := range sl.Methods {
		ruleType := fmt.Sprintf("fake%s%sRule", sl.Name, m.Name)
		var paramDecls []string
		for _, p := range m.Params {
			paramDecls = append(paramDecls, fmt.Sprintf("%s %s", p.Name, lay.goType(p.Type, false)))
		}
		retGo := lay.goType(m.ReturnType, false)

		if m.ReturnType != "" {
			stubWithArgs := append([]string{}, paramDecls...)
			stubWithArgs = append(stubWithArgs, fmt.Sprintf("ret %s", retGo))
			sb.WriteString(fmt.Sprintf("func (f *%s) Stub%sReturn(%s) {\n\tf.mu.Lock()\n\tdefer f.mu.Unlock()\n\tf.rules%s = append(f.rules%s, %s{\n\t\thasArgs: true,\n",
				fakeType, m.Name, strings.Join(stubWithArgs, ", "), m.Name, m.Name, ruleType))
			for _, p := range m.Params {
				sb.WriteString(fmt.Sprintf("\t\targ_%s: %s,\n", p.Name, p.Name))
			}
			sb.WriteString("\t\tret: ret,\n\t})\n}\n\n")

			sb.WriteString(fmt.Sprintf("func (f *%s) Stub%sReturnAny(ret %s) {\n\tf.mu.Lock()\n\tdefer f.mu.Unlock()\n\tf.rules%s = append(f.rules%s, %s{hasArgs: false, ret: ret})\n}\n\n",
				fakeType, m.Name, retGo, m.Name, m.Name, ruleType))
		}

		if len(paramDecls) > 0 {
			errWithArgs := append([]string{}, paramDecls...)
			errWithArgs = append(errWithArgs, "err error")
			sb.WriteString(fmt.Sprintf("func (f *%s) Stub%sError(%s) {\n\tf.mu.Lock()\n\tdefer f.mu.Unlock()\n\tf.rules%s = append(f.rules%s, %s{\n\t\thasArgs: true,\n",
				fakeType, m.Name, strings.Join(errWithArgs, ", "), m.Name, m.Name, ruleType))
			for _, p := range m.Params {
				sb.WriteString(fmt.Sprintf("\t\targ_%s: %s,\n", p.Name, p.Name))
			}
			sb.WriteString("\t\terr: err,\n\t})\n}\n\n")
		}

		sb.WriteString(fmt.Sprintf("func (f *%s) Stub%sErrorAny(err error) {\n\tf.mu.Lock()\n\tdefer f.mu.Unlock()\n\tf.rules%s = append(f.rules%s, %s{hasArgs: false, err: err})\n}\n\n",
			fakeType, m.Name, m.Name, m.Name, ruleType))

		callParams := append([]string{"ctx context.Context"}, paramDecls...)
		var forwardArgs []string
		forwardArgs = append(forwardArgs, "ctx")
		for _, p := range m.Params {
			forwardArgs = append(forwardArgs, p.Name)
		}

		if m.ReturnType != "" {
			sb.WriteString(fmt.Sprintf("func (f *%s) %s(%s) (%s, error) {\n\tf.mu.RLock()\n\trules := append([]%s(nil), f.rules%s...)\n\tf.mu.RUnlock()\n",
				fakeType, m.Name, strings.Join(callParams, ", "), retGo, ruleType, m.Name))
			sb.WriteString("\tfor i := len(rules) - 1; i >= 0; i-- {\n\t\tr := rules[i]\n\t\tif !r.hasArgs")
			if len(m.Params) > 0 {
				var conds []string
				for _, p := range m.Params {
					conds = append(conds, fmt.Sprintf("reflect.DeepEqual(r.arg_%s, %s)", p.Name, p.Name))
				}
				sb.WriteString(" || (" + strings.Join(conds, " && ") + ")")
			}
			sb.WriteString(" {\n\t\t\treturn r.ret, r.err\n\t\t}\n\t}\n")
			sb.WriteString(fmt.Sprintf("\tif f.fallback != nil {\n\t\treturn f.fallback.%s(%s)\n\t}\n\tvar zero %s\n\treturn zero, nil\n}\n\n",
				m.Name, strings.Join(forwardArgs, ", "), retGo))
		} else {
			sb.WriteString(fmt.Sprintf("func (f *%s) %s(%s) error {\n\tf.mu.RLock()\n\trules := append([]%s(nil), f.rules%s...)\n\tf.mu.RUnlock()\n",
				fakeType, m.Name, strings.Join(callParams, ", "), ruleType, m.Name))
			sb.WriteString("\tfor i := len(rules) - 1; i >= 0; i-- {\n\t\tr := rules[i]\n\t\tif !r.hasArgs")
			if len(m.Params) > 0 {
				var conds []string
				for _, p := range m.Params {
					conds = append(conds, fmt.Sprintf("reflect.DeepEqual(r.arg_%s, %s)", p.Name, p.Name))
				}
				sb.WriteString(" || (" + strings.Join(conds, " && ") + ")")
			}
			sb.WriteString(" {\n\t\t\treturn r.err\n\t\t}\n\t}\n")
			sb.WriteString(fmt.Sprintf("\tif f.fallback != nil {\n\t\treturn f.fallback.%s(%s)\n\t}\n\treturn nil\n}\n\n",
				m.Name, strings.Join(forwardArgs, ", ")))
		}
	}

	return sb.String()
}

func (lay goLayout) emitGoContractsTest(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package scenarios_test

import (
	"context"
	"errors"
	"testing"

	"%s/%s"
	"%s/%s"
	"%s/internal/slots"
)

`, archIR.Module, lay.DomainDir, archIR.Module, lay.PortsDir, archIR.Module))

	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("func TestContract_%sSlot(t *testing.T) {\n", sl.Name))
		sb.WriteString("\tctx := context.Background()\n")
		sb.WriteString(fmt.Sprintf("\tvar impl %s.%sSlot = slots.New%sSlot()\n", lay.PortsPkg, sl.Name, sl.Name))
		sb.WriteString(fmt.Sprintf("\tif impl == nil {\n\t\tt.Fatalf(\"expected non-nil %sSlot implementation\")\n\t}\n", sl.Name))

		for _, m := range sl.Methods {
			var callArgs []string
			callArgs = append(callArgs, "ctx")
			for idx, p := range m.Params {
				argVar := fmt.Sprintf("arg_%s_%d", m.Name, idx)
				sb.WriteString(fmt.Sprintf("\tvar %s %s\n", argVar, lay.goType(p.Type, false)))
				callArgs = append(callArgs, argVar)
			}
			if m.ReturnType != "" {
				sb.WriteString(fmt.Sprintf("\tres, err := impl.%s(%s)\n\t_ = res\n", m.Name, strings.Join(callArgs, ", ")))
			} else {
				sb.WriteString(fmt.Sprintf("\terr := impl.%s(%s)\n", m.Name, strings.Join(callArgs, ", ")))
			}
			sb.WriteString("\tif err != nil {\n")
			if len(m.Throws) > 0 {
				var checks []string
				for _, th := range m.Throws {
					checks = append(checks, fmt.Sprintf("errors.Is(err, %s.Err%s)", lay.DomainPkg, th))
				}
				sb.WriteString(fmt.Sprintf("\t\tif !(%s) {\n\t\t\tt.Fatalf(\"slot %s.%s returned undeclared error %%v; allowed throws: %v\", err)\n\t\t}\n",
					strings.Join(checks, " || "), sl.Name, m.Name, m.Throws))
			} else {
				sb.WriteString(fmt.Sprintf("\t\tt.Fatalf(\"slot %s.%s declares no throws errors, but returned: %%v\", err)\n", sl.Name, m.Name))
			}
			sb.WriteString("\t}\n")
		}
		sb.WriteString(fmt.Sprintf("\t_ = errors.Is\n\t_ = %s.DomainError{}\n}\n\n", lay.DomainPkg))
	}

	return sb.String()
}
