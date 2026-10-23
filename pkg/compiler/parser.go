package compiler

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenKind string

const (
	tokEOF       tokenKind = "EOF"
	tokIdent     tokenKind = "IDENT"
	tokString    tokenKind = "STRING"
	tokNumber    tokenKind = "NUMBER"
	tokLBrace    tokenKind = "{"
	tokRBrace    tokenKind = "}"
	tokLParen    tokenKind = "("
	tokRParen    tokenKind = ")"
	tokColon     tokenKind = ":"
	tokComma     tokenKind = ","
	tokDot       tokenKind = "."
	tokAt        tokenKind = "@"
	tokQMark     tokenKind = "?"
	tokAssign    tokenKind = "="
	tokEq        tokenKind = "=="
	tokNeq       tokenKind = "!="
	tokGte       tokenKind = ">="
	tokLte       tokenKind = "<="
	tokGt        tokenKind = ">"
	tokLt        tokenKind = "<"
	tokArrow     tokenKind = "->"
	tokPlus      tokenKind = "+"
	tokMinus     tokenKind = "-"
	tokStar      tokenKind = "*"
	tokSlash     tokenKind = "/"
	tokAnd       tokenKind = "&&"
	tokOr        tokenKind = "||"
)

type token struct {
	kind  tokenKind
	text  string
	span  Span
}

type lexer struct {
	file   string
	src    []rune
	pos    int
	line   int
	col    int
	diags  []Diagnostic
}

func newLexer(file, src string) *lexer {
	return &lexer{
		file: file,
		src:  []rune(src),
		pos:  0,
		line: 1,
		col:  1,
	}
}

func (l *lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) peekNext() rune {
	if l.pos+1 >= len(l.src) {
		return 0
	}
	return l.src[l.pos+1]
}

func (l *lexer) next() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	r := l.src[l.pos]
	l.pos++
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

