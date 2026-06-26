package vm

import (
	"bytes"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

func rodaVM(t *testing.T, input string) (object.Object, string) {
	t.Helper()
	prog := parser.New(lexer.New(input)).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile %q: %v", input, err)
	}
	var buf bytes.Buffer
	maq := New(comp.Bytecode(), &buf)
	if err := maq.Run(); err != nil {
		t.Fatalf("vm %q: %v", input, err)
	}
	return maq.LastPoppedStackElem(), buf.String()
}

func TestVMAritmetica(t *testing.T) {
	casos := []struct{ input, esp string }{
		{"1 + 2", "3"},
		{"2 * 3 + 4", "10"},
		{"(1 + 2) * 3", "9"},
		{"10 % 3", "1"},
		{"-5 + 8", "3"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("%q => got %s, esperado %s", c.input, got.Inspect(), c.esp)
		}
	}
}

func TestVMComparacao(t *testing.T) {
	casos := []struct{ input, esp string }{
		{"1 < 2", "deu_bom"},
		{"2 < 1", "deu_ruim"},
		{"1 == 1", "deu_bom"},
		{"1 != 2", "deu_bom"},
		{"2 >= 2", "deu_bom"},
		{"nao deu_bom", "deu_ruim"},
		{`"a" == "a"`, "deu_bom"},
		{`"a" == "b"`, "deu_ruim"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("%q => got %s, esperado %s", c.input, got.Inspect(), c.esp)
		}
	}
}

func TestVMConcatenacao(t *testing.T) {
	got, _ := rodaVM(t, `"oi " + "tropa"`)
	if got.Inspect() != "oi tropa" {
		t.Fatalf("got %q", got.Inspect())
	}
}

func TestVMMostra(t *testing.T) {
	_, out := rodaVM(t, "mostra 1 + 2")
	if out != "3\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMDivisaoPorZeroDaErro(t *testing.T) {
	prog := parser.New(lexer.New("1 / 0")).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	maq := New(comp.Bytecode(), &bytes.Buffer{})
	if err := maq.Run(); err == nil {
		t.Fatal("divisao por zero deveria dar erro na VM")
	}
}

// --- fase 6b: vars, control flow, loops ---

func TestVMBotaEIdentificador(t *testing.T) {
	_, out := rodaVM(t, `bota x = 5
bota y = 7
mostra x + y`)
	if out != "12\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMBotaReatribui(t *testing.T) {
	_, out := rodaVM(t, `bota x = 1
bota x = x + 10
mostra x`)
	if out != "11\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMSeColarTrue(t *testing.T) {
	_, out := rodaVM(t, `bota n = 18
se_colar n >= 18
    mostra "maior"
se_nao_colar
    mostra "menor"
acabou_finalmente`)
	if out != "maior\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMSeColarFalse(t *testing.T) {
	_, out := rodaVM(t, `bota n = 10
se_colar n >= 18
    mostra "maior"
se_nao_colar
    mostra "menor"
acabou_finalmente`)
	if out != "menor\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMSeColarEncadeado(t *testing.T) {
	_, out := rodaVM(t, `bota n = 5
se_colar n > 10
    mostra "alto"
se_nao_colar se_colar n > 3
    mostra "medio"
se_nao_colar
    mostra "baixo"
acabou_finalmente`)
	if out != "medio\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMELogicoShortCircuit(t *testing.T) {
	got, _ := rodaVM(t, "deu_bom e deu_bom")
	if got.Inspect() != "deu_bom" {
		t.Fatalf("got %s", got.Inspect())
	}
	got, _ = rodaVM(t, "deu_ruim e deu_bom")
	if got.Inspect() != "deu_ruim" {
		t.Fatalf("got %s", got.Inspect())
	}
}

func TestVMOuLogicoShortCircuit(t *testing.T) {
	got, _ := rodaVM(t, "deu_bom ou deu_ruim")
	if got.Inspect() != "deu_bom" {
		t.Fatalf("got %s", got.Inspect())
	}
	got, _ = rodaVM(t, "deu_ruim ou deu_ruim")
	if got.Inspect() != "deu_ruim" {
		t.Fatalf("got %s", got.Inspect())
	}
}

func TestVMEnquantoSimples(t *testing.T) {
	_, out := rodaVM(t, `bota i = 0
enquanto i < 3
    mostra i
    bota i = i + 1
acabou_finalmente`)
	if out != "0\n1\n2\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMPraCadaNum(t *testing.T) {
	_, out := rodaVM(t, `pra_cada i de 1 ate 5
    mostra i
acabou_finalmente`)
	if out != "1\n2\n3\n4\n5\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMEnquantoComVaza(t *testing.T) {
	_, out := rodaVM(t, `bota i = 0
enquanto deu_bom
    bota i = i + 1
    se_colar i == 4
        vaza
    acabou_finalmente
    mostra i
acabou_finalmente`)
	if out != "1\n2\n3\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMEnquantoComContinua(t *testing.T) {
	_, out := rodaVM(t, `bota i = 0
enquanto i < 5
    bota i = i + 1
    se_colar i == 3
        continua
    acabou_finalmente
    mostra i
acabou_finalmente`)
	if out != "1\n2\n4\n5\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMPraCadaComContinuaEVaza(t *testing.T) {
	_, out := rodaVM(t, `pra_cada i de 1 ate 10
    se_colar i == 3
        continua
    acabou_finalmente
    se_colar i == 7
        vaza
    acabou_finalmente
    mostra i
acabou_finalmente`)
	if out != "1\n2\n4\n5\n6\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMLoopsAninhados(t *testing.T) {
	_, out := rodaVM(t, `bota s = 0
pra_cada i de 1 ate 3
    pra_cada j de 1 ate 3
        bota s = s + 1
    acabou_finalmente
acabou_finalmente
mostra s`)
	if out != "9\n" {
		t.Fatalf("got %q", out)
	}
}

func TestFibonacciNaVM(t *testing.T) {
	// sem funcoes ainda (fase 6d) — mas dá pra calcular iterativamente.
	_, out := rodaVM(t, `bota a = 0
bota b = 1
bota i = 0
enquanto i < 10
    mostra a
    bota t = a + b
    bota a = b
    bota b = t
    bota i = i + 1
acabou_finalmente`)
	if out != "0\n1\n1\n2\n3\n5\n8\n13\n21\n34\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMBuiltinsAgoraSuportados(t *testing.T) {
	// fase 6d: builtins e chamadas de funcao agora rodam na VM
	_, out := rodaVM(t, `mostra tamanho([1, 2, 3])`)
	if out != "3\n" {
		t.Fatalf("got %q", out)
	}
}

func TestVMFuncoesEBuiltinsAgoraFunfa(t *testing.T) {
	_, out := rodaVM(t, `
gambiarra soma(a, b)
    funciona a + b
acabou_finalmente
mostra soma(2, 3)
mostra tamanho([4, 5, 6, 7])`)
	if out != "5\n4\n" {
		t.Fatalf("got %q", out)
	}
}
