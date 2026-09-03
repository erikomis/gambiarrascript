package vm

import (
	"bytes"
	"strings"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// OpBinConst funde `OpConstant K` + operacao binaria. Como ele passa a ser o
// caminho da MAIORIA das operacoes com literal (i + 1, i < N, i * 2), qualquer
// divergencia dele com o tree-walker vira bug em todo programa. Este teste
// compara os dois engines operador por operador, incluindo os casos de erro.
func TestBinConstBateComTreeWalker(t *testing.T) {
	casos := []string{
		// aritmetica
		"bota i = 7\nmostra i + 1", "bota i = 7\nmostra i - 1",
		"bota i = 7\nmostra i * 2", "bota i = 7\nmostra i / 2",
		"bota i = 7\nmostra i % 3", "bota i = 7\nmostra i + 0.5",
		// comparacao
		"bota i = 7\nmostra i < 8", "bota i = 7\nmostra i <= 7",
		"bota i = 7\nmostra i > 8", "bota i = 7\nmostra i >= 7",
		"bota i = 7\nmostra i == 7", "bota i = 7\nmostra i != 7",
		// bitwise
		"bota i = 7\nmostra i & 3", "bota i = 7\nmostra i | 8",
		"bota i = 7\nmostra i ^ 1", "bota i = 7\nmostra i << 2",
		"bota i = 7\nmostra i >> 1",
		// texto
		`bota t = "oi"` + "\n" + `mostra t + " tropa"`,
		`bota t = "oi"` + "\n" + `mostra t == "oi"`,
		`bota t = "oi"` + "\n" + `mostra t != "oi"`,
		// inteiro exato acima de 2^53 nao pode virar float
		"mostra 9007199254740993 + 1",
		// lado esquerdo literal NAO funde: tem que dar o mesmo resultado
		"bota i = 7\nmostra 1 + i", "bota i = 7\nmostra 10 - i",
		// encadeado: o resultado do primeiro vira operando do segundo
		"bota i = 7\nmostra i + 1 * 2", "bota i = 7\nmostra (i + 1) * 2",
		// dentro de laco e de condicional
		"bota s = 0\nbota i = 0\nenquanto i < 5\n    bota s = s + i * 2\n    bota i = i + 1\nacabou_finalmente\nmostra s",
		"bota i = 3\nse_colar i > 2\n    mostra \"maior\"\nse_nao_colar\n    mostra \"menor\"\nacabou_finalmente",
	}
	for _, src := range casos {
		naVM := strings.TrimSpace(rodaFonte(t, src))
		tree := strings.TrimSpace(rodaNoTree(t, src))
		if naVM != tree {
			t.Errorf("divergiu em %q:\n  VM:   %q\n  tree: %q", src, naVM, tree)
		}
	}
}

// TestBinConstErrosMantemALinha: a superinstrucao ocupa uma posicao so no
// bytecode, entao a tabela pc->linha precisa continuar apontando certo — senao
// o erro sai com a linha errada.
func TestBinConstErrosMantemALinha(t *testing.T) {
	casos := []struct{ src, contem string }{
		{"bota t = \"oi\"\n\n\nmostra t - 1", "linha 4"},
		{"bota i = 1\n\nmostra i / 0", "linha 3"},
		{"bota t = \"oi\"\n\n\n\nmostra t > 1", "linha 5"},
	}
	for _, c := range casos {
		got := rodaFonteComErro(t, c.src)
		if !strings.Contains(got, c.contem) {
			t.Errorf("erro de %q deu %q, esperava conter %q", c.src, got, c.contem)
		}
		if tree := strings.TrimSpace(rodaNoTreeComErro(t, c.src)); tree != strings.TrimSpace(got) {
			t.Errorf("mensagem divergiu de %q:\n  VM:   %q\n  tree: %q", c.src, got, tree)
		}
	}
}

// rodaFonteComErro roda na VM esperando erro de runtime, devolvendo a mensagem.
func rodaFonteComErro(t *testing.T, src string) string {
	t.Helper()
	prog := parser.New(lexer.New(src)).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	var buf bytes.Buffer
	maq := New(comp.Bytecode(), &buf)
	err := maq.Run()
	if err == nil {
		t.Fatalf("esperava erro em %q", src)
	}
	if e := ErroDoRun(err); e != nil {
		return e.Message
	}
	return err.Error()
}

// rodaNoTreeComErro faz o mesmo no tree-walker, pra comparar a mensagem.
func rodaNoTreeComErro(t *testing.T, src string) string {
	t.Helper()
	prog := parser.New(lexer.New(src)).ParseProgram()
	var buf bytes.Buffer
	interp := interpreter.New(&buf)
	res := interp.Eval(prog, object.NewEnvironment())
	if res == nil || res.Type() != object.ERRO_OBJ {
		t.Fatalf("tree-walker nao deu erro em %q", src)
	}
	return res.(*object.Erro).Message
}
