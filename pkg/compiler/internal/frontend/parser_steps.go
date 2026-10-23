package frontend

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func (p *parser) parseStepBlock() ([]ir.StepDecl, bool) {
	if _, ok := p.expect(tokLBrace); !ok {
		return nil, false
	}
	var steps []ir.StepDecl
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		step, ok := p.parseStep()
		if !ok {
			return nil, false
		}
		steps = append(steps, step)
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return nil, false
	}
	return steps, true
}

func (p *parser) parseStep() (ir.StepDecl, bool) {
	t := p.cur()
	if t.kind != tokIdent {
		p.diags = append(p.diags, ir.Diagnostic{
			Code:     "FDN0105",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("expected action statement, got %q", t.text),
			Span:     t.span,
		})
		p.advance()
		return ir.StepDecl{}, false
	}
	switch t.text {
	case "let":
		p.advance()
		varTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expect(tokAssign); !ok {
			return ir.StepDecl{}, false
		}
		next := p.cur()
		if next.kind == tokIdent && next.text == "query" {
			p.advance()
			if _, ok := p.expectIdentText("one"); !ok {
				return ir.StepDecl{}, false
			}
			entTok, ok := p.expect(tokIdent)
			if !ok {
				return ir.StepDecl{}, false
			}
			if _, ok := p.expectIdentText("where"); !ok {
				return ir.StepDecl{}, false
			}
			fieldTok, ok := p.expect(tokIdent)
			if !ok {
				return ir.StepDecl{}, false
			}
			if _, ok := p.expect(tokEq); !ok {
				return ir.StepDecl{}, false
			}
			valExpr, ok := p.parseExpr()
			if !ok {
				return ir.StepDecl{}, false
			}
			elseErr := ""
			if p.cur().kind == tokIdent && p.cur().text == "else" {
				p.advance()
				errTok, ok := p.expect(tokIdent)
				if !ok {
					return ir.StepDecl{}, false
				}
				elseErr = errTok.text
			}
			return ir.StepDecl{
				Kind:       ir.StepQuery,
				VarName:    varTok.text,
				EntityName: entTok.text,
				WhereField: fieldTok.text,
				WhereValue: &valExpr,
				ElseError:  elseErr,
				Span:       t.span,
			}, true
		}
		if next.kind == tokIdent && next.text == "create" {
			p.advance()
			entTok, ok := p.expect(tokIdent)
			if !ok {
				return ir.StepDecl{}, false
			}
			fields, ok := p.parseFieldInitBlock()
			if !ok {
				return ir.StepDecl{}, false
			}
			return ir.StepDecl{
				Kind:       ir.StepCreate,
				VarName:    varTok.text,
				EntityName: entTok.text,
				Fields:     fields,
				Span:       t.span,
			}, true
		}
		if next.kind == tokIdent && next.text == "slot" {
			return p.parseSlotCallStep(varTok.text, t.span)
		}
		expr, ok := p.parseExpr()
		if !ok {
			return ir.StepDecl{}, false
		}
		return ir.StepDecl{
			Kind:    ir.StepLet,
			VarName: varTok.text,
			Expr:    &expr,
			Span:    t.span,
		}, true

	case "require":
		p.advance()
		polTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expect(tokLParen); !ok {
			return ir.StepDecl{}, false
		}
		var args []ir.Expr
		for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
			arg, ok := p.parseExpr()
			if !ok {
				return ir.StepDecl{}, false
			}
			args = append(args, arg)
			if p.cur().kind == tokComma {
				p.advance()
			}
		}
		if _, ok := p.expect(tokRParen); !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expectIdentText("else"); !ok {
			return ir.StepDecl{}, false
		}
		errTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		return ir.StepDecl{
			Kind:       ir.StepRequire,
			PolicyName: polTok.text,
			PolicyArgs: args,
			ElseError:  errTok.text,
			Span:       t.span,
		}, true

	case "guard":
		p.advance()
		condExpr, ok := p.parseExpr()
		if !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expectIdentText("else"); !ok {
			return ir.StepDecl{}, false
		}
		errTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		return ir.StepDecl{
			Kind:      ir.StepGuard,
			Expr:      &condExpr,
			ElseError: errTok.text,
			Span:      t.span,
		}, true

	case "update":
		p.advance()
		entTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expectIdentText("where"); !ok {
			return ir.StepDecl{}, false
		}
		fieldTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expect(tokEq); !ok {
			return ir.StepDecl{}, false
		}
		whereVal, ok := p.parseExpr()
		if !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expectIdentText("set"); !ok {
			return ir.StepDecl{}, false
		}
		fields, ok := p.parseFieldInitBlock()
		if !ok {
			return ir.StepDecl{}, false
		}
		return ir.StepDecl{
			Kind:       ir.StepUpdate,
			EntityName: entTok.text,
			WhereField: fieldTok.text,
			WhereValue: &whereVal,
			Fields:     fields,
			Span:       t.span,
		}, true

	case "delete":
		p.advance()
		entTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expectIdentText("where"); !ok {
			return ir.StepDecl{}, false
		}
		fieldTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expect(tokEq); !ok {
			return ir.StepDecl{}, false
		}
		whereVal, ok := p.parseExpr()
		if !ok {
			return ir.StepDecl{}, false
		}
		return ir.StepDecl{
			Kind:       ir.StepDelete,
			EntityName: entTok.text,
			WhereField: fieldTok.text,
			WhereValue: &whereVal,
			Span:       t.span,
		}, true

	case "if":
		p.advance()
		cond, ok := p.parseExpr()
		if !ok {
			return ir.StepDecl{}, false
		}
		thenSteps, ok := p.parseStepBlock()
		if !ok {
			return ir.StepDecl{}, false
		}
		var elseSteps []ir.StepDecl
		if p.cur().kind == tokIdent && p.cur().text == "else" {
			p.advance()
			if p.cur().kind == tokIdent && p.cur().text == "if" {
				nestedIf, ok := p.parseStep()
				if !ok {
					return ir.StepDecl{}, false
				}
				elseSteps = []ir.StepDecl{nestedIf}
			} else {
				var ok bool
				elseSteps, ok = p.parseStepBlock()
				if !ok {
					return ir.StepDecl{}, false
				}
			}
		}
		return ir.StepDecl{
			Kind:      ir.StepIf,
			Expr:      &cond,
			ThenSteps: thenSteps,
			ElseSteps: elseSteps,
			Span:      t.span,
		}, true

	case "match":
		p.advance()
		targetExpr, ok := p.parseExpr()
		if !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expect(tokLBrace); !ok {
			return ir.StepDecl{}, false
		}
		var arms []ir.MatchArmDecl
		for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
			armSpan := p.cur().span
			if _, ok := p.expect(tokDot); !ok {
				return ir.StepDecl{}, false
			}
			vTok, ok := p.expect(tokIdent)
			if !ok {
				return ir.StepDecl{}, false
			}
			if _, ok := p.expect(tokFatArrow); !ok {
				return ir.StepDecl{}, false
			}
			var armSteps []ir.StepDecl
			if p.cur().kind == tokLBrace {
				var sOk bool
				armSteps, sOk = p.parseStepBlock()
				if !sOk {
					return ir.StepDecl{}, false
				}
			} else {
				single, sOk := p.parseStep()
				if !sOk {
					return ir.StepDecl{}, false
				}
				armSteps = []ir.StepDecl{single}
			}
			if p.cur().kind == tokComma {
				p.advance()
			}
			arms = append(arms, ir.MatchArmDecl{
				Variant: vTok.text,
				Steps:   armSteps,
				Span:    armSpan,
			})
		}
		if _, ok := p.expect(tokRBrace); !ok {
			return ir.StepDecl{}, false
		}
		return ir.StepDecl{
			Kind:      ir.StepMatch,
			Expr:      &targetExpr,
			MatchArms: arms,
			Span:      t.span,
		}, true

	case "for":
		p.advance()
		iterTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		if _, ok := p.expectIdentText("in"); !ok {
			return ir.StepDecl{}, false
		}
		collExpr, ok := p.parseExpr()
		if !ok {
			return ir.StepDecl{}, false
		}
		bodySteps, ok := p.parseStepBlock()
		if !ok {
			return ir.StepDecl{}, false
		}
		return ir.StepDecl{
			Kind:      ir.StepFor,
			IterVar:   iterTok.text,
			Expr:      &collExpr,
			ThenSteps: bodySteps,
			Span:      t.span,
		}, true

	case "while":
		p.advance()
		condExpr, ok := p.parseExpr()
		if !ok {
			return ir.StepDecl{}, false
		}
		bodySteps, ok := p.parseStepBlock()
		if !ok {
			return ir.StepDecl{}, false
		}
		return ir.StepDecl{
			Kind:      ir.StepWhile,
			Expr:      &condExpr,
			ThenSteps: bodySteps,
			Span:      t.span,
		}, true

	case "emit":
		p.advance()
		evTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		fields, ok := p.parseFieldInitBlock()
		if !ok {
			return ir.StepDecl{}, false
		}
		return ir.StepDecl{
			Kind:      ir.StepEmit,
			EventName: evTok.text,
			Fields:    fields,
			Span:      t.span,
		}, true

	case "slot":
		return p.parseSlotCallStep("", t.span)

	case "return":
		p.advance()
		expr, ok := p.parseExpr()
		if !ok {
			return ir.StepDecl{}, false
		}
		return ir.StepDecl{
			Kind: ir.StepReturn,
			Expr: &expr,
			Span: t.span,
		}, true

	default:
		p.diags = append(p.diags, ir.Diagnostic{
			Code:     "FDN0105",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("unknown action step keyword %q", t.text),
			Span:     t.span,
		})
		p.advance()
		return ir.StepDecl{}, false
	}
}

