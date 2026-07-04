package vm

import (
	"strings"
	"testing"
)

// Cobre o unwinding de frames: arruma/quebrou capturando erro que estoura
// DENTRO de funcao chamada no try (antes o handler assumia o mesmo frame e o
// erro vazava com a pilha corrompida).

func TestVMCatchCrossFrame(t *testing.T) {
	src := `gambiarra f()
    funciona 1 / 0
acabou_finalmente
bota r = "nao pegou"
arruma
    mostra f()
quebrou err
    bota r = "pegou"
acabou_finalmente
r`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "pegou" {
		t.Fatalf("catch cross-frame => %s, esperado pegou", got.Inspect())
	}
}

func TestVMCatchCrossFrameDoisNiveis(t *testing.T) {
	src := `gambiarra g()
    funciona 1 / 0
acabou_finalmente
gambiarra f()
    funciona g()
acabou_finalmente
bota r = "nao pegou"
arruma
    mostra f()
quebrou err
    bota r = "pegou"
acabou_finalmente
r`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "pegou" {
		t.Fatalf("catch 2 niveis => %s, esperado pegou", got.Inspect())
	}
}

func TestVMContinuaDepoisDoCatchEmFuncao(t *testing.T) {
	// catch DENTRO de uma funcao: depois do catch a funcao retorna normal e
	// o CHAMADOR continua (regressao do resume com baseIdx original).
	src := `gambiarra tenta()
    arruma
        bota x = 1 / 0
    quebrou err
        funciona "recuperou"
    acabou_finalmente
acabou_finalmente
bota a = tenta()
bota b = "e seguiu"
a + " " + b`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "recuperou e seguiu" {
		t.Fatalf("=> %s", got.Inspect())
	}
}

func TestVMFuncionaDentroDeTryNaoDeixaHandlerOrfao(t *testing.T) {
	// `funciona` dentro de arruma sai da funcao sem passar pelo OpTryEnd —
	// o handler orfao nao pode capturar erro de DEPOIS do retorno.
	src := `gambiarra f()
    arruma
        funciona "saiu do try"
    quebrou err
        funciona "catch errado"
    acabou_finalmente
acabou_finalmente
bota r = f()
arruma
    bota x = 1 / 0
quebrou err2
    bota r = r + " / pegou no lugar certo"
acabou_finalmente
r`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "saiu do try / pegou no lugar certo" {
		t.Fatalf("handler orfao vazou: %s", got.Inspect())
	}
}

func TestVMTracoDePilha(t *testing.T) {
	// traço externo->interno com linha do call site (igual tree-walker)
	src := `gambiarra g()
    funciona 1 / 0
acabou_finalmente
gambiarra f()
    funciona g()
acabou_finalmente
mostra f()`
	msg := rodaVMErro(t, src)
	if !strings.Contains(msg, "deu ruim na linha 2") {
		t.Fatalf("linha do erro errada: %q", msg)
	}
	// o traço fica no *object.Erro (via ErroDoRun) — refaz pegando o erro
	eo := ErroDoRun(rodaVMErroObj(t, src))
	if eo == nil {
		t.Fatalf("ErroDoRun devolveu nil")
	}
	traco := eo.Traco()
	// f chamado do main na linha 7; g chamado de f na linha 5
	if !strings.Contains(traco, "em f (linha 7)") || !strings.Contains(traco, "em g (linha 5)") {
		t.Fatalf("traço errado:\n%s", traco)
	}
	// ordem externo->interno: f antes de g
	if strings.Index(traco, "em f") > strings.Index(traco, "em g") {
		t.Fatalf("ordem do traço invertida:\n%s", traco)
	}
}

func TestVMErroPilhaBuiltin(t *testing.T) {
	// erro_pilha(err) depois do quebrou devolve o traço na VM tambem
	src := `gambiarra f()
    funciona 1 / 0
acabou_finalmente
bota t = ""
arruma
    mostra f()
quebrou err
    bota t = erro_pilha(err)
acabou_finalmente
t`
	got, _ := rodaVM(t, src)
	if !strings.Contains(got.Inspect(), "em f (linha 6)") {
		t.Fatalf("erro_pilha sem o frame de f: %q", got.Inspect())
	}
}