func (l *lexer) tokenize() ([]token, []Diagnostic) {
	var tokens []token
	for {
		l.skipWhitespaceAndComments()
		span := Span{File: l.file, Line: l.line, Column: l.col}
		r := l.peek()
		if r == 0 {
			tokens = append(tokens, token{kind: tokEOF, span: span})
			break
		}

		switch r {
		case '{':
			l.next()
			tokens = append(tokens, token{kind: tokLBrace, text: "{", span: span})
		case '}':
			l.next()
			tokens = append(tokens, token{kind: tokRBrace, text: "}", span: span})
		case '(':
			l.next()
			tokens = append(tokens, token{kind: tokLParen, text: "(", span: span})
		case ')':
			l.next()
			tokens = append(tokens, token{kind: tokRParen, text: ")", span: span})
		case ':':
			l.next()
			tokens = append(tokens, token{kind: tokColon, text: ":", span: span})
		case ',':
			l.next()
			tokens = append(tokens, token{kind: tokComma, text: ",", span: span})
		case '.':
			l.next()
			tokens = append(tokens, token{kind: tokDot, text: ".", span: span})
		case '@':
			l.next()
			tokens = append(tokens, token{kind: tokAt, text: "@", span: span})
		case '?':
			l.next()
			tokens = append(tokens, token{kind: tokQMark, text: "?", span: span})
		case '+':
			l.next()
			tokens = append(tokens, token{kind: tokPlus, text: "+", span: span})
		case '*':
			l.next()
			tokens = append(tokens, token{kind: tokStar, text: "*", span: span})
		case '/':
			l.next()
			tokens = append(tokens, token{kind: tokSlash, text: "/", span: span})
		case '-':
			l.next()
			if l.peek() == '>' {
				l.next()
				tokens = append(tokens, token{kind: tokArrow, text: "->", span: span})
			} else {
				tokens = append(tokens, token{kind: tokMinus, text: "-", span: span})
			}
		case '=':
			l.next()
			if l.peek() == '=' {
				l.next()
				tokens = append(tokens, token{kind: tokEq, text: "==", span: span})
			} else {
				tokens = append(tokens, token{kind: tokAssign, text: "=", span: span})
			}
		case '!':
			l.next()
			if l.peek() == '=' {
				l.next()
				tokens = append(tokens, token{kind: tokNeq, text: "!=", span: span})
			} else {
				l.diags = append(l.diags, Diagnostic{
					Code:     "FDN0100",
					Severity: SeverityError,
					Message:  "unexpected character '!'",
					Span:     span,
				})
			}
		case '>':
			l.next()
			if l.peek() == '=' {
				l.next()
				tokens = append(tokens, token{kind: tokGte, text: ">=", span: span})
			} else {
				tokens = append(tokens, token{kind: tokGt, text: ">", span: span})
			}
		case '<':
			l.next()
			if l.peek() == '=' {
				l.next()
				tokens = append(tokens, token{kind: tokLte, text: "<=", span: span})
			} else {
				tokens = append(tokens, token{kind: tokLt, text: "<", span: span})
			}
		case '&':
			l.next()
			if l.peek() == '&' {
				l.next()
				tokens = append(tokens, token{kind: tokAnd, text: "&&", span: span})
			} else {
				l.diags = append(l.diags, Diagnostic{
					Code:     "FDN0100",
					Severity: SeverityError,
					Message:  "unexpected character '&'",
					Span:     span,
				})
			}
		case '|':
			l.next()
			if l.peek() == '|' {
				l.next()
				tokens = append(tokens, token{kind: tokOr, text: "||", span: span})
			} else {
				l.diags = append(l.diags, Diagnostic{
					Code:     "FDN0100",
					Severity: SeverityError,
					Message:  "unexpected character '|'",
					Span:     span,
				})
			}
		case '"':
			l.next()
			var sb strings.Builder
			closed := false
			for l.peek() != 0 {
				c := l.next()
				if c == '"' {
					closed = true
					break
				}
				if c == '\\' && l.peek() != 0 {
					c = l.next()
				}
				sb.WriteRune(c)
			}
			if !closed {
				l.diags = append(l.diags, Diagnostic{
					Code:     "FDN0101",
					Severity: SeverityError,
					Message:  "unterminated string literal",
					Span:     span,
				})
			}
			tokens = append(tokens, token{kind: tokString, text: sb.String(), span: span})
		default:
			if unicode.IsDigit(r) {
				var sb strings.Builder
				for unicode.IsDigit(l.peek()) || l.peek() == '.' {
					sb.WriteRune(l.next())
				}
				tokens = append(tokens, token{kind: tokNumber, text: sb.String(), span: span})
			} else if unicode.IsLetter(r) || r == '_' {
				var sb strings.Builder
				for unicode.IsLetter(l.peek()) || unicode.IsDigit(l.peek()) || l.peek() == '_' {
					sb.WriteRune(l.next())
				}
				tokens = append(tokens, token{kind: tokIdent, text: sb.String(), span: span})
			} else {
				l.next()
				l.diags = append(l.diags, Diagnostic{
					Code:     "FDN0100",
					Severity: SeverityError,
					Message:  fmt.Sprintf("unexpected character %q", r),
					Span:     span,
				})
			}
		}
	}
	return tokens, l.diags
}

func (l *lexer) skipWhitespaceAndComments() {
	for {
		r := l.peek()
		if unicode.IsSpace(r) {
			l.next()
			continue
		}
		if r == '/' && l.peekNext() == '/' {
			for l.peek() != 0 && l.peek() != '\n' {
				l.next()
			}
			continue
		}
		if r == '#' {
			for l.peek() != 0 && l.peek() != '\n' {
				l.next()
			}
			continue
		}
		break
	}
}

type parser struct {
	tokens []token
	pos    int
	diags  []Diagnostic
}

