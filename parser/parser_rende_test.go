package parser

import (
	"strings"
	"testing"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
)

func parseSemErro(t *testing.T, fonte string) *ast.Program {
	t.Helper()
	p := New(lexer.New(fonte))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("erros de parse em %q: %v", fonte, errs)
	}
	return prog
}

// O `rende` marca a gambiarra de cima como gerador: nomeada, lambda e metodo.
func TestParseRendeMarcaGerador(t *testing.T) {
	prog := parseSemErro(t, `gambiarra conta(n)
    pra_cada i de 1 ate n
        rende i * 2
    acabou_finalmente
acabou_finalmente
gambiarra comum()
    funciona 1
acabou_finalmente
bota g = gambiarra()
    rende "oi"
acabou_finalmente
treta Caixa
    itens
acabou_finalmente
gambiarra (c Caixa) itera()
    pra_cada x em c.itens
        rende x
    acabou_finalmente
acabou_finalmente`)
	if g := prog.Statements[0].(*ast.GambiarraStatement); !g.Gerador {
		t.Fatalf("conta devia ser gerador")
	}
	if g := prog.Statements[1].(*ast.GambiarraStatement); g.Gerador {
		t.Fatalf("comum nao devia ser gerador")
	}
	lam := prog.Statements[2].(*ast.BotaStatement).Value.(*ast.FuncaoLiteral)
	if !lam.Gerador {
		t.Fatalf("lambda com rende devia ser gerador")
	}
	if m := prog.Statements[4].(*ast.MetodoDecl); !m.Gerador {
		t.Fatalf("metodo com rende devia ser gerador")
	}
	r := prog.Statements[0].(*ast.GambiarraStatement).Body.Statements[0].(*ast.PraCadaNumStatement).Body.Statements[0]
	if r.String() != "rende (i * 2)" {
		t.Fatalf("String do rende: %q", r.String())
	}
}

// O `rende` de uma lambda de dentro e da lambda: a de fora continua comum.
func TestParseRendeDaLambdaDeDentro(t *testing.T) {
	prog := parseSemErro(t, `gambiarra fabrica()
    funciona gambiarra()
        rende 1
    acabou_finalmente
acabou_finalmente`)
	f := prog.Statements[0].(*ast.GambiarraStatement)
	if f.Gerador {
		t.Fatalf("fabrica nao rende nada: nao e gerador")
	}
	lam := f.Body.Statements[0].(*ast.FuncionaStatement).Value.(*ast.FuncaoLiteral)
	if !lam.Gerador {
		t.Fatalf("a lambda de dentro devia ser gerador")
	}
}

func TestParseRendeForaDeGambiarra(t *testing.T) {
	for _, fonte := range []string{
		"rende 1",
		"pra_cada x em [1]\n    rende x\nacabou_finalmente",
		"treta T\n    n numero = 1\nacabou_finalmente\nrende 2",
	} {
		p := New(lexer.New(fonte))
		p.ParseProgram()
		errs := p.Errors()
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, "\n"), "rende fora de gambiarra") {
			t.Fatalf("%q: esperava erro de rende fora de gambiarra, veio %v", fonte, errs)
		}
	}
}

func TestParseRendeSemValor(t *testing.T) {
	p := New(lexer.New("gambiarra g()\n    rende\nacabou_finalmente"))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("rende sem valor devia dar erro de parse")
	}
}
