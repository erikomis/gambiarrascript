package interpreter

import (
	"bytes"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

func rodar(t *testing.T, input string) string {
	t.Helper()
	p := parser.New(lexer.New(input))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("erros de parse: %v", errs)
	}
	var buf bytes.Buffer
	i := New(&buf)
	res := i.Eval(prog, object.NewEnvironment())
	if isError(res) {
		t.Fatalf("erro de runtime inesperado: %s", res.Inspect())
	}
	return buf.String()
}

func TestSeColarEElse(t *testing.T) {
	out := rodar(t, `bota idade = 16
se_colar idade >= 18
    mostra "pode entrar"
se_nao_colar
    mostra "volta depois"
acabou_finalmente`)
	if out != "volta depois\n" {
		t.Fatalf("got %q", out)
	}
}

func TestEnquantoComVazaEContinua(t *testing.T) {
	out := rodar(t, `bota c = 0
enquanto c < 10
    bota c = c + 1
    se_colar c == 2
        continua
    acabou_finalmente
    se_colar c == 4
        vaza
    acabou_finalmente
    mostra c
acabou_finalmente`)
	// imprime 1, 3 (pula 2 com continua, para em 4 com vaza)
	if out != "1\n3\n" {
		t.Fatalf("got %q", out)
	}
}

func TestGambiarraComRetorno(t *testing.T) {
	out := rodar(t, `gambiarra soma(a, b)
    funciona a + b
acabou_finalmente
mostra soma(4, 5)`)
	if out != "9\n" {
		t.Fatalf("got %q", out)
	}
}

func TestPraCadaNumericoELista(t *testing.T) {
	out := rodar(t, `pra_cada i de 1 ate 3
    mostra i
acabou_finalmente
pra_cada nome em ["Ana", "Bia"]
    mostra nome
acabou_finalmente`)
	if out != "1\n2\n3\nAna\nBia\n" {
		t.Fatalf("got %q", out)
	}
}

func TestArrumaQuebrou(t *testing.T) {
	out := rodar(t, `arruma
    bota x = 10 / 0
quebrou erro
    mostra "peguei: " + erro
acabou_finalmente`)
	if out == "" || out[:7] != "peguei:" {
		t.Fatalf("catch nao capturou o erro, got %q", out)
	}
}
