package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"gambiarrascript/code"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Todo opcode definido tem que ter efeito de pilha conhecido. Quem cria um
// opcode novo e esquece de ensinar o MaxPilha cai aqui — senao toda funcao
// que usasse o opcode iria pro caminho checado da VM sem ninguem perceber.
func TestEfeitoPilhaCobreTodoOpcode(t *testing.T) {
	consts := []object.Object{
		&object.Texto{Value: "01"},
		&object.DescTreta{},
		&object.DescCombinado{},
		&object.DescLiteral{},
	}
	idxConst := map[code.Opcode]int{
		code.OpTreta: 1, code.OpCombinado: 2, code.OpInstancia: 3,
	}
	// definidos mas nunca emitidos (a VM nao executa)
	mortos := map[code.Opcode]bool{code.OpVaza: true, code.OpContinua: true}
	for b := 0; b < 256; b++ {
		op := code.Opcode(b)
		def, err := code.Lookup(byte(b))
		if err != nil || mortos[op] {
			continue
		}
		operandos := make([]int, len(def.OperandWidths))
		if len(operandos) > 0 {
			operandos[0] = idxConst[op]
		}
		ins := code.Make(op, operandos...)
		if _, ok := efeitoPilha(op, ins[1:], consts); !ok {
			t.Errorf("%s sem efeito de pilha em efeitoPilha (compiler/pilha.go)", def.Name)
		}
	}
}

func compilaPilha(t *testing.T, src string) *Bytecode {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse: %v", errs)
	}
	c := New()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	return c.Bytecode()
}

// funcoesSemTeto lista o que ficou com MaxStack 0 (caminho checado): o fluxo
// principal, as gambiarras do pool e o corpo dos modulos importados.
func funcoesSemTeto(bc *Bytecode) []string {
	var sem []string
	if bc.MaxStack <= 0 {
		sem = append(sem, "<main>")
	}
	for _, k := range bc.Constants {
		switch v := k.(type) {
		case *object.CompiledFunction:
			if v.MaxStack <= 0 {
				sem = append(sem, v.Name)
			}
		case *object.Modulo:
			if v.Corpo != nil && v.Corpo.MaxStack <= 0 {
				sem = append(sem, "<modulo "+v.Caminho+">")
			}
		}
	}
	return sem
}

func TestMaxPilhaValoresExatos(t *testing.T) {
	casos := []struct {
		src  string
		want int
	}{
		// a, a, a empilhados antes do primeiro * desempilhar
		{"bota a = 1\nmostra a + a * a", 3},
		// [a, b, c] = 3 elementos; o OpArray devolve 1
		{"bota a = 1\nbota b = [a, a, a]", 3},
		// chamada: callee + 2 args
		{"gambiarra f(x, y)\n    funciona x\nacabou_finalmente\nmostra f(1, 2)", 3},
		// so constante: 1 slot
		{"mostra 1", 1},
	}
	for _, c := range casos {
		if got := compilaPilha(t, c.src).MaxStack; got != c.want {
			t.Errorf("%q: MaxStack %d, queria %d", c.src, got, c.want)
		}
	}
}

// O que a analise nao consegue garantir tem que dar 0 (desconhecido), nunca
// um numero pequeno demais.
func TestMaxPilhaDesconhecido(t *testing.T) {
	cat := func(partes ...[]byte) code.Instructions {
		var out code.Instructions
		for _, p := range partes {
			out = append(out, p...)
		}
		return out
	}
	consts := []object.Object{object.NumInt(1)}
	casos := map[string]code.Instructions{
		// laco que empilha a cada volta: sem teto
		"laco crescendo": cat(code.Make(code.OpConstant, 0), code.Make(code.OpJump, 0)),
		// tira do vazio
		"pilha negativa": cat(code.Make(code.OpPop), code.Make(code.OpHalt)),
		// jump pro meio de uma instrucao
		"jump torto": cat(code.Make(code.OpConstant, 0), code.Make(code.OpJump, 1)),
		// opcode que a VM nao executa
		"opcode morto": cat(code.Make(code.OpVaza), code.Make(code.OpHalt)),
		// byte que nem opcode e
		"lixo": {0xFF},
		// instrucao cortada no meio
		"cortada": code.Make(code.OpConstant, 0)[:2],
	}
	for nome, ins := range casos {
		if got := MaxPilha(ins, consts); got != 0 {
			t.Errorf("%s: MaxStack %d, queria 0 (desconhecido)", nome, got)
		}
	}
	// laco que devolve o que empilha fecha o ponto fixo normalmente
	ok := cat(code.Make(code.OpConstant, 0), code.Make(code.OpPop), code.Make(code.OpJump, 0))
	if got := MaxPilha(ok, consts); got != 1 {
		t.Errorf("laco equilibrado: MaxStack %d, queria 1", got)
	}
}

// Construcoes com fluxo nao trivial (jumps, cadeia de escopo, arruma com
// finalmente, espalha, POO) tem que ganhar teto — nenhuma pode cair no caminho
// checado.
func TestMaxPilhaConstrucoes(t *testing.T) {
	src := `gambiarra soma(...xs)
    bota t = 0
    pra_cada x em xs
        bota t = t + x
    acabou_finalmente
    funciona t
acabou_finalmente
gambiarra arrisca(n)
    arruma
        se_colar n > 2
            bota z = n / 0
        acabou_finalmente
        funciona n ?? 0
    quebrou err
        funciona -1
    finalmente
        mostra "fim"
    acabou_finalmente
acabou_finalmente
gambiarra conta()
    bota c = 0
    gambiarra mais()
        bota c = c + 1
        funciona c
    acabou_finalmente
    funciona mais
acabou_finalmente
treta Ponto
    x = 0
    y
acabou_finalmente
gambiarra (p Ponto) norma()
    funciona p.x * p.x + p.y * p.y
acabou_finalmente
bota d = {"a": 1, "b": 2}
pra_cada k, v em d
    mostra [k, v]
acabou_finalmente
mostra [soma(...[1, 2], 3), arrisca(5), conta()(), Ponto{3, 4}.norma()]
mostra (1 < 2 e 3 > 2) ou nada
escolhe d["a"]
    caso 1, 2
        mostra "um"
    caso 3
        mostra "tres"
acabou_finalmente
`
	bc := compilaPilha(t, src)
	if sem := funcoesSemTeto(bc); len(sem) > 0 {
		t.Fatalf("sem teto: %v", sem)
	}
}

// Os exemplos do repo cobrem a linguagem quase toda: compilados, nenhuma
// funcao pode ficar sem teto.
func TestMaxPilhaExemplos(t *testing.T) {
	arquivos, _ := filepath.Glob("../examples/*.gs")
	if len(arquivos) == 0 {
		t.Skip("sem examples/")
	}
	compilados := 0
	for _, arq := range arquivos {
		fonte, err := os.ReadFile(arq)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(string(fonte)))
		prog := p.ParseProgram()
		if len(p.Errors()) != 0 {
			continue
		}
		abs, _ := filepath.Abs(arq)
		c := New()
		c.Arquivo = abs
		if err := c.Compile(prog); err != nil {
			continue // exemplo que so o tree-walker roda: nao interessa aqui
		}
		compilados++
		if sem := funcoesSemTeto(c.Bytecode()); len(sem) > 0 {
			t.Errorf("%s: sem teto: %v", filepath.Base(arq), sem)
		}
	}
	if compilados < len(arquivos)*9/10 {
		t.Fatalf("so %d de %d exemplos compilaram", compilados, len(arquivos))
	}
}
