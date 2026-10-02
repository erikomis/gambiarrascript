package parser

import (
	"strings"
	"testing"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
)

// TestParsePotenciaPrecedencia: ** associa pela direita, prende mais que o
// menos da frente e que * / %.
func TestParsePotenciaPrecedencia(t *testing.T) {
	casos := []struct {
		input    string
		esperado string
	}{
		{"mostra 2 ** 3 ** 2", "mostra (2 ** (3 ** 2))"},
		{"mostra -2 ** 2", "mostra (-(2 ** 2))"},
		{"mostra (-2) ** 2", "mostra ((-2) ** 2)"},
		{"mostra 2 ** -1", "mostra (2 ** (-1))"},
		{"mostra 2 * 3 ** 2", "mostra (2 * (3 ** 2))"},
		{"mostra 2 ** 3 * 4", "mostra ((2 ** 3) * 4)"},
		{"mostra xs[0] ** 2", "mostra ((xs[0]) ** 2)"},
		{"mostra f(2) ** 2", "mostra (f(2) ** 2)"},
	}
	for _, c := range casos {
		prog := parse(t, c.input)
		if prog.String() != c.esperado {
			t.Errorf("input %q => got %q, esperado %q", c.input, prog.String(), c.esperado)
		}
	}
}

// TestParsePotenciaComposta: `x **= 2` desugara pra `bota x = x ** 2`.
func TestParsePotenciaComposta(t *testing.T) {
	prog := parse(t, "x **= 2")
	b, ok := prog.Statements[0].(*ast.BotaStatement)
	if !ok {
		t.Fatalf("esperava BotaStatement, veio %T", prog.Statements[0])
	}
	if b.OpComposto != "**=" || b.String() != "bota x = (x ** 2)" {
		t.Fatalf("composto errado: %q (%q)", b.String(), b.OpComposto)
	}
}

func TestParseCrava(t *testing.T) {
	prog := parse(t, "crava PI = 3.14")
	c, ok := prog.Statements[0].(*ast.CravaStatement)
	if !ok {
		t.Fatalf("esperava CravaStatement, veio %T", prog.Statements[0])
	}
	if c.Name.Value != "PI" || c.String() != "crava PI = 3.14" {
		t.Fatalf("crava errado: %q", c.String())
	}
}

// TestParseCravaErros: crava so aceita nome simples com `=`.
func TestParseCravaErros(t *testing.T) {
	casos := map[string]string{
		"crava = 3":       "depois do crava eu esperava um nome",
		"crava xs[0] = 1": "esperava \"=\"",
		"crava X 3":       "esperava \"=\"",
	}
	for src, msg := range casos {
		p := New(lexer.New(src))
		p.ParseProgram()
		errs := strings.Join(p.Errors(), "; ")
		if !strings.Contains(errs, msg) {
			t.Errorf("%q: esperava erro com %q, veio %q", src, msg, errs)
		}
	}
}
