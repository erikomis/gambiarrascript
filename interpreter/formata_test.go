package interpreter

import "testing"

func TestFormataPadding(t *testing.T) {
	casos := []struct{ src, esp string }{
		{`mostra formata("%05d", 42)`, "00042\n"},
		{`mostra formata("%.2f", 3.14159)`, "3.14\n"},
		{`mostra formata("[%-4s]", "oi")`, "[oi  ]\n"},
		{`mostra formata("%v %v", deu_bom, nada)`, "deu_bom nada\n"},
		{`mostra formata("%d%%", 99)`, "99%\n"},
		{`mostra formata("sem args")`, "sem args\n"},
	}
	for _, c := range casos {
		if out := rodar(t, c.src); out != c.esp {
			t.Errorf("%q => %q, esperado %q", c.src, out, c.esp)
		}
	}
}

func TestEsperaBoraRoundTrip(t *testing.T) {
	// async/await: bora dispara, espera aguarda o Futuro
	out := rodar(t, `gambiarra demorada(n)
    funciona n * 2
acabou_finalmente
bota fut = bora demorada(21)
mostra espera(fut)`)
	if out != "42\n" {
		t.Fatalf("saida %q", out)
	}
}