func parseFile(file, src string) (*SemanticIR, []Diagnostic) {
	lex := newLexer(file, src)
	tokens, lexDiags := lex.tokenize()
	if len(lexDiags) > 0 {
		return nil, lexDiags
	}
	p := &parser{tokens: tokens}
	ir := p.parseBlueprint()
	return ir, p.diags
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
		p.diags = append(p.diags, Diagnostic{
			Code:     "FDN0102",
			Severity: SeverityError,
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
		p.diags = append(p.diags, Diagnostic{
			Code:     "FDN0102",
			Severity: SeverityError,
			Message:  fmt.Sprintf("expected keyword %q, got %q", text, t.text),
			Span:     t.span,
		})
		return t, false
	}
	p.advance()
	return t, true
}

func (p *parser) parseBlueprint() *SemanticIR {
	ir := &SemanticIR{}
	for p.cur().kind != tokEOF {
		t := p.cur()
		if t.kind != tokIdent {
			p.diags = append(p.diags, Diagnostic{
				Code:     "FDN0103",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unexpected token %q at top level", t.text),
				Span:     t.span,
			})
			p.advance()
			continue
		}
		switch t.text {
		case "value":
			if v, ok := p.parseValue(); ok {
				ir.Values = append(ir.Values, v)
			}
		case "entity":
			if e, ok := p.parseEntity(); ok {
				ir.Entities = append(ir.Entities, e)
			}
		case "error":
			if e, ok := p.parseError(); ok {
				ir.Errors = append(ir.Errors, e)
			}
		case "action":
			if a, ok := p.parseAction(); ok {
				ir.Actions = append(ir.Actions, a)
			}
		case "scenario":
			if s, ok := p.parseScenario(); ok {
				ir.Scenarios = append(ir.Scenarios, s)
			}
		default:
			p.diags = append(p.diags, Diagnostic{
				Code:     "FDN0103",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown top-level declaration %q", t.text),
				Span:     t.span,
			})
			p.advance()
		}
	}
	return ir
}

func (p *parser) parseNamedStruct() (string, []FieldDecl, Span, bool) {
	kw := p.advance()
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return "", nil, Span{}, false
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return "", nil, Span{}, false
	}
	fields, ok := p.parseFieldList()
	if !ok {
		return "", nil, Span{}, false
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return "", nil, Span{}, false
	}
	return nameTok.text, fields, kw.span, true
}

func (p *parser) parseValue() (ValueDecl, bool) {
	name, fields, span, ok := p.parseNamedStruct()
	if !ok {
		return ValueDecl{}, false
	}
	return ValueDecl{Name: name, Fields: fields, Span: span}, true
}

func (p *parser) parseEntity() (EntityDecl, bool) {
	name, fields, span, ok := p.parseNamedStruct()
	if !ok {
		return EntityDecl{}, false
	}
	return EntityDecl{Name: name, Fields: fields, Span: span}, true
}

func (p *parser) parseFieldList() ([]FieldDecl, bool) {
	var fields []FieldDecl
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		fName, ok := p.expect(tokIdent)
		if !ok {
			return nil, false
		}
		if _, ok := p.expect(tokColon); !ok {
			return nil, false
		}
		fType, ok := p.expect(tokIdent)
		if !ok {
			return nil, false
		}
		typeStr := fType.text
		// Support parameterized types like decimal(10, 2)
		if p.cur().kind == tokLParen {
			p.advance()
			for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
				p.advance()
			}
			p.expect(tokRParen)
		}
		optional := false
		if p.cur().kind == tokQMark {
			p.advance()
			optional = true
		}
		var traits []TraitDecl
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
		fields = append(fields, FieldDecl{
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

func (p *parser) parseTrait() (TraitDecl, bool) {
	atTok, ok := p.expect(tokAt)
	if !ok {
		return TraitDecl{}, false
	}
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return TraitDecl{}, false
	}
	args := make(map[string]string)
	if p.cur().kind == tokLParen {
		p.advance()
		for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
			if p.peekOffset(1).kind == tokColon {
				kTok, ok := p.expect(tokIdent)
				if !ok {
					return TraitDecl{}, false
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
			return TraitDecl{}, false
		}
	}
	return TraitDecl{
		Name: nameTok.text,
		Args: args,
		Span: atTok.span,
	}, true
}

func (p *parser) parseError() (ErrorDecl, bool) {
	kw := p.advance() // error
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return ErrorDecl{}, false
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return ErrorDecl{}, false
	}
	errDecl := ErrorDecl{
		Name: nameTok.text,
		Span: kw.span,
	}
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		propTok, ok := p.expect(tokIdent)
		if !ok {
			return ErrorDecl{}, false
		}
		if _, ok := p.expect(tokColon); !ok {
			return ErrorDecl{}, false
		}
		switch propTok.text {
		case "kind":
			kTok, ok := p.expect(tokIdent)
			if !ok {
				return ErrorDecl{}, false
			}
			errDecl.Kind = kTok.text
		case "message":
			mTok, ok := p.expect(tokString)
			if !ok {
				return ErrorDecl{}, false
			}
			errDecl.Message = mTok.text
		default:
			p.diags = append(p.diags, Diagnostic{
				Code:     "FDN0104",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown error property %q", propTok.text),
				Span:     propTok.span,
			})
			p.advance()
		}
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return ErrorDecl{}, false
	}
	return errDecl, true
}

