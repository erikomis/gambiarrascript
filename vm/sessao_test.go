package vm

import (
	"bytes"
	"testing"

	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// entrada compila e roda uma linha na sessao, devolvendo (valor mostrado, erro,
// saida impressa).
func entrada(t *testing.T, s *Sessao, buf *bytes.Buffer, src string) (string, string, string) {
	t.Helper()
	buf.Reset()
	prog := parser.New(lexer.New(src)).ParseProgram()
	val, err := s.Avalia(prog)
	msgErr := ""
	if err != nil {
		msgErr = err.Error()
		if e := ErroDoRun(err); e != nil {
			msgErr = e.Message
		}
	}
	// mesmo filtro do REPL: `nada` nao vira `=> nada` na tela (comando como
	// `ordena(xs)` devolve nada e nao tem o que mostrar).
	txt := ""
	if val != nil && val.Type() != object.NADA_OBJ {
		txt = val.Inspect()
	}
	return txt, msgErr, buf.String()
}

// TestSessaoMantemEstado: o ponto todo da sessao — o que uma entrada define, a
// proxima enxerga.
func TestSessaoMantemEstado(t *testing.T) {
	var buf bytes.Buffer
	s := NovaSessao(&buf)

	if _, err, _ := entrada(t, s, &buf, "bota x = 10"); err != "" {
		t.Fatalf("bota: %s", err)
	}
	if v, err, _ := entrada(t, s, &buf, "x + 5"); err != "" || v != "15" {
		t.Fatalf("x + 5 deu (%q, %q), queria 15", v, err)
	}
	// gambiarra definida numa entrada tem que ser chamavel na seguinte
	if _, err, _ := entrada(t, s, &buf, "gambiarra dobra(n)\n    funciona n * 2\nacabou_finalmente"); err != "" {
		t.Fatalf("gambiarra: %s", err)
	}
	if v, _, _ := entrada(t, s, &buf, "dobra(x)"); v != "20" {
		t.Fatalf("dobra(x) deu %q, queria 20", v)
	}
	// e a global tem que continuar mutavel
	if _, err, _ := entrada(t, s, &buf, "bota x = x + 1"); err != "" {
		t.Fatalf("reatribuir: %s", err)
	}
	if v, _, _ := entrada(t, s, &buf, "x"); v != "11" {
		t.Fatalf("x depois de mexer deu %q, queria 11", v)
	}
}

// TestSessaoSoMostraValorDeExpressao: comando nao tem valor pra imprimir; o
// REPL nao pode inventar um `=>` depois de um `bota` ou `mostra`.
func TestSessaoSoMostraValorDeExpressao(t *testing.T) {
	var buf bytes.Buffer
	s := NovaSessao(&buf)
	casos := []struct {
		src      string
		temValor bool
	}{
		{"bota a = 1", false},
		{"mostra 1", false},
		{"se_colar deu_bom\n    bota b = 2\nacabou_finalmente", false},
		{"1 + 1", true},
		{`"texto"`, true},
		{"a", true},
	}
	for _, c := range casos {
		v, err, _ := entrada(t, s, &buf, c.src)
		if err != "" {
			t.Fatalf("%q: %s", c.src, err)
		}
		if (v != "") != c.temValor {
			t.Errorf("%q devolveu %q (temValor=%v), esperava temValor=%v", c.src, v, v != "", c.temValor)
		}
	}
}

// TestSessaoSobreviveAErro: erro numa entrada nao pode derrubar a sessao nem
// perder o que ja foi definido.
func TestSessaoSobreviveAErro(t *testing.T) {
	var buf bytes.Buffer
	s := NovaSessao(&buf)
	entrada(t, s, &buf, "bota x = 3")

	// erro de compilacao (nome que nao existe)
	if _, err, _ := entrada(t, s, &buf, "naoexiste + 1"); err == "" {
		t.Fatal("esperava erro de compilacao pra nome desconhecido")
	}
	if v, _, _ := entrada(t, s, &buf, "x"); v != "3" {
		t.Fatalf("x sumiu depois de erro de compilacao: %q", v)
	}

	// erro de runtime (divisao por zero)
	if _, err, _ := entrada(t, s, &buf, "x / 0"); err == "" {
		t.Fatal("esperava erro de runtime")
	}
	if v, _, _ := entrada(t, s, &buf, "x + 1"); v != "4" {
		t.Fatalf("sessao quebrou depois do erro de runtime: %q", v)
	}

	// global declarada numa entrada que quebrou vale `nada`, nao nil
	entrada(t, s, &buf, "bota y = 1 / 0")
	// vale `nada` (nao nil, que derrubaria a VM): o filtro do REPL faz o valor
	// sair vazio, e o importante e nao quebrar.
	if v, err, _ := entrada(t, s, &buf, "y"); err != "" || v != "" {
		t.Fatalf("y depois de entrada que quebrou deu (%q, %q), queria vazio sem erro", v, err)
	}
}

// TestSessaoBateComTreeWalker: mesma sequencia de entradas nos dois engines.
func TestSessaoBateComTreeWalker(t *testing.T) {
	entradas := []string{
		"bota xs = [3, 1, 2]",
		"ordena(xs)",
		"bota d = {\"b\": 1, \"a\": 2}",
		"chaves(d)",
		"gambiarra soma_ate(n)\n    bota t = 0\n    bota i = 1\n    enquanto i <= n\n        bota t = t + i\n        bota i = i + 1\n    acabou_finalmente\n    funciona t\nacabou_finalmente",
		"soma_ate(100)",
		"mapeia([1,2,3], gambiarra(v) funciona v * 10 acabou_finalmente)",
		"pra_json(d)",
	}

	var bufVM bytes.Buffer
	s := NovaSessao(&bufVM)
	var naVM []string
	for _, e := range entradas {
		v, err, saida := entrada(t, s, &bufVM, e)
		if err != "" {
			t.Fatalf("VM em %q: %s", e, err)
		}
		naVM = append(naVM, saida+v)
	}

	// tree-walker: mesmo env entre entradas
	var bufTree bytes.Buffer
	interp := interpreter.New(&bufTree)
	env := object.NewEnvironment()
	var noTree []string
	for _, e := range entradas {
		bufTree.Reset()
		prog := parser.New(lexer.New(e)).ParseProgram()
		res := interp.Eval(prog, env)
		txt := ""
		if res != nil && res.Type() != object.NADA_OBJ {
			if res.Type() == object.ERRO_OBJ {
				t.Fatalf("tree em %q: %s", e, res.Inspect())
			}
			txt = res.Inspect()
		}
		noTree = append(noTree, bufTree.String()+txt)
	}

	for i := range entradas {
		if naVM[i] != noTree[i] {
			t.Errorf("entrada %q divergiu:\n  VM:   %q\n  tree: %q", entradas[i], naVM[i], noTree[i])
		}
	}
}
