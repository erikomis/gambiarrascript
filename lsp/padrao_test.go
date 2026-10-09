package lsp

import "testing"

// pattern matching: nome solto dentro de padrao liga (definicao/referencias,
// typecheck, lint de sombra); cardapio liga o nome e entra no lint de escolhe
// incompleto.

const srcPadrao = `gambiarra f(v)
    escolhe v
    caso [primeiro, ...resto] se primeiro > 0
        funciona resto
    caso {"msg": m, tipo}
        funciona m + tipo
    acabou_finalmente
acabou_finalmente`

func TestDefinicaoPadrao(t *testing.T) {
	s := servidorCom(map[string]string{uriMain: srcPadrao})
	if loc := definicaoEm(t, s, uriMain, acha(t, srcPadrao, "primeiro > 0", 0)); loc == nil || loc.Range.Start != (Posicao{2, 10}) {
		t.Fatalf("primeiro na guarda deveria ir pro padrao: %+v", loc)
	}
	if loc := definicaoEm(t, s, uriMain, acha(t, srcPadrao, "resto", 1)); loc == nil || loc.Range.Start != (Posicao{2, 23}) {
		t.Fatalf("resto no corpo deveria ir pro ...resto: %+v", loc)
	}
	if loc := definicaoEm(t, s, uriMain, acha(t, srcPadrao, "tipo", 1)); loc == nil || loc.Range.Start != (Posicao{4, 20}) {
		t.Fatalf("tipo no corpo deveria ir pra forma curta: %+v", loc)
	}
	// m: decl + uso
	if refs := referenciasEm(t, s, uriMain, Posicao{4, 17}, true); len(refs) != 2 {
		t.Fatalf("m: esperava 2 referencias, veio %+v", refs)
	}
}

func TestTypecheckPadrao(t *testing.T) {
	if diags := diagsDeTypecheck(t, srcPadrao); len(diags) != 0 {
		t.Fatalf("nomes do padrao sao ligados: %v", diags)
	}
	// fora de padrao, `caso x` le x (continua checado)
	diags := diagsDeTypecheck(t, `escolhe 1
caso naoexiste
    mostra 1
acabou_finalmente`)
	if !contemMsg(diags, "naoexiste") {
		t.Fatalf("caso x le a variavel: %v", diags)
	}
}

func TestLintSombraPadrao(t *testing.T) {
	// ler a global antes do padrao que amarra o mesmo nome: o nome e local
	// na funcao inteira
	diags := diagsDeTypecheck(t, `bota a = 1
gambiarra f(v)
    mostra a
    escolhe v
    caso [a]
        funciona a
    acabou_finalmente
acabou_finalmente
mostra f([2])`)
	if !contemMsg(diags, msgSombra) {
		t.Fatalf("leitura antes do padrao devia avisar sombra: %v", diags)
	}
}

func TestLintCardapioIncompleto(t *testing.T) {
	base := `cardapio Cor
    vermelho
    verde
    azul
acabou_finalmente
`
	diags := diagsDeTypecheck(t, base+`bota c = Cor.verde
escolhe c
caso Cor.vermelho
    mostra 1
caso Cor.verde
    mostra 2
acabou_finalmente`)
	if !contemMsg(diags, "esquece Cor.azul") {
		t.Fatalf("faltou avisar Cor.azul: %v", diags)
	}
	// com se_nao_colar, com guarda, cobrindo tudo ou misturando: nao avisa
	semAviso := []string{
		`escolhe Cor.azul
caso Cor.vermelho
    mostra 1
se_nao_colar
    mostra 2
acabou_finalmente`,
		`escolhe Cor.azul
caso Cor.vermelho, Cor.verde
    mostra 1
caso Cor.azul
    mostra 2
acabou_finalmente`,
		`escolhe Cor.azul
caso Cor.vermelho se deu_bom
    mostra 1
acabou_finalmente`,
		`escolhe Cor.azul
caso Cor.vermelho, "x"
    mostra 1
acabou_finalmente`,
	}
	for _, src := range semAviso {
		if diags := diagsDeTypecheck(t, base+src); contemMsg(diags, "esquece") {
			t.Fatalf("nao devia avisar em\n%s\n%v", src, diags)
		}
	}
}
