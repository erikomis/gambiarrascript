package compiler

import (
	"testing"

	"gambiarrascript/ast"
	"gambiarrascript/code"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

func parse(input string) *ast.Program {
	return parser.New(lexer.New(input)).ParseProgram()
}

func concat(ins []code.Instructions) code.Instructions {
	out := code.Instructions{}
	for _, i := range ins {
		out = append(out, i...)
	}
	return out
}

type casoComp struct {
	input       string
	constantes  []interface{}
	instrucoes  []code.Instructions
}

func roda(t *testing.T, casos []casoComp) {
	t.Helper()
	for _, c := range casos {
		comp := New()
		if err := comp.Compile(parse(c.input)); err != nil {
			t.Fatalf("Compile(%q): %v", c.input, err)
		}
		bc := comp.Bytecode()
		esp := concat(c.instrucoes)
		if bc.Instructions.String() != esp.String() {
			t.Fatalf("input %q instrucoes:\ngot:\n%s\nesperado:\n%s", c.input, bc.Instructions.String(), esp.String())
		}
		if len(bc.Constants) != len(c.constantes) {
			t.Fatalf("input %q: %d constantes, esperado %d", c.input, len(bc.Constants), len(c.constantes))
		}
		for i, cte := range c.constantes {
			switch e := cte.(type) {
			case float64:
				n, ok := bc.Constants[i].(*object.Numero)
				if !ok || n.Value != e {
					t.Fatalf("const %d: got %v, esperado numero %v", i, bc.Constants[i], e)
				}
			case string:
				s, ok := bc.Constants[i].(*object.Texto)
				if !ok || s.Value != e {
					t.Fatalf("const %d: got %v, esperado texto %q", i, bc.Constants[i], e)
				}
			}
		}
	}
}

func TestCompilaMostraEAritmetica(t *testing.T) {
	roda(t, []casoComp{
		{
			input:      "mostra 1 + 2",
			constantes: []interface{}{1.0, 2.0},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpAdd),
				code.Make(code.OpMostra),
			},
		},
		{
			input:      "1 + 2 * 3",
			constantes: []interface{}{1.0, 2.0, 3.0},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpMul),
				code.Make(code.OpAdd),
				code.Make(code.OpPop),
			},
		},
	})
}

func TestCompilaMenorTrocaOperandos(t *testing.T) {
	roda(t, []casoComp{
		{
			input:      "1 < 2",
			constantes: []interface{}{2.0, 1.0}, // 2 e compilado ANTES de 1 (swap)
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGreaterThan),
				code.Make(code.OpPop),
			},
		},
	})
}

func TestCompilaPrefixoELiterais(t *testing.T) {
	roda(t, []casoComp{
		{
			input:      "nao deu_bom",
			constantes: []interface{}{},
			instrucoes: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpNao),
				code.Make(code.OpPop),
			},
		},
		{
			input:      "-5",
			constantes: []interface{}{5.0},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpMinus),
				code.Make(code.OpPop),
			},
		},
		{
			input:      "nada",
			constantes: []interface{}{},
			instrucoes: []code.Instructions{
				code.Make(code.OpNada),
				code.Make(code.OpPop),
			},
		},
	})
}

func TestCompilaNaoSuportadoDaErro(t *testing.T) {
	comp := New()
	if err := comp.Compile(parse("deu_bom e deu_bom")); err == nil {
		t.Fatal("'e' ainda nao e suportado na 6a, deveria dar erro de compilacao")
	}
	comp2 := New()
	if err := comp2.Compile(parse("bota x = 1")); err == nil {
		t.Fatal("'bota' ainda nao e suportado na 6a, deveria dar erro de compilacao")
	}
}
