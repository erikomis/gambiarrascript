package parser

import (
	"strings"
	"testing"

	"gambiarrascript/ast"
)

func TestParsePadroesNoCaso(t *testing.T) {
	prog := parse(t, `escolhe v
caso 1, x
    mostra 1
caso []
    mostra 2
caso [a, 0, ...resto]
    mostra 3
caso {"tipo": "erro", msg, 2: [_, b]}
    mostra 4
caso Ponto{x: 0, y}, geo.Ponto{z: Cor.azul}
    mostra 5
caso [a, b] se a > b
    mostra 6
caso _
    mostra 7
acabou_finalmente`)
	e := prog.Statements[0].(*ast.EscolheStatement)
	if len(e.Casos) != 7 {
		t.Fatalf("esperava 7 casos, veio %d", len(e.Casos))
	}
	// valor comum continua expressao (x compara com a variavel x)
	if _, ok := e.Casos[0].Values[1].(*ast.Identifier); !ok {
		t.Fatalf("`caso x` tem que continuar Identifier, veio %T", e.Casos[0].Values[1])
	}
	esperados := []string{
		"[]",
		"[a, 0, ...resto]",
		`{"tipo": "erro", msg, 2: [_, b]}`,
		"Ponto{x: 0, y}",
	}
	for i, esp := range esperados {
		v := e.Casos[i+1].Values[0]
		if !ast.EhPadrao(v) || v.String() != esp {
			t.Errorf("caso %d: veio %T %q, esperado padrao %q", i+1, v, v.String(), esp)
		}
	}
	if _, ok := e.Casos[4].Values[1].(*ast.PadraoTreta); !ok {
		t.Errorf("padrao de treta de modulo: %T", e.Casos[4].Values[1])
	}
	if e.Casos[5].Guarda == nil || e.Casos[5].Guarda.String() != "(a > b)" {
		t.Errorf("guarda nao lida: %v", e.Casos[5].Guarda)
	}
	if p, ok := e.Casos[6].Values[0].(*ast.PadraoNome); !ok || !p.Curinga() {
		t.Errorf("`caso _` e curinga, veio %T", e.Casos[6].Values[0])
	}
	nomes := ast.NomesDoPadrao(e.Casos[3].Values[0])
	if len(nomes) != 2 || nomes[0].Value != "msg" || nomes[1].Value != "b" {
		t.Errorf("nomes do padrao de dicionario: %v", nomes)
	}
}

// `se` em linha propria nao e guarda: e o comeco do corpo (nome comum).
func TestGuardaSoNaMesmaLinha(t *testing.T) {
	prog := parse(t, `escolhe v
caso [a]
    se(1)
acabou_finalmente`)
	e := prog.Statements[0].(*ast.EscolheStatement)
	if e.Casos[0].Guarda != nil {
		t.Fatalf("guarda nao devia existir: %v", e.Casos[0].Guarda)
	}
}

func TestErrosDePadrao(t *testing.T) {
	casos := []struct{ src, trecho string }{
		{"escolhe v\ncaso [x, x]\n    mostra 1\nacabou_finalmente", "duas vezes"},
		{"escolhe v\ncaso [x + 1]\n    mostra 1\nacabou_finalmente", "nao vale dentro de padrao"},
		{"escolhe v\ncaso [...r, x]\n    mostra 1\nacabou_finalmente", "tem que ser o ultimo"},
		{"escolhe v\ncaso {x: 1}\n    mostra 1\nacabou_finalmente", "chave e literal"},
		{"escolhe v\ncaso Ponto{x: 0, 1}\n    mostra 1\nacabou_finalmente", "nao mistura"},
		{"escolhe v\ncaso Ponto{x: 0, _}\n    mostra 1\nacabou_finalmente", "nao mistura"},
		{"cardapio cor\n    a\nacabou_finalmente", "maiuscula"},
		{"cardapio Cor\nacabou_finalmente", "vazio"},
		{"cardapio Cor\n    a\n    a\nacabou_finalmente", "ja tem a"},
		{"cardapio Cor\n    1\nacabou_finalmente", "nome de uma opcao"},
		{"gambiarra f()\n    cardapio Cor\n        a\n    acabou_finalmente\nacabou_finalmente", "no topo"},
	}
	for _, c := range casos {
		errs := errosDe(c.src)
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, "\n"), c.trecho) {
			t.Errorf("em\n%s\nesperava erro com %q, veio %v", c.src, c.trecho, errs)
		}
	}
}

func TestParseCardapio(t *testing.T) {
	prog := parse(t, `cardapio Cor
    vermelho
    verde
acabou_finalmente
mostra Cor.verde`)
	d, ok := prog.Statements[0].(*ast.CardapioDecl)
	if !ok {
		t.Fatalf("esperava *ast.CardapioDecl, veio %T", prog.Statements[0])
	}
	if d.Nome.Value != "Cor" || strings.Join(d.NomesMembros(), ",") != "vermelho,verde" {
		t.Fatalf("cardapio errado: %s", d.String())
	}
	if len(prog.Statements) != 2 {
		t.Fatalf("esperava 2 statements, veio %d", len(prog.Statements))
	}
}