func (p *parser) parseAction() (ActionDecl, bool) {
	kw := p.advance() // action
	nameTok, ok := p.expect(tokIdent)
	if !ok {
		return ActionDecl{}, false
	}
	if _, ok := p.expect(tokLParen); !ok {
		return ActionDecl{}, false
	}
	var params []ParamDecl
	for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
		pName, ok := p.expect(tokIdent)
		if !ok {
			return ActionDecl{}, false
		}
		if _, ok := p.expect(tokColon); !ok {
			return ActionDecl{}, false
		}
		pType, ok := p.expect(tokIdent)
		if !ok {
			return ActionDecl{}, false
		}
		params = append(params, ParamDecl{
			Name: pName.text,
			Type: pType.text,
			Span: pName.span,
		})
		if p.cur().kind == tokComma {
			p.advance()
		}
	}
	if _, ok := p.expect(tokRParen); !ok {
		return ActionDecl{}, false
	}
	retType := ""
	if p.cur().kind == tokArrow {
		p.advance()
		rTok, ok := p.expect(tokIdent)
		if !ok {
			return ActionDecl{}, false
		}
		retType = rTok.text
	}
	var traits []TraitDecl
	for p.cur().kind == tokAt {
		tr, ok := p.parseTrait()
		if !ok {
			return ActionDecl{}, false
		}
		traits = append(traits, tr)
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return ActionDecl{}, false
	}
	var steps []StepDecl
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		step, ok := p.parseStep()
		if !ok {
			return ActionDecl{}, false
		}
		steps = append(steps, step)
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return ActionDecl{}, false
	}
	return ActionDecl{
		Name:       nameTok.text,
		Params:     params,
		ReturnType: retType,
		Traits:     traits,
		Steps:      steps,
		Span:       kw.span,
	}, true
}

