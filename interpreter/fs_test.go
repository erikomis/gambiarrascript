package interpreter

import (
	"os"
	"path/filepath"
	"testing"

	"gambiarrascript/object"
)

func TestExisteEhDir(t *testing.T) {
	dir := t.TempDir()
	if eval(t, `existe("`+dir+`")`).Type() != object.BOOLEANO_OBJ {
		t.Fatal("existe dir devolve booleano")
	}
	if eval(t, `existe("`+dir+`")`).Inspect() != "deu_bom" {
		t.Fatal("dir que existe -> deu_bom")
	}
	if eval(t, `existe("`+filepath.Join(dir, "nao")+"zzz"+`")`).Inspect() != "deu_ruim" {
		t.Fatal("caminho inexistente -> deu_ruim")
	}
	if eval(t, `eh_dir("`+dir+`")`).Inspect() != "deu_bom" {
		t.Fatal("eh_dir em diretorio -> deu_bom")
	}
	f := filepath.Join(dir, "x.txt")
	os.WriteFile(f, []byte("oi"), 0644)
	if eval(t, `eh_dir("`+f+`")`).Inspect() != "deu_ruim" {
		t.Fatal("eh_dir em arquivo -> deu_ruim")
	}
}

func TestCriaDirDeletaLeDir(t *testing.T) {
	base := t.TempDir()
	sub := filepath.Join(base, "level1", "level2")
	rodar(t, `cria_dir("`+sub+`")`)
	if _, err := os.Stat(sub); err != nil {
		t.Fatalf("cria_dir nao criou: %v", err)
	}
	// cria alguns arquivos
	os.WriteFile(filepath.Join(sub, "z.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(sub, "y.gs"), []byte("2"), 0644)
	r := eval(t, `le_dir("`+sub+`")`)
	l, ok := r.(*object.Lista)
	if !ok {
		t.Fatalf("esperava lista")
	}
	if len(l.Elements) != 2 {
		t.Fatalf("tamanho %d, esperado 2", len(l.Elements))
	}
	// ordem alfabetica
	if l.Elements[0].Inspect() != "y.gs" || l.Elements[1].Inspect() != "z.txt" {
		t.Fatalf("ordem: %v", l)
	}
	// deleta
	rodar(t, `deleta("`+sub+`")`)
	if eval(t, `existe("`+sub+`")`).Inspect() != "deu_ruim" {
		t.Fatal("deleta deveria remover o subdir")
	}
	// deleta idempotente (nao da erro)
	if v := eval(t, `deleta("`+sub+`")`); v.Type() != object.NADA_OBJ {
		t.Fatalf("deleta idempotente devolve nada, veio %s", v.Type())
	}
}

func TestCaminhoJuntaBaseDirExtAbs(t *testing.T) {
	full := eval(t, `caminho_junta("a", "b", "c.gs")`).Inspect()
	esp := filepath.Join("a", "b", "c.gs")
	if full != esp {
		t.Fatalf("junta: %q esperado %q", full, esp)
	}
	if eval(t, `caminho_base("/tmp/x/y.gs")`).Inspect() != "y.gs" {
		t.Fatal("caminho_base")
	}
	if eval(t, `caminho_dir("/tmp/x/y.gs")`).Inspect() != filepath.Dir("/tmp/x/y.gs") {
		t.Fatal("caminho_dir")
	}
	if eval(t, `caminho_ext("arquivo.gs")`).Inspect() != ".gs" {
		t.Fatal("caminho_ext")
	}
	if eval(t, `caminho_ext("semext")`).Inspect() != "" {
		t.Fatal("caminho_ext sem ext -> \"\"")
	}
	abs := eval(t, `caminho_abs("foo/bar")`).Inspect()
	if !filepath.IsAbs(abs) {
		t.Fatalf("caminho_abs deveria ser absoluto: %q", abs)
	}
}
