package lsp

import "testing"

// TestTypecheckCravaReatribuida: mexer em cravada vira ERRO (severidade 1),
// sublinhando o nome inteiro, pro editor e pro `gs check`.
func TestTypecheckCravaReatribuida(t *testing.T) {
	diags := diagsDeTypecheck(t, "crava LIMITE = 10\nmostra LIMITE\nLIMITE += 1")
	var achou *Diagnostico
	for i := range diags {
		if strContains(diags[i].Message, "`LIMITE` foi cravada") {
			achou = &diags[i]
		}
	}
	if achou == nil {
		t.Fatalf("nao achou erro de cravada: %v", diags)
	}
	if achou.Severity != 1 {
		t.Fatalf("cravada devia ser erro (1), veio %d", achou.Severity)
	}
	r := achou.Range
	if r.Start.Line != 2 || r.Start.Character != 0 || r.End.Character != len("LIMITE") {
		t.Fatalf("faixa errada: %+v", r)
	}
}

func TestTypecheckCravaValidaNaoAvisa(t *testing.T) {
	diags := diagsDeTypecheck(t, `crava XS = [1]
bota XS[0] = 2
gambiarra f()
    bota XS = 3
    funciona XS
acabou_finalmente
mostra f()`)
	if len(diags) != 0 {
		t.Fatalf("esperava 0 diags, veio %v", diags)
	}
}

// TestTypecheckMatematicaConhecida: as builtins novas e o `pi` nao sao
// "indefinidos".
func TestTypecheckMatematicaConhecida(t *testing.T) {
	diags := diagsDeTypecheck(t, `mostra seno(pi) + cosseno(pi) + tangente(1)
mostra log(2) + log10(10) + exp(1) + 2 ** 3`)
	if len(diags) != 0 {
		t.Fatalf("esperava 0 diags, veio %v", diags)
	}
}

func TestCompletionEHoverCravaEPi(t *testing.T) {
	s := NovoServidor(nil)
	tem := map[string]int{}
	for _, it := range s.itensCompletion("") {
		tem[it.Label] = it.Kind
	}
	if tem["crava"] != 14 || tem["seno"] != 3 || tem["pi"] != 21 {
		t.Fatalf("completion: crava=%d seno=%d pi=%d", tem["crava"], tem["seno"], tem["pi"])
	}
	if hoverConteudo("crava X = 1", 0, 2) == "" {
		t.Fatal("hover sobre crava devia ter doc")
	}
	if hoverConteudo("mostra pi", 0, 8) == "" {
		t.Fatal("hover sobre pi devia ter doc")
	}
}
