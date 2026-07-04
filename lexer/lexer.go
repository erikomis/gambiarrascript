package lexer

import (
	"unicode"
	"unicode/utf8"

	"gambiarrascript/token"
)

type Lexer struct {
	input   string
	pos     int  // posicao em bytes do inicio do char atual
	readPos int  // proxima posicao a ler (bytes)
	ch      rune // char atual (rune). 0 = EOF.
	w       int  // largura em bytes do char atual
	line    int
	col     int // coluna em RUNES (nao bytes) — ser humano-amigavel
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
		l.w = 0
		l.pos = l.readPos
		l.col++
		return
	}
	r, w := utf8.DecodeRuneInString(l.input[l.readPos:])
	l.pos = l.readPos
	l.ch = r
	l.w = w
	l.readPos += w
	l.col++
}

func (l *Lexer) peekChar() rune {
	if l.readPos >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.readPos:])
	return r
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
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.PLUSASSIGN, Literal: "+=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.PLUS, l.ch, linha, coluna)
		}
	case '-':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.MINUSASSIGN, Literal: "-=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.MINUS, l.ch, linha, coluna)
		}
	case '*':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.STARASSIGN, Literal: "*=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.STAR, l.ch, linha, coluna)
		}
	case '/':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.SLASHASSIGN, Literal: "/=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.SLASH, l.ch, linha, coluna)
		}
	case '%':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.PERCENTASSIGN, Literal: "%=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.PERCENT, l.ch, linha, coluna)
		}
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
		} else if l.peekChar() == '<' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				tok = token.Token{Type: token.LSHIFTASSIGN, Literal: "<<=", Line: linha, Coluna: coluna}
			} else {
				tok = token.Token{Type: token.LSHIFT, Literal: "<<", Line: linha, Coluna: coluna}
			}
		} else {
			tok = newToken(token.LT, l.ch, linha, coluna)
		}
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.GTE, Literal: ">=", Line: linha, Coluna: coluna}
		} else if l.peekChar() == '>' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				tok = token.Token{Type: token.RSHIFTASSIGN, Literal: ">>=", Line: linha, Coluna: coluna}
			} else {
				tok = token.Token{Type: token.RSHIFT, Literal: ">>", Line: linha, Coluna: coluna}
			}
		} else {
			tok = newToken(token.GT, l.ch, linha, coluna)
		}
	case '&':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.BANDASSIGN, Literal: "&=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.BAND, l.ch, linha, coluna)
		}
	case '|':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.BORASSIGN, Literal: "|=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.BOR, l.ch, linha, coluna)
		}
	case '^':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.BXORASSIGN, Literal: "^=", Line: linha, Coluna: coluna}
		} else {
			tok = newToken(token.BXOR, l.ch, linha, coluna)
		}
	case '~':
		tok = newToken(token.BNOT, l.ch, linha, coluna)
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
	case '.':
		if l.peekChar() == '.' {
			l.readChar()
			tok = token.Token{Type: token.RANGE, Literal: "..", Line: linha, Coluna: coluna}
		} else {
			// acesso por ponto: obj.campo (acucar pra obj["campo"])
			tok = newToken(token.DOT, l.ch, linha, coluna)
		}
	case '"':
		tok = token.Token{Type: token.TEXTO, Literal: l.readString(), Line: linha, Coluna: coluna}
	case '`':
		tok = token.Token{Type: token.TEXTO, Literal: l.readRawString(), Line: linha, Coluna: coluna}
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
		case l.ch == '/' && l.peekChar() == '*':
			l.readChar() // consome '/'
			l.readChar() // consome '*'
			for !(l.ch == '*' && l.peekChar() == '/') && l.ch != 0 {
				l.readChar()
			}
			if l.ch == '*' {
				l.readChar() // consome '*'
				l.readChar() // consome '/'
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
	// prefixos 0x 0o 0b (case insensitive) — hex/oct/bin literal int.
	if l.ch == '0' && (l.peekChar() == 'x' || l.peekChar() == 'X') {
		l.readChar() // consome 0
		l.readChar() // consome x
		for isHexDigit(l.ch) {
			l.readChar()
		}
		return l.input[inicio:l.pos]
	}
	if l.ch == '0' && (l.peekChar() == 'o' || l.peekChar() == 'O') {
		l.readChar()
		l.readChar()
		for isOctDigit(l.ch) {
			l.readChar()
		}
		return l.input[inicio:l.pos]
	}
	if l.ch == '0' && (l.peekChar() == 'b' || l.peekChar() == 'B') {
		l.readChar()
		l.readChar()
		for isBinDigit(l.ch) {
			l.readChar()
		}
		return l.input[inicio:l.pos]
	}
	// range `1..10`: o parser precisa do token RANGE separado. Se vir `..`
	// apos digitos (nao casa decimal), sai sem consumir o primeiro `.`.
	for isDigit(l.ch) || (l.ch == '.' && !temPonto && l.peekChar() != '.') {
		if l.ch == '.' {
			temPonto = true
		}
		l.readChar()
	}
	return l.input[inicio:l.pos]
}

