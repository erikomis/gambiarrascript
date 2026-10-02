package ast_test

import (
	"strings"
	"testing"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

func checa(t *testing.T, src string) []ast.ErroCrava {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse: %v", errs)
	}
	return ast.ChecaCravadas(prog, nil)
}

// TestChecaCravadasPega: tudo que reatribui um nome cravado no mesmo escopo.
func TestChecaCravadasPega(t *testing.T) {
	casos := []struct {
		src, msg string
		linha    int
	}{
		{"crava PI = 3\nbota PI = 4", "`PI` foi cravada", 2},
		{"crava PI = 3\nPI += 1", "`PI` foi cravada", 2},
		{"crava PI = 3\nPI **= 2", "`PI` foi cravada", 2},
		{"crava PI = 3\ncrava PI = 4", "`PI` ja foi cravada", 2},
		{"crava A = 1\nbota [A, b] = [1, 2]", "`A` foi cravada", 2},
		{"crava I = 1\npra_cada I de 1 ate 3\nacabou_finalmente", "`I` foi cravada", 2},
		{"crava V = 1\npra_cada k, V em {\"a\": 1}\nacabou_finalmente", "`V` foi cravada", 2},
		{"crava F = 1\ngambiarra F()\n    funciona 1\nacabou_finalmente", "`F` foi cravada", 2},
		{"crava E = 1\narruma\n    quebra(\"x\")\nquebrou E\n    mostra E\nacabou_finalmente", "`E` foi cravada", 4},
		// bloco nao abre escopo: o se_colar divide o escopo de fora
		{"crava N = 1\nse_colar deu_bom\n    bota N = 2\nacabou_finalmente", "`N` foi cravada", 3},
		// dentro da gambiarra vale o escopo dela
		{"gambiarra f()\n    crava L = 1\n    bota L = 2\nacabou_finalmente", "`L` foi cravada", 3},
		// e dentro de lambda tambem
		{"bota g = gambiarra()\n    crava L = 1\n    L += 1\nacabou_finalmente", "`L` foi cravada", 3},
	}
	for _, c := range casos {
		errs := checa(t, c.src)
		if len(errs) == 0 {
			t.Errorf("%q: esperava erro de crava", c.src)
			continue
		}
		if !strings.Contains(errs[0].Msg, c.msg) || errs[0].Linha != c.linha {
			t.Errorf("%q: got linha %d %q, queria linha %d com %q", c.src, errs[0].Linha, errs[0].Msg, c.linha, c.msg)
		}
	}
}

// TestChecaCravadasLibera: o que NAO e reatribuicao do nome cravado.
func TestChecaCravadasLibera(t *testing.T) {
	casos := []string{
		// mexer por dentro de lista/dict cravado pode (igual const do JS)
		"crava XS = [1]\nbota XS[0] = 2\nXS[0] += 1",
		"crava D = {}\nbota D.nome = \"ze\"",
		// gambiarra de dentro: bota cria local que sombreia
		"crava PI = 3\ngambiarra f()\n    bota PI = 4\n    funciona PI\nacabou_finalmente",
		// parametro com o mesmo nome e local da gambiarra
		"crava X = 1\ngambiarra f(X)\n    funciona X\nacabou_finalmente",
		// crava dentro de laco: um texto so, nao reclama na volta seguinte
		"pra_cada i de 1 ate 3\n    crava K = i\nacabou_finalmente",
		// bota antes do crava: so depois do crava o nome fica fixo
		"bota Y = 1\ncrava Y = 2",
		// mesmo nome cravado em gambiarras diferentes
		"gambiarra a()\n    crava T = 1\nacabou_finalmente\ngambiarra b()\n    crava T = 2\nacabou_finalmente",
	}
	for _, src := range casos {
		if errs := checa(t, src); len(errs) != 0 {
			t.Errorf("%q: nao devia dar erro, deu %v", src, errs)
		}
	}
}

// TestChecaCravadasGlobais: o mapa de globais entra (REPL) e sai atualizado.
func TestChecaCravadasGlobais(t *testing.T) {
	globais := map[string]bool{}
	prog := parser.New(lexer.New("crava A = 1")).ParseProgram()
	if errs := ast.ChecaCravadas(prog, globais); len(errs) != 0 {
		t.Fatalf("erro inesperado: %v", errs)
	}
	if !globais["A"] {
		t.Fatalf("A devia sair marcada nas globais: %v", globais)
	}
	prog = parser.New(lexer.New("bota A = 2")).ParseProgram()
	if errs := ast.ChecaCravadas(prog, globais); len(errs) == 0 {
		t.Fatalf("esperava erro mexendo em A cravada numa entrada anterior")
	}
}
