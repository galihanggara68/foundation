package lsp

import (
	"fmt"
	"path"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// SymbolKind identifies the kind of a Foundation declaration or binding.
type SymbolKind string

const (
	KindEntity   SymbolKind = "entity"
	KindActor    SymbolKind = "actor"
	KindValue    SymbolKind = "value"
	KindEnum     SymbolKind = "enum"
	KindEvent    SymbolKind = "event"
	KindError    SymbolKind = "error"
	KindPolicy   SymbolKind = "policy"
	KindSlot     SymbolKind = "slot"
	KindAction   SymbolKind = "action"
	KindScenario SymbolKind = "scenario"
	KindMethod   SymbolKind = "method"
	KindParam    SymbolKind = "parameter"
	KindField    SymbolKind = "field"
	KindVariable SymbolKind = "variable"
)

func (k SymbolKind) isLocal() bool {
	return k == KindVariable || k == KindParam
}

// Symbol is a declaration or local binding discovered in a Blueprint file.
type Symbol struct {
	Name   string
	Kind   SymbolKind
	Doc    string // markdown rendered for hover
	URI    string
	Line   int    // 1-based line of the name
	Column int    // 1-based column of the name
	Length int    // length of the name in bytes
	Type   string // declared or inferred type for bindings and fields ("" when unknown)
}

// DocumentIndex holds every symbol declared in a single Blueprint file.
type DocumentIndex struct {
	URI     string
	Content string
	Symbols []Symbol
	Structs map[string][]ir.FieldDecl // type name -> fields, for member completion
}

// IndexDocument parses a Blueprint file and collects its declarations.
func IndexDocument(uri, filePath, content string) *DocumentIndex {
	idx := &DocumentIndex{URI: uri, Content: content}

	sem, _ := frontend.ParseFile(filePath, content)
	if sem == nil {
		return idx
	}

	idx.Symbols, idx.Structs = collectSymbols(sem, uri, path.Base(filePath), content)
	return idx
}

func collectSymbols(sem *ir.SemanticIR, uri, file, content string) ([]Symbol, map[string][]ir.FieldDecl) {
	var out []Symbol
	structs := make(map[string][]ir.FieldDecl)

	structDecl := func(kind SymbolKind, name string, fields []ir.FieldDecl, traits []ir.TraitDecl, span ir.Span) {
		extra := []string{}
		if len(traits) > 0 {
			extra = append(extra, fmt.Sprintf("**Traits:** %s", traitNames(traits)))
		}
		if len(fields) > 0 {
			extra = append(extra, "**Fields**")
			extra = append(extra, fieldLines(fields)...)
		}
		out = append(out, newSymbol(uri, content, file, kind, name, span, docFor(kind, name, extra, file, span.Line)))
		if len(fields) > 0 {
			structs[name] = fields
		}
		for _, f := range fields {
			sym := newSymbol(uri, content, file, KindField, f.Name, f.Span,
				docFor(KindField, f.Name, []string{fmt.Sprintf("`%s` on `%s`", fieldType(f), name)}, file, f.Span.Line))
			sym.Type = fieldType(f)
			out = append(out, sym)
		}
	}

	for _, e := range sem.Entities {
		structDecl(KindEntity, e.Name, e.Fields, e.Traits, e.Span)
	}
	for _, a := range sem.Actors {
		structDecl(KindActor, a.Name, a.Fields, nil, a.Span)
	}
	for _, v := range sem.Values {
		structDecl(KindValue, v.Name, v.Fields, nil, v.Span)
	}
	for _, e := range sem.Events {
		structDecl(KindEvent, e.Name, e.Fields, nil, e.Span)
	}

	// Declaration tables used to infer binding types that are not on the step itself.
	tc := typeContext{
		slotReturns:   make(map[string]string),
		actionReturns: make(map[string]string),
		structs:       structs,
	}
	for _, s := range sem.Slots {
		for _, m := range s.Methods {
			if m.ReturnType != "" {
				tc.slotReturns[s.Name+"."+m.Name] = m.ReturnType
			}
		}
	}
	for _, a := range sem.Actions {
		if a.ReturnType != "" {
			tc.actionReturns[a.Name] = a.ReturnType
		}
	}

	for _, e := range sem.Enums {
		extra := []string{}
		if len(e.Variants) > 0 {
			extra = append(extra, fmt.Sprintf("**Variants:** %s", strings.Join(e.Variants, ", ")))
		}
		out = append(out, newSymbol(uri, content, file, KindEnum, e.Name, e.Span, docFor(KindEnum, e.Name, extra, file, e.Span.Line)))
	}

	for _, e := range sem.Errors {
		extra := []string{fmt.Sprintf("kind: `%s`", e.Kind)}
		if e.Message != "" {
			extra = append(extra, fmt.Sprintf("message: `%s`", e.Message))
		}
		out = append(out, newSymbol(uri, content, file, KindError, e.Name, e.Span, docFor(KindError, e.Name, extra, file, e.Span.Line)))
	}

	for _, p := range sem.Policies {
		sig := fmt.Sprintf("policy %s(%s)", p.Name, paramList(p.Params))
		out = append(out, newSymbol(uri, content, file, KindPolicy, p.Name, p.Span,
			docFor(KindPolicy, p.Name, []string{"```fdn", sig, "```"}, file, p.Span.Line)))
		for _, pr := range p.Params {
			sym := newSymbol(uri, content, file, KindParam, pr.Name, pr.Span,
				docFor(KindParam, pr.Name, []string{fmt.Sprintf("`%s` on `%s`", pr.Type, p.Name)}, file, pr.Span.Line))
			sym.Type = pr.Type
			out = append(out, sym)
		}
	}

	for _, s := range sem.Slots {
		extra := []string{}
		if len(s.Methods) > 0 {
			extra = append(extra, "**Methods**")
			for _, m := range s.Methods {
				extra = append(extra, fmt.Sprintf("- `%s`", slotMethodSig(m)))
			}
		}
		out = append(out, newSymbol(uri, content, file, KindSlot, s.Name, s.Span, docFor(KindSlot, s.Name, extra, file, s.Span.Line)))
		for _, m := range s.Methods {
			out = append(out, newSymbol(uri, content, file, KindMethod, m.Name, m.Span,
				docFor(KindMethod, m.Name, []string{"```fdn", slotMethodSig(m), "```", fmt.Sprintf("Method of `%s`", s.Name)}, file, m.Span.Line)))
		}
	}

	for _, a := range sem.Actions {
		sig := fmt.Sprintf("action %s(%s)", a.Name, paramList(a.Params))
		if a.ReturnType != "" {
			sig += " -> " + a.ReturnType
		}
		extra := []string{"```fdn", sig, "```"}
		if len(a.Traits) > 0 {
			extra = append(extra, fmt.Sprintf("**Traits:** %s", traitNames(a.Traits)))
		}
		out = append(out, newSymbol(uri, content, file, KindAction, a.Name, a.Span, docFor(KindAction, a.Name, extra, file, a.Span.Line)))
		for _, pr := range a.Params {
			sym := newSymbol(uri, content, file, KindParam, pr.Name, pr.Span,
				docFor(KindParam, pr.Name, []string{fmt.Sprintf("`%s` on `%s`", pr.Type, a.Name)}, file, pr.Span.Line))
			sym.Type = pr.Type
			out = append(out, sym)
		}
		bindings := make(map[string]string, len(a.Params))
		for _, pr := range a.Params {
			bindings[pr.Name] = pr.Type
		}
		out = append(out, stepSymbols(a.Steps, uri, file, content, tc, bindings)...)
	}

	for _, sc := range sem.Scenarios {
		out = append(out, newSymbol(uri, content, file, KindScenario, sc.Name, sc.Span,
			docFor(KindScenario, sc.Name, []string{fmt.Sprintf("**Action under test:** `%s`", sc.WhenAction)}, file, sc.Span.Line)))
	}

	return out, structs
}

func stepSymbols(steps []ir.StepDecl, uri, file, content string, tc typeContext, bindings map[string]string) []Symbol {
	var out []Symbol
	for _, s := range steps {
		if s.VarName != "" {
			typ := resolveStepType(s, tc, bindings)
			src := strings.TrimSpace(lineTextOf(content, s.Span.Line))
			extra := []string{}
			if typ != "" {
				extra = append(extra, fmt.Sprintf("**Type:** `%s`", typ))
			}
			if src != "" {
				extra = append(extra, "```fdn", src, "```")
			}
			sym := newSymbol(uri, content, file, KindVariable, s.VarName, s.Span,
				docFor(KindVariable, s.VarName, extra, file, s.Span.Line))
			sym.Type = typ
			out = append(out, sym)
			bindings[s.VarName] = typ
		}
		if s.IterVar != "" {
			typ := ""
			if s.Expr != nil {
				typ = strings.TrimPrefix(resolveExprType(*s.Expr, tc, bindings), "[]")
			}
			extra := []string{}
			if typ != "" {
				extra = append(extra, fmt.Sprintf("**Type:** `%s`", typ))
			}
			sym := newSymbol(uri, content, file, KindVariable, s.IterVar, s.Span,
				docFor(KindVariable, s.IterVar, extra, file, s.Span.Line))
			sym.Type = typ
			out = append(out, sym)
			bindings[s.IterVar] = typ
		}
		out = append(out, stepSymbols(s.ThenSteps, uri, file, content, tc, bindings)...)
		out = append(out, stepSymbols(s.ElseSteps, uri, file, content, tc, bindings)...)
		for _, arm := range s.MatchArms {
			out = append(out, stepSymbols(arm.Steps, uri, file, content, tc, bindings)...)
		}
	}
	return out
}

// typeContext carries the declaration tables used to infer binding types that are
// not recorded directly on a step.
type typeContext struct {
	slotReturns   map[string]string // "Slot.Method" -> return type
	actionReturns map[string]string // action name -> return type
	structs       map[string][]ir.FieldDecl
}

// resolveStepType returns the declared or inferred type of a `let` binding.
func resolveStepType(s ir.StepDecl, tc typeContext, bindings map[string]string) string {
	if s.EntityName != "" {
		if s.Kind == ir.StepQueryMany {
			return "[]" + s.EntityName
		}
		return s.EntityName
	}
	if s.Kind == ir.StepSlot {
		return tc.slotReturns[s.SlotName+"."+s.SlotMethod]
	}
	if s.Expr != nil {
		return resolveExprType(*s.Expr, tc, bindings)
	}
	return ""
}

// resolveExprType infers the type produced by an expression.
func resolveExprType(e ir.Expr, tc typeContext, bindings map[string]string) string {
	switch e.Kind {
	case ir.ExprConstruct:
		return e.TypeName
	case ir.ExprCall:
		return tc.actionReturns[e.Value]
	case ir.ExprIdent:
		return bindings[e.Value]
	case ir.ExprCombinator:
		if e.Left != nil {
			return resolveExprType(*e.Left, tc, bindings)
		}
	case ir.ExprField:
		recv := ""
		if e.Target != "" {
			recv = bindings[e.Target]
		} else if e.Left != nil {
			recv = resolveExprType(*e.Left, tc, bindings)
		}
		for _, f := range tc.structs[strings.TrimPrefix(recv, "[]")] {
			if f.Name == e.Field {
				return fieldType(f)
			}
		}
	}
	return ""
}

// LookupSymbol resolves name at the given line, preferring locals declared before
// the cursor in the current document, then declarations in the current document,
// then anything else in the workspace.
func LookupSymbol(indexes []*DocumentIndex, currentURI, name string, line int) *Symbol {
	if name == "" {
		return nil
	}
	if s := bestLocal(indexes, currentURI, name, line, true); s != nil {
		return s
	}
	if s := bestLocal(indexes, currentURI, name, line, false); s != nil {
		return s
	}
	if s := firstNamed(indexes, currentURI, name, true); s != nil {
		return s
	}
	return firstNamed(indexes, currentURI, name, false)
}

func bestLocal(indexes []*DocumentIndex, currentURI, name string, line int, sameDoc bool) *Symbol {
	var best *Symbol
	for _, idx := range indexes {
		if sameDoc && idx.URI != currentURI {
			continue
		}
		for i := range idx.Symbols {
			s := &idx.Symbols[i]
			if s.Name != name || !s.Kind.isLocal() || s.Line > line {
				continue
			}
			if best == nil || s.Line > best.Line {
				best = s
			}
		}
	}
	return best
}

func firstNamed(indexes []*DocumentIndex, currentURI, name string, sameDoc bool) *Symbol {
	for _, idx := range indexes {
		if sameDoc && idx.URI != currentURI {
			continue
		}
		for i := range idx.Symbols {
			s := &idx.Symbols[i]
			if s.Name == name && !s.Kind.isLocal() {
				return s
			}
		}
	}
	return nil
}

// ComputeHoverAt resolves the token under the cursor to a declared symbol,
// falling back to the built-in keyword/trait/type catalog.
func ComputeHoverAt(indexes []*DocumentIndex, currentURI, docContent string, pos Position) *Hover {
	if word := extractWordAt(docContent, pos); word != "" {
		name := strings.TrimPrefix(word, "@")
		if sym := LookupSymbol(indexes, currentURI, name, pos.Line+1); sym != nil {
			return &Hover{Contents: MarkupContent{Kind: "markdown", Value: sym.Doc}}
		}
	}
	return ComputeHover(docContent, pos)
}

// ComputeDefinition returns the declaration location for the token under the cursor.
func ComputeDefinition(indexes []*DocumentIndex, currentURI, docContent string, pos Position) []Location {
	word := extractWordAt(docContent, pos)
	if word == "" {
		return nil
	}
	sym := LookupSymbol(indexes, currentURI, strings.TrimPrefix(word, "@"), pos.Line+1)
	if sym == nil {
		return nil
	}
	return []Location{{URI: sym.URI, Range: symbolRange(sym)}}
}

func symbolRange(sym *Symbol) Range {
	line := sym.Line - 1
	if line < 0 {
		line = 0
	}
	start := sym.Column - 1
	if start < 0 {
		start = 0
	}
	length := sym.Length
	if length <= 0 {
		length = 1
	}
	return Range{
		Start: Position{Line: line, Character: start},
		End:   Position{Line: line, Character: start + length},
	}
}
