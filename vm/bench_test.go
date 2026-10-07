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

// fonteMapeia exercita a ponte interpreter->VM: `mapeia` e um builtin do
// interpreter que chama a gambiarra do usuario (CompiledFunction) uma vez por
// elemento, via ChamaCompilada. E o caminho que mais custava por chamada.
const fonteMapeia = `bota xs = 1..5000
gambiarra dobra(x)
    funciona x * 2
acabou_finalmente
mostra mapeia(xs, dobra)[4999]`

func BenchmarkMapeia(b *testing.B) { rodaBench(b, compilaBench(b, fonteMapeia)) }

// fonteLoop e o caso mais comum de "linguagem parece lenta": laco apertado com
// aritmetica e comparacao, sem chamada de funcao.
const fonteLoop = `bota s = 0
bota i = 0
enquanto i < 200000
    bota s = s + i * 2
    bota i = i + 1
acabou_finalmente
mostra s`

func BenchmarkLoop(b *testing.B) { rodaBench(b, compilaBench(b, fonteLoop)) }

// fonteLoopLocal e o mesmo laco dentro de uma gambiarra: tudo vira local (o
// caminho de OpGetLocal/OpSetLocal, que o fonteLoop nao toca).
const fonteLoopLocal = `gambiarra laco()
    bota s = 0
    bota i = 0
    enquanto i < 200000
        bota s = s + i * 2
        bota i = i + 1
    acabou_finalmente
    funciona s
acabou_finalmente
mostra laco()`

func BenchmarkLoopLocal(b *testing.B) { rodaBench(b, compilaBench(b, fonteLoopLocal)) }

// BenchmarkNovaVM isola o custo de CRIAR a VM (pilha, globais, frames, tabela
// de builtins) sem rodar nada. Os benchmarks acima criam uma VM por iteracao —
// como `gs roda` faz — entao o numero deles e setup + execucao. Pra cargas
// pequenas (fib(22) e ~1,5ms) o setup pesava bastante: antes de a VM passar a
// alocar sob demanda ele sozinho custava ~2ms, mais que a propria execucao.
// Compare os dois quando for atribuir um ganho a "execucao" ou a "startup".
func BenchmarkNovaVM(b *testing.B) {
	bc := compilaBench(b, `mostra 1`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New(bc, io.Discard)
	}
}
