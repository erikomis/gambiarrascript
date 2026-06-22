package lexer

import "gambiarrascript/token"

type Lexer struct {
	input   string
	pos     int  // posicao do char atual
	readPos int  // proxima posicao a ler
	ch      byte // char atual
	line    int
	col     int
}

func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1, col: 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.ch == '\n' {
		l.line++
		l.col = 0
	}
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	l.col++
}

func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) NextToken() token.Token {
	l.skipWhitespaceAndComments()

	linha := l.line
	coluna := l.col
	var tok token.Token

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.EQ, Literal: "==", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.ASSIGN, l.ch, linha, coluna)
		}
	case '+':
		tok = newToken(token.PLUS, l.ch, linha, coluna)
	case '-':
		tok = newToken(token.MINUS, l.ch, linha, coluna)
	case '*':
		tok = newToken(token.STAR, l.ch, linha, coluna)
	case '/':
		tok = newToken(token.SLASH, l.ch, linha, coluna)
	case '%':
		tok = newToken(token.PERCENT, l.ch, linha, coluna)
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.NEQ, Literal: "!=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.ILLEGAL, l.ch, linha, coluna)
		}
	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.LTE, Literal: "<=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.LT, l.ch, linha, coluna)
		}
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.GTE, Literal: ">=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.GT, l.ch, linha, coluna)
		}
	case ',':
		tok = newToken(token.COMMA, l.ch, linha, coluna)
	case '(':
		tok = newToken(token.LPAREN, l.ch, linha, coluna)
	case ')':
		tok = newToken(token.RPAREN, l.ch, linha, coluna)
	case '[':
		tok = newToken(token.LBRACKET, l.ch, linha, coluna)
	case ']':
		tok = newToken(token.RBRACKET, l.ch, linha, coluna)
	case '{':
		tok = newToken(token.LBRACE, l.ch, linha, coluna)
	case '}':
		tok = newToken(token.RBRACE, l.ch, linha, coluna)
	case ':':
		tok = newToken(token.COLON, l.ch, linha, coluna)
	case '"':
		tok = token.Token{Type: token.TEXTO, Literal: l.readString(), Line: linha, Coluna: coluna}
	case 0:
		tok = token.Token{Type: token.EOF, Literal: "", Line: linha, Coluna: coluna}
	default:
		if isLetter(l.ch) {
			lit := l.readIdentifier()
			return token.Token{Type: token.LookupIdent(lit), Literal: lit, Line: linha, Coluna: coluna}
		} else if isDigit(l.ch) {
			return token.Token{Type: token.NUMERO, Literal: l.readNumber(), Line: linha, Coluna: coluna}
		}
		tok = newToken(token.ILLEGAL, l.ch, linha, coluna)
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		switch {
		case l.ch == ' ' || l.ch == '\t' || l.ch == '\r' || l.ch == '\n':
			l.readChar()
		case l.ch == '#':
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
		default:
			return
		}
	}
}

func (l *Lexer) readIdentifier() string {
	inicio := l.pos
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[inicio:l.pos]
}

func (l *Lexer) readNumber() string {
	inicio := l.pos
	temPonto := false
	for isDigit(l.ch) || (l.ch == '.' && !temPonto) {
		if l.ch == '.' {
			temPonto = true
		}
		l.readChar()
	}
	return l.input[inicio:l.pos]
}

func (l *Lexer) readString() string {
	var sb []byte
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
		sb = append(sb, l.ch)
	}
	return string(sb)
}

func newToken(t token.TokenType, ch byte, linha, coluna int) token.Token {
	return token.Token{Type: t, Literal: string(ch), Line: linha, Coluna: coluna}
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
