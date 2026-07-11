package interpreter

import (
	"strings"
	"testing"
)

func TestSementeReproduzEmbaralha(t *testing.T) {
	out := rodar(t, `semente(7)
bota a = embaralha([1, 2, 3, 4, 5, 6, 7, 8])
semente(7)
bota b = embaralha([1, 2, 3, 4, 5, 6, 7, 8])
mostra a == b`)
	if out != "deu_bom\n" {
		t.Fatalf("semente/embaralha reproduz: %q", out)
	}
}

func TestEmbaralhaEhPermutacao(t *testing.T) {
	out := rodar(t, `bota emb = embaralha([3, 1, 2])
ordena(emb)
mostra emb == [1, 2, 3]`)
	if out != "deu_bom\n" {
		t.Fatalf("embaralha permutacao: %q", out)
	}
}

func TestEmbaralhaNaoMutaOriginal(t *testing.T) {
	out := rodar(t, `bota xs = [1, 2, 3, 4, 5]
embaralha(xs)
mostra xs == [1, 2, 3, 4, 5]`)
	if out != "deu_bom\n" {
		t.Fatalf("embaralha mutou original: %q", out)
	}
}

func TestEscolheUmUnico(t *testing.T) {
	out := rodar(t, `mostra escolhe_um([42])`)
	if out != "42\n" {
		t.Fatalf("escolhe_um unico: %q", out)
	}
}

func TestEscolheUmVaziaDaErro(t *testing.T) {
	out := rodarErro(t, `escolhe_um([])`)
	if !strings.Contains(out, "vazia") {
		t.Fatalf("escolhe_um vazia: %q", out)
	}
}

func TestEscolheUmReproduz(t *testing.T) {
	out := rodar(t, `semente(3)
bota x = escolhe_um([10, 20, 30, 40, 50])
semente(3)
bota y = escolhe_um([10, 20, 30, 40, 50])
mostra x == y`)
	if out != "deu_bom\n" {
		t.Fatalf("escolhe_um reproduz: %q", out)
	}
}

func TestUuidFormatoV4(t *testing.T) {
	out := rodar(t, `mostra busca_regex("^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$", uuid())`)
	if out != "deu_bom\n" {
		t.Fatalf("uuid formato v4: %q", out)
	}
}

func TestUuidDoisDiferentes(t *testing.T) {
	out := rodar(t, `mostra uuid() == uuid()`)
	if out != "deu_ruim\n" {
		t.Fatalf("uuid deviam diferir: %q", out)
	}
}

func TestSementeNaoNumeroDaErro(t *testing.T) {
	out := rodarErro(t, `semente("x")`)
	if !strings.Contains(out, "numero") {
		t.Fatalf("semente nao numero: %q", out)
	}
}
