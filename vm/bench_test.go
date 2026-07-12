package vm

import (
	"io"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

// Suite fixa de benchmarks de regressao: fib (recursao/chamadas), sort
// (builtin + lista grande) e json (de_json/pra_json). Rode com:
//
//	go test -bench=. -benchmem ./vm/
//
// Compara commits pra pegar regressao de performance na VM.

// compilaBench compila a fonte uma vez (fora do loop de medicao) e devolve o
// bytecode. Falha o bench se nao compilar.
func compilaBench(b *testing.B, src string) *compiler.Bytecode {
	b.Helper()
	prog := parser.New(lexer.New(src)).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		b.Fatalf("compile: %v", err)
	}
	return comp.Bytecode()
}

// rodaBench roda o bytecode na VM b.N vezes (VM nova a cada iteracao pra medir
// execucao limpa, sem estado acumulado).
func rodaBench(b *testing.B, bc *compiler.Bytecode) {
	b.Helper()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		maq := New(bc, io.Discard)
		if err := maq.Run(); err != nil {
			b.Fatalf("run: %v", err)
		}
	}
}

const fonteFib = `gambiarra fib(n)
    se_colar n < 2
        funciona n
    acabou_finalmente
    funciona fib(n - 1) + fib(n - 2)
acabou_finalmente
mostra fib(22)`

const fonteSort = `bota xs = []
bota i = 0
enquanto i < 500
    adiciona(xs, (i * 7919) % 500)
    bota i = i + 1
acabou_finalmente
ordena(xs)
mostra xs[0]`

const fonteJson = `bota obj = {"nome": "tropa", "tags": ["go", "gs", "vm"], "n": 42, "ok": deu_bom}
bota i = 0
enquanto i < 200
    bota txt = pra_json(obj)
    bota volta = de_json(txt)
    bota i = i + 1
acabou_finalmente
mostra "ok"`

func BenchmarkFib(b *testing.B)  { rodaBench(b, compilaBench(b, fonteFib)) }
func BenchmarkSort(b *testing.B) { rodaBench(b, compilaBench(b, fonteSort)) }
func BenchmarkJson(b *testing.B) { rodaBench(b, compilaBench(b, fonteJson)) }
