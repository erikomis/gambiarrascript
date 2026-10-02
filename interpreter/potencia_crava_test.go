package interpreter

import (
	"strings"
	"testing"

	"gambiarrascript/object"
)

func TestEvalPotencia(t *testing.T) {
	casos := []struct{ input, esp string }{
		{"mostra 2 ** 10", "1024\n"},
		{"mostra 2 ** 3 ** 2", "512\n"},
		{"mostra -2 ** 2", "-4\n"},
		{"mostra 2 ** -1", "0.5\n"},
		{"mostra 9 ** 0.5", "3\n"},
		{"bota x = 5\nx **= 2\nmostra x", "25\n"},
	}
	for _, c := range casos {
		if got := rodarComStdin(t, c.input, nil); got != c.esp {
			t.Errorf("%q: got %q, esperado %q", c.input, got, c.esp)
		}
	}
}

func TestBuiltinsMatematicaNovas(t *testing.T) {
	casos := []struct{ input, esp string }{
		{"mostra pi", "3.141592653589793\n"},
		{"mostra seno(0)", "0\n"},
		{"mostra seno(pi / 2)", "1\n"},
		{"mostra cosseno(0)", "1\n"},
		{"mostra tangente(0)", "0\n"},
		{"mostra log(1)", "0\n"},
		{"mostra log10(100)", "2\n"},
		{"mostra exp(0)", "1\n"},
		{"mostra arredonda(log(exp(3)))", "3\n"},
	}
	for _, c := range casos {
		if got := rodarComStdin(t, c.input, nil); got != c.esp {
			t.Errorf("%q: got %q, esperado %q", c.input, got, c.esp)
		}
	}
}

func TestBuiltinsMatematicaErros(t *testing.T) {
	casos := map[string]string{
		"mostra log(0)":    "log() so aceita numero maior que zero",
		"mostra log(-2)":   "log() so aceita numero maior que zero",
		"mostra log10(0)":  "log10() so aceita numero maior que zero",
		`mostra seno("a")`: "seno() espera numero",
		"mostra exp(1, 2)": "exp() quer 1 argumento",
		"mostra 0 ** -2":   "zero elevado a negativo",
		`mostra [1] ** 2`:  "nao da pra fazer LISTA ** NUMERO",
	}
	for src, msg := range casos {
		res := eval(t, src)
		if res.Type() != object.ERRO_OBJ || !strings.Contains(res.Inspect(), msg) {
			t.Errorf("%q: got %q, queria erro com %q", src, res.Inspect(), msg)
		}
	}
}

// TestEvalCrava: crava no tree-walker barra ANTES de rodar (nada impresso).
func TestEvalCrava(t *testing.T) {
	if got := rodarComStdin(t, "crava XS = [1]\nbota XS[0] = 2\nmostra XS", nil); got != "[2]\n" {
		t.Fatalf("crava de lista: got %q", got)
	}
	res := eval(t, "mostra 1\ncrava PI = 3\nbota PI = 4")
	e, ok := res.(*object.Erro)
	if !ok || e.Message != "deu ruim na linha 3: `PI` foi cravada, nao da pra mudar" || e.Line != 3 {
		t.Fatalf("esperava erro de cravada na linha 3, veio %q", res.Inspect())
	}
}
