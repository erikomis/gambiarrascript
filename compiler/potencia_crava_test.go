package compiler

import (
	"math"
	"strings"
	"testing"

	"gambiarrascript/code"
)

func TestCompilaPotencia(t *testing.T) {
	roda(t, []casoComp{
		{
			// inteiro ** inteiro dobra em compile time (mesma conta do runtime)
			input:      "2 ** 10",
			constantes: []interface{}{1024.0},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
		{
			// resultado float nao dobra: fica pro runtime
			input:      "bota x = 2\n2 ** x",
			constantes: []interface{}{2.0, "x"},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpGetGlobalChk, 0, 1),
				code.Make(code.OpPow),
				code.Make(code.OpPop),
			},
		},
		{
			// float nao dobra: vira OpBinConst
			input:      "2 ** 0.5",
			constantes: []interface{}{2.0, 0.5},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpBinConst, 1, int(code.OpPow)),
				code.Make(code.OpPop),
			},
		},
		{
			// direita literal: OpBinConst com OpPow
			input:      "bota x = 3\nx ** 2",
			constantes: []interface{}{3.0, "x", 2.0},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobalChk, 0, 1),
				code.Make(code.OpBinConst, 2, int(code.OpPow)),
				code.Make(code.OpPop),
			},
		},
		{
			// pi nao tem slot: vira constante direto
			input:      "pi",
			constantes: []interface{}{math.Pi},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
			},
		},
	})
}

// TestCompilaCravaErro: mexer em cravada e erro de compilacao, com a linha.
func TestCompilaCravaErro(t *testing.T) {
	casos := map[string]string{
		"crava A = 1\nbota A = 2":  "linha 2: `A` foi cravada, nao da pra mudar",
		"crava A = 1\nA += 2":      "linha 2: `A` foi cravada, nao da pra mudar",
		"crava A = 1\ncrava A = 2": "linha 2: `A` ja foi cravada, nao da pra cravar de novo",
	}
	for src, esp := range casos {
		err := New().Compile(parse(src))
		if err == nil || !strings.Contains(err.Error(), esp) {
			t.Errorf("%q: erro %v, queria %q", src, err, esp)
		}
	}
	// crava normal compila igual a um bota
	if err := New().Compile(parse("crava A = 1\nbota XS = [A]\nbota XS[0] = 2")); err != nil {
		t.Fatalf("crava valido nao compilou: %v", err)
	}
}
