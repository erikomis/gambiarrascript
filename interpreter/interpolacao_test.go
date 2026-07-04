package interpreter

import "testing"

func TestInterpolacaoSimples(t *testing.T) {
	out := rodar(t, `bota n = 42
mostra "n = ${n}"`)
	if out != "n = 42\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestInterpolacaoExpressao(t *testing.T) {
	out := rodar(t, `mostra "soma: ${1 + 2 * 3}"`)
	if out != "soma: 7\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestInterpolacaoChamaFuncao(t *testing.T) {
	out := rodar(t, `gambiarra dobra(x)
    funciona x * 2
acabou_finalmente
bota n = 21
mostra "Dobro: ${dobra(n)}"`)
	if out != "Dobro: 42\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestInterpolacaoEscape(t *testing.T) {
	out := rodar(t, `bota n = 1
mostra "literal \${n} nao interpola"`)
	esperado := "literal ${n} nao interpola\n"
	if out != esperado {
		t.Fatalf("saida %q, esperado %q", out, esperado)
	}
}

func TestInterpolacaoSemMarker(t *testing.T) {
	out := rodar(t, `mostra "ola mundo"`)
	if out != "ola mundo\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestInterpolacaoMultiplas(t *testing.T) {
	out := rodar(t, `bota a = 1
bota b = "dois"
mostra "${a} e ${b}"`)
	if out != "1 e dois\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestInterpolacaoIndiceDicionario(t *testing.T) {
	out := rodar(t, `bota d = {"chave": "valor"}
mostra "v: ${d["chave"]}"`)
	if out != "v: valor\n" {
		t.Fatalf("saida %q", out)
	}
}
