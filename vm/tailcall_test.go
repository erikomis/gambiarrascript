package vm

import (
	"strings"
	"testing"
)

// Recursao em cauda (funciona f(args)) deve rodar em profundidade CONSTANTE de
// frames — bem acima de MaxFrames=1024 — sem estourar.
func TestVMTailCallProfundoNaoEstoura(t *testing.T) {
	src := `gambiarra soma_ate(n, acc)
    se_colar n == 0
        funciona acc
    acabou_finalmente
    funciona soma_ate(n - 1, acc + n)
acabou_finalmente
mostra soma_ate(50000, 0)`
	_, saida, errStr := rodaVMComp(t, src)
	if errStr != "" {
		t.Fatalf("tail call estourou (devia rodar constante): %s", errStr)
	}
	// soma 1..50000 = 50000*50001/2 = 1250025000
	if strings.TrimSpace(saida) != "1250025000" {
		t.Fatalf("resultado errado: %q", saida)
	}
}

// O resultado do tail call tem que bater com o tree-walker (correcao).
func TestVMTailCallResultadoParidade(t *testing.T) {
	comparaEngines(t, `gambiarra fatorial(n, acc)
    se_colar n <= 1
        funciona acc
    acabou_finalmente
    funciona fatorial(n - 1, acc * n)
acabou_finalmente
mostra fatorial(6, 1)`)
}

// Recursao funda NAO-cauda (funciona 1 + f(...)) continua empilhando; agora
// deve dar um ERRO LIMPO de overflow em vez de panic do Go.
func TestVMRecursaoFundaErroGracioso(t *testing.T) {
	src := `gambiarra conta(n)
    se_colar n == 0
        funciona 0
    acabou_finalmente
    funciona 1 + conta(n - 1)
acabou_finalmente
mostra conta(3000)`
	_, _, errStr := rodaVMComp(t, src)
	if !strings.Contains(errStr, "recursao funda") {
		t.Fatalf("esperava erro de overflow gracioso, veio: %q", errStr)
	}
}
