package parser

import (
	"testing"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
)

func parse(t *testing.T, input string) *ast.Program {
	t.Helper()
	p := New(lexer.New(input))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parser teve erros: %v", errs)
	}
	return prog
}

func TestParseExpressionsPrecedence(t *testing.T) {
	casos := []struct {
		input    string
		esperado string
	}{
		{"mostra 1 + 2 * 3", "mostra (1 + (2 * 3))"},
		{"mostra (1 + 2) * 3", "mostra ((1 + 2) * 3)"},
		{"mostra nao deu_bom", "mostra (naodeu_bom)"},
		{"mostra -5 + 3", "mostra ((-5) + 3)"},
		{"mostra a e b ou c", "mostra ((a e b) ou c)"},
		{"mostra soma(1, 2 * 3)", "mostra soma(1, (2 * 3))"},
		{"mostra lista[1 + 1]", "mostra (lista[(1 + 1)])"},
		{"mostra [1, 2, 3]", "mostra [1, 2, 3]"},
	}
	for _, c := range casos {
		prog := parse(t, c.input)
		if prog.String() != c.esperado {
			t.Errorf("input %q => got %q, esperado %q", c.input, prog.String(), c.esperado)
		}
	}
}
