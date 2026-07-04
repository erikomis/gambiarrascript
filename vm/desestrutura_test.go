package vm

import "testing"

func TestVMDesestrutura(t *testing.T) {
	casos := []struct{ input, esp string }{
		{"bota [a, b] = [1, 2, 3]\na + b", "3"},
		{"bota [a, b, c] = [1]\nc", "nada"},
		{`bota {nome} = {"nome": "Erik"}` + "\nnome", "Erik"},
		{`bota {sumido} = {"outra": 1}` + "\nsumido", "nada"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("%q => %s, esperado %s", c.input, got.Inspect(), c.esp)
		}
	}
}

func TestVMDesestruturaDentroDeFuncao(t *testing.T) {
	// locals: o temp __des_gs e os nomes viram slots do frame
	src := `gambiarra soma_par(par)
    bota [a, b] = par
    funciona a + b
acabou_finalmente
soma_par([20, 22])`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "42" {
		t.Errorf("=> %s, esperado 42", got.Inspect())
	}
}
