package lexer

import (
	"testing"

	"gambiarrascript/token"
)

func TestNextToken(t *testing.T) {
	input := `bota x = 10
mostra "salve"
se_colar x >= 5 e x != 0
    mostra x % 2
acabou_finalmente
# isso aqui e um comentario
[1, 2]`

	esperado := []struct {
		tipo    token.TokenType
		literal string
		linha   int
	}{
		{token.BOTA, "bota", 1},
		{token.IDENT, "x", 1},
		{token.ASSIGN, "=", 1},
		{token.NUMERO, "10", 1},
		{token.MOSTRA, "mostra", 2},
		{token.TEXTO, "salve", 2},
		{token.SE_COLAR, "se_colar", 3},
		{token.IDENT, "x", 3},
		{token.GTE, ">=", 3},
		{token.NUMERO, "5", 3},
		{token.E, "e", 3},
		{token.IDENT, "x", 3},
		{token.NEQ, "!=", 3},
		{token.NUMERO, "0", 3},
		{token.MOSTRA, "mostra", 4},
		{token.IDENT, "x", 4},
		{token.PERCENT, "%", 4},
		{token.NUMERO, "2", 4},
		{token.ACABOU, "acabou_finalmente", 5},
		{token.LBRACKET, "[", 7},
		{token.NUMERO, "1", 7},
		{token.COMMA, ",", 7},
		{token.NUMERO, "2", 7},
		{token.RBRACKET, "]", 7},
		{token.EOF, "", 7},
	}

	l := New(input)
	for i, esp := range esperado {
		tok := l.NextToken()
		if tok.Type != esp.tipo {
			t.Fatalf("[%d] tipo errado: got %q, esperado %q", i, tok.Type, esp.tipo)
		}
		if tok.Literal != esp.literal {
			t.Fatalf("[%d] literal errado: got %q, esperado %q", i, tok.Literal, esp.literal)
		}
		if tok.Line != esp.linha {
			t.Fatalf("[%d] linha errada para %q: got %d, esperado %d", i, tok.Literal, tok.Line, esp.linha)
		}
	}
}
