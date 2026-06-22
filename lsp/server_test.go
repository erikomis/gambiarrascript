package lsp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDiagnosticosComErro(t *testing.T) {
	var out bytes.Buffer
	s := NovoServidor(&out)
	s.docs["file:///x.gs"] = "bota = 5" // falta o identificador depois de bota
	s.PublicarDiagnosticos("file:///x.gs")

	got := out.String()
	if !strings.Contains(got, "textDocument/publishDiagnostics") {
		t.Fatalf("esperava publishDiagnostics, got %q", got)
	}
	if strings.Contains(got, `"diagnostics":[]`) {
		t.Fatalf("esperava ao menos um diagnostico, got %q", got)
	}
}

func TestDiagnosticosSemErro(t *testing.T) {
	var out bytes.Buffer
	s := NovoServidor(&out)
	s.docs["file:///ok.gs"] = `mostra "tudo certo"`
	s.PublicarDiagnosticos("file:///ok.gs")

	got := out.String()
	if !strings.Contains(got, `"diagnostics":[]`) {
		t.Fatalf("codigo valido deveria ter zero diagnosticos, got %q", got)
	}
}

func TestShutdownRespondeResultNull(t *testing.T) {
	var out bytes.Buffer
	s := NovoServidor(&out)
	id := json.RawMessage(`1`)
	s.tratar(&Mensagem{Method: "shutdown", ID: &id})
	if !strings.Contains(out.String(), `"result":null`) {
		t.Fatalf("shutdown deveria responder result:null, got %q", out.String())
	}
}

func TestCompletionTemKeywords(t *testing.T) {
	s := NovoServidor(&bytes.Buffer{})
	itens := s.itensCompletion("bota contador = 0")
	temBota, temContador := false, false
	for _, it := range itens {
		if it.Label == "bota" {
			temBota = true
		}
		if it.Label == "contador" {
			temContador = true
		}
	}
	if !temBota {
		t.Error("completion deveria conter a keyword 'bota'")
	}
	if !temContador {
		t.Error("completion deveria conter o identificador 'contador' visto no texto")
	}
}

func TestCompletionTemBuiltin(t *testing.T) {
	s := NovoServidor(&bytes.Buffer{})
	itens := s.itensCompletion("")
	for _, it := range itens {
		if it.Label == "tamanho" {
			return
		}
	}
	t.Error("completion deveria conter o builtin 'tamanho'")
}
