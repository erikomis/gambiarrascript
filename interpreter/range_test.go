package interpreter

import (
	"testing"

	"gambiarrascript/object"
)

func TestRangeCrescente(t *testing.T) {
	if out := rodar(t, `mostra 1..5`); out != "[1, 2, 3, 4, 5]\n" {
		t.Fatalf("1..5 => %q", out)
	}
}

func TestRangeUmElemento(t *testing.T) {
	if out := rodar(t, `mostra 3..3`); out != "[3]\n" {
		t.Fatalf("3..3 => %q", out)
	}
}

func TestRangeDecrescente(t *testing.T) {
	if out := rodar(t, `mostra 5..1`); out != "[5, 4, 3, 2, 1]\n" {
		t.Fatalf("5..1 => %q", out)
	}
}

func TestRangePrecedenciaSubtracao(t *testing.T) {
	// `0..n-1` tem que virar `0..(n-1)` (subtracao mais forte que o range).
	out := rodar(t, `bota n = 5
mostra 0..n-1`)
	if out != "[0, 1, 2, 3, 4]\n" {
		t.Fatalf("0..n-1 => %q", out)
	}
}

func TestRangeComTamanho(t *testing.T) {
	if out := rodar(t, `mostra tamanho(1..10)`); out != "10\n" {
		t.Fatalf("tamanho(1..10) => %q", out)
	}
}

func TestRangeNoPraCada(t *testing.T) {
	out := rodar(t, `bota soma = 0
pra_cada x em 1..4
    bota soma = soma + x
acabou_finalmente
mostra soma`)
	if out != "10\n" {
		t.Fatalf("soma 1..4 => %q", out)
	}
}

func TestRangeSoInteiro(t *testing.T) {
	if r := eval(t, `1.5..3`); r.Type() != object.ERRO_OBJ {
		t.Fatalf("range com float devia dar erro, veio %s (%s)", r.Type(), r.Inspect())
	}
}
