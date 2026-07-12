package repl

import (
	"strings"
	"testing"
)

func TestPalavraAntes(t *testing.T) {
	p, i := palavraAntes("mostra tam", 10)
	if p != "tam" || i != 7 {
		t.Fatalf("palavraAntes: got %q %d", p, i)
	}
	// pos no meio de espaco: palavra vazia
	p2, _ := palavraAntes("mostra ", 7)
	if p2 != "" {
		t.Fatalf("esperava vazio, got %q", p2)
	}
}

func TestAutocompletaUnico(t *testing.T) {
	nova, pos, ok := autocompleta("mostra tama", 11, []string{"tamanho", "tem"})
	if !ok || nova != "mostra tamanho" || pos != 14 {
		t.Fatalf("autocompleta unico: %q %d %v", nova, pos, ok)
	}
}

func TestAutocompletaPrefixoComum(t *testing.T) {
	// "ca" -> "cano"/"canoa" -> prefixo comum "cano" (avanca ate ali)
	nova, _, ok := autocompleta("ca", 2, []string{"cano", "canoa"})
	if !ok || nova != "cano" {
		t.Fatalf("autocompleta prefixo comum: %q %v", nova, ok)
	}
}

func TestAutocompletaSemMatch(t *testing.T) {
	if _, _, ok := autocompleta("xyz", 3, []string{"tamanho"}); ok {
		t.Fatalf("nao devia completar sem match")
	}
}

func TestTrataComandoAjudaLimpa(t *testing.T) {
	var b strings.Builder
	if !trataComando(":ajuda", &b) {
		t.Fatalf(":ajuda devia ser tratado")
	}
	if !trataComando(":limpa", &b) {
		t.Fatalf(":limpa devia ser tratado")
	}
	if !trataComando(":naoexiste", &b) {
		t.Fatalf(": prefix sempre e tratado (msg de desconhecido)")
	}
}
