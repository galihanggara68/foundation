package frontend

import (
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func (p *parser) parseIfStep(span ir.Span) (ir.StepDecl, bool) {
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
		Span:      span,
	}, true
}

func (p *parser) parseMatchStep(span ir.Span) (ir.StepDecl, bool) {
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
		Span:      span,
	}, true
}

func (p *parser) parseForStep(span ir.Span) (ir.StepDecl, bool) {
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
		Span:      span,
	}, true
}

func (p *parser) parseWhileStep(span ir.Span) (ir.StepDecl, bool) {
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
		Span:      span,
	}, true
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
