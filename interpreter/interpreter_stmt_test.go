package interpreter

import (
	"bytes"
	"strings"
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
	if !strings.HasPrefix(out, "peguei: ") {
		t.Fatalf("saida nao comeca com 'peguei: ', got %q", out)
	}
	if !strings.Contains(out, "dividir por zero") {
		t.Fatalf("mensagem de erro nao contem 'dividir por zero', got %q", out)
	}
}

func TestVazaForaDeLoop(t *testing.T) {
	// vaza no topo do programa deve virar erro
	got := eval(t, `vaza`)
	if got.Type() != object.ERRO_OBJ {
		t.Errorf("vaza top-level deveria gerar Erro, got %s (%q)", got.Type(), got.Inspect())
	}

	// vaza dentro do corpo de uma funcao (fora de qualquer loop) deve virar erro
	got = eval(t, `gambiarra f()
    vaza
acabou_finalmente
f()`)
	if got.Type() != object.ERRO_OBJ {
		t.Errorf("vaza dentro de funcao sem loop deveria gerar Erro, got %s (%q)", got.Type(), got.Inspect())
	}
}

func TestGambiarraRecursiva(t *testing.T) {
	out := rodar(t, `gambiarra fatorial(n)
    se_colar n <= 1
        funciona 1
    acabou_finalmente
    funciona n * fatorial(n - 1)
acabou_finalmente
mostra fatorial(5)`)
	if out != "120\n" {
		t.Fatalf("esperado '120\\n', got %q", out)
	}
}
