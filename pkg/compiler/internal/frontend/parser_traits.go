package frontend

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

func (p *parser) parseTrait() (ir.TraitDecl, bool) {
	atTok, ok := p.expect(tokAt)
	if !ok {
		return ir.TraitDecl{}, false
	}
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return ir.TraitDecl{}, false
	}
	traitName := nameTok.text
	if p.cur().kind == tokColon {
		p.advance()
		localTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.TraitDecl{}, false
		}
		traitName = nameTok.text + ":" + localTok.text
		if nameTok.text == platform.ReservedTraitNamespace {
			p.diags = append(p.diags, ir.Diagnostic{
				Code:     "FDN0225",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("trait namespace %q is reserved for standard traits; standard traits are written without a namespace", platform.ReservedTraitNamespace),
				Span:     nameTok.span,
			})
		}
	} else if nameTok.text == "cron" {
		p.diags = append(p.diags, ir.Diagnostic{
			Code:     "FDN0115",
			Severity: ir.SeverityError,
			Message:  "@cron trait is reserved until distributed leader election is implemented; un-coordinated tickers double-fire under multiple replicas",
			Span:     nameTok.span,
		})
	}
	args := make(map[string]string)
	listArgs := make(map[string][]string)
	if p.cur().kind == tokLParen {
		p.advance()
		for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
			key := "_"
			if p.cur().kind == tokIdent && p.peekOffset(1).kind == tokColon {
				kTok, ok := p.expect(tokIdent)
				if !ok {
					return ir.TraitDecl{}, false
				}
				p.expect(tokColon)
				key = kTok.text
			}

			if p.cur().kind == tokLBracket {
				items, ok := p.parseTraitListArg()
				if !ok {
					return ir.TraitDecl{}, false
				}
				listArgs[key] = items
			} else {
				vTok := p.advance()
				val := vTok.text
				for p.cur().kind == tokDot {
					p.advance()
					nextTok := p.advance()
					val += "." + nextTok.text
				}
				args[key] = val
			}

			if p.cur().kind == tokComma {
				p.advance()
			}
		}
		if _, ok := p.expect(tokRParen); !ok {
			return ir.TraitDecl{}, false
		}
	}
	tr := ir.TraitDecl{
		Name: traitName,
		Args: args,
		Span: atTok.span,
	}
	if len(listArgs) > 0 {
		tr.ListArgs = listArgs
	}
	return tr, true
}

func (p *parser) parseTraitListArg() ([]string, bool) {
	if _, ok := p.expect(tokLBracket); !ok {
		return nil, false
	}
	items := make([]string, 0)
	for p.cur().kind != tokRBracket && p.cur().kind != tokEOF {
		if p.cur().kind == tokString {
			items = append(items, p.cur().text)
			p.advance()
		} else {
			p.diags = append(p.diags, ir.Diagnostic{
				Code:     "FDN0102",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("expected string literal in list, got %q", p.cur().text),
				Span:     p.cur().span,
			})
			return nil, false
		}
		if p.cur().kind == tokComma {
			p.advance()
		} else if p.cur().kind != tokRBracket {
			p.diags = append(p.diags, ir.Diagnostic{
				Code:     "FDN0102",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("expected ',' or ']', got %q", p.cur().text),
				Span:     p.cur().span,
			})
			return nil, false
		}
	}
	if _, ok := p.expect(tokRBracket); !ok {
		return nil, false
	}
	return items, true
}
