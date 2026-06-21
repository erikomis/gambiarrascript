package interpreter

import (
	"io"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

func eval(t *testing.T, input string) object.Object {
	t.Helper()
	p := parser.New(lexer.New(input))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("erros de parse: %v", errs)
	}
	i := New(io.Discard)
	return i.Eval(prog, object.NewEnvironment())
}

func TestEvalAritmeticaEComparacao(t *testing.T) {
	casos := []struct {
		input string
		esp   string
	}{
		{"mostra 2 + 3 * 4", "14"},
		{"mostra (2 + 3) * 4", "20"},
		{"mostra 10 % 3", "1"},
		{"mostra 5 < 10", "deu_bom"},
		{"mostra 5 == 6", "deu_ruim"},
		{"mostra nao deu_bom", "deu_ruim"},
		{"mostra deu_bom e deu_ruim", "deu_ruim"},
		{"mostra deu_ruim ou deu_bom", "deu_bom"},
		{`mostra "oi " + "tropa"`, "oi tropa"},
		{`mostra "n = " + 5`, "n = 5"},
	}
	for _, c := range casos {
		got := eval(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("input %q => got %q, esperado %q", c.input, got.Inspect(), c.esp)
		}
	}
}

func TestEvalListaEIndex(t *testing.T) {
	got := eval(t, "mostra [10, 20, 30][1]")
	if got.Inspect() != "20" {
		t.Fatalf("index de lista falhou: got %q", got.Inspect())
	}
}

func TestEvalErros(t *testing.T) {
	casos := []string{
		"mostra 1 / 0",     // divisao por zero
		"mostra naoexiste", // variavel indefinida
		"mostra [1, 2][9]", // fora do range
	}
	for _, in := range casos {
		got := eval(t, in)
		if got.Type() != object.ERRO_OBJ {
			t.Errorf("input %q deveria gerar Erro, got %s (%q)", in, got.Type(), got.Inspect())
		}
	}
}
