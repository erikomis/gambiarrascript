package vm

import (
	"bytes"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

func rodaVM(t *testing.T, input string) (object.Object, string) {
	t.Helper()
	prog := parser.New(lexer.New(input)).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile %q: %v", input, err)
	}
	var buf bytes.Buffer
	maq := New(comp.Bytecode(), &buf)
	if err := maq.Run(); err != nil {
		t.Fatalf("vm %q: %v", input, err)
	}
	return maq.LastPoppedStackElem(), buf.String()
}

func TestVMAritmetica(t *testing.T) {
	casos := []struct{ input, esp string }{
		{"1 + 2", "3"},
		{"2 * 3 + 4", "10"},
		{"(1 + 2) * 3", "9"},
		{"10 % 3", "1"},
		{"-5 + 8", "3"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("%q => got %s, esperado %s", c.input, got.Inspect(), c.esp)
		}
	}
}

func TestVMComparacao(t *testing.T) {
	casos := []struct{ input, esp string }{
		{"1 < 2", "deu_bom"},
		{"2 < 1", "deu_ruim"},
		{"1 == 1", "deu_bom"},
		{"1 != 2", "deu_bom"},
		{"2 >= 2", "deu_bom"},
		{"nao deu_bom", "deu_ruim"},
		{`"a" == "a"`, "deu_bom"},
		{`"a" == "b"`, "deu_ruim"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("%q => got %s, esperado %s", c.input, got.Inspect(), c.esp)
		}
	}
}

func TestVMConcatenacao(t *testing.T) {
	got, _ := rodaVM(t, `"oi " + "tropa"`)
	if got.Inspect() != "oi tropa" {
		t.Fatalf("got %q", got.Inspect())
	}
}

func TestVMMostra(t *testing.T) {
	_, out := rodaVM(t, "mostra 1 + 2")
	if out != "3\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMDivisaoPorZeroDaErro(t *testing.T) {
	prog := parser.New(lexer.New("1 / 0")).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	maq := New(comp.Bytecode(), &bytes.Buffer{})
	if err := maq.Run(); err == nil {
		t.Fatal("divisao por zero deveria dar erro na VM")
	}
}
