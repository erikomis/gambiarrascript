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
