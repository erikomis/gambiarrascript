package interpreter

import "testing"

func TestInteiroGrandePreservaPrecisao(t *testing.T) {
	casos := []struct{ in, esp string }{
		// 2^53 + 1: impossivel de representar exato em float64
		{`mostra 9007199254740993`, "9007199254740993\n"},
		{`mostra 9007199254740992 + 1`, "9007199254740993\n"},
		{`mostra 99999999 * 99999999`, "9999999800000001\n"},
		// soma de 5 termos perto de 2^53 (5 iteracoes, mas exige precisao inteira)
		{`bota a = 0
pra_cada i de 9007199254740991 ate 9007199254740995
    bota a = a + i
acabou_finalmente
mostra a`, "45035996273704965\n"},
		// divisao exata entre inteiros continua inteiro; inexata vira float
		{`mostra 10 / 2`, "5\n"},
		{`mostra 7 / 2`, "3.5\n"},
	}
	for _, c := range casos {
		if got := rodar(t, c.in); got != c.esp {
			t.Errorf("%q => got %q, esperado %q", c.in, got, c.esp)
		}
	}
}
