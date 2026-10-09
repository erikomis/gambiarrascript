package vm

import (
	"runtime"
	"testing"
	"time"
)

// Gerador abandonado no meio (vaza no pra_cada, proximo() que ninguem chama
// de novo) nao pode deixar goroutine (tree-walker) nem memoria presa. Na VM o
// gerador nem usa goroutine; no tree-walker a produtora parada no `rende` sai
// quando o gerador vira lixo (finalizer).
const geradorAbandonado = `gambiarra naturais()
    bota n = 0
    enquanto deu_bom
        rende n
        n += 1
    acabou_finalmente
acabou_finalmente
bota total = 0
pra_cada k de 1 ate 300
    pra_cada x em naturais()
        se_colar x == 2
            vaza
        acabou_finalmente
    acabou_finalmente
    total += proximo(naturais())
    bota g = naturais()
    proximo(g)
    proximo(g)
acabou_finalmente
mostra total`

// esperaGoroutines roda GC ate o numero de goroutines voltar pro limite (ou
// desistir depois de uns segundos) e devolve o ultimo valor visto.
func esperaGoroutines(limite int) int {
	fim := time.Now().Add(5 * time.Second)
	n := runtime.NumGoroutine()
	for n > limite && time.Now().Before(fim) {
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
		n = runtime.NumGoroutine()
	}
	return n
}

func TestGeradorAbandonadoNaoVazaGoroutine(t *testing.T) {
	for _, e := range []struct {
		nome string
		roda func(*testing.T, string) (string, string, string)
	}{{"tree", rodaTWComp}, {"vm", rodaVMComp}} {
		antes := esperaGoroutines(runtime.NumGoroutine())
		_, saida, errStr := e.roda(t, geradorAbandonado)
		if errStr != "" || saida != "0\n" {
			t.Fatalf("[%s] saida %q erro %q", e.nome, saida, errStr)
		}
		t.Logf("[%s] goroutines: antes %d, logo depois %d", e.nome, antes, runtime.NumGoroutine())
		if depois := esperaGoroutines(antes + 2); depois > antes+2 {
			t.Fatalf("[%s] goroutines vazaram: antes %d, depois %d", e.nome, antes, depois)
		}
	}
}

// Na VM o gerador e uma corrotina na mesma goroutine: nem durante o laco
// nasce goroutine nova.
func TestGeradorVMSemGoroutine(t *testing.T) {
	antes := runtime.NumGoroutine()
	_, saida, errStr := rodaVMComp(t, `gambiarra conta()
    pra_cada i de 1 ate 3
        rende i
    acabou_finalmente
acabou_finalmente
bota gs = []
pra_cada k de 1 ate 200
    bota g = conta()
    proximo(g)
    adiciona(gs, g)
acabou_finalmente
mostra tamanho(gs)`)
	if errStr != "" || saida != "200\n" {
		t.Fatalf("saida %q erro %q", saida, errStr)
	}
	if depois := runtime.NumGoroutine(); depois > antes {
		t.Fatalf("a VM criou goroutine pra gerador: antes %d, depois %d", antes, depois)
	}
}
