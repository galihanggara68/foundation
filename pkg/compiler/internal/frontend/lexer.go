package frontend

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

type tokenKind string

const (
	tokEOF      tokenKind = "EOF"
	tokIdent    tokenKind = "IDENT"
	tokString   tokenKind = "STRING"
	tokNumber   tokenKind = "NUMBER"
	tokLBrace   tokenKind = "{"
	tokRBrace   tokenKind = "}"
	tokLBracket tokenKind = "["
	tokRBracket tokenKind = "]"
	tokLParen   tokenKind = "("
	tokRParen   tokenKind = ")"
	tokColon    tokenKind = ":"
	tokComma    tokenKind = ","
	tokDot      tokenKind = "."
	tokAt       tokenKind = "@"
	tokQMark    tokenKind = "?"
	tokAssign   tokenKind = "="
	tokFatArrow tokenKind = "=>"
	tokEq       tokenKind = "=="
	tokNeq      tokenKind = "!="
	tokGte      tokenKind = ">="
	tokLte      tokenKind = "<="
	tokGt       tokenKind = ">"
	tokLt       tokenKind = "<"
	tokArrow    tokenKind = "->"
	tokPlus     tokenKind = "+"
	tokMinus    tokenKind = "-"
	tokStar     tokenKind = "*"
	tokSlash    tokenKind = "/"
	tokAnd      tokenKind = "&&"
	tokOr       tokenKind = "||"
)

type token struct {
	kind tokenKind
	text string
	span ir.Span
}

type lexer struct {
	file  string
	src   []rune
	pos   int
	line  int
	col   int
	diags []ir.Diagnostic
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

func (l *lexer) tokenize() ([]token, []ir.Diagnostic) {
	var tokens []token
	for {
		l.skipWhitespaceAndComments()
		span := ir.Span{File: l.file, Line: l.line, Column: l.col}
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
		case '[':
			l.next()
			tokens = append(tokens, token{kind: tokLBracket, text: "[", span: span})
		case ']':
			l.next()
			tokens = append(tokens, token{kind: tokRBracket, text: "]", span: span})
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
			} else if l.peek() == '>' {
				l.next()
				tokens = append(tokens, token{kind: tokFatArrow, text: "=>", span: span})
			} else {
				tokens = append(tokens, token{kind: tokAssign, text: "=", span: span})
			}
		case '!':
			l.next()
			if l.peek() == '=' {
				l.next()
				tokens = append(tokens, token{kind: tokNeq, text: "!=", span: span})
			} else {
				l.diags = append(l.diags, ir.Diagnostic{
					Code:     "FDN0100",
					Severity: ir.SeverityError,
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
				l.diags = append(l.diags, ir.Diagnostic{
					Code:     "FDN0100",
					Severity: ir.SeverityError,
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
				l.diags = append(l.diags, ir.Diagnostic{
					Code:     "FDN0100",
					Severity: ir.SeverityError,
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
				l.diags = append(l.diags, ir.Diagnostic{
					Code:     "FDN0101",
					Severity: ir.SeverityError,
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
				txt := sb.String()
				if txt == "and" {
					tokens = append(tokens, token{kind: tokAnd, text: "and", span: span})
				} else if txt == "or" {
					tokens = append(tokens, token{kind: tokOr, text: "or", span: span})
				} else {
					tokens = append(tokens, token{kind: tokIdent, text: txt, span: span})
				}
			} else {
				l.next()
				l.diags = append(l.diags, ir.Diagnostic{
					Code:     "FDN0100",
					Severity: ir.SeverityError,
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
