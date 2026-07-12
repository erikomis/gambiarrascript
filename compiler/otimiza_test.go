package compiler

import (
	"testing"

	"gambiarrascript/code"
	"gambiarrascript/object"
)

func TestConstantFoldingInteiros(t *testing.T) {
	roda(t, []casoComp{
		{
			// 2 + 3 dobra pra 5 (uma constante so, sem OpAdd)
			input:      "mostra 2 + 3",
			constantes: []interface{}{5.0},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpMostra),
			},
		},
		{
			// 1 + 2 * 3 dobra pra 7 (recursivo)
			input:      "1 + 2 * 3",
			constantes: []interface{}{7.0},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
		{
			// concat de textos literais dobra
			input:      `"oi" + " tropa"`,
			constantes: []interface{}{"oi tropa"},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
	})
}

func TestConstantFoldingNaoDobraDivisaoNemFloat(t *testing.T) {
	// divisao NAO e dobrada (fica pro runtime, pra dar erro na linha certa):
	// esperamos OpConstant/OpConstant/OpDiv (dois operandos + a op).
	comp := New()
	if err := comp.Compile(parse("1 / 0")); err != nil {
		t.Fatalf("compile: %v", err)
	}
	if n := len(comp.Bytecode().Constants); n != 2 {
		t.Fatalf("1/0 nao devia dobrar; esperava 2 constantes, veio %d", n)
	}
}

func TestInterningDedupePool(t *testing.T) {
	comp := New()
	if err := comp.Compile(parse(`mostra 5
mostra 5
mostra "oi"
mostra "oi"`)); err != nil {
		t.Fatalf("compile: %v", err)
	}
	consts := comp.Bytecode().Constants
	// 5 (int) uma vez + "oi" uma vez = 2, apesar de 4 usos
	if len(consts) != 2 {
		t.Fatalf("esperava 2 constantes interned, veio %d: %v", len(consts), consts)
	}
}

func TestInterningIntEFloatNaoColidem(t *testing.T) {
	comp := New()
	// 5 (int) e 5.0 (float) sao constantes distintas (a VM usa aritimetica
	// diferente), entao NAO devem ser deduplicadas entre si.
	if err := comp.Compile(parse("mostra 5\nmostra 5.5")); err != nil {
		t.Fatalf("compile: %v", err)
	}
	consts := comp.Bytecode().Constants
	if len(consts) != 2 {
		t.Fatalf("esperava 2 constantes (int e float), veio %d", len(consts))
	}
	if a, ok := consts[0].(*object.Numero); !ok || !a.EhInt {
		t.Fatalf("primeira constante devia ser inteiro exato")
	}
}