func (p *parser) parseSlotCallStep(varName string, span ir.Span) (ir.StepDecl, bool) {
	p.advance() // consume 'slot'
	slotTok, ok := p.expect(tokIdent)
	if !ok {
		return ir.StepDecl{}, false
	}
	if _, ok := p.expect(tokDot); !ok {
		return ir.StepDecl{}, false
	}
	mTok, ok := p.expect(tokIdent)
	if !ok {
		return ir.StepDecl{}, false
	}
	if _, ok := p.expect(tokLParen); !ok {
		return ir.StepDecl{}, false
	}
	var args []ir.Expr
	for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
		arg, ok := p.parseExpr()
		if !ok {
			return ir.StepDecl{}, false
		}
		args = append(args, arg)
		if p.cur().kind == tokComma {
			p.advance()
		}
	}
	if _, ok := p.expect(tokRParen); !ok {
		return ir.StepDecl{}, false
	}
	elseErr := ""
	if p.cur().kind == tokIdent && p.cur().text == "else" {
		p.advance()
		errTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.StepDecl{}, false
		}
		elseErr = errTok.text
	}
	return ir.StepDecl{
		Kind:       ir.StepSlot,
		VarName:    varName,
		SlotName:   slotTok.text,
		SlotMethod: mTok.text,
		SlotArgs:   args,
		ElseError:  elseErr,
		Span:       span,
	}, true
}

func (p *parser) parseFieldInitBlock() ([]ir.FieldInitDecl, bool) {
	if _, ok := p.expect(tokLBrace); !ok {
		return nil, false
	}
	var inits []ir.FieldInitDecl
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		fTok, ok := p.expect(tokIdent)
		if !ok {
			return nil, false
		}
		if _, ok := p.expect(tokColon); !ok {
			return nil, false
		}
		valExpr, ok := p.parseExpr()
		if !ok {
			return nil, false
		}
		inits = append(inits, ir.FieldInitDecl{
			Name:  fTok.text,
			Value: valExpr,
			Span:  fTok.span,
		})
		if p.cur().kind == tokComma {
			p.advance()
		}
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return nil, false
	}
	return inits, true
}
