package interpreter

import "testing"

// fatia de lista e copia: adiciona na fatia nao sobrescreve a original.
func TestFatiaListaNaoCompartilha(t *testing.T) {
	out := rodar(t, `bota xs = [1, 2, 3]
bota ys = xs[:2]
adiciona(ys, 7)
bota zs = xs[:]
bota zs[0] = 9
mostra xs
mostra ys
mostra zs`)
	esp := "[1, 2, 3]\n[1, 2, 7]\n[9, 2, 3]\n"
	if out != esp {
		t.Fatalf("saida %q, esperado %q", out, esp)
	}
}
