package compiler

import (
	"testing"

	"gambiarrascript/code"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// O MaxStack do gerador cobre o corpo inteiro (que roda depois do
// OpGerador, na sub-VM), nao so o prologo.
func TestGeradorMaxStackCobreCorpo(t *testing.T) {
	p := parser.New(lexer.New(`gambiarra g(a = 1)
    rende [a, 2, 3, [4, 5, 6]]
acabou_finalmente`))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}
	c := New()
	if err := c.Compile(prog); err != nil {
		t.Fatal(err)
	}
	var fn *object.CompiledFunction
	for _, k := range c.Bytecode().Constants {
		if cf, ok := k.(*object.CompiledFunction); ok {
			fn = cf
		}
	}
	if fn == nil {
		t.Fatal("cade a gambiarra compilada")
	}
	temGerador, temRende := false, false
	for pc := 0; pc < len(fn.Bytecode); {
		op := code.Opcode(fn.Bytecode[pc])
		temGerador = temGerador || op == code.OpGerador
		temRende = temRende || op == code.OpRende
		def, _ := code.Lookup(fn.Bytecode[pc])
		pc++
		for _, w := range def.OperandWidths {
			pc += w
		}
	}
	if !temGerador || !temRende {
		t.Fatalf("esperava OpGerador e OpRende no corpo (gerador=%v rende=%v)", temGerador, temRende)
	}
	if fn.MaxStack < 6 {
		t.Fatalf("MaxStack %d nao cobre o corpo do gerador (precisa de 6)", fn.MaxStack)
	}
}
