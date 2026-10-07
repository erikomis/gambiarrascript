package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
	"gambiarrascript/vm"
)

// Os descritores de POO (treta/combinado/literal) moram no pool de
// constantes: o .gsc tem que gravar e ler eles de volta (gob registrado).
func TestCachePOOIdaEVolta(t *testing.T) {
	fonte := []byte(`treta Animal
    nome = "rex"
    xs = []
acabou_finalmente
treta Cachorro
    Animal
    raca
acabou_finalmente
combinado Falante
    fala()
acabou_finalmente
gambiarra (c Cachorro) fala()
    funciona c.nome + " late"
acabou_finalmente
bota c = Cachorro{raca: "vira"}
mostra c.fala()
mostra satisfaz(c, Falante)
mostra Cachorro{Animal{"bidu", [1]}, "poodle"}
`)
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatal(err)
	}
	gsc := filepath.Join(t.TempDir(), "x.gsc")
	gravaCache(gsc, fonte, comp.Bytecode())
	bc := carregaCache(gsc, fonte)
	if bc == nil {
		t.Fatal("o cache nao voltou (gob sem os descritores registrados?)")
	}
	var out bytes.Buffer
	if err := vm.New(bc, &out).Run(); err != nil {
		t.Fatal(err)
	}
	esperado := "rex late\ndeu_bom\nCachorro{Animal: Animal{nome: \"bidu\", xs: [1]}, raca: \"poodle\"}\n"
	if out.String() != esperado {
		t.Fatalf("veio %q", out.String())
	}
}
