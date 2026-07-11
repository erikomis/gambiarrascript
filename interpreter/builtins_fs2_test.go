package interpreter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopia(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	dst := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(src, []byte("oi tropa"), 0o644); err != nil {
		t.Fatal(err)
	}
	rodar(t, fmt.Sprintf(`copia(%q, %q)`, src, dst))
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "oi tropa" {
		t.Fatalf("copia: got %q err %v", got, err)
	}
	// original continua la
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("copia apagou a origem: %v", err)
	}
}

func TestCopiaOrigemInexistenteDaErro(t *testing.T) {
	dir := t.TempDir()
	out := rodarErro(t, fmt.Sprintf(`copia(%q, %q)`, filepath.Join(dir, "nao_existe"), filepath.Join(dir, "x")))
	if !strings.Contains(out, "copia") {
		t.Fatalf("copia origem inexistente: %q", out)
	}
}

func TestMove(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	dst := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(src, []byte("conteudo"), 0o644); err != nil {
		t.Fatal(err)
	}
	rodar(t, fmt.Sprintf(`move(%q, %q)`, src, dst))
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("move: origem ainda existe")
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "conteudo" {
		t.Fatalf("move: got %q err %v", got, err)
	}
}
