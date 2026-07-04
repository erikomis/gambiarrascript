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

func TestColunaAposStringMultilinha(t *testing.T) {
	// a string ocupa da linha 1 ate a 2; o 'x' vem depois na linha 2
	input := "bota s = \"abre\nfecha\"\nmostra x"
	l := New(input)
	var ultimo token.Token
	for {
		tok := l.NextToken()
		if tok.Type == token.EOF {
			break
		}
		ultimo = tok
	}
	// 'x' e o ultimo token antes do EOF: linha 3, coluna 8 (apos "mostra ")
	if ultimo.Type != token.IDENT || ultimo.Literal != "x" {
		t.Fatalf("ultimo token deveria ser IDENT x, got %q (%q)", ultimo.Type, ultimo.Literal)
	}
	if ultimo.Line != 3 {
		t.Fatalf("linha do x: got %d, esperado 3", ultimo.Line)
	}
	if ultimo.Coluna != 8 {
		t.Fatalf("coluna do x: got %d, esperado 8", ultimo.Coluna)
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

func TestStringEscapes(t *testing.T) {
	input := `"a\"b\\c\nd\te"`
	expected := "a\"b\\c\nd\te"
	l := New(input)
	tok := l.NextToken()
	if tok.Type != token.TEXTO {
		t.Fatalf("tipo errado: got %q, esperado %q", tok.Type, token.TEXTO)
	}
	if tok.Literal != expected {
		t.Fatalf("literal errado: got %q, esperado %q", tok.Literal, expected)
	}
	eof := l.NextToken()
	if eof.Type != token.EOF {
		t.Fatalf("proximo token deveria ser EOF, got %q", eof.Type)
	}
}

func TestRawStringCrase(t *testing.T) {
	// fonte gs: `{"nome": "Erik"}`  (crase, json com aspas, crase)
	input := "\x60{\"nome\": \"Erik\"}\x60"
	l := New(input)
	tok := l.NextToken()
	if tok.Type != token.TEXTO {
		t.Fatalf("tipo: got %q, esperado TEXTO", tok.Type)
	}
	if tok.Literal != `{"nome": "Erik"}` {
		t.Fatalf("literal: got %q", tok.Literal)
	}
	if l.NextToken().Type != token.EOF {
		t.Fatal("esperava EOF apos a string de crase")
	}
}

func TestRawStringNaoProcessaEscape(t *testing.T) {
	// fonte gs: `a\nb`  -> conteudo cru "a\nb" (barra-n literal, 4 chars)
	input := "\x60a\\nb\x60"
	l := New(input)
	tok := l.NextToken()
	if tok.Literal != "a\\nb" {
		t.Fatalf("crase nao deveria processar escape: got %q (esperado %q)", tok.Literal, "a\\nb")
	}
}

func TestRawStringMultilinha(t *testing.T) {
	// fonte gs: `ab\n(real)cd` numa crase, depois quebra real, depois x
	input := "\x60ab\ncd\x60\nx"
	l := New(input)
	str := l.NextToken()
	if str.Literal != "ab\ncd" {
		t.Fatalf("multilinha: got %q", str.Literal)
	}
	id := l.NextToken() // x, na linha 3 (linha1: `ab, linha2: cd`, linha3: x)
	if id.Type != token.IDENT || id.Literal != "x" {
		t.Fatalf("esperava IDENT x, got %q (%q)", id.Type, id.Literal)
	}
	if id.Line != 3 {
		t.Fatalf("linha do x apos raw string multilinha: got %d, esperado 3", id.Line)
	}
}

func TestTokensDicionario(t *testing.T) {
	input := `{"a": 1}`
	esperado := []token.TokenType{
		token.LBRACE, token.TEXTO, token.COLON, token.NUMERO, token.RBRACE, token.EOF,
	}
	l := New(input)
	for idx, tt := range esperado {
		tok := l.NextToken()
		if tok.Type != tt {
			t.Fatalf("[%d] tipo errado: got %q, esperado %q", idx, tok.Type, tt)
		}
	}
}

func TestUnicodeIdentifierETexto(t *testing.T) {
	input := `bota ção = "café résumé 数値"`
	l := New(input)
	esperado := []struct {
		tipo token.TokenType
		lit  string
	}{
		{token.BOTA, "bota"},
		{token.IDENT, "ção"},
		{token.ASSIGN, "="},
		{token.TEXTO, "café résumé 数値"},
		{token.EOF, ""},
	}
	for i, esp := range esperado {
		tok := l.NextToken()
		if tok.Type != esp.tipo {
			t.Fatalf("[%d] tipo: got %q, esperado %q", i, tok.Type, esp.tipo)
		}
		if tok.Literal != esp.lit {
			t.Fatalf("[%d] literal: got %q, esperado %q", i, tok.Literal, esp.lit)
		}
	}
}

func TestUnicodeNumeroETextoLen(t *testing.T) {
	l := New(`mostra tamanho("nação")`)
	tok := l.NextToken() // MOSTRA
	tok = l.NextToken()  // IDENT "tamanho"
	tok = l.NextToken()  // LPAREN
	tok = l.NextToken()  // TEXTO "nação"
	if tok.Type != token.TEXTO {
		t.Fatalf("TEXTO")
	}
	if tok.Literal != "nação" {
		t.Fatalf("lit %q", tok.Literal)
	}
	tok = l.NextToken() // RPAREN
	if tok.Type != token.RPAREN {
		t.Fatalf("RPAREN")
	}
}
