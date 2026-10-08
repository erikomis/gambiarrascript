package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func migraOk(t *testing.T, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := executaMigra(args, &buf); err != nil {
		t.Fatalf("gs migra %v: %v\nsaida: %s", args, err, buf.String())
	}
	return buf.String()
}

func TestGsMigraFluxoCompleto(t *testing.T) {
	dir := t.TempDir()
	pasta := filepath.Join(dir, "migracoes")
	t.Setenv("GS_BANCO", "sqlite:"+filepath.Join(dir, "app.db"))

	out := migraOk(t, "--pasta", pasta, "novo", "cria", "usuarios")
	if !strings.Contains(out, "001_cria_usuarios.sobe.sql") || !strings.Contains(out, "001_cria_usuarios.desce.sql") {
		t.Fatalf("novo: %s", out)
	}
	migraOk(t, "--pasta="+pasta, "novo", "cria_posts")
	escreveArq := func(nome, sql string) {
		if err := os.WriteFile(filepath.Join(pasta, nome), []byte(sql), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escreveArq("001_cria_usuarios.sobe.sql", "CREATE TABLE usuarios (id INTEGER PRIMARY KEY, email TEXT UNIQUE);\nCREATE INDEX usuarios_email ON usuarios (email);\n")
	escreveArq("001_cria_usuarios.desce.sql", "DROP INDEX usuarios_email;\nDROP TABLE usuarios;\n")
	escreveArq("002_cria_posts.sobe.sql", "CREATE TABLE posts (id INTEGER PRIMARY KEY, autor INTEGER REFERENCES usuarios(id));")
	escreveArq("002_cria_posts.desce.sql", "DROP TABLE posts;")

	out = migraOk(t, "--pasta", pasta, "status")
	if !strings.Contains(out, "[ ] 001_cria_usuarios") || !strings.Contains(out, "0 aplicada(s), 2 pendente(s)") {
		t.Fatalf("status antes: %s", out)
	}
	out = migraOk(t, "--pasta", pasta) // sem subcomando = sobe
	if !strings.Contains(out, "aplicada  001_cria_usuarios") || !strings.Contains(out, "aplicada  002_cria_posts") || !strings.Contains(out, "2 migracoes aplicadas") {
		t.Fatalf("sobe: %s", out)
	}
	if out = migraOk(t, "--pasta", pasta, "sobe"); !strings.Contains(out, "banco em dia") {
		t.Fatalf("sobe de novo: %s", out)
	}
	out = migraOk(t, "--pasta", pasta, "status")
	if !strings.Contains(out, "[x] 002_cria_posts") || !strings.Contains(out, "2 aplicada(s), 0 pendente(s)") {
		t.Fatalf("status depois: %s", out)
	}
	if out = migraOk(t, "--pasta", pasta, "desce", "2"); !strings.Contains(out, "desfeita  002_cria_posts\n  desfeita  001_cria_usuarios") {
		t.Fatalf("desce 2: %s", out)
	}
	if out = migraOk(t, "--pasta", pasta, "desce"); !strings.Contains(out, "nada pra desfazer") {
		t.Fatalf("desce sem nada: %s", out)
	}

	// arquivo aplicado mudou: sobe recusa, status aponta
	migraOk(t, "--pasta", pasta, "--banco", os.Getenv("GS_BANCO"))
	escreveArq("001_cria_usuarios.sobe.sql", "CREATE TABLE usuarios (id INTEGER);")
	var buf bytes.Buffer
	err := executaMigra([]string{"--pasta", pasta}, &buf)
	if err == nil || !strings.Contains(err.Error(), "001_cria_usuarios ja foi aplicada mas o arquivo mudou") {
		t.Fatalf("esperava recusa por checksum, veio %v", err)
	}
	buf.Reset()
	err = executaMigra([]string{"--pasta", pasta, "status"}, &buf)
	if err == nil || !strings.Contains(buf.String(), "[!] 001_cria_usuarios") {
		t.Fatalf("status com arquivo mudado: %v\n%s", err, buf.String())
	}
}

func TestGsMigraErros(t *testing.T) {
	t.Setenv("GS_BANCO", "")
	casos := map[string][]string{
		"qual banco?":             {"--pasta", t.TempDir()},
		"subcomando desconhecido": {"--banco", "sqlite::memory:", "voa"},
		"desce quer um numero":    {"--banco", "sqlite::memory:", "desce", "zero"},
		"falta o nome":            {"novo"},
		"opcao desconhecida":      {"--banana"},
		"--banco precisa de um":   {"--banco"},
		"banco desconhecido":      {"--banco", "oracle://x", "status"},
	}
	for msg, args := range casos {
		var buf bytes.Buffer
		if err := executaMigra(args, &buf); err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("gs migra %v: esperava erro com %q, veio %v", args, msg, err)
		}
	}
}
