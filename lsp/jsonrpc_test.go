package lsp

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func TestEscreverELerRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := EscreverMensagem(&buf, &Mensagem{Method: "ping"}); err != nil {
		t.Fatalf("EscreverMensagem: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "Content-Length: ") {
		t.Fatalf("faltou header Content-Length: %q", buf.String())
	}
	m, err := LerMensagem(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("LerMensagem: %v", err)
	}
	if m.Method != "ping" || m.JSONRPC != "2.0" {
		t.Fatalf("round-trip errado: %+v", m)
	}
}

func TestLerMensagemFramed(t *testing.T) {
	corpo := `{"jsonrpc":"2.0","method":"initialize","params":{}}`
	raw := "Content-Length: " + strconv.Itoa(len(corpo)) + "\r\n\r\n" + corpo
	m, err := LerMensagem(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatalf("LerMensagem: %v", err)
	}
	if m.Method != "initialize" {
		t.Fatalf("method: got %q", m.Method)
	}
}
