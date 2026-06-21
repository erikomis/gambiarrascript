package lexer

import (
	"testing"

	"gambiarrascript/token"
)

func TestNextToken(t *testing.T) {
	input := "bota x = 10\nmostra \"salve\"\nse_colar x >= 5 e x != 0\n    mostra x % 2\nacabou_finalmente\n# isso aqui e um comentario\n[1, 2]\n! 3.14\n\"multi\nlinha\" x"

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
		{token.ILLEGAL, "!", 8},
		{token.NUMERO, "3.14", 8},
		{token.TEXTO, "multi\nlinha", 9},
		{token.IDENT, "x", 10},
		{token.EOF, "", 10},
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

func TestColuna(t *testing.T) {
	input := "bota x = 10\nmostra x"
	esperado := []struct {
		tipo   token.TokenType
		linha  int
		coluna int
	}{
		{token.BOTA, 1, 1},
		{token.IDENT, 1, 6},
		{token.ASSIGN, 1, 8},
		{token.NUMERO, 1, 10},
		{token.MOSTRA, 2, 1},
		{token.IDENT, 2, 8},
		{token.EOF, 2, 9},
	}
	l := New(input)
	for i, esp := range esperado {
		tok := l.NextToken()
		if tok.Type != esp.tipo {
			t.Fatalf("[%d] tipo: got %q, esperado %q", i, tok.Type, esp.tipo)
		}
		if tok.Line != esp.linha {
			t.Fatalf("[%d] linha de %q: got %d, esperado %d", i, tok.Literal, tok.Line, esp.linha)
		}
		if tok.Coluna != esp.coluna {
			t.Fatalf("[%d] coluna de %q: got %d, esperado %d", i, tok.Literal, tok.Coluna, esp.coluna)
		}
	}
}
