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

func TestTamanhoArquivo(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := rodar(t, fmt.Sprintf(`mostra tamanho_arquivo(%q)`, f))
	if out != "5\n" {
		t.Fatalf("tamanho_arquivo: %q", out)
	}
}

func TestTamanhoArquivoInexistenteDaErro(t *testing.T) {
	out := rodarErro(t, `tamanho_arquivo("/nao/existe/xyz123")`)
	if !strings.Contains(out, "tamanho_arquivo") {
		t.Fatalf("tamanho_arquivo inexistente: %q", out)
	}
}

func TestModificadoEm(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.TrimSpace(rodar(t, fmt.Sprintf(`mostra modificado_em(%q)`, f)))
	if out != fmt.Sprintf("%d", info.ModTime().Unix()) {
		t.Fatalf("modificado_em: got %q want %d", out, info.ModTime().Unix())
	}
}

func TestGlob(t *testing.T) {
	dir := t.TempDir()
	for _, nome := range []string{"a.gs", "b.gs", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, nome), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := rodar(t, fmt.Sprintf(`mostra tamanho(glob(%q))`, filepath.Join(dir, "*.gs")))
	if out != "2\n" {
		t.Fatalf("glob: %q", out)
	}
}

func TestGlobSemMatchListaVazia(t *testing.T) {
	dir := t.TempDir()
	out := rodar(t, fmt.Sprintf(`mostra tamanho(glob(%q))`, filepath.Join(dir, "*.xyz")))
	if out != "0\n" {
		t.Fatalf("glob sem match: %q", out)
	}
}

func TestGlobPadraoInvalidoDaErro(t *testing.T) {
	out := rodarErro(t, `glob("[")`)
	if !strings.Contains(out, "glob") {
		t.Fatalf("glob padrao invalido: %q", out)
	}
}
