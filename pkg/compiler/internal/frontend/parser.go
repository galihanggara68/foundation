package frontend

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

type parser struct {
	tokens []token
	pos    int
	diags  []ir.Diagnostic
}

// ParseFile tokenizes and parses a single .fdn Blueprint file into SemanticIR declarations.
func ParseFile(file, src string) (*ir.SemanticIR, []ir.Diagnostic) {
	lex := newLexer(file, src)
	tokens, lexDiags := lex.tokenize()
	if len(lexDiags) > 0 {
		return nil, lexDiags
	}
	p := &parser{tokens: tokens}
	sem := p.parseBlueprint()
	return sem, p.diags
}

func (p *parser) cur() token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}

func (p *parser) peekOffset(offset int) token {
	idx := p.pos + offset
	if idx >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[idx]
}

func (p *parser) advance() token {
	t := p.cur()
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return t
}

func (p *parser) expect(kind tokenKind) (token, bool) {
	t := p.cur()
	if t.kind != kind {
		p.diags = append(p.diags, diagnostics.New("FDN0102", t.span, kind, t.text))
		return t, false
	}
	p.advance()
	return t, true
}

func (p *parser) expectIdentText(text string) (token, bool) {
	t := p.cur()
	if t.kind != tokIdent || t.text != text {
		p.diags = append(p.diags, diagnostics.New("FDN0102", t.span, fmt.Sprintf("keyword %q", text), t.text))
		return t, false
	}
	p.advance()
	return t, true
}

func (p *parser) parseBlueprint() *ir.SemanticIR {
	sem := &ir.SemanticIR{}
	for p.cur().kind != tokEOF {
		var preTraits []ir.TraitDecl
		for p.cur().kind == tokAt {
			tr, ok := p.parseTrait()
			if !ok {
				break
			}
			preTraits = append(preTraits, tr)
		}
		t := p.cur()
		if t.kind != tokIdent {
			p.diags = append(p.diags, diagnostics.New("FDN0103", t.span, t.text))
			p.advance()
			continue
		}
		switch t.text {
		case "actor":
			if act, ok := p.parseActor(); ok {
				sem.Actors = append(sem.Actors, act)
				if sem.Actor == nil {
					sem.Actor = &act
				}
			}
		case "enum":
			if e, ok := p.parseEnum(); ok {
				sem.Enums = append(sem.Enums, e)
			}
		case "value":
			if v, ok := p.parseValue(); ok {
				sem.Values = append(sem.Values, v)
			}
		case "entity":
			if e, ok := p.parseEntity(); ok {
				e.Traits = append(preTraits, e.Traits...)
				e.TableName = arch.EntityTableName(e)
				sem.Entities = append(sem.Entities, e)
			}
		case "error":
			if e, ok := p.parseError(); ok {
				sem.Errors = append(sem.Errors, e)
			}
		case "event":
			if ev, ok := p.parseEvent(); ok {
				sem.Events = append(sem.Events, ev)
			}
		case "policy":
			if pol, ok := p.parsePolicy(); ok {
				sem.Policies = append(sem.Policies, pol)
			}
		case "slot":
			if len(preTraits) > 0 {
				p.diags = append(p.diags, diagnostics.New("FDN0223", preTraits[0].Span))
			}
			if sl, ok := p.parseSlot(); ok {
				sem.Slots = append(sem.Slots, sl)
			}
		case "action":
			if a, ok := p.parseAction(); ok {
				a.Traits = append(preTraits, a.Traits...)
				ir.EnrichActionParamsInPath(&a)
				sem.Actions = append(sem.Actions, a)
			}
		case "scenario":
			if s, ok := p.parseScenario(); ok {
				sem.Scenarios = append(sem.Scenarios, s)
			}
		default:
			p.diags = append(p.diags, diagnostics.NewWithDetail(
				"FDN0103",
				t.span,
				fmt.Sprintf("unknown top-level declaration %q", t.text),
			))
			p.advance()
		}
	}
	return sem
}
