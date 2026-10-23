package frontend

import (
	"fmt"

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
		p.diags = append(p.diags, ir.Diagnostic{
			Code:     "FDN0102",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("expected %s, got %q", kind, t.text),
			Span:     t.span,
		})
		return t, false
	}
	p.advance()
	return t, true
}

func (p *parser) expectIdentText(text string) (token, bool) {
	t := p.cur()
	if t.kind != tokIdent || t.text != text {
		p.diags = append(p.diags, ir.Diagnostic{
			Code:     "FDN0102",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("expected keyword %q, got %q", text, t.text),
			Span:     t.span,
		})
		return t, false
	}
	p.advance()
	return t, true
}

func (p *parser) parseBlueprint() *ir.SemanticIR {
	sem := &ir.SemanticIR{}
	for p.cur().kind != tokEOF {
		t := p.cur()
		if t.kind != tokIdent {
			p.diags = append(p.diags, ir.Diagnostic{
				Code:     "FDN0103",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("unexpected token %q at top level", t.text),
				Span:     t.span,
			})
			p.advance()
			continue
		}
		switch t.text {
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
		case "action":
			if a, ok := p.parseAction(); ok {
				sem.Actions = append(sem.Actions, a)
			}
		case "scenario":
			if s, ok := p.parseScenario(); ok {
				sem.Scenarios = append(sem.Scenarios, s)
			}
		default:
			p.diags = append(p.diags, ir.Diagnostic{
				Code:     "FDN0103",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("unknown top-level declaration %q", t.text),
				Span:     t.span,
			})
			p.advance()
		}
	}
	return sem
}

func (p *parser) parseEnum() (ir.EnumDecl, bool) {
	kw := p.advance()
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return ir.EnumDecl{}, false
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return ir.EnumDecl{}, false
	}
	var variants []string
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		vTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.EnumDecl{}, false
		}
		variants = append(variants, vTok.text)
		if p.cur().kind == tokComma {
			p.advance()
		}
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return ir.EnumDecl{}, false
	}
	return ir.EnumDecl{
		Name:     nameTok.text,
		Variants: variants,
		Span:     kw.span,
	}, true
}

func (p *parser) parseEvent() (ir.EventDecl, bool) {
	name, fields, span, ok := p.parseNamedStruct()
	if !ok {
		return ir.EventDecl{}, false
	}
	return ir.EventDecl{Name: name, Fields: fields, Span: span}, true
}

func (p *parser) parsePolicy() (ir.PolicyDecl, bool) {
	kw := p.advance()
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return ir.PolicyDecl{}, false
	}
	if _, ok := p.expect(tokLParen); !ok {
		return ir.PolicyDecl{}, false
	}
	var params []ir.ParamDecl
	for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
		pName, ok := p.expect(tokIdent)
		if !ok {
			return ir.PolicyDecl{}, false
		}
		if _, ok := p.expect(tokColon); !ok {
			return ir.PolicyDecl{}, false
		}
		pType, ok := p.expect(tokIdent)
		if !ok {
			return ir.PolicyDecl{}, false
		}
		params = append(params, ir.ParamDecl{
			Name: pName.text,
			Type: pType.text,
			Span: pName.span,
		})
		if p.cur().kind == tokComma {
			p.advance()
		}
	}
	if _, ok := p.expect(tokRParen); !ok {
		return ir.PolicyDecl{}, false
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return ir.PolicyDecl{}, false
	}
	body, ok := p.parseExpr()
	if !ok {
		return ir.PolicyDecl{}, false
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return ir.PolicyDecl{}, false
	}
	return ir.PolicyDecl{
		Name:   nameTok.text,
		Params: params,
		Body:   body,
		Span:   kw.span,
	}, true
}

func (p *parser) parseNamedStruct() (string, []ir.FieldDecl, ir.Span, bool) {
	kw := p.advance()
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return "", nil, ir.Span{}, false
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return "", nil, ir.Span{}, false
	}
	fields, ok := p.parseFieldList()
	if !ok {
		return "", nil, ir.Span{}, false
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return "", nil, ir.Span{}, false
	}
	return nameTok.text, fields, kw.span, true
}

func (p *parser) parseValue() (ir.ValueDecl, bool) {
	name, fields, span, ok := p.parseNamedStruct()
	if !ok {
		return ir.ValueDecl{}, false
	}
	return ir.ValueDecl{Name: name, Fields: fields, Span: span}, true
}

func (p *parser) parseEntity() (ir.EntityDecl, bool) {
	name, fields, span, ok := p.parseNamedStruct()
	if !ok {
		return ir.EntityDecl{}, false
	}
	return ir.EntityDecl{Name: name, Fields: fields, Span: span}, true
}