func (p *parser) parseStep() (StepDecl, bool) {
	t := p.cur()
	if t.kind != tokIdent {
		p.diags = append(p.diags, Diagnostic{
			Code:     "FDN0105",
			Severity: SeverityError,
			Message:  fmt.Sprintf("expected action statement, got %q", t.text),
			Span:     t.span,
		})
		p.advance()
		return StepDecl{}, false
	}
	switch t.text {
	case "let":
		p.advance()
		varTok, ok := p.expect(tokIdent)
		if !ok {
			return StepDecl{}, false
		}
		if _, ok := p.expect(tokAssign); !ok {
			return StepDecl{}, false
		}
		next := p.cur()
		if next.kind == tokIdent && next.text == "query" {
			p.advance() // query
			if _, ok := p.expectIdentText("one"); !ok {
				return StepDecl{}, false
			}
			entTok, ok := p.expect(tokIdent)
			if !ok {
				return StepDecl{}, false
			}
			if _, ok := p.expectIdentText("where"); !ok {
				return StepDecl{}, false
			}
			fieldTok, ok := p.expect(tokIdent)
			if !ok {
				return StepDecl{}, false
			}
			if _, ok := p.expect(tokEq); !ok {
				return StepDecl{}, false
			}
			valExpr, ok := p.parseExpr()
			if !ok {
				return StepDecl{}, false
			}
			elseErr := ""
			if p.cur().kind == tokIdent && p.cur().text == "else" {
				p.advance()
				errTok, ok := p.expect(tokIdent)
				if !ok {
					return StepDecl{}, false
				}
				elseErr = errTok.text
			}
			return StepDecl{
				Kind:       StepQuery,
				VarName:    varTok.text,
				EntityName: entTok.text,
				WhereField: fieldTok.text,
				WhereValue: &valExpr,
				ElseError:  elseErr,
				Span:       t.span,
			}, true
		}
		if next.kind == tokIdent && next.text == "create" {
			p.advance() // create
			entTok, ok := p.expect(tokIdent)
			if !ok {
				return StepDecl{}, false
			}
			fields, ok := p.parseFieldInitBlock()
			if !ok {
				return StepDecl{}, false
			}
			return StepDecl{
				Kind:       StepCreate,
				VarName:    varTok.text,
				EntityName: entTok.text,
				Fields:     fields,
				Span:       t.span,
			}, true
		}
		expr, ok := p.parseExpr()
		if !ok {
			return StepDecl{}, false
		}
		return StepDecl{
			Kind:    StepLet,
			VarName: varTok.text,
			Expr:    &expr,
			Span:    t.span,
		}, true

	case "return":
		p.advance()
		expr, ok := p.parseExpr()
		if !ok {
			return StepDecl{}, false
		}
		return StepDecl{
			Kind: StepReturn,
			Expr: &expr,
			Span: t.span,
		}, true

	default:
		p.diags = append(p.diags, Diagnostic{
			Code:     "FDN0105",
			Severity: SeverityError,
			Message:  fmt.Sprintf("unknown action step keyword %q", t.text),
			Span:     t.span,
		})
		p.advance()
		return StepDecl{}, false
	}
}

