package object

import "testing"

func TestFormatNumero(t *testing.T) {
	casos := map[float64]string{
		10:  "10",
		-3:  "-3",
		2.5: "2.5",
		0:   "0",
	}
	for in, esp := range casos {
		if got := FormatNumero(in); got != esp {
			t.Errorf("FormatNumero(%v) = %q, esperado %q", in, got, esp)
		}
	}
}

func TestEnvironmentEncadeado(t *testing.T) {
	fora := NewEnvironment()
	fora.Set("x", &Numero{Value: 1})
	dentro := NewEnclosedEnvironment(fora)
	dentro.Set("y", &Numero{Value: 2})

	if _, ok := dentro.Get("x"); !ok {
		t.Fatal("escopo interno deveria enxergar x do externo")
	}
	if _, ok := fora.Get("y"); ok {
		t.Fatal("escopo externo NAO deveria enxergar y do interno")
	}
}

func TestInspectBooleano(t *testing.T) {
	if (&Booleano{Value: true}).Inspect() != "deu_bom" {
		t.Fatal("true deveria inspecionar como deu_bom")
	}
	if (&Booleano{Value: false}).Inspect() != "deu_ruim" {
		t.Fatal("false deveria inspecionar como deu_ruim")
	}
}

func TestChaveHash(t *testing.T) {
	if (&Texto{Value: "a"}).ChaveHash() != (&Texto{Value: "a"}).ChaveHash() {
		t.Fatal("textos iguais deveriam ter a mesma ChaveHash")
	}
	if (&Texto{Value: "a"}).ChaveHash() == (&Texto{Value: "b"}).ChaveHash() {
		t.Fatal("textos diferentes nao deveriam colidir")
	}
	if (&Numero{Value: 1}).ChaveHash() == (&Texto{Value: "1"}).ChaveHash() {
		t.Fatal("numero 1 e texto \"1\" nao deveriam ter a mesma chave (tipos diferentes)")
	}
}

func TestDicionarioInspect(t *testing.T) {
	d := &Dicionario{Pares: map[HashKey]ParDic{}}
	chave := &Texto{Value: "nome"}
	d.Pares[chave.ChaveHash()] = ParDic{Chave: chave, Valor: &Texto{Value: "Erik"}}
	if d.Inspect() != `{"nome": "Erik"}` {
		t.Fatalf("Inspect errado: got %q", d.Inspect())
	}
}
