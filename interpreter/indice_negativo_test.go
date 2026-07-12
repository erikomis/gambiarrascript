package interpreter

import (
	"strings"
	"testing"
)

func TestIndiceNegativoLista(t *testing.T) {
	out := rodar(t, `bota xs = [10, 20, 30]
mostra xs[-1]
mostra xs[-3]`)
	if out != "30\n10\n" {
		t.Fatalf("indice negativo lista: %q", out)
	}
}

func TestIndiceNegativoListaForaDaErro(t *testing.T) {
	out := rodarErro(t, `mostra [1, 2, 3][-4]`)
	if !strings.Contains(out, "fora da lista") {
		t.Fatalf("indice negativo fora: %q", out)
	}
}

func TestIndiceTextoPositivo(t *testing.T) {
	out := rodar(t, `mostra "abc"[0]
mostra "abc"[2]`)
	if out != "a\nc\n" {
		t.Fatalf("indice texto positivo: %q", out)
	}
}

func TestIndiceTextoNegativo(t *testing.T) {
	out := rodar(t, `mostra "abc"[-1]`)
	if out != "c\n" {
		t.Fatalf("indice texto negativo: %q", out)
	}
}

func TestIndiceTextoUnicode(t *testing.T) {
	out := rodar(t, `mostra "café"[3]`)
	if out != "é\n" {
		t.Fatalf("indice texto unicode: %q", out)
	}
}

func TestIndiceTextoForaDaErro(t *testing.T) {
	out := rodarErro(t, `mostra "abc"[10]`)
	if !strings.Contains(out, "fora do texto") {
		t.Fatalf("indice texto fora: %q", out)
	}
}

func TestIndiceNegativoAtribuicao(t *testing.T) {
	out := rodar(t, `bota xs = [1, 2, 3]
xs[-1] += 5
mostra xs`)
	if out != "[1, 2, 8]\n" {
		t.Fatalf("atribuicao indice negativo: %q", out)
	}
}