func (p *parser) parseFieldInitBlock() ([]FieldInitDecl, bool) {
	if _, ok := p.expect(tokLBrace); !ok {
		return nil, false
	}
	var inits []FieldInitDecl
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
		inits = append(inits, FieldInitDecl{
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

func (p *parser) parseExpr() (Expr, bool) {
	left, ok := p.parsePrimaryExpr()
	if !ok {
		return Expr{}, false
	}
	for {
		k := p.cur().kind
		if k == tokPlus || k == tokMinus || k == tokStar || k == tokSlash ||
			k == tokEq || k == tokNeq || k == tokGte || k == tokLte || k == tokGt || k == tokLt ||
			k == tokAnd || k == tokOr {
			opTok := p.advance()
			right, ok := p.parsePrimaryExpr()
			if !ok {
				return Expr{}, false
			}
			lCopy := left
			rCopy := right
			left = Expr{
				Kind:  ExprBinary,
				Op:    opTok.text,
				Left:  &lCopy,
				Right: &rCopy,
				Span:  opTok.span,
			}
			continue
		}
		break
	}
	return left, true
}

func (p *parser) parsePrimaryExpr() (Expr, bool) {
	t := p.cur()
	switch t.kind {
	case tokString:
		p.advance()
		return Expr{Kind: ExprString, Value: t.text, Span: t.span}, true
	case tokNumber:
		p.advance()
		if strings.Contains(t.text, ".") {
			return Expr{Kind: ExprFloat, Value: t.text, Span: t.span}, true
		}
		return Expr{Kind: ExprInt, Value: t.text, Span: t.span}, true
	case tokDot:
		p.advance()
		idTok, ok := p.expect(tokIdent)
		if !ok {
			return Expr{}, false
		}
		return Expr{Kind: ExprEnumLit, Value: idTok.text, Span: t.span}, true
	case tokIdent:
		p.advance()
		if t.text == "true" || t.text == "false" {
			return Expr{Kind: ExprBool, Value: t.text, Span: t.span}, true
		}
		if p.cur().kind == tokDot {
			p.advance()
			fieldTok, ok := p.expect(tokIdent)
			if !ok {
				return Expr{}, false
			}
			return Expr{
				Kind:   ExprField,
				Target: t.text,
				Field:  fieldTok.text,
				Span:   t.span,
			}, true
		}
		if p.cur().kind == tokLBrace {
			fields, ok := p.parseFieldInitBlock()
			if !ok {
				return Expr{}, false
			}
			return Expr{
				Kind:     ExprConstruct,
				TypeName: t.text,
				Fields:   fields,
				Span:     t.span,
			}, true
		}
		return Expr{Kind: ExprIdent, Value: t.text, Span: t.span}, true
	default:
		p.diags = append(p.diags, Diagnostic{
			Code:     "FDN0106",
			Severity: SeverityError,
			Message:  fmt.Sprintf("expected expression, got %q", t.text),
			Span:     t.span,
		})
		p.advance()
		return Expr{}, false
	}
}

func (p *parser) parseScenario() (ScenarioDecl, bool) {
	kw := p.advance() // scenario
	nameTok, ok := p.expect(tokString)
	if !ok {
		return ScenarioDecl{}, false
	}
	if _, ok := p.expect(tokLBrace); !ok {
		return ScenarioDecl{}, false
	}
	scen := ScenarioDecl{
		Name: nameTok.text,
		Span: kw.span,
	}
	for p.cur().kind != tokRBrace && p.cur().kind != tokEOF {
		clause := p.cur()
		if clause.kind != tokIdent {
			p.diags = append(p.diags, Diagnostic{
				Code:     "FDN0107",
				Severity: SeverityError,
				Message:  fmt.Sprintf("expected scenario clause (given, when, expect), got %q", clause.text),
				Span:     clause.span,
			})
			p.advance()
			return ScenarioDecl{}, false
		}
		switch clause.text {
		case "given":
			p.advance()
			entTok, ok := p.expect(tokIdent)
			if !ok {
				return ScenarioDecl{}, false
			}
			fields, ok := p.parseFieldInitBlock()
			if !ok {
				return ScenarioDecl{}, false
			}
			scen.Givens = append(scen.Givens, GivenSeedDecl{
				EntityName: entTok.text,
				Fields:     fields,
				Span:       clause.span,
			})
		case "when":
			p.advance()
			actTok, ok := p.expect(tokIdent)
			if !ok {
				return ScenarioDecl{}, false
			}
			scen.WhenAction = actTok.text
			if _, ok := p.expect(tokLParen); !ok {
				return ScenarioDecl{}, false
			}
			for p.cur().kind != tokRParen && p.cur().kind != tokEOF {
				argExpr, ok := p.parseExpr()
				if !ok {
					return ScenarioDecl{}, false
				}
				scen.WhenArgs = append(scen.WhenArgs, argExpr)
				if p.cur().kind == tokComma {
					p.advance()
				}
			}
			if _, ok := p.expect(tokRParen); !ok {
				return ScenarioDecl{}, false
			}
		case "expect":
			p.advance()
			if p.cur().kind == tokIdent && p.cur().text == "error" {
				p.advance()
				errTok, ok := p.expect(tokIdent)
				if !ok {
					return ScenarioDecl{}, false
				}
				scen.Expects = append(scen.Expects, ExpectDecl{
					ErrorRaised: errTok.text,
					Span:        clause.span,
				})
				continue
			}
			left, ok := p.parsePrimaryExpr()
			if !ok {
				return ScenarioDecl{}, false
			}
			opTok := p.advance()
			right, ok := p.parsePrimaryExpr()
			if !ok {
				return ScenarioDecl{}, false
			}
			scen.Expects = append(scen.Expects, ExpectDecl{
				Left:  left,
				Op:    opTok.text,
				Right: right,
				Span:  clause.span,
			})
		default:
			p.diags = append(p.diags, Diagnostic{
				Code:     "FDN0107",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown scenario clause %q", clause.text),
				Span:     clause.span,
			})
			p.advance()
			return ScenarioDecl{}, false
		}
	}
	if _, ok := p.expect(tokRBrace); !ok {
		return ScenarioDecl{}, false
	}
	return scen, true
}
