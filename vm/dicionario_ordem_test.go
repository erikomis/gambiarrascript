package vm

import (
	"bytes"
	"strings"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

// rodaFonte compila e roda a fonte na VM, devolvendo o que foi impresso.
func rodaFonte(t *testing.T, src string) string {
	t.Helper()
	prog := parser.New(lexer.New(src)).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	var buf bytes.Buffer
	maq := New(comp.Bytecode(), &buf)
	if err := maq.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	return buf.String()
}

// TestDicionarioOrdemInsercao: iterar um dicionario tem que sair na ordem em
// que as chaves entraram, igual Python/JS. Iterar o map do Go direto dava uma
// ordem diferente a CADA execucao.
func TestDicionarioOrdemInsercao(t *testing.T) {
	src := `bota d = {"z": 1, "a": 2, "m": 3, "b": 4, "y": 5, "c": 6, "x": 7}
pra_cada k em d
    mostra k
acabou_finalmente`
	querido := "z\na\nm\nb\ny\nc\nx\n"
	for i := 0; i < 30; i++ {
		if got := rodaFonte(t, src); got != querido {
			t.Fatalf("rodada %d: ordem de iteracao saiu %q, queria %q", i, got, querido)
		}
	}
}

// TestDicionarioOrdemDepoisDeMexer: chave nova vai pro fim; sobrescrever chave
// existente NAO muda o lugar dela.
func TestDicionarioOrdemDepoisDeMexer(t *testing.T) {
	src := `bota d = {"a": 1, "b": 2}
bota d["c"] = 3
bota d["a"] = 99
mostra chaves(d)`
	for i := 0; i < 20; i++ {
		got := strings.TrimSpace(rodaFonte(t, src))
		if got != "[a, b, c]" {
			t.Fatalf("rodada %d: chaves saiu %q, queria %q", i, got, "[a, b, c]")
		}
	}
}

// TestDicionarioInspectOrdenado: mostrar o dicionario tambem tem que ser
// estavel entre execucoes.
func TestDicionarioInspectOrdenado(t *testing.T) {
	src := `mostra {"nome": "tropa", "n": 42, "ok": deu_bom, "z": nada}`
	querido := `{"nome": "tropa", "n": 42, "ok": deu_bom, "z": nada}`
	for i := 0; i < 20; i++ {
		if got := strings.TrimSpace(rodaFonte(t, src)); got != querido {
			t.Fatalf("rodada %d: Inspect saiu %q, queria %q", i, got, querido)
		}
	}
}
