package interpreter

import (
	"bytes"
	"strings"
	"testing"

	"gambiarrascript/object"
)

// Reaproveita rodarComStdin(...) ja definido em builtins_novos_test.go:
// rodarComStdin(t, input, stdin *bytes.Buffer) string

func TestLeTudoLeTodosOsStdin(t *testing.T) {
	// "linha um\n" (9) + "linha dois\n" (11) + "linha tres\n" (11) = 31 chars
	out := rodarComStdin(t, `
bota conteudo = le_tudo()
mostra tamanho(conteudo)`, bytes.NewBufferString("linha um\nlinha dois\nlinha tres\n"))
	if !strings.Contains(out, "31") {
		t.Fatalf("esperava contar 31 chars no le_tudo, got %q", out)
	}
}

func TestLeLinhasIteraCadaUma(t *testing.T) {
	out := rodarComStdin(t, `
pra_cada linha em le_linhas()
    escreve("[" + linha + "]")
acabou_finalmente
mostra "fim"`, bytes.NewBufferString("aaa\nbbb\nccc\n"))
	// escreve nun ин põe \n; mostra "fim" adiciona \n no fim.
	if out != "[aaa][bbb][ccc]fim\n" {
		t.Fatalf("saida: %q", out)
	}
}

func TestEscreveEscreveSemQuebra(t *testing.T) {
	out := rodarComStdin(t, `
escreve("ola ")
escreve("mundo")
mostra "!"`, nil)
	if out != "ola mundo!\n" {
		t.Fatalf("saida: %q", out)
	}
}

func TestAnexaArquivo(t *testing.T) {
	dir := t.TempDir()
	caminho := dir + "/log.txt"
	rodarComStdin(t, `
escreve_arquivo("`+caminho+`", "linha1\n")
anexa_arquivo("`+caminho+`", "linha2\n")
anexa_arquivo("`+caminho+`", "linha3\n")`, nil)
	out2 := rodarComStdin(t, `mostra le_arquivo("`+caminho+`")`, nil)
	if out2 != "linha1\nlinha2\nlinha3\n\n" {
		t.Fatalf("conteudo final: %q", out2)
	}
}

func TestEnvVarInexistenteDevolveNada(t *testing.T) {
	interp := New(&bytes.Buffer{})
	res := interp.builtinEnv([]object.Object{&object.Texto{Value: "GAMBIARRA_VARIOS_TESTES_NAO_EXISTE_XYZ"}})
	if res.Type() != object.NADA_OBJ {
		t.Fatalf("esperava NADA, got %s", res.Type())
	}
}
