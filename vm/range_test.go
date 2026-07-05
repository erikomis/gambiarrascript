package vm

import "testing"

func TestVMRange(t *testing.T) {
	casos := []struct{ input, esp string }{
		{"1..5", "[1, 2, 3, 4, 5]"},
		{"3..3", "[3]"},
		{"5..1", "[5, 4, 3, 2, 1]"},
		{"tamanho(1..10)", "10"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("%q => got %s, esperado %s", c.input, got.Inspect(), c.esp)
		}
	}
}

// TestVMLoopContadorInteiroExato cobre uma regressao: o incremento do loop
// numerico (`pra_cada i de A ate B`) era compilado como object.Numero float
// (EhInt=false), entao depois da primeira iteracao o contador `i` degradava
// pra float64 e perdia exatidao acima de 2^53 (o tree-walker mantinha int64).
// Aqui i chega a 3 e 2^53+3 (=9007199254740995, impar) NAO cabe em float64
// (arredondaria pra ...996) — so o caminho inteiro exato acerta.
func TestVMLoopContadorInteiroExato(t *testing.T) {
	src := `bota r = 0
pra_cada i de 1 ate 3
    bota r = 9007199254740992 + i
acabou_finalmente
r`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "9007199254740995" {
		t.Errorf("contador do loop perdeu exatidao: got %s, esperado 9007199254740995", got.Inspect())
	}
}

func TestVMIndexSet(t *testing.T) {
	// dicionario: bota d[k] = v
	dic := `bota d = {"a": 1}
bota d["b"] = 2
d["b"]`
	if got, _ := rodaVM(t, dic); got.Inspect() != "2" {
		t.Errorf("dict index-set => %s, esperado 2", got.Inspect())
	}
	// lista: bota xs[i] = v
	lst := `bota xs = [10, 20, 30]
bota xs[1] = 99
xs[1]`
	if got, _ := rodaVM(t, lst); got.Inspect() != "99" {
		t.Errorf("list index-set => %s, esperado 99", got.Inspect())
	}
}

func TestVMPraCadaDicionario(t *testing.T) {
	// pra_cada em dicionario: itera as CHAVES (nao dict[0], dict[1]...).
	// Soma os valores por chave — resultado independe da ordem do mapa.
	src := `bota d = {"a": 1, "b": 2, "c": 3}
bota soma = 0
pra_cada k em d
    bota soma = soma + d[k]
acabou_finalmente
soma`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "6" {
		t.Errorf("soma dos valores por chave => %s, esperado 6", got.Inspect())
	}
}

func TestVMFuncaoComLocaisEPraCada(t *testing.T) {
	// regressao: funcao com locals alem dos params + pra_cada dentro.
	// Antes, OpCall nao reservava os slots de local e a pilha de trabalho
	// sobrescrevia __seq/__it/__len (dava "tamanho() nao funciona com NUMERO").
	src := `gambiarra conta(lista)
    bota total = 0
    pra_cada item em lista
        bota total = total + 1
    acabou_finalmente
    funciona total
acabou_finalmente
conta(["a", "b", "c", "d"])`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "4" {
		t.Errorf("conta(4 itens) => %s, esperado 4", got.Inspect())
	}
}

func TestVMPraCadaLista(t *testing.T) {
	// regressao: pra_cada em lista continua iterando os ELEMENTOS.
	src := `bota xs = [10, 20, 30]
bota soma = 0
pra_cada v em xs
    bota soma = soma + v
acabou_finalmente
soma`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "60" {
		t.Errorf("soma dos elementos => %s, esperado 60", got.Inspect())
	}
}

func TestVMRangePraCada(t *testing.T) {
	src := `bota soma = 0
pra_cada x em 1..4
    bota soma = soma + x
acabou_finalmente
soma`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "10" {
		t.Errorf("soma 1..4 => %s", got.Inspect())
	}
}
