package lsp

import (
	"testing"

	"gambiarrascript/compiler"
)

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

func TestLintReatribuicaoEmBlocoNaoAvisa(t *testing.T) {
	// blocos dividem o escopo da funcao: `bota total = ...` dentro do laco
	// reatribui a var de fora, nao declara outra
	diags := diagsDeTypecheck(t, `gambiarra f(xs)
    bota total = 0
    pra_cada x em xs
        se_colar x > 0
            bota total = total + x
        acabou_finalmente
    acabou_finalmente
    funciona total
acabou_finalmente`)
	if contemMsg(diags, "nunca usada") {
		t.Fatalf("falso positivo em reatribuicao dentro de bloco: %v", diags)
	}
}

func TestLintBlocosDividemEscopoDaFuncao(t *testing.T) {
	// igual ao runtime: var do pra_cada e `bota` dentro de bloco continuam
	// valendo depois que o bloco fecha
	diags := diagsDeTypecheck(t, `gambiarra f(xs)
    pra_cada x em xs
        bota ultimo = x
    acabou_finalmente
    funciona ultimo + x
acabou_finalmente`)
	if contemMsg(diags, "nunca usada") || contemMsg(diags, "indefinido") {
		t.Fatalf("falso positivo de escopo de bloco: %v", diags)
	}
}

func TestLintVariavelGlobalNaoAvisa(t *testing.T) {
	// top-level nao e checado (scripts tem vars de conveniencia)
	diags := diagsDeTypecheck(t, `bota resultado = 42`)
	if contemMsg(diags, "nunca usada") {
		t.Fatalf("nao devia avisar var top-level: %v", diags)
	}
}

func TestTodoBuiltinDoCompiladorEConhecido(t *testing.T) {
	// builtin que a VM conhece nao pode virar "pode estar indefinido"
	for _, nome := range compiler.BuiltinNomes() {
		if !builtinsSet[nome] {
			t.Errorf("builtin %q fora da lista do LSP", nome)
		}
	}
	diags := diagsDeTypecheck(t, `rota_ws("/x", gambiarra(ws, p) acabou_finalmente)
antes(gambiarra(p) acabou_finalmente)
mostra responde_json(1)`)
	if contemMsg(diags, "indefinido") {
		t.Fatalf("builtin do servidor acusado como indefinido: %v", diags)
	}
}
