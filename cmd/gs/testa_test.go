package main

import "testing"

func parseOK(t *testing.T, args ...string) opcoesTesta {
	t.Helper()
	op, err := parseArgsTesta(args)
	if err != nil {
		t.Fatalf("parseArgsTesta(%v): %v", args, err)
	}
	return op
}

func TestParseArgsTesta(t *testing.T) {
	op := parseOK(t, "--vm", "-so", "soma", "./testes")
	if !op.usarVM {
		t.Fatalf("--vm nao reconhecido")
	}
	if op.filtro != "soma" {
		t.Fatalf("filtro errado: %q", op.filtro)
	}
	if op.dir != "./testes" {
		t.Fatalf("dir errado: %q", op.dir)
	}
	if op.cobertura {
		t.Fatalf("cobertura ligou sozinha")
	}
}

func TestParseArgsTestaDefaults(t *testing.T) {
	// sem flag = VM: a suite tem que validar o engine que roda em producao.
	op := parseOK(t)
	if op.dir != "." || !op.usarVM || op.filtro != "" || op.cobertura {
		t.Fatalf("defaults errados: %+v", op)
	}
}

func TestParseArgsTestaTree(t *testing.T) {
	if parseOK(t, "--tree").usarVM {
		t.Fatalf("--tree devia voltar pro tree-walker")
	}
}

func TestParseArgsTestaCobertura(t *testing.T) {
	op := parseOK(t, "--cobertura", "pasta")
	if !op.cobertura || op.dir != "pasta" || op.perfil != "" || op.html != "" {
		t.Fatalf("--cobertura: %+v", op)
	}
	// perfil e html ligam a cobertura sozinhos, com espaco ou com =
	op = parseOK(t, "--cobertura-perfil", "c.out", "--cobertura-html=c.html", "--tree")
	if !op.cobertura || op.perfil != "c.out" || op.html != "c.html" || op.usarVM || op.dir != "." {
		t.Fatalf("perfil/html: %+v", op)
	}
	if _, err := parseArgsTesta([]string{"--cobertura-perfil"}); err == nil {
		t.Fatalf("--cobertura-perfil sem arquivo devia dar erro")
	}
	if _, err := parseArgsTesta([]string{"--cobertora"}); err == nil {
		t.Fatalf("flag desconhecida devia dar erro")
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
