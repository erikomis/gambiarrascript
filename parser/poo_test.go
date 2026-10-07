package parser

import (
	"strings"
	"testing"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
)

func errosDe(src string) []string {
	p := New(lexer.New(src))
	p.ParseProgram()
	return p.Errors()
}

func TestParseTreta(t *testing.T) {
	prog := parse(t, `treta Cachorro
    Animal
    geo.Ponto
    raca
    patas = 2 + 2
acabou_finalmente`)
	d, ok := prog.Statements[0].(*ast.TretaDecl)
	if !ok {
		t.Fatalf("esperava *ast.TretaDecl, veio %T", prog.Statements[0])
	}
	if d.Nome.Value != "Cachorro" || len(d.Campos) != 4 {
		t.Fatalf("treta errada: %s", d.String())
	}
	if d.Campos[0].Embutida == nil || d.Campos[0].Nome.Value != "Animal" {
		t.Fatalf("puxadinho Animal nao reconhecido: %+v", d.Campos[0])
	}
	if d.Campos[1].Embutida == nil || d.Campos[1].Nome.Value != "Ponto" {
		t.Fatalf("puxadinho geo.Ponto nao reconhecido: %+v", d.Campos[1])
	}
	if d.Campos[2].Embutida != nil || d.Campos[2].Padrao != nil {
		t.Fatalf("raca e campo simples: %+v", d.Campos[2])
	}
	if d.Campos[3].Padrao == nil || d.Campos[3].Padrao.String() != "(2 + 2)" {
		t.Fatalf("padrao errado: %+v", d.Campos[3])
	}
}

func TestParseCombinadoEMetodo(t *testing.T) {
	prog := parse(t, `combinado Forma
    area()
    escala(fator, centro)
    Nomeavel
acabou_finalmente
gambiarra (q Quadrado) area()
    funciona q.lado * q.lado
acabou_finalmente
bota f = gambiarra(x) funciona x acabou_finalmente
bota g = gambiarra (x, y) funciona x acabou_finalmente`)
	c, ok := prog.Statements[0].(*ast.CombinadoDecl)
	if !ok || len(c.Metodos) != 3 {
		t.Fatalf("combinado errado: %T", prog.Statements[0])
	}
	if len(c.Metodos[1].Parametros) != 2 || c.Metodos[2].Embutido == nil {
		t.Fatalf("assinaturas erradas: %s", c.String())
	}
	m, ok := prog.Statements[1].(*ast.MetodoDecl)
	if !ok || m.Receptor.Value != "q" || m.Tipo.Value != "Quadrado" || m.Nome.Value != "area" {
		t.Fatalf("metodo errado: %T %v", prog.Statements[1], prog.Statements[1])
	}
	// lambda continua lambda (com e sem espaco antes do parentese)
	for _, i := range []int{2, 3} {
		b := prog.Statements[i].(*ast.BotaStatement)
		if _, ok := b.Value.(*ast.FuncaoLiteral); !ok {
			t.Fatalf("lambda virou outra coisa: %T", b.Value)
		}
	}
}

func TestParseTretaLiteral(t *testing.T) {
	casos := []struct {
		src, quer string
	}{
		{`bota p = Ponto{x: 1, y: 2}`, "Ponto{x: 1, y: 2}"},
		{`bota p = Ponto{1, 2}`, "Ponto{1, 2}"},
		{`bota p = Ponto{}`, "Ponto{}"},
		{`bota p = geo.Ponto{x: 1}`, "(geo.Ponto){x: 1}"},
		{"bota p = Ponto{\n    x: 1,\n    y: 2,\n}", "Ponto{x: 1, y: 2}"},
		{`bota p = Ponto{1, 2}.soma()`, "(Ponto{1, 2}.soma)()"},
		{`bota ok = a == Ponto{1, 2}`, "(a == Ponto{1, 2})"},
	}
	for _, c := range casos {
		prog := parse(t, c.src)
		b := prog.Statements[0].(*ast.BotaStatement)
		if got := b.Value.String(); got != c.quer {
			t.Errorf("%s\n  veio %q, queria %q", c.src, got, c.quer)
		}
	}
}

// Sem maiuscula ou com o `{` na linha de baixo nao e literal: o dicionario
// fica sendo statement solto, como sempre foi.
func TestParseTretaLiteralNaoGruda(t *testing.T) {
	prog := parse(t, "mostra x {\"a\": 1}")
	if len(prog.Statements) != 2 {
		t.Fatalf("minusculo nao e literal: %d statements", len(prog.Statements))
	}
	prog = parse(t, "mostra LIMITE\n{\"a\": 1}")
	if len(prog.Statements) != 2 {
		t.Fatalf("`{` na outra linha nao e literal: %d statements", len(prog.Statements))
	}
	if _, ok := prog.Statements[1].(*ast.ExpressionStatement).Expression.(*ast.DicionarioLiteral); !ok {
		t.Fatalf("a segunda linha devia ser dicionario")
	}
}

func TestParsePOOErros(t *testing.T) {
	casos := []struct{ src, erro string }{
		{"treta ponto\n    x\nacabou_finalmente", "nome de treta comeca com maiuscula (tipo Ponto)"},
		{"combinado forma\nacabou_finalmente", "nome de combinado comeca com maiuscula"},
		{"treta Ponto\n    x\n", "cade o acabou_finalmente da treta Ponto?"},
		{"treta Ponto\n    1\nacabou_finalmente", "na treta Ponto eu esperava um campo"},
		{"treta A\n    A\nacabou_finalmente", "a treta A nao pode ser puxadinho dela mesma"},
		{"combinado F\n    area\nacabou_finalmente", "no combinado o metodo vem com os parenteses: area(...)"},
		{"gambiarra f()\n    treta A\n    acabou_finalmente\nacabou_finalmente", "treta so pode ser declarada no topo do arquivo"},
		{"se_colar deu_bom\n    gambiarra (p P) m()\n    acabou_finalmente\nacabou_finalmente", "gambiarra com receiver (metodo) so pode ser declarada no topo"},
		{"gambiarra (p P) m(p)\nacabou_finalmente", "o parametro p tem o mesmo nome do receiver"},
		{"gambiarra (p P) m()\n    mostra 1\n", "cade o acabou_finalmente do metodo m?"},
		{"bota p = Ponto{x: 1, 2}", "ou vai tudo com nome (x: 1) ou tudo na ordem (1, 2), nao mistura"},
		{"bota p = Ponto{1, x: 2}", "ou vai tudo com nome (x: 1) ou tudo na ordem (1, 2), nao mistura"},
	}
	for _, c := range casos {
		errs := errosDe(c.src)
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, "; "), c.erro) {
			t.Errorf("%q\n  queria erro com %q, veio %v", c.src, c.erro, errs)
		}
	}
}