func isHexDigit(ch rune) bool {
	return isDigit(ch) || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F')
}
func isOctDigit(ch rune) bool { return '0' <= ch && ch <= '7' }
func isBinDigit(ch rune) bool { return ch == '0' || ch == '1' }

func (l *Lexer) readString() string {
	var sb []byte
	for {
		l.readChar()
		if l.ch == 0 {
			break
		}
		if l.ch == '"' {
			break
		}
		// interpolation: `${ expr }` — copia balanceado, incluindo `${...}`.
		// Importante: `"s dentro da expressao sao mantidos (nao fecham a string).
		if l.ch == '$' && l.peekChar() == '{' {
			sb = append(sb, '$', '{')
			l.readChar() // consome '$'
			l.readChar() // consome '{'
			depth := 1
			for depth > 0 && l.ch != 0 {
				switch l.ch {
				case '{':
					depth++
				case '}':
					depth--
					if depth == 0 {
						sb = append(sb, '}')
						// proximo char pode ser '"' que fecha string OU algo else
						// sai do loop interno; outer loop decide.
						goto interpFim
					}
				}
				sb = appendUtf8(sb, l.ch)
				l.readChar()
			}
		interpFim:
			continue
		}
		if l.ch == '\\' {
			switch l.peekChar() {
			case '"':
				l.readChar()
				sb = append(sb, '"')
			case '\\':
				l.readChar()
				sb = append(sb, '\\')
			case 'n':
				l.readChar()
				sb = append(sb, '\n')
			case 't':
				l.readChar()
				sb = append(sb, '\t')
			default:
				sb = append(sb, '\\')
			}
			continue
		}
		sb = appendUtf8(sb, l.ch)
	}
	return string(sb)
}

// readRawString le uma string crua entre crases: nada e escapado e quebras de
// linha reais fazem parte do valor (padrao raw string do Go). Para no ` ou no EOF.
func (l *Lexer) readRawString() string {
	var sb []byte
	for {
		l.readChar()
		if l.ch == '`' || l.ch == 0 {
			break
		}
		sb = appendUtf8(sb, l.ch)
	}
	return string(sb)
}

// appendUtf8 adiciona a rune r em codificacao UTF-8 no []byte. Evita alloc
// extra do string(r) -> []byte(r).
func appendUtf8(sb []byte, r rune) []byte {
	var buf [4]byte
	n := utf8.EncodeRune(buf[:], r)
	return append(sb, buf[:n]...)
}

func newToken(t token.TokenType, ch rune, linha, coluna int) token.Token {
	return token.Token{Type: t, Literal: string(ch), Line: linha, Coluna: coluna}
}

// isLetter aceita ASCII a-zA-Z e underscore, e qualquer rune Unicode marcada
// como letra (inclui acentos, cyrillic, kanji, etc).
func isLetter(ch rune) bool {
	if ch == '_' || ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') {
		return true
	}
	return unicode.IsLetter(ch)
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}
