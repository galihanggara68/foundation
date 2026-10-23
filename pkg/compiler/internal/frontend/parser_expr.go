package frontend

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func (p *parser) parseExpr() (ir.Expr, bool) {
	return p.parseOrExpr()
}

func (p *parser) parseLeftAssocBinary(nextPrec func() (ir.Expr, bool), matchOp func(tokenKind) bool) (ir.Expr, bool) {
	left, ok := nextPrec()
	if !ok {
		return ir.Expr{}, false
	}
	for matchOp(p.cur().kind) {
		opTok := p.advance()
		right, ok := nextPrec()
		if !ok {
			return ir.Expr{}, false
		}
		lCopy, rCopy := left, right
		left = ir.Expr{Kind: ir.ExprBinary, Op: opTok.text, Left: &lCopy, Right: &rCopy, Span: opTok.span}
	}
	return left, true
}

func (p *parser) parseOrExpr() (ir.Expr, bool) {
	return p.parseLeftAssocBinary(p.parseAndExpr, func(k tokenKind) bool { return k == tokOr })
}

func (p *parser) parseAndExpr() (ir.Expr, bool) {
	return p.parseLeftAssocBinary(p.parseCompareExpr, func(k tokenKind) bool { return k == tokAnd })
}

func (p *parser) parseCompareExpr() (ir.Expr, bool) {
	return p.parseLeftAssocBinary(p.parseAddExpr, func(k tokenKind) bool {
		return k == tokEq || k == tokNeq || k == tokGte || k == tokLte || k == tokGt || k == tokLt
	})
}

func (p *parser) parseAddExpr() (ir.Expr, bool) {
	return p.parseLeftAssocBinary(p.parseMulExpr, func(k tokenKind) bool { return k == tokPlus || k == tokMinus })
}

func (p *parser) parseMulExpr() (ir.Expr, bool) {
	return p.parseLeftAssocBinary(p.parsePostfixExpr, func(k tokenKind) bool { return k == tokStar || k == tokSlash })
}

func (p *parser) parsePostfixExpr() (ir.Expr, bool) {
	expr, ok := p.parsePrimaryExpr()
	if !ok {
		return ir.Expr{}, false
	}
	for p.cur().kind == tokDot {
		dotTok := p.advance()
		memberTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.Expr{}, false
		}
		if p.cur().kind == tokLParen {
			p.advance()
			var paramName string
			var bodyPtr *ir.Expr
			if p.cur().kind != tokRParen {
				if p.cur().kind == tokIdent && p.peekOffset(1).kind == tokFatArrow {
					paramTok := p.advance()
					p.advance()
					bodyExpr, bOk := p.parseExpr()
					if !bOk {
						return ir.Expr{}, false
					}
					paramName = paramTok.text
					bodyPtr = &bodyExpr
				} else {
					argExpr, aOk := p.parseExpr()
					if !aOk {
						return ir.Expr{}, false
					}
					bodyPtr = &argExpr
				}
			}
			if _, ok := p.expect(tokRParen); !ok {
				return ir.Expr{}, false
			}
			recvCopy := expr
			expr = ir.Expr{
				Kind:   ir.ExprCombinator,
				Op:     memberTok.text,
				Target: paramName,
				Left:   &recvCopy,
				Right:  bodyPtr,
				Span:   dotTok.span,
			}
			continue
		}
		if expr.Kind == ir.ExprIdent {
			expr = ir.Expr{
				Kind:   ir.ExprField,
				Target: expr.Value,
				Field:  memberTok.text,
				Span:   expr.Span,
			}
		} else {
			recvCopy := expr
			expr = ir.Expr{
				Kind:  ir.ExprField,
				Left:  &recvCopy,
				Field: memberTok.text,
				Span:  dotTok.span,
			}
		}
	}
	return expr, true
}

func (p *parser) parsePrimaryExpr() (ir.Expr, bool) {
	t := p.cur()
	switch t.kind {
	case tokString:
		p.advance()
		return ir.Expr{Kind: ir.ExprString, Value: t.text, Span: t.span}, true
	case tokNumber:
		p.advance()
		if strings.Contains(t.text, ".") {
			return ir.Expr{Kind: ir.ExprFloat, Value: t.text, Span: t.span}, true
		}
		return ir.Expr{Kind: ir.ExprInt, Value: t.text, Span: t.span}, true
	case tokDot:
		p.advance()
		idTok, ok := p.expect(tokIdent)
		if !ok {
			return ir.Expr{}, false
		}
		return ir.Expr{Kind: ir.ExprEnumLit, Value: idTok.text, Span: t.span}, true
	case tokLBracket:
		p.advance()
		var items []ir.Expr
		for p.cur().kind != tokRBracket && p.cur().kind != tokEOF {
			item, ok := p.parseExpr()
			if !ok {
				return ir.Expr{}, false
			}
			items = append(items, item)
			if p.cur().kind == tokComma {
				p.advance()
			}
		}
		if _, ok := p.expect(tokRBracket); !ok {
			return ir.Expr{}, false
		}
		return ir.Expr{
			Kind:  ir.ExprList,
			Items: items,
			Span:  t.span,
		}, true
	case tokIdent:
		p.advance()
		if t.text == "true" || t.text == "false" {
			return ir.Expr{Kind: ir.ExprBool, Value: t.text, Span: t.span}, true
		}
		if p.cur().kind == tokLParen {
			p.advance()
			var args []ir.Expr
			for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
				arg, ok := p.parseExpr()
				if !ok {
					return ir.Expr{}, false
				}
				args = append(args, arg)
				if p.cur().kind == tokComma {
					p.advance()
				}
			}
			if _, ok := p.expect(tokRParen); !ok {
				return ir.Expr{}, false
			}
			return ir.Expr{
				Kind:  ir.ExprCall,
				Value: t.text,
				Items: args,
				Span:  t.span,
			}, true
		}
		if p.isStructLiteralStart() {
			fields, ok := p.parseFieldInitBlock()
			if !ok {
				return ir.Expr{}, false
			}
			return ir.Expr{
				Kind:     ir.ExprConstruct,
				TypeName: t.text,
				Fields:   fields,
				Span:     t.span,
			}, true
		}
		return ir.Expr{Kind: ir.ExprIdent, Value: t.text, Span: t.span}, true
	default:
		p.diags = append(p.diags, ir.Diagnostic{
			Code:     "FDN0106",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("expected expression, got %q", t.text),
			Span:     t.span,
		})
		p.advance()
		return ir.Expr{}, false
	}
}

func (p *parser) isStructLiteralStart() bool {
	if p.cur().kind != tokLBrace {
		return false
	}
	next := p.peekOffset(1)
	if next.kind == tokRBrace {
		return true
	}
	return next.kind == tokIdent && p.peekOffset(2).kind == tokColon
}
