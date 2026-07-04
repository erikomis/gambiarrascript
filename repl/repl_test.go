package repl

import (
	"bytes"
	"strings"
	"testing"
)

func TestStartAvaliaLinhas(t *testing.T) {
	entrada := strings.NewReader("bota x = 21\nmostra x * 2\n")
	var out bytes.Buffer
	Start(entrada, &out)
	if !strings.Contains(out.String(), "42") {
		t.Fatalf("REPL nao avaliou as linhas mantendo estado; saida: %q", out.String())
	}
}

func TestStartMultiline(t *testing.T) {
	// bloco aberto atravessa linhas: so avalia no acabou_finalmente
	entrada := strings.NewReader("gambiarra dobra(n)\nfunciona n * 2\nacabou_finalmente\ndobra(21)\n")
	var out bytes.Buffer
	Start(entrada, &out)
	if !strings.Contains(out.String(), "=> 42") {
		t.Fatalf("REPL multiline nao funcionou; saida: %q", out.String())
	}
	if !strings.Contains(out.String(), promptCont) {
		t.Fatalf("cade o prompt de continuacao? saida: %q", out.String())
	}
}

func TestStartMultilineElif(t *testing.T) {
	// `se_nao_colar se_colar` (elif) compartilha o MESMO acabou_finalmente —
	// o contador de blocos nao pode contar o se_colar do elif como abertura.
	entrada := strings.NewReader("bota x = 5\nse_colar x > 10\nmostra \"grande\"\nse_nao_colar se_colar x > 3\nmostra \"medio\"\nacabou_finalmente\n")
	var out bytes.Buffer
	Start(entrada, &out)
	if !strings.Contains(out.String(), "medio") {
		t.Fatalf("elif multiline nao avaliou; saida: %q", out.String())
	}
}
