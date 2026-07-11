package interpreter

import (
	"bytes"
	"strings"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// rodarErro roda o input esperando um *Erro em runtime e devolve o Inspect dele.
func rodarErro(t *testing.T, input string) string {
	t.Helper()
	p := parser.New(lexer.New(input))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("erros de parse: %v", errs)
	}
	var buf bytes.Buffer
	i := New(&buf)
	res := i.Eval(prog, object.NewEnvironment())
	if !isError(res) {
		t.Fatalf("esperava erro, veio: %s", res.Inspect())
	}
	return res.Inspect()
}

func TestSomaInteiros(t *testing.T) {
	out := rodar(t, `mostra soma([1, 2, 3, 4])`)
	if out != "10\n" {
		t.Fatalf("soma inteiros: %q", out)
	}
}

func TestSomaComFloatViraFloat(t *testing.T) {
	out := rodar(t, `mostra soma([1, 2, 0.5])`)
	if out != "3.5\n" {
		t.Fatalf("soma float: %q", out)
	}
}

func TestSomaVaziaEhZero(t *testing.T) {
	out := rodar(t, `mostra soma([])`)
	if out != "0\n" {
		t.Fatalf("soma vazia: %q", out)
	}
}

func TestSomaNaoNumeroDaErro(t *testing.T) {
	out := rodarErro(t, `soma([1, "oi", 3])`)
	if !strings.Contains(out, "nao e numero") {
		t.Fatalf("soma erro: %q", out)
	}
}

func TestMedia(t *testing.T) {
	out := rodar(t, `mostra media([1, 2, 3, 4])`)
	if out != "2.5\n" {
		t.Fatalf("media: %q", out)
	}
}

func TestMediaVaziaDaErro(t *testing.T) {
	out := rodarErro(t, `media([])`)
	if !strings.Contains(out, "vazia") {
		t.Fatalf("media vazia: %q", out)
	}
}

func TestZip(t *testing.T) {
	out := rodar(t, `mostra zip([1, 2, 3], ["a", "b", "c"])`)
	if out != "[[1, a], [2, b], [3, c]]\n" {
		t.Fatalf("zip: %q", out)
	}
}

func TestZipTruncaNoMenor(t *testing.T) {
	out := rodar(t, `mostra zip([1, 2, 3], [10, 20])`)
	if out != "[[1, 10], [2, 20]]\n" {
		t.Fatalf("zip trunca: %q", out)
	}
}

func TestZipNaoListaDaErro(t *testing.T) {
	out := rodarErro(t, `zip([1, 2], 5)`)
	if !strings.Contains(out, "lista") {
		t.Fatalf("zip erro: %q", out)
	}
}

func TestEnumera(t *testing.T) {
	out := rodar(t, `mostra enumera(["a", "b", "c"])`)
	if out != "[[0, a], [1, b], [2, c]]\n" {
		t.Fatalf("enumera: %q", out)
	}
}

func TestEnumeraNaoListaDaErro(t *testing.T) {
	out := rodarErro(t, `enumera(5)`)
	if !strings.Contains(out, "lista") {
		t.Fatalf("enumera erro: %q", out)
	}
}
