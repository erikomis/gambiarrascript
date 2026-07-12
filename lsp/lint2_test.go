package lsp

import "testing"

func TestLintCodigoMortoAposFunciona(t *testing.T) {
	diags := diagsDeTypecheck(t, `gambiarra f()
    funciona 1
    mostra "nunca roda"
acabou_finalmente`)
	if !contemMsg(diags, "codigo morto") {
		t.Fatalf("nao detectou codigo morto apos funciona: %v", diags)
	}
}

func TestLintCodigoMortoAposVaza(t *testing.T) {
	diags := diagsDeTypecheck(t, `enquanto deu_bom
    vaza
    mostra "nunca"
acabou_finalmente`)
	if !contemMsg(diags, "codigo morto") {
		t.Fatalf("nao detectou codigo morto apos vaza: %v", diags)
	}
}

func TestLintSemCodigoMorto(t *testing.T) {
	diags := diagsDeTypecheck(t, `gambiarra f()
    mostra "ok"
    funciona 1
acabou_finalmente`)
	if contemMsg(diags, "codigo morto") {
		t.Fatalf("falso positivo de codigo morto: %v", diags)
	}
}

func TestLintVariavelNaoUsada(t *testing.T) {
	diags := diagsDeTypecheck(t, `gambiarra f()
    bota naoUsada = 10
    funciona 1
acabou_finalmente`)
	if !contemMsg(diags, "nunca usada") {
		t.Fatalf("nao detectou variavel nao usada: %v", diags)
	}
}

func TestLintVariavelUsadaNaoAvisa(t *testing.T) {
	diags := diagsDeTypecheck(t, `gambiarra f()
    bota x = 10
    funciona x
acabou_finalmente`)
	if contemMsg(diags, "nunca usada") {
		t.Fatalf("falso positivo de variavel usada: %v", diags)
	}
}

func TestLintVariavelGlobalNaoAvisa(t *testing.T) {
	// top-level nao e checado (scripts tem vars de conveniencia)
	diags := diagsDeTypecheck(t, `bota resultado = 42`)
	if contemMsg(diags, "nunca usada") {
		t.Fatalf("nao devia avisar var top-level: %v", diags)
	}
}
