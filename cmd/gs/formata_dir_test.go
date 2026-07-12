package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestColetaArquivosGsRecursivo(t *testing.T) {
	dir := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "a.gs"), []byte(""), 0o644))
	must(os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "sub", "b.gs"), []byte(""), 0o644))
	must(os.WriteFile(filepath.Join(dir, "c.txt"), []byte(""), 0o644)) // deve ser ignorado

	got, err := coletaArquivosGs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("esperava 2 arquivos .gs (recursivo), veio %d: %v", len(got), got)
	}
	// ordem estavel (lexical) do WalkDir: a.gs antes de sub/b.gs
	if filepath.Base(got[0]) != "a.gs" || filepath.Base(got[1]) != "b.gs" {
		t.Fatalf("ordem/arquivos inesperados: %v", got)
	}
}

func TestColetaArquivosGsArquivoDireto(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "x.gs")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := coletaArquivosGs([]string{f})
	if err != nil || len(got) != 1 || got[0] != f {
		t.Fatalf("arquivo direto falhou: got=%v err=%v", got, err)
	}
}
