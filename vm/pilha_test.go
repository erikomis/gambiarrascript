package vm

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

func compilaPilhaVM(t *testing.T, src string) *compiler.Bytecode {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse: %v", errs)
	}
	c := compiler.New()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	return c.Bytecode()
}

// semTeto zera o MaxStack de tudo (fluxo principal, gambiarras, modulos) —
// forca a VM pelo caminho checado, o que ela usa quando o compilador nao
// consegue garantir um teto.
func semTeto(bc *compiler.Bytecode) {
	bc.MaxStack = 0
	for _, k := range bc.Constants {
		switch v := k.(type) {
		case *object.CompiledFunction:
			v.MaxStack = 0
		case *object.Modulo:
			if v.Corpo != nil {
				v.Corpo.MaxStack = 0
			}
		}
	}
}

func rodaBC(t *testing.T, bc *compiler.Bytecode) string {
	t.Helper()
	var out bytes.Buffer
	if err := New(bc, &out).Run(); err != nil {
		if e := ErroDoRun(err); e != nil {
			return out.String() + "ERRO: " + e.Message
		}
		return out.String() + "ERRO: " + err.Error()
	}
	return out.String()
}

// literal com mais elementos que a pilha inicial (StackInicial): o teto do
// frame passa de 512 e a reserva tem que crescer a pilha antes dos pushes
func listaGrande(n int) string {
	partes := make([]string, n)
	for i := range partes {
		partes[i] = fmt.Sprint(i)
	}
	return "[" + strings.Join(partes, ", ") + "]"
}

// O mesmo programa roda igual com o teto calculado (push sem checagem) e sem
// teto nenhum (caminho checado: reserva folgaSemTeto por frame, refeita a
// cada volta de laco).
func TestPilhaComESemTeto(t *testing.T) {
	progs := map[string]string{
		"literal grande no topo": "bota xs = " + listaGrande(1500) + "\nmostra tamanho(xs)",
		"literal grande em gambiarra": "gambiarra f()\n    funciona " + listaGrande(1500) +
			"\nacabou_finalmente\nmostra tamanho(f())",
		"recursao": `gambiarra fib(n)
    se_colar n < 2
        funciona n
    acabou_finalmente
    funciona fib(n - 1) + fib(n - 2)
acabou_finalmente
mostra fib(15)`,
		"laco com arruma e chamada": `gambiarra div(a, b)
    funciona a / b
acabou_finalmente
bota ok = 0
bota ruim = 0
pra_cada i de 1 ate 3000
    arruma
        bota x = div(i, i % 3)
        bota ok = ok + 1
    quebrou err
        bota ruim = ruim + 1
    finalmente
        bota fim = deu_bom
    acabou_finalmente
acabou_finalmente
mostra ok
mostra ruim`,
		"mapeia, espalha e metodo": `treta P
    x
acabou_finalmente
gambiarra (p P) mais(...ns)
    bota t = p.x
    pra_cada n em ns
        bota t = t + n
    acabou_finalmente
    funciona t
acabou_finalmente
bota p = P{1}
mostra mapeia(1..2000, gambiarra(v) funciona p.mais(...[v, v]) acabou_finalmente)[1999]`,
		"bora": `gambiarra soma(n)
    bota t = 0
    pra_cada i de 1 ate n
        bota t = t + i
    acabou_finalmente
    funciona t
acabou_finalmente
bota f = bora soma(1000)
mostra espera(f)`,
	}
	for nome, src := range progs {
		t.Run(nome, func(t *testing.T) {
			com := rodaBC(t, compilaPilhaVM(t, src))
			bc := compilaPilhaVM(t, src)
			semTeto(bc)
			sem := rodaBC(t, bc)
			if com != sem {
				t.Fatalf("com teto %q, sem teto %q", com, sem)
			}
			if strings.Contains(com, "ERRO") {
				t.Fatalf("programa falhou: %q", com)
			}
		})
	}
}
