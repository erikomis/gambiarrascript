package interpreter

import (
	"testing"

	"gambiarrascript/object"
)

func TestDesestruturaLista(t *testing.T) {
	out := rodar(t, `bota [a, b] = [1, 2, 3]
mostra a
mostra b`)
	if out != "1\n2\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestDesestruturaListaCurta(t *testing.T) {
	// lista menor que o padrao: sobras viram nada (lenient)
	out := rodar(t, `bota [a, b, c] = [1]
mostra a
mostra b
mostra c`)
	if out != "1\nnada\nnada\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestDesestruturaDict(t *testing.T) {
	out := rodar(t, `bota {nome, idade} = {"nome": "Erik", "idade": 25, "extra": 1}
mostra nome
mostra idade`)
	if out != "Erik\n25\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestDesestruturaDictChaveFaltando(t *testing.T) {
	out := rodar(t, `bota {sumido} = {"outra": 1}
mostra sumido`)
	if out != "nada\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestDesestruturaTipoErrado(t *testing.T) {
	if r := eval(t, `bota [a] = 42`); r.Type() != object.ERRO_OBJ {
		t.Fatalf("desestruturar numero devia dar erro, veio %s", r.Type())
	}
	if r := eval(t, `bota {x} = [1]`); r.Type() != object.ERRO_OBJ {
		t.Fatalf("{} em lista devia dar erro, veio %s", r.Type())
	}
}

func TestDesestruturaRetornoDeFuncao(t *testing.T) {
	out := rodar(t, `gambiarra minmax(xs)
    funciona [1, 9]
acabou_finalmente
bota [menor, maior] = minmax([5, 1, 9])
mostra maior - menor`)
	if out != "8\n" {
		t.Fatalf("saida %q", out)
	}
}
