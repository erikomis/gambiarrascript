package lsp

import "testing"

// multi-catch: cada `quebrou NOME se COND` liga o nome dele (o filtro ja
// enxerga); definicao/referencias e o typecheck tratam igual ao quebrou
// simples.

const srcMultiCatch = `arruma
    quebra("x")
quebrou erro se erro_tipo(erro) == "rede"
    mostra erro_msg(erro)
quebrou outro se contem(erro_msg(outro), "x")
    mostra outro
quebrou resto
    mostra resto
acabou_finalmente`

func TestDefinicaoMultiCatch(t *testing.T) {
	s := servidorCom(map[string]string{uriMain: srcMultiCatch})
	if loc := definicaoEm(t, s, uriMain, acha(t, srcMultiCatch, "erro)", 0)); loc == nil || loc.Range.Start != (Posicao{2, 8}) {
		t.Fatalf("erro no filtro deveria ir pro 1o quebrou: %+v", loc)
	}
	if loc := definicaoEm(t, s, uriMain, acha(t, srcMultiCatch, "outro)", 0)); loc == nil || loc.Range.Start != (Posicao{4, 8}) {
		t.Fatalf("outro no filtro deveria ir pro 2o quebrou: %+v", loc)
	}
	if loc := definicaoEm(t, s, uriMain, acha(t, srcMultiCatch, "resto", 1)); loc == nil || loc.Range.Start != (Posicao{6, 8}) {
		t.Fatalf("resto no corpo deveria ir pro 3o quebrou: %+v", loc)
	}
	// `se` do filtro e palavra-chave ali: nao tem definicao
	if loc := definicaoEm(t, s, uriMain, acha(t, srcMultiCatch, "se erro_tipo", 0)); loc != nil {
		t.Fatalf("`se` nao e variavel: %+v", loc)
	}
}

func TestReferenciasMultiCatch(t *testing.T) {
	s := servidorCom(map[string]string{uriMain: srcMultiCatch})
	// decl + filtro + corpo
	if refs := referenciasEm(t, s, uriMain, Posicao{4, 8}, true); len(refs) != 3 {
		t.Fatalf("outro: esperava 3 referencias, veio %+v", refs)
	}
	if refs := referenciasEm(t, s, uriMain, Posicao{2, 8}, true); len(refs) != 3 {
		t.Fatalf("erro: esperava 3 referencias, veio %+v", refs)
	}
}

func TestTypecheckMultiCatch(t *testing.T) {
	if diags := diagsDeTypecheck(t, srcMultiCatch); len(diags) != 0 {
		t.Fatalf("cada quebrou amarra o nome (filtro incluso): %v", diags)
	}
	// nome de outra clausula no filtro: escopo de funcao, ja foi amarrado
	diags := diagsDeTypecheck(t, `arruma
    quebra("x")
quebrou a se erro_tipo(a) == "rede"
    mostra a
quebrou b se naoexiste(b)
    mostra b
acabou_finalmente`)
	if !contemMsg(diags, "naoexiste") {
		t.Fatalf("filtro tem que ser checado (naoexiste): %v", diags)
	}
}

func TestLintSombraMultiCatch(t *testing.T) {
	// o nome de cada quebrou e local da funcao e ja esta amarrado no filtro:
	// ler `erro` no filtro nao e leitura da global antes do bota
	diags := diagsDeTypecheck(t, `bota erro = "global"
bota outro = 1
gambiarra f()
    arruma
        quebra("x")
    quebrou erro se erro_tipo(erro) == "rede"
        funciona 1
    quebrou outro se erro_msg(outro) != ""
        funciona erro_msg(outro) + erro_msg(erro)
    acabou_finalmente
acabou_finalmente
mostra f()`)
	if contemMsg(diags, msgSombra) {
		t.Fatalf("quebrou com filtro nao devia avisar sombra: %v", diags)
	}
	// ler o nome do 2o quebrou ANTES dele (no filtro do 1o) e leitura da
	// global: avisa
	diags = diagsDeTypecheck(t, `bota outro = 1
gambiarra g()
    arruma
        quebra("x")
    quebrou erro se outro == 1
        funciona 1
    quebrou outro
        funciona 2
    acabou_finalmente
acabou_finalmente
mostra g()`)
	if !contemMsg(diags, "a global `outro` nao muda") {
		t.Fatalf("leitura de `outro` antes do quebrou dele devia avisar: %v", diags)
	}
}
