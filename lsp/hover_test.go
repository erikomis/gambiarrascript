package lsp

import "testing"

func TestHoverBuiltin(t *testing.T) {
	doc := "bota x = tamanho([1, 2])"
	got := hoverConteudo(doc, 0, 14) // cursor sobre "tamanho"
	if got == "" {
		t.Fatal("hover sobre 'tamanho' deveria devolver documentacao")
	}
	if !contiene(got, "tamanho") {
		t.Fatalf("hover deveria mencionar tamanho, got %q", got)
	}
}

func TestHoverKeyword(t *testing.T) {
	doc := "se_colar x > 0"
	got := hoverConteudo(doc, 0, 4) // cursor sobre "se_colar"
	if got == "" {
		t.Fatal("hover sobre 'se_colar' deveria devolver documentacao")
	}
}

func TestHoverMelhorNome(t *testing.T) {
	// palavra que nao e builtin nem keyword -> sem hover
	got := hoverConteudo("bota xyz = 1", 0, 6)
	if got != "" {
		t.Fatalf("hover sobre 'xyz' nao deveria ter docs, got %q", got)
	}
}

func contiene(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
