package interpreter

import (
	"testing"

	"gambiarrascript/object"
)

func TestConjuntoDedup(t *testing.T) {
	c := eval(t, `conjunto([1, 2, 2, 3, 1])`)
	cs, ok := c.(*object.Conjunto)
	if !ok {
		t.Fatalf("esperava conjunto, got %s", c.Type())
	}
	if cs.Tamanho() != 3 {
		t.Fatalf("tamanho %d, esperado 3", cs.Tamanho())
	}
}

func TestConjuntoOperacoes(t *testing.T) {
	if eval(t, `contem_conjunto(conjunto([1, 2, 3]), 2)`).Type() != object.BOOLEANO_OBJ {
		t.Fatal("contem_conjunto deve devolver booleano")
	}
	if eval(t, `contem_conjunto(conjunto([1, 2, 3]), 9)`).Inspect() != "deu_ruim" {
		t.Fatal("contem_conjunto 9 espera false")
	}
	u := eval(t, `uniao(conjunto([1, 2]), conjunto([2, 3]))`)
	if u.(*object.Conjunto).Tamanho() != 3 {
		t.Fatalf("uniao deve ter 3 items, veio %d", u.(*object.Conjunto).Tamanho())
	}
	i := eval(t, `intersecao(conjunto([1, 2, 3]), conjunto([2, 3, 4]))`)
	if i.(*object.Conjunto).Tamanho() != 2 {
		t.Fatalf("intersecao deve ter 2 items")
	}
	d := eval(t, `diferenca(conjunto([1, 2, 3]), conjunto([2]))`)
	if d.(*object.Conjunto).Tamanho() != 2 {
		t.Fatalf("diferenca deve ter 2 items")
	}
}

func TestConjuntoTextoKeys(t *testing.T) {
	c := eval(t, `conjunto("abcab")`)
	cs := c.(*object.Conjunto)
	if cs.Tamanho() != 3 {
		t.Fatalf("dedup de chars esperava 3, veio %d", cs.Tamanho())
	}
}

func TestUnicosPreservaOrdem(t *testing.T) {
	r := eval(t, `unicos([1, 2, 1, 3, 2, 4])`)
	l, ok := r.(*object.Lista)
	if !ok {
		t.Fatalf("esperava lista")
	}
	esp := []int64{1, 2, 3, 4}
	if l.Tamanho() != 4 {
		t.Fatalf("tamanho %d, esperado 4", l.Tamanho())
	}
	for i, e := range l.Visao() {
		n, _ := e.(*object.Numero)
		if n.Int != esp[i] {
			t.Fatalf("idx %d: %d esperado %d", i, n.Int, esp[i])
		}
	}
}

func TestAchatada(t *testing.T) {
	r := eval(t, `achatada([[1, 2], [3], [4, 5]])`)
	l, ok := r.(*object.Lista)
	if !ok || l.Tamanho() != 5 {
		t.Fatalf("achatada deveria devolver 5 elementos, got %v", r)
	}
}
