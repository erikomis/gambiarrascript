package vm

import "testing"

// Builtins de ordem superior chamando funcoes do usuario NA VM — cobre o
// gancho ChamaCompilada (interpreter -> vm.chamaCompilada).
func TestVMMapeiaFiltra(t *testing.T) {
	casos := []struct{ input, esp string }{
		{`gambiarra dobra(n)
    funciona n * 2
acabou_finalmente
mapeia([1, 2, 3], dobra)`, "[2, 4, 6]"},
		{`mapeia([1, 2, 3], gambiarra(x) funciona x + 10 acabou_finalmente)`, "[11, 12, 13]"},
		{`filtra([1, 2, 3, 4], gambiarra(n) funciona n % 2 == 0 acabou_finalmente)`, "[2, 4]"},
		{`reduz([1, 2, 3, 4], gambiarra(acc, n) funciona acc + n acabou_finalmente, 0)`, "10"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("%q => %s, esperado %s", c.input, got.Inspect(), c.esp)
		}
	}
}

func TestVMOrdenaCom(t *testing.T) {
	src := `bota xs = [5, 2, 8, 1]
ordena_com(xs, gambiarra(a, b) funciona b - a acabou_finalmente)
xs`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "[8, 5, 2, 1]" {
		t.Errorf("ordena_com desc => %s", got.Inspect())
	}
}

func TestVMOrdemSuperiorPropagaErro(t *testing.T) {
	// erro dentro da funcao do usuario tem que propagar (nao engolir)
	src := `arruma
    mapeia([1], gambiarra(x) funciona x / 0 acabou_finalmente)
quebrou err
    "pegou"
acabou_finalmente`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "pegou" {
		t.Errorf("erro na fn do usuario => %s, esperado pegou", got.Inspect())
	}
}
