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
		// O op.OpHalt é emitido no fim de cada Program — necessario pra VM
		// parar. Sera desconsiderado nas comparacos de teste.
		ins := bc.Instructions
		if len(ins) > 0 && ins[len(ins)-1] == byte(code.OpHalt) {
			ins = ins[:len(ins)-1]
		}
		esp := concat(c.instrucoes)
		if ins.String() != esp.String() {
			t.Fatalf("input %q instrucoes:\ngot:\n%s\nesperado:\n%s", c.input, ins.String(), esp.String())
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

func TestCompilaMenor(t *testing.T) {
	roda(t, []casoComp{
		{
			input:      "1 < 2",
			constantes: []interface{}{1.0, 2.0},
			instrucoes: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpMenor),
				code.Make(code.OpPop),
			},
		},
	})
}

func TestCompilaEComShortCircuit(t *testing.T) {
	// Novo layout do `e` na VM (cada jump condicional ja popa):
	//   0000 OpTrue                 ; left
	//   0001 OpJumpIfFalse 12      ; se left false -> 12 (OpFalse)
	//   0004 OpTrue                 ; right
	//   0005 OpJumpIfFalse 12       ; se right false -> 12
	//   0008 OpTrue                 ; ambos truthy
	//   0009 OpJump 13              ; pula o OpFalse
	//   0012 OpFalse                ; label falso
	//   0013 OpPop                  ; ExpressionStatement
	roda(t, []casoComp{
		{
			input:      "deu_bom e deu_bom",
			constantes: []interface{}{},
			instrucoes: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpJumpIfFalse, 12),
				code.Make(code.OpTrue),
				code.Make(code.OpJumpIfFalse, 12),
				code.Make(code.OpTrue),
				code.Make(code.OpJump, 13),
				code.Make(code.OpFalse),
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
	// Fase 6d concluida: gambiarra/bota/etc compilam. Hoje so `importa`
	// continua nao suportado pela VM.
	comp := New()
	if err := comp.Compile(parse("importa \"nao_existe.gs\"")); err == nil {
		t.Fatal("'importa' nao e suportado na VM, deveria dar erro de compilacao")
	}
	comp2 := New()
	if err := comp2.Compile(parse("bota x = 1")); err != nil {
		t.Fatalf("'bota' deveria compilar agora: %v", err)
	}
	comp3 := New()
	if err := comp3.Compile(parse("gambiarra f()\n    funciona 1\nacabou_finalmente")); err != nil {
		t.Fatalf("'gambiarra' deveria compilar agora (fase 6d): %v", err)
	}
}
