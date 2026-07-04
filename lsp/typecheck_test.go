package lsp

import (
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

func diagsDeTypecheck(t *testing.T, codigo string) []Diagnostico {
	p := parser.New(lexer.New(codigo))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse: %v", errs)
	}
	return typecheck(prog)
}

func TestTypecheckVariavelIndefinida(t *testing.T) {
	diags := diagsDeTypecheck(t, `mostra x`)
	if len(diags) == 0 {
		t.Fatalf("esperava warning pra `x` indefinido")
	}
	if !contemMsg(diags, "indefinido") {
		t.Fatalf("msg: %v", diags)
	}
}

func TestTypecheckVariavelDefinidaNaoAvisa(t *testing.T) {
	diags := diagsDeTypecheck(t, `bota x = 10
mostra x`)
	if len(diags) != 0 {
		t.Fatalf("esperava 0 diags, veio %d: %v", len(diags), diags)
	}
}

func TestTypecheckBuiltinNaoAvisa(t *testing.T) {
	diags := diagsDeTypecheck(t, `mostra tamanho([1, 2, 3])`)
	if len(diags) != 0 {
		t.Fatalf("builtin deveria ser reconhecido: %v", diags)
	}
}

func TestTypecheckGambiarraEDentroDeEscopo(t *testing.T) {
	diags := diagsDeTypecheck(t, `
gambiarra dobra(n)
    funciona n * 2
acabou_finalmente
mostra dobra(21)`)
	if len(diags) != 0 {
		t.Fatalf("prog valido: %v", diags)
	}
}

func TestTypecheckCatchBoundErr(t *testing.T) {
	diags := diagsDeTypecheck(t, `
arruma
    mostra 1
quebrou err
    mostra err
acabou_finalmente`)
	if len(diags) != 0 {
		t.Fatalf("catch deve amarrar o nome: %v", diags)
	}
}

func TestTypecheckParamentroDeGambiarra(t *testing.T) {
	diags := diagsDeTypecheck(t, `
gambiarra f(x, y)
    mostra x + y
acabou_finalmente`)
	if len(diags) != 0 {
		t.Fatalf("params sao locais: %v", diags)
	}
}

func contemMsg(ds []Diagnostico, sub string) bool {
	for _, d := range ds {
		if strContains(d.Message, sub) {
			return true
		}
	}
	return false
}

func strContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
