package main

import "testing"

func TestParseArgsTesta(t *testing.T) {
	dir, usarVM, filtro := parseArgsTesta([]string{"--vm", "-so", "soma", "./testes"})
	if !usarVM {
		t.Fatalf("--vm nao reconhecido")
	}
	if filtro != "soma" {
		t.Fatalf("filtro errado: %q", filtro)
	}
	if dir != "./testes" {
		t.Fatalf("dir errado: %q", dir)
	}
}

func TestParseArgsTestaDefaults(t *testing.T) {
	dir, usarVM, filtro := parseArgsTesta(nil)
	if dir != "." || usarVM || filtro != "" {
		t.Fatalf("defaults errados: dir=%q vm=%v filtro=%q", dir, usarVM, filtro)
	}
}

func TestFiltraTestes(t *testing.T) {
	arqs := []string{"a/soma_test.gs", "a/lista_test.gs", "a/soma_extra_test.gs"}
	got := filtraTestes(arqs, "soma")
	if len(got) != 2 {
		t.Fatalf("filtro soma devia dar 2, veio %d: %v", len(got), got)
	}
	// filtro vazio devolve tudo
	if len(filtraTestes(arqs, "")) != 3 {
		t.Fatalf("filtro vazio devia devolver tudo")
	}
}
