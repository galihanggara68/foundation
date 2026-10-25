package frontend

import "github.com/galihanggara68/foundation/pkg/compiler/internal/ir"

func (p *parser) parseParamListAndReturn() ([]ir.ParamDecl, string, bool) {
	if _, ok := p.expect(tokLParen); !ok {
		return nil, "", false
	}
	var params []ir.ParamDecl
	for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
		pName, ok := p.expect(tokIdent)
		if !ok {
			return nil, "", false
		}
		if _, ok := p.expect(tokColon); !ok {
			return nil, "", false
		}
		pType, ok := p.parseTypeName()
		if !ok {
			return nil, "", false
		}
		params = append(params, ir.ParamDecl{
			Name: pName.text,
			Type: pType,
			Span: pName.span,
		})
		if p.cur().kind == tokComma {
			p.advance()
		}
	}
	if _, ok := p.expect(tokRParen); !ok {
		return nil, "", false
	}
	retType := ""
	if p.cur().kind == tokArrow {
		p.advance()
		rType, ok := p.parseTypeName()
		if !ok {
			return nil, "", false
		}
		retType = rType
	}
	return params, retType, true
}

func (p *parser) parseSlot() (ir.SlotDecl, bool) {
	kw := p.advance()
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return ir.SlotDecl{}, false
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return ir.SlotDecl{}, false
	}
	var methods []ir.SlotMethodDecl
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		var mTraits []ir.TraitDecl
		for p.cur().kind == tokAt {
			tr, tOk := p.parseTrait()
			if !tOk {
				return ir.SlotDecl{}, false
			}
			mTraits = append(mTraits, tr)
		}
		mName, ok := p.expect(tokIdent)
		if !ok {
			return ir.SlotDecl{}, false
		}
		params, retType, ok := p.parseParamListAndReturn()
		if !ok {
			return ir.SlotDecl{}, false
		}
		var throws []string
		if p.cur().kind == tokIdent && p.cur().text == "throws" {
			p.advance()
			errTok, ok := p.expect(tokIdent)
			if !ok {
				return ir.SlotDecl{}, false
			}
			throws = append(throws, errTok.text)
			for p.cur().kind == tokComma {
				p.advance()
				eTok, ok := p.expect(tokIdent)
				if !ok {
					return ir.SlotDecl{}, false
				}
				throws = append(throws, eTok.text)
			}
		}
		for p.cur().kind == tokAt {
			tr, tOk := p.parseTrait()
			if !tOk {
				return ir.SlotDecl{}, false
			}
			mTraits = append(mTraits, tr)
		}
		methods = append(methods, ir.SlotMethodDecl{
			Name:       mName.text,
			Params:     params,
			ReturnType: retType,
			Throws:     throws,
			Traits:     mTraits,
			Span:       mName.span,
		})
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return ir.SlotDecl{}, false
	}
	return ir.SlotDecl{
		Name:    nameTok.text,
		Methods: methods,
		Span:    kw.span,
	}, true
}

func (p *parser) parseAction() (ir.ActionDecl, bool) {
	kw := p.advance()
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return ir.ActionDecl{}, false
	}
	params, retType, ok := p.parseParamListAndReturn()
	if !ok {
		return ir.ActionDecl{}, false
	}
	var traits []ir.TraitDecl
	for p.cur().kind == tokAt {
		tr, ok := p.parseTrait()
		if !ok {
			return ir.ActionDecl{}, false
		}
		traits = append(traits, tr)
	}
	steps, ok := p.parseStepBlock()
	if !ok {
		return ir.ActionDecl{}, false
	}
	return ir.ActionDecl{
		Name:       nameTok.text,
		Params:     params,
		ReturnType: retType,
		Traits:     traits,
		Steps:      steps,
		Span:       kw.span,
	}, true
}
