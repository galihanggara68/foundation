package frontend

import (
	"fmt"
	"strconv"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

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

func (p *parser) parseActor() (ir.ActorDecl, bool) {
	name, _, fields, span, ok := p.parseNamedStruct()
	if !ok {
		return ir.ActorDecl{}, false
	}
	return ir.ActorDecl{Name: name, Fields: fields, Span: span}, true
}

func (p *parser) parseEvent() (ir.EventDecl, bool) {
	name, _, fields, span, ok := p.parseNamedStruct()
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
	params, _, ok := p.parseParamListAndReturn()
	if !ok {
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

func (p *parser) parseNamedStruct() (string, []ir.TraitDecl, []ir.FieldDecl, ir.Span, bool) {
	kw := p.advance()
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return "", nil, nil, ir.Span{}, false
	}
	var traits []ir.TraitDecl
	for p.cur().kind == tokAt {
		tr, ok := p.parseTrait()
		if !ok {
			return "", nil, nil, ir.Span{}, false
		}
		traits = append(traits, tr)
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return "", nil, nil, ir.Span{}, false
	}
	fields, ok := p.parseFieldList()
	if !ok {
		return "", nil, nil, ir.Span{}, false
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return "", nil, nil, ir.Span{}, false
	}
	return nameTok.text, traits, fields, kw.span, true
}

func (p *parser) parseValue() (ir.ValueDecl, bool) {
	name, _, fields, span, ok := p.parseNamedStruct()
	if !ok {
		return ir.ValueDecl{}, false
	}
	return ir.ValueDecl{Name: name, Fields: fields, Span: span}, true
}

func (p *parser) parseEntity() (ir.EntityDecl, bool) {
	name, traits, fields, span, ok := p.parseNamedStruct()
	if !ok {
		return ir.EntityDecl{}, false
	}
	return ir.EntityDecl{Name: name, Traits: traits, Fields: fields, Span: span}, true
}

func (p *parser) parseTypeNameWithParams() (string, string, bool) {
	if p.cur().kind == tokLBracket {
		p.advance()
		if _, ok := p.expect(tokRBracket); !ok {
			return "", "", false
		}
		elemType, ok := p.expect(tokIdent)
		if !ok {
			return "", "", false
		}
		return "[]" + elemType.text, "", true
	}
	fType, ok := p.expect(tokIdent)
	if !ok {
		return "", "", false
	}
	typeStr := fType.text
	typeParams := ""
	if p.cur().kind == tokLParen {
		p.advance()
		for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
			if p.cur().kind == tokComma {
				p.advance()
				typeParams += ", "
				continue
			}
			tok := p.advance()
			typeParams += tok.text
		}
		p.expect(tokRParen)
	}
	return typeStr, typeParams, true
}

func (p *parser) parseTypeName() (string, bool) {
	t, _, ok := p.parseTypeNameWithParams()
	return t, ok
}

func (p *parser) parseFieldList() ([]ir.FieldDecl, bool) {
	var fields []ir.FieldDecl
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		var preTraits []ir.TraitDecl
		for p.cur().kind == tokAt {
			tr, ok := p.parseTrait()
			if !ok {
				return nil, false
			}
			preTraits = append(preTraits, tr)
		}
		fName, ok := p.expect(tokIdent)
		if !ok {
			return nil, false
		}
		if _, ok := p.expect(tokColon); !ok {
			return nil, false
		}
		typeStr, typeParams, ok := p.parseTypeNameWithParams()
		if !ok {
			return nil, false
		}
		optional := false
		if p.cur().kind == tokQMark {
			p.advance()
			optional = true
		}
		traits := preTraits
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
			TypeParams: typeParams,
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
	if nameTok.text == "cron" {
		p.diags = append(p.diags, ir.Diagnostic{
			Code:     "FDN0115",
			Severity: ir.SeverityError,
			Message:  "@cron trait is reserved until distributed leader election is implemented; un-coordinated tickers double-fire under multiple replicas",
			Span:     nameTok.span,
		})
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
				val := vTok.text
				for p.cur().kind == tokDot {
					p.advance()
					nextTok := p.advance()
					val += "." + nextTok.text
				}
				args["_"] = val
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
			errDecl.Kind = ir.ErrorKind(kTok.text)
		case "message":
			mTok, ok := p.expect(tokString)
			if !ok {
				return ir.ErrorDecl{}, false
			}
			errDecl.Message = mTok.text
		case "status":
			sign := 1
			if p.cur().kind == tokMinus {
				p.advance()
				sign = -1
			}
			sTok, ok := p.expect(tokNumber)
			if !ok {
				return ir.ErrorDecl{}, false
			}
			status, atoiErr := strconv.Atoi(sTok.text)
			if atoiErr != nil {
				p.diags = append(p.diags, ir.Diagnostic{
					Code:     "FDN0220",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("error status %q must be an integer", sTok.text),
					Span:     sTok.span,
				})
			} else {
				errDecl.Status = sign * status
			}
		case "messages":
			messages, ok := p.parseErrorMessages()
			if !ok {
				return ir.ErrorDecl{}, false
			}
			errDecl.Messages = messages
		case "posture":
			pTok, ok := p.expect(tokIdent)
			if !ok {
				return ir.ErrorDecl{}, false
			}
			errDecl.Posture = ir.ErrorPosture(pTok.text)
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

// parseErrorMessages reads a `messages: { <locale>: "<text>" }` block. A locale is a
// bare identifier or a quoted tag, so a region subtag can be written as "en-US". The
// keys are otherwise free-form: the compiler records what a blueprint declares rather
// than deciding which locales exist.
func (p *parser) parseErrorMessages() (map[string]string, bool) {
	if _, ok := p.expect(tokLBrace); !ok {
		return nil, false
	}
	messages := make(map[string]string)
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		localeTok := p.cur()
		if localeTok.kind != tokIdent && localeTok.kind != tokString {
			p.diags = append(p.diags, ir.Diagnostic{
				Code:     "FDN0102",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("expected LOCALE, got %q", localeTok.text),
				Span:     localeTok.span,
			})
			return nil, false
		}
		p.advance()
		if _, ok := p.expect(tokColon); !ok {
			return nil, false
		}
		textTok, ok := p.expect(tokString)
		if !ok {
			return nil, false
		}
		if _, duplicate := messages[localeTok.text]; duplicate {
			p.diags = append(p.diags, ir.Diagnostic{
				Code:     "FDN0108",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("duplicate locale %q in error messages", localeTok.text),
				Span:     localeTok.span,
			})
		}
		messages[localeTok.text] = textTok.text
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return nil, false
	}
	return messages, true
}
