package frontend

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func (p *parser) parseScenario() (ir.ScenarioDecl, bool) {
	kw := p.advance()
	nameTok, ok := p.expect(tokString)
	if !ok {
		return ir.ScenarioDecl{}, false
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return ir.ScenarioDecl{}, false
	}
	scen := ir.ScenarioDecl{
		Name: nameTok.text,
		Span: kw.span,
	}
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		clause := p.cur()
		if clause.kind != tokIdent {
			p.diags = append(p.diags, ir.Diagnostic{
				Code:     "FDN0107",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("expected scenario clause (given, when, expect), got %q", clause.text),
				Span:     clause.span,
			})
			p.advance()
			return ir.ScenarioDecl{}, false
		}
		switch clause.text {
		case "given":
			p.advance()
			entTok, ok := p.expect(tokIdent)
			if !ok {
				return ir.ScenarioDecl{}, false
			}
			fields, ok := p.parseFieldInitBlock()
			if !ok {
				return ir.ScenarioDecl{}, false
			}
			scen.Givens = append(scen.Givens, ir.GivenSeedDecl{
				EntityName: entTok.text,
				Fields:     fields,
				Span:       clause.span,
			})
		case "when":
			p.advance()
			actTok, ok := p.expect(tokIdent)
			if !ok {
				return ir.ScenarioDecl{}, false
			}
			scen.WhenAction = actTok.text
			if _, ok := p.expect(tokLParen); !ok {
				return ir.ScenarioDecl{}, false
			}
			for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
				argExpr, ok := p.parseExpr()
				if !ok {
					return ir.ScenarioDecl{}, false
				}
				scen.WhenArgs = append(scen.WhenArgs, argExpr)
				if p.cur().kind == tokComma {
					p.advance()
				}
			}
			if _, ok := p.expect(tokRParen); !ok {
				return ir.ScenarioDecl{}, false
			}
		case "expect":
			p.advance()
			if p.cur().kind == tokIdent && p.cur().text == "error" {
				p.advance()
				errTok, ok := p.expect(tokIdent)
				if !ok {
					return ir.ScenarioDecl{}, false
				}
				scen.Expects = append(scen.Expects, ir.ExpectDecl{
					ErrorRaised: errTok.text,
					Span:        clause.span,
				})
				continue
			}
			if p.cur().kind == tokIdent && p.cur().text == "emitted" {
				p.advance()
				evTok, ok := p.expect(tokIdent)
				if !ok {
					return ir.ScenarioDecl{}, false
				}
				var fields []ir.FieldInitDecl
				if p.cur().kind == tokLBrace {
					var fOk bool
					fields, fOk = p.parseFieldInitBlock()
					if !fOk {
						return ir.ScenarioDecl{}, false
					}
				}
				scen.Expects = append(scen.Expects, ir.ExpectDecl{
					EmittedEvent:  evTok.text,
					EmittedFields: fields,
					Span:          clause.span,
				})
				continue
			}
			if p.cur().kind == tokIdent && (p.cur().text == "stored" || p.cur().text == "not_stored") {
				notStored := p.advance().text == "not_stored"
				entTok, ok := p.expect(tokIdent)
				if !ok {
					return ir.ScenarioDecl{}, false
				}
				if _, ok := p.expectIdentText("where"); !ok {
					return ir.ScenarioDecl{}, false
				}
				wField, ok := p.expect(tokIdent)
				if !ok {
					return ir.ScenarioDecl{}, false
				}
				if _, ok := p.expect(tokEq); !ok {
					return ir.ScenarioDecl{}, false
				}
				wVal, ok := p.parseExpr()
				if !ok {
					return ir.ScenarioDecl{}, false
				}
				var storedFields []ir.FieldInitDecl
				if !notStored && p.cur().kind == tokLBrace {
					var fOk bool
					storedFields, fOk = p.parseFieldInitBlock()
					if !fOk {
						return ir.ScenarioDecl{}, false
					}
				}
				scen.Expects = append(scen.Expects, ir.ExpectDecl{
					StoredEntity: entTok.text,
					WhereField:   wField.text,
					WhereValue:   &wVal,
					StoredFields: storedFields,
					NotStored:    notStored,
					Span:         clause.span,
				})
				continue
			}
			left, ok := p.parsePostfixExpr()
			if !ok {
				return ir.ScenarioDecl{}, false
			}
			opTok := p.advance()
			right, ok := p.parseExpr()
			if !ok {
				return ir.ScenarioDecl{}, false
			}
			scen.Expects = append(scen.Expects, ir.ExpectDecl{
				Left:  left,
				Op:    opTok.text,
				Right: right,
				Span:  clause.span,
			})
		default:
			p.diags = append(p.diags, ir.Diagnostic{
				Code:     "FDN0107",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("unknown scenario clause %q", clause.text),
				Span:     clause.span,
			})
			p.advance()
			return ir.ScenarioDecl{}, false
		}
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return ir.ScenarioDecl{}, false
	}
	return scen, true
}
