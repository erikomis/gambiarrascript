package vm

import "testing"

func TestVMEscolhe(t *testing.T) {
	casos := []struct{ input, esp string }{
		{`bota r = "x"
escolhe 2
caso 1
    bota r = "um"
caso 2
    bota r = "dois"
acabou_finalmente
r`, "dois"},
		{`bota r = "x"
escolhe "dom"
caso "sab", "dom"
    bota r = "fds"
se_nao_colar
    bota r = "trampo"
acabou_finalmente
r`, "fds"},
		{`bota r = "x"
escolhe 99
caso 1
    bota r = "um"
se_nao_colar
    bota r = "outro"
acabou_finalmente
r`, "outro"},
		{`bota r = "nada casou"
escolhe 99
caso 1
    bota r = "um"
acabou_finalmente
r`, "nada casou"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("%q => %s, esperado %s", c.input, got.Inspect(), c.esp)
		}
	}
}

func TestVMEscolheDentroDeFuncao(t *testing.T) {
	src := `gambiarra nome(n)
    escolhe n
    caso 1
        funciona "um"
    se_nao_colar
        funciona "muitos"
    acabou_finalmente
acabou_finalmente
nome(1) + "-" + nome(9)`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "um-muitos" {
		t.Errorf("=> %s", got.Inspect())
	}
}
