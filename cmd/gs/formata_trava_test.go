package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gambiarrascript/formatter"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

const fonteComComentario = "# explica a conta\nbota x = 1   # inline\n\n/* bloco */\nmostra x\n"

func arquivoTemp(t *testing.T, conteudo string) string {
	t.Helper()
	arq := filepath.Join(t.TempDir(), "x.gs")
	if err := os.WriteFile(arq, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
	return arq
}

func TestFormataEscreveGuardaComentarios(t *testing.T) {
	arq := arquivoTemp(t, fonteComComentario)
	mudou, err := escreveFormatado(arq, formatter.FormataFonte)
	if err != nil || !mudou {
		t.Fatalf("devia formatar: mudou=%v err=%v", mudou, err)
	}
	b, _ := os.ReadFile(arq)
	if want := "# explica a conta\nbota x = 1  # inline\n\n/* bloco */\nmostra x\n"; string(b) != want {
		t.Fatalf("arquivo ficou:\n%s", b)
	}
	// de novo: nada muda
	if mudou, err := escreveFormatado(arq, formatter.FormataFonte); err != nil || mudou {
		t.Fatalf("segunda vez nao devia mudar: mudou=%v err=%v", mudou, err)
	}
}

// formatter com bug (o antigo, que reimprime so o AST e perde comentario):
// a trava recusa e o arquivo fica intacto.
func TestFormataEscreveTravaNaoDestroi(t *testing.T) {
	soAST := func(src string) (string, []string) {
		p := parser.New(lexer.New(src))
		prog := p.ParseProgram()
		return formatter.Formata(prog), p.Errors()
	}
	arq := arquivoTemp(t, fonteComComentario)
	mudou, err := escreveFormatado(arq, soAST)
	if err == nil || mudou {
		t.Fatalf("trava devia recusar: mudou=%v err=%v", mudou, err)
	}
	if !strings.Contains(err.Error(), "comentario") {
		t.Fatalf("erro devia falar dos comentarios: %v", err)
	}
	if b, _ := os.ReadFile(arq); string(b) != fonteComComentario {
		t.Fatalf("arquivo foi alterado mesmo com a trava:\n%s", b)
	}

	// formatter que muda o codigo: tambem recusa
	mudaCodigo := func(src string) (string, []string) { return "bota x = 2\n", nil }
	if _, err := escreveFormatado(arq, mudaCodigo); err == nil {
		t.Fatal("trava devia recusar AST diferente")
	}
	if b, _ := os.ReadFile(arq); string(b) != fonteComComentario {
		t.Fatalf("arquivo foi alterado mesmo com a trava:\n%s", b)
	}
}

func TestFormataEscreveErroDeParse(t *testing.T) {
	arq := arquivoTemp(t, "bota = 5\n")
	_, err := escreveFormatado(arq, formatter.FormataFonte)
	if _, ok := err.(errParse); !ok {
		t.Fatalf("esperava errParse, veio %v", err)
	}
}
