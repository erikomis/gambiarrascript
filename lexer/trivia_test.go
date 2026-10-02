package lexer

import (
	"testing"

	"gambiarrascript/token"
)

func todosTokens(l *Lexer) []token.Token {
	var out []token.Token
	for {
		tok := l.NextToken()
		out = append(out, tok)
		if tok.Type == token.EOF {
			return out
		}
	}
}

func TestTriviaNaoMudaOsTokens(t *testing.T) {
	src := "# topo\nbota x = 1 /* a */ + 2 # b\n\n\nmostra \"#nao ${x}\" /* c\n d */ # e\n"
	normal := todosTokens(New(src))
	l := NewComTrivia(src)
	comTrivia := todosTokens(l)
	if len(normal) != len(comTrivia) {
		t.Fatalf("trivia mudou a quantidade de tokens: %d vs %d", len(normal), len(comTrivia))
	}
	for i := range normal {
		if normal[i] != comTrivia[i] {
			t.Fatalf("tok %d diferente: %+v vs %+v", i, normal[i], comTrivia[i])
		}
	}
	if New(src).Trivia() != nil {
		t.Fatal("lexer normal nao devia coletar trivia")
	}

	tr := l.Trivia()
	textos := []string{"# topo", "/* a */", "# b", "/* c\n d */", "# e"}
	if len(tr.Comentarios) != len(textos) {
		t.Fatalf("esperava %d comentarios, veio %+v", len(textos), tr.Comentarios)
	}
	for i, c := range tr.Comentarios {
		if c.Texto != textos[i] {
			t.Errorf("comentario %d: %q, esperado %q", i, c.Texto, textos[i])
		}
	}
	c := tr.Comentarios
	if !c[0].Sozinho || c[1].Sozinho || c[2].Sozinho || c[3].Sozinho || c[4].Sozinho {
		t.Errorf("Sozinho errado: %+v", c)
	}
	// /* a */ vem depois do `1` (linha 2, coluna 10)
	if c[1].Linha != 2 || c[1].AntesLinha != 2 || c[1].AntesColuna != 10 {
		t.Errorf("posicao do /* a */ errada: %+v", c[1])
	}
	// `mostra` tem linha em branco antes
	if !tr.BrancoAntes[[2]int{5, 1}] {
		t.Errorf("esperava linha em branco antes do mostra: %v", tr.BrancoAntes)
	}
	if tr.BrancoAntes[[2]int{2, 1}] {
		t.Errorf("bota nao tem linha em branco antes: %v", tr.BrancoAntes)
	}
}

func TestTriviaComentarioDepoisDeComentarioSozinho(t *testing.T) {
	l := NewComTrivia("bota x = 1\n/* a */ /* b */\n\n# c")
	todosTokens(l)
	c := l.Trivia().Comentarios
	if len(c) != 3 || !c[0].Sozinho || !c[1].Sozinho || !c[2].Sozinho || !c[2].BrancoAntes || c[1].BrancoAntes {
		t.Fatalf("comentarios: %+v", c)
	}
}
