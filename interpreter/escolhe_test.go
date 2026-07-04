package interpreter

import "testing"

func TestEscolheBasico(t *testing.T) {
	out := rodar(t, `escolhe 2
caso 1
    mostra "um"
caso 2
    mostra "dois"
acabou_finalmente`)
	if out != "dois\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestEscolheMultiplosValores(t *testing.T) {
	out := rodar(t, `escolhe "sab"
caso "sab", "dom"
    mostra "fds"
se_nao_colar
    mostra "trampo"
acabou_finalmente`)
	if out != "fds\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestEscolheDefault(t *testing.T) {
	out := rodar(t, `escolhe 99
caso 1
    mostra "um"
se_nao_colar
    mostra "outro"
acabou_finalmente`)
	if out != "outro\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestEscolheSemMatchSemDefault(t *testing.T) {
	out := rodar(t, `escolhe 99
caso 1
    mostra "um"
acabou_finalmente
mostra "seguiu"`)
	if out != "seguiu\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestEscolheSemFallthrough(t *testing.T) {
	out := rodar(t, `escolhe 1
caso 1
    mostra "um"
caso 1, 2
    mostra "NAO era pra rodar"
acabou_finalmente`)
	if out != "um\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestEscolheComFunciona(t *testing.T) {
	out := rodar(t, `gambiarra nome(n)
    escolhe n
    caso 1
        funciona "um"
    se_nao_colar
        funciona "muitos"
    acabou_finalmente
acabou_finalmente
mostra nome(1)
mostra nome(7)`)
	if out != "um\nmuitos\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestEscolheExpressaoNoCaso(t *testing.T) {
	out := rodar(t, `bota n = 4
escolhe n
caso 2 + 2
    mostra "quatro"
acabou_finalmente`)
	if out != "quatro\n" {
		t.Fatalf("saida %q", out)
	}
}
