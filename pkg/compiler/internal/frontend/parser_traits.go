package frontend

import (
	"fmt"
	"strconv"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

// unquoteTraitArg removes surrounding quotes from a string literal.
// If strconv.Unquote fails, the function removes outer quote characters directly.
func unquoteTraitArg(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			if unquoted, err := strconv.Unquote(s); err == nil {
				return unquoted
			}
			return s[1 : len(s)-1]
		}
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			return s[1 : len(s)-1]
		}
	}
	return s
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
	traitName := nameTok.text
	if p.cur().kind == tokColon {
		p.advance()
		localTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.TraitDecl{}, false
		}
		traitName = nameTok.text + ":" + localTok.text
		if nameTok.text == platform.ReservedTraitNamespace {
			p.diags = append(p.diags, diagnostics.NewWithDetail(
				"FDN0225",
				nameTok.span,
				fmt.Sprintf("trait namespace %q is reserved for standard traits; standard traits are written without a namespace", platform.ReservedTraitNamespace),
			))
		}
	} else if nameTok.text == "cron" {
		p.diags = append(p.diags, diagnostics.NewWithDetail(
			"FDN0115",
			nameTok.span,
			"@cron trait is reserved until distributed leader election is implemented; un-coordinated tickers double-fire under multiple replicas",
		))
	}
	args := make(map[string]string)
	listArgs := make(map[string][]string)
	if p.cur().kind == tokLParen {
		p.advance()
		posIndex := 0
		traitDef, hasDef := platform.StandardTrait(traitName)
		if !hasDef {
			traitDef, hasDef = platform.StandardTrait(nameTok.text)
		}
		for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
			key := ""
			if p.cur().kind == tokIdent && (p.peekOffset(1).kind == tokColon || p.peekOffset(1).kind == tokAssign) {
				kTok, ok := p.expect(tokIdent)
				if !ok {
					return ir.TraitDecl{}, false
				}
				p.advance() // consume ':' or '='
				key = kTok.text
			} else {
				if hasDef && posIndex < len(traitDef.Args) {
					key = traitDef.Args[posIndex].Name
				} else {
					key = fmt.Sprintf("_%d", posIndex)
				}
				posIndex++
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
				args[key] = unquoteTraitArg(val)
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
			items = append(items, unquoteTraitArg(p.cur().text))
			p.advance()
		} else {
			p.diags = append(p.diags, diagnostics.New("FDN0102", p.cur().span, "string literal in list", p.cur().text))
			return nil, false
		}
		if p.cur().kind == tokComma {
			p.advance()
		} else if p.cur().kind != tokRBracket {
			p.diags = append(p.diags, diagnostics.New("FDN0102", p.cur().span, "',' or ']'", p.cur().text))
			return nil, false
		}
	}
	if _, ok := p.expect(tokRBracket); !ok {
		return nil, false
	}
	return items, true
}
