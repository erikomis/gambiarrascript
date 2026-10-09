package lsp

import (
	"strings"
	"testing"
)

const msgFuncionaGerador = "num gerador o funciona so encerra"

func contaAviso(diags []Diagnostico, trecho string) int {
	n := 0
	for _, d := range diags {
		if strings.Contains(d.Message, trecho) {
			n++
		}
	}
	return n
}

func TestLintFuncionaComValorEmGerador(t *testing.T) {
	diags := diagsDeTypecheck(t, `gambiarra g(n)
    rende n
    funciona n + 1
acabou_finalmente
bota h = gambiarra()
    rende 1
    funciona nada
acabou_finalmente
gambiarra comum()
    funciona 2
acabou_finalmente
gambiarra fabrica()
    bota f = gambiarra()
        rende 1
    acabou_finalmente
    funciona f
acabou_finalmente
mostra g(1)
mostra h()
mostra comum()
mostra fabrica()`)
	if n := contaAviso(diags, msgFuncionaGerador); n != 1 {
		t.Fatalf("esperava 1 aviso de funciona em gerador, veio %d: %+v", n, diags)
	}
}

// O valor do rende conta como uso das variaveis (nada de "nao usada").
func TestLintRendeUsaVariavel(t *testing.T) {
	diags := diagsDeTypecheck(t, `gambiarra g()
    bota x = 1
    rende x
acabou_finalmente
mostra g()`)
	for _, d := range diags {
		if strings.Contains(d.Message, "x") {
			t.Fatalf("aviso inesperado: %+v", d)
		}
	}
}
