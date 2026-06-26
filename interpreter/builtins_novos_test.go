package interpreter

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

func rodarComStdin(t *testing.T, input string, in *bytes.Buffer) string {
	t.Helper()
	p := parser.New(lexer.New(input))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("erros de parse: %v", errs)
	}
	var buf bytes.Buffer
	i := New(&buf)
	if in != nil {
		i.DefinirStdin(in)
	}
	res := i.Eval(prog, object.NewEnvironment())
	if isError(res) {
		t.Fatalf("erro de runtime inesperado: %s", res.Inspect())
	}
	return buf.String()
}

func TestBuiltinsTexto(t *testing.T) {
	casos := []struct{ input, esp string }{
		{`mostra maiusculo("salve")`, "SALVE\n"},
		{`mostra minusculo("Salve")`, "salve\n"},
		{`mostra substitui("banana", "a", "o")`, "bonono\n"},
		{`mostra contem("gambiarra", "arra")`, "deu_bom\n"},
		{`mostra comeca_com("salve", "sa")`, "deu_bom\n"},
		{`mostra comeca_com("salve", "xx")`, "deu_ruim\n"},
		{`mostra termina_com("salve.gs", ".gs")`, "deu_bom\n"},
		{`mostra tira_espaco("   salve   ")`, "salve\n"},
		{`mostra fatia("gambiarra", 0, 4)`, "gamb\n"},
		{`mostra fatia("gambiarra", 4)`, "iarra\n"},
		{`mostra tamanho(separa("a,b,c", ","))`, "3\n"},
		{`mostra junta(["a", "b", "c"], "-")`, "a-b-c\n"},
	}
	for _, c := range casos {
		out := rodar(t, c.input)
		if out != c.esp {
			t.Errorf("input %q => got %q, esperado %q", c.input, out, c.esp)
		}
	}
}

func TestBuiltinsLista(t *testing.T) {
	out := rodar(t, `bota l = [3, 1, 2]
		adiciona(l, 9)
ordena(l)
mostra l`)
	if out != "[1, 2, 3, 9]\n" {
		t.Fatalf("adiciona/ordena: got %q", out)
	}

	out = rodar(t, `bota l = [1, 2, 3]
inverte(l)
mostra l`)
	if out != "[3, 2, 1]\n" {
		t.Fatalf("inverte: got %q", out)
	}

	out = rodar(t, `bota l = [1, 2, 3]
remove(l, 2)
mostra l`)
	if out != "[1, 3]\n" {
		t.Fatalf("remove: got %q", out)
	}
}

func TestMapeiaEFiltra(t *testing.T) {
	out := rodar(t, `gambiarra dobro(n)
    funciona n * 2
acabou_finalmente
mostra mapeia([1, 2, 3], dobro)`)
	if out != "[2, 4, 6]\n" {
		t.Fatalf("mapeia: got %q", out)
	}

	out = rodar(t, `gambiarra par(n)
    funciona n % 2 == 0
acabou_finalmente
mostra filtra([1, 2, 3, 4, 5], par)`)
	if out != "[2, 4]\n" {
		t.Fatalf("filtra: got %q", out)
	}
}

func TestBuiltinsMat(t *testing.T) {
	casos := []struct{ input, esp string }{
		{`mostra raiz(9)`, "3\n"},
		{`mostra arredonda(2.7)`, "3\n"},
		{`mostra teto(2.1)`, "3\n"},
		{`mostra chao(2.9)`, "2\n"},
		{`mostra abs(-7)`, "7\n"},
		{`mostra min(3, 1, 2)`, "1\n"},
		{`mostra max(3, 1, 2)`, "3\n"},
	}
	for _, c := range casos {
		out := rodar(t, c.input)
		if out != c.esp {
			t.Errorf("input %q => got %q, esperado %q", c.input, out, c.esp)
		}
	}
	if got := eval(t, `aleatorio()`); got.Type() != object.NUMERO_OBJ {
		t.Errorf("aleatorio() deveria devolver numero, got %s", got.Type())
	}
}

func TestArquivoLeEscreve(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "arquivo.txt")
	out := rodar(t, `escreve_arquivo("`+caminho+`", "salve tropa")
mostra le_arquivo("`+caminho+`")`)
	if out != "salve tropa\n" {
		t.Fatalf("arquivo: got %q", out)
	}
}

func TestPergunta(t *testing.T) {
	out := rodarComStdin(t, `bota nome = pergunta("teu nome: ")
mostra "eai " + nome`, bytes.NewBufferString("erik\n"))
	if out != "teu nome: eai erik\n" {
		t.Fatalf("pergunta: got %q", out)
	}
}

func TestArgumentos(t *testing.T) {
	p := parser.New(lexer.New(`mostra argumentos()`))
	prog := p.ParseProgram()
	var buf bytes.Buffer
	i := New(&buf)
	i.DefinirArgumentos([]string{"abc", "123"})
	res := i.Eval(prog, object.NewEnvironment())
	if isError(res) {
		t.Fatalf("erro: %s", res.Inspect())
	}
	if out := buf.String(); out != "[abc, 123]\n" {
		t.Fatalf("argumentos: got %q", out)
	}
}

func TestImporta(t *testing.T) {
	dir := t.TempDir()
	modulo := filepath.Join(dir, "modulo.gs")
	if err := os.WriteFile(modulo, []byte("bota saudacao = \"salve\"\ngambiarra dobra(x)\n    funciona x * 2\nacabou_finalmente"), 0644); err != nil {
		t.Fatal(err)
	}
	principal := "importa \"modulo.gs\"\nmostra saudacao\nmostra dobra(21)"
	out := rodarComDir(t, principal, dir)
	if out != "salve\n42\n" {
		t.Fatalf("importa: got %q", out)
	}
}

func rodarComDir(t *testing.T, input, dir string) string {
	t.Helper()
	p := parser.New(lexer.New(input))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("erros de parse: %v", errs)
	}
	var buf bytes.Buffer
	i := New(&buf)
	i.DefinirDirBase(dir)
	res := i.Eval(prog, object.NewEnvironment())
	if isError(res) {
		t.Fatalf("erro de runtime inesperado: %s", res.Inspect())
	}
	return buf.String()
}