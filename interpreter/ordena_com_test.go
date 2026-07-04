package interpreter

import "testing"

func TestOrdenaComNumero(t *testing.T) {
	out := rodar(t, `gambiarra cmp(a, b)
    funciona b - a
acabou_finalmente
bota xs = [5, 2, 8, 1]
ordena_com(xs, cmp)
mostra xs`)
	if out != "[8, 5, 2, 1]\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestOrdenaComBooleano(t *testing.T) {
	out := rodar(t, `gambiarra cmp(a, b)
    funciona a["n"] < b["n"]
acabou_finalmente
bota gente = [{"n": 3}, {"n": 1}, {"n": 2}]
ordena_com(gente, cmp)
mostra gente[0]["n"]
mostra gente[2]["n"]`)
	if out != "1\n3\n" {
		t.Fatalf("saida %q", out)
	}
}