func (p *parser) parseTypeName() (string, bool) {
	if p.cur().kind == tokLBracket {
		p.advance()
		if _, ok := p.expect(tokRBracket); !ok {
			return "", false
		}
		elemType, ok := p.expect(tokIdent)
		if !ok {
			return "", false
		}
		return "[]" + elemType.text, true
	}
	fType, ok := p.expect(tokIdent)
	if !ok {
		return "", false
	}
	typeStr := fType.text
	if p.cur().kind == tokLParen {
		p.advance()
		for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
			p.advance()
		}
		p.expect(tokRParen)
	}
	return typeStr, true
}

func (p *parser) parseFieldList() ([]ir.FieldDecl, bool) {
	var fields []ir.FieldDecl
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		fName, ok := p.expect(tokIdent)
		if !ok {
			return nil, false
		}
		if _, ok := p.expect(tokColon); !ok {
			return nil, false
		}
		typeStr, ok := p.parseTypeName()
		if !ok {
			return nil, false
		}
		optional := false
		if p.cur().kind == tokQMark {
			p.advance()
			optional = true
		}
		var traits []ir.TraitDecl
		for p.cur().kind == tokAt {
			tr, ok := p.parseTrait()
			if !ok {
				return nil, false
			}
			traits = append(traits, tr)
		}
		defaultVal := ""
		if p.cur().kind == tokAssign {
			p.advance()
			if p.cur().kind == tokDot {
				p.advance()
				tok, ok := p.expect(tokIdent)
				if !ok {
					return nil, false
				}
				defaultVal = "." + tok.text
			} else {
				tok := p.advance()
				defaultVal = tok.text
			}
		}
		for p.cur().kind == tokAt {
			tr, ok := p.parseTrait()
			if !ok {
				return nil, false
			}
			traits = append(traits, tr)
		}
		fields = append(fields, ir.FieldDecl{
			Name:       fName.text,
			Type:       typeStr,
			Optional:   optional,
			DefaultVal: defaultVal,
			Traits:     traits,
			Span:       fName.span,
		})
	}
	return fields, true
}

func (p *parser) parseTrait() (ir.TraitDecl, bool) {
	atTok, ok := p.expect(tokAt)
	if !ok {
		return ir.TraitDecl{}, false
	}
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return ir.TraitDecl{}, false
	}
	args := make(map[string]string)
	if p.cur().kind == tokLParen {
		p.advance()
		for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
			if p.peekOffset(1).kind == tokColon {
				kTok, ok := p.expect(tokIdent)
				if !ok {
					return ir.TraitDecl{}, false
				}
				p.expect(tokColon)
				vTok := p.advance()
				val := vTok.text
				for p.cur().kind == tokDot {
					p.advance()
					nextTok := p.advance()
					val += "." + nextTok.text
				}
				args[kTok.text] = val
			} else {
				vTok := p.advance()
				args["_"] = vTok.text
			}
			if p.cur().kind == tokComma {
				p.advance()
			}
		}
		if _, ok := p.expect(tokRParen); !ok {
			return ir.TraitDecl{}, false
		}
	}
	return ir.TraitDecl{
		Name: nameTok.text,
		Args: args,
		Span: atTok.span,
	}, true
}

func (p *parser) parseError() (ir.ErrorDecl, bool) {
	kw := p.advance()
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return ir.ErrorDecl{}, false
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return ir.ErrorDecl{}, false
	}
	errDecl := ir.ErrorDecl{
		Name: nameTok.text,
		Span: kw.span,
	}
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		propTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.ErrorDecl{}, false
		}
		if _, ok := p.expect(tokColon); !ok {
			return ir.ErrorDecl{}, false
		}
		switch propTok.text {
		case "kind":
			kTok, ok := p.expect(tokIdent)
			if !ok {
				return ir.ErrorDecl{}, false
			}
			errDecl.Kind = kTok.text
		case "message":
			mTok, ok := p.expect(tokString)
			if !ok {
				return ir.ErrorDecl{}, false
			}
			errDecl.Message = mTok.text
		default:
			p.diags = append(p.diags, ir.Diagnostic{
				Code:     "FDN0104",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("unknown error property %q", propTok.text),
				Span:     propTok.span,
			})
			p.advance()
		}
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return ir.ErrorDecl{}, false
	}
	return errDecl, true
}

func (p *parser) parseAction() (ir.ActionDecl, bool) {
	kw := p.advance()
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return ir.ActionDecl{}, false
	}
	if _, ok := p.expect(tokLParen); !ok {
		return ir.ActionDecl{}, false
	}
	var params []ir.ParamDecl
	for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
		pName, ok := p.expect(tokIdent)
		if !ok {
			return ir.ActionDecl{}, false
		}
		if _, ok := p.expect(tokColon); !ok {
			return ir.ActionDecl{}, false
		}
		pType, ok := p.parseTypeName()
		if !ok {
			return ir.ActionDecl{}, false
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
		return ir.ActionDecl{}, false
	}
	retType := ""
	if p.cur().kind == tokArrow {
		p.advance()
		rType, ok := p.parseTypeName()
		if !ok {
			return ir.ActionDecl{}, false
		}
		retType = rType
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
