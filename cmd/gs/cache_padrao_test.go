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

// Os descritores de padrao e de cardapio moram no pool de constantes: o .gsc
// tem que gravar e ler eles de volta (gob registrado, chave de dicionario
// dentro do padrao inclusa).
func TestCachePadraoIdaEVolta(t *testing.T) {
	fonte := []byte(`cardapio Cor
    azul
    verde
acabou_finalmente
treta P
    x
    y
acabou_finalmente
gambiarra f(v)
    escolhe v
    caso [Cor.azul, ...r]
        funciona "azul ${r}"
    caso {"k": 1, 2: [a, _]}
        funciona "dic ${a}"
    caso P{x: 0, y}, P{y, _}
        funciona "p ${y}"
    caso _ se deu_bom
        funciona "resto"
    acabou_finalmente
acabou_finalmente
mostra f([Cor.azul, 1, 2])
mostra f({"k": 1, 2: [7, 8], "z": 0})
mostra f(P{0, 5})
mostra f(P{6, 1})
mostra f(Cor.verde)
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
	esperado := "azul [1, 2]\ndic 7\np 5\np 6\nresto\n"
	if out.String() != esperado {
		t.Fatalf("veio %q", out.String())
	}
}
