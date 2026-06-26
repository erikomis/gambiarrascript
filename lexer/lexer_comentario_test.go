package lexer

import "testing"

func TestComentarioBloco(t *testing.T) {
	input := `/* um comentario
de varias linhas */
mostra "salve"`

	// tipos esperados: keyword MOSTRA, texto "salve", EOF
	esperado := []string{"MOSTRA", "TEXTO", "EOF"}

	l := New(input)
	for i, esp := range esperado {
		tok := l.NextToken()
		if string(tok.Type) != esp {
			t.Fatalf("tok %d: esperava tipo %q, veio %q (literal %q)", i, esp, tok.Type, tok.Literal)
		}
	}
}

func TestComentarioBlocoEntreStatements(t *testing.T) {
	input := "bota x = 1 /* inline */ mostra x"
	l := New(input)
	toks := []string{}
	for {
		tok := l.NextToken()
		if tok.Type == "EOF" {
			break
		}
		toks = append(toks, tok.Literal)
	}
	// deve ter os tokens de "bota x = 1" e "mostra x", sem o comentario
	if toks[0] != "bota" || toks[1] != "x" || toks[2] != "=" || toks[3] != "1" {
		t.Fatalf("tokens iniciais errados: %v", toks)
	}
}