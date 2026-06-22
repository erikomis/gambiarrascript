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

func TestDicionarioAcesso(t *testing.T) {
	out := rodar(t, `bota pessoa = {"nome": "Erik", "idade": 25}
mostra pessoa["nome"]
mostra pessoa["idade"]
mostra pessoa["faltando"]`)
	if out != "Erik\n25\nnada\n" {
		t.Fatalf("got %q", out)
	}
}

func TestDicionarioIgualdade(t *testing.T) {
	out := rodar(t, `bota a = {"x": 1}
bota b = {"x": 1}
bota c = {"x": 2}
mostra a == b
mostra a == c`)
	if out != "deu_bom\ndeu_ruim\n" {
		t.Fatalf("got %q", out)
	}
}

func TestAtribuiIndiceDicionario(t *testing.T) {
	out := rodar(t, `bota d = {"a": 1}
bota d["a"] = 99
bota d["novo"] = 7
mostra d["a"]
mostra d["novo"]`)
	if out != "99\n7\n" {
		t.Fatalf("got %q", out)
	}
}

func TestAtribuiIndiceLista(t *testing.T) {
	out := rodar(t, `bota nums = [10, 20, 30]
bota nums[1] = 99
mostra nums[1]`)
	if out != "99\n" {
		t.Fatalf("got %q", out)
	}
}

func TestPraCadaDicionario(t *testing.T) {
	out := rodar(t, `bota d = {"a": 1, "b": 2, "c": 3}
bota soma = 0
pra_cada chave em d
    bota soma = soma + d[chave]
acabou_finalmente
mostra soma`)
	if out != "6\n" {
		t.Fatalf("got %q", out)
	}
}

func TestBuiltins(t *testing.T) {
	casos := []struct {
		input string
		esp   string
	}{
		{`mostra tamanho([1, 2, 3])`, "3\n"},
		{`mostra tamanho({"a": 1, "b": 2})`, "2\n"},
		{`mostra tamanho("salve")`, "5\n"},
		{`mostra tem({"a": 1}, "a")`, "deu_bom\n"},
		{`mostra tem({"a": 1}, "z")`, "deu_ruim\n"},
		{`mostra texto(42)`, "42\n"},
		{`mostra numero("10") + 5`, "15\n"},
	}
	for _, c := range casos {
		out := rodar(t, c.input)
		if out != c.esp {
			t.Errorf("input %q => got %q, esperado %q", c.input, out, c.esp)
		}
	}
}

func TestBuiltinChaves(t *testing.T) {
	out := rodar(t, `mostra tamanho(chaves({"a": 1, "b": 2}))`)
	if out != "2\n" {
		t.Fatalf("got %q", out)
	}
}

func TestBuiltinErroCapturavel(t *testing.T) {
	out := rodar(t, `arruma
    bota x = numero("abc")
quebrou erro
    mostra "peguei"
acabou_finalmente`)
	if out != "peguei\n" {
		t.Fatalf("erro de builtin deveria ser capturavel, got %q", out)
	}
}

func TestBuiltinSombreavel(t *testing.T) {
	out := rodar(t, `gambiarra tamanho(x)
    funciona 999
acabou_finalmente
mostra tamanho([1, 2, 3])`)
	if out != "999\n" {
		t.Fatalf("a funcao do usuario deveria sombrear o builtin, got %q", out)
	}
}

func TestDicionarioErros(t *testing.T) {
	casos := []string{
		// atribuição fora do range em lista
		"bota l = [1, 2]\nbota l[9] = 5",
		// chave não-chaveável em literal de dicionário (lista não é Chaveavel)
		`bota d = {[1, 2]: "x"}`,
		// indexar um não-container
		`mostra 42["x"]`,
		// builtin com aridade errada
		`mostra tamanho()`,
		// chaves em não-dicionário
		`mostra chaves([1, 2])`,
	}
	for _, in := range casos {
		got := eval(t, in)
		if got.Type() != object.ERRO_OBJ {
			t.Errorf("input %q deveria gerar Erro, got %s (%q)", in, got.Type(), got.Inspect())
		}
	}
}

func TestAtribuiIndiceAninhada(t *testing.T) {
	out := rodar(t, `bota m = {"x": {"y": 1}}
bota m["x"]["y"] = 42
mostra m["x"]["y"]`)
	if out != "42\n" {
		t.Fatalf("atribuicao aninhada falhou: got %q", out)
	}
}
