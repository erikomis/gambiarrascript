package vm

import (
	"bytes"
	"strings"
	"testing"

	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// A arena de Numeros (object.ArenaNum) NAO tem lock: cada VM tem a sua porque
// uma VM roda numa goroutine so. Este teste bate exatamente nos dois caminhos
// que criam VM nova — `bora` (clone) e a ponte do mapeia (sub-VM do pool) —
// ao mesmo tempo, com numeros grandes o bastante pra sair do cache de inteiros
// e forcar a arena. Roda com -race pra pegar compartilhamento indevido.
func TestArenaNaoVazaEntreGoroutines(t *testing.T) {
	src := `gambiarra dobra(x)
    funciona x * 2
acabou_finalmente
gambiarra trabalha(base)
    bota xs = base..(base + 2000)
    bota ys = mapeia(xs, dobra)
    funciona ys[1999]
acabou_finalmente
bota fs = []
bota k = 0
enquanto k < 8
    adiciona(fs, bora trabalha(k * 100000))
    bota k = k + 1
acabou_finalmente
bota total = 0
pra_cada f em fs
    bota total = total + espera(f)
acabou_finalmente
mostra total`

	// referencia: o mesmo programa no tree-walker, que nao tem arena.
	querido := strings.TrimSpace(rodaNoTree(t, src))

	// e o valor tem que ser o MESMO em toda rodada: se uma goroutine
	// sobrescrever o Numero de outra (arena compartilhada), a soma varia.
	for i := 0; i < 20; i++ {
		if got := strings.TrimSpace(rodaFonte(t, src)); got != querido {
			t.Fatalf("rodada %d: VM deu %q, tree-walker da %q", i, got, querido)
		}
	}
}

// rodaNoTree roda a mesma fonte no tree-walker, pra servir de referencia.
func rodaNoTree(t *testing.T, src string) string {
	t.Helper()
	prog := parser.New(lexer.New(src)).ParseProgram()
	var buf bytes.Buffer
	interp := interpreter.New(&buf)
	if res := interp.Eval(prog, object.NewEnvironment()); res != nil && res.Type() == object.ERRO_OBJ {
		t.Fatalf("tree-walker: %s", res.Inspect())
	}
	return buf.String()
}

// TestArenaNumerosGrandesSaoIndependentes: numeros fora do cache de inteiros
// vem da arena, que os aloca em blocos reusados. Guardar varios numa lista tem
// que preservar cada valor — um bug de indice na arena sobrescreveria o
// anterior.
func TestArenaNumerosGrandesSaoIndependentes(t *testing.T) {
	src := `bota xs = []
bota i = 0
enquanto i < 200
    adiciona(xs, 1000000 + i * 7)
    bota i = i + 1
acabou_finalmente
mostra xs[0]
mostra xs[99]
mostra xs[199]`
	got := strings.TrimSpace(rodaFonte(t, src))
	querido := "1000000\n1000693\n1001393"
	if got != querido {
		t.Fatalf("valores da arena bateram errado:\n got: %q\nquer: %q", got, querido)
	}
}
