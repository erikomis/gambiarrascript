package interpreter

import "testing"

func TestLambdaAtribuida(t *testing.T) {
	out := rodar(t, `bota dobra = gambiarra(n)
    funciona n * 2
acabou_finalmente
mostra dobra(21)`)
	if out != "42\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestLambdaInlineEmBuiltin(t *testing.T) {
	out := rodar(t, `mostra mapeia([1, 2, 3], gambiarra(x) funciona x + 10 acabou_finalmente)`)
	if out != "[11, 12, 13]\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestLambdaClosure(t *testing.T) {
	// captura de variavel: leitura funciona; mutacao persistente usa um
	// objeto de referencia (dict), porque `bota` cria binding no escopo local
	// (mesma semantica nos dois engines — freevars da VM capturam por valor).
	out := rodar(t, `gambiarra contador()
    bota estado = {"n": 0}
    funciona gambiarra()
        estado.n += 1
        funciona estado.n
    acabou_finalmente
acabou_finalmente
bota tick = contador()
tick()
tick()
mostra tick()`)
	if out != "3\n" {
		t.Fatalf("closure com estado: %q", out)
	}
}

func TestReduzAchaComGambiarraDoUsuario(t *testing.T) {
	// regressao: reduz/acha/acha_indice so aceitavam builtin como fn;
	// agora usam applyFunction e aceitam gambiarra/lambda do usuario.
	out := rodar(t, `mostra reduz([1, 2, 3, 4], gambiarra(acc, n) funciona acc + n acabou_finalmente, 0)
mostra acha([3, 7, 12], gambiarra(n) funciona n > 5 acabou_finalmente)
mostra acha_indice([3, 7, 12], gambiarra(n) funciona n > 5 acabou_finalmente)`)
	if out != "10\n7\n1\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestLambdaComoValorDeDict(t *testing.T) {
	out := rodar(t, `bota ops = {"dobro": gambiarra(x) funciona x * 2 acabou_finalmente}
mostra ops.dobro(21)`)
	if out != "42\n" {
		t.Fatalf("saida %q", out)
	}
}
