package main

import (
	"path/filepath"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// O .gsc tem que guardar o MaxStack do fluxo principal e das gambiarras: sem
// ele o bytecode do cache rodaria todo pelo caminho checado da VM.
func TestCacheGuardaMaxStack(t *testing.T) {
	fonte := []byte(`gambiarra f(a, b)
    funciona [a, b, a + b]
acabou_finalmente
bota x = 1
mostra f(x, x + x * x)
`)
	comp := compiler.New()
	if err := comp.Compile(parser.New(lexer.New(string(fonte))).ParseProgram()); err != nil {
		t.Fatal(err)
	}
	orig := comp.Bytecode()
	gsc := filepath.Join(t.TempDir(), "x.gsc")
	gravaCache(gsc, fonte, orig)
	bc := carregaCache(gsc, fonte)
	if bc == nil {
		t.Fatal("o cache nao voltou")
	}
	if bc.MaxStack <= 0 || bc.MaxStack != orig.MaxStack {
		t.Fatalf("MaxStack do principal: gravado %d, lido %d", orig.MaxStack, bc.MaxStack)
	}
	achou := false
	for _, k := range bc.Constants {
		if cf, ok := k.(*object.CompiledFunction); ok {
			achou = true
			if cf.MaxStack <= 0 {
				t.Fatalf("gambiarra %s voltou do cache sem MaxStack", cf.Name)
			}
		}
	}
	if !achou {
		t.Fatal("cade a gambiarra no pool?")
	}
}
