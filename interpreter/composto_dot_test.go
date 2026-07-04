package interpreter

import "testing"

func TestAtribuicaoCompostaAritmetica(t *testing.T) {
	out := rodar(t, `bota x = 10
x += 5
x -= 3
x *= 4
x /= 2
x %= 7
mostra x`)
	// 10+5=15, -3=12, *4=48, /2=24, %7=3
	if out != "3\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestAtribuicaoCompostaBitwise(t *testing.T) {
	out := rodar(t, `bota x = 0b1010
x &= 0b1100
x |= 0b0001
x ^= 0b1111
x <<= 2
x >>= 1
mostra x`)
	// 10&12=8, |1=9, ^15=6, <<2=24, >>1=12
	if out != "12\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestAtribuicaoCompostaIndice(t *testing.T) {
	out := rodar(t, `bota xs = [1, 2, 3]
xs[1] += 40
mostra xs[1]
bota d = {"n": 5}
d["n"] *= 2
mostra d["n"]`)
	if out != "42\n10\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestDotAccessLeitura(t *testing.T) {
	out := rodar(t, `bota p = {"nome": "Erik", "idade": 25}
mostra p.nome
mostra p.idade`)
	if out != "Erik\n25\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestDotAccessEscrita(t *testing.T) {
	out := rodar(t, `bota p = {"nome": "Erik"}
bota p.nome = "Zeh"
bota p.novo = 1
mostra p.nome
mostra p.novo`)
	if out != "Zeh\n1\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestDotAccessComposto(t *testing.T) {
	out := rodar(t, `bota c = {"total": 0}
c.total += 10
c.total *= 3
mostra c.total`)
	if out != "30\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestDotAccessAninhado(t *testing.T) {
	out := rodar(t, `bota cfg = {"db": {"porta": 5432}}
mostra cfg.db.porta`)
	if out != "5432\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestDotAccessMetodo(t *testing.T) {
	// "metodo": funcao guardada no dict, chamada via ponto passando o proprio
	// objeto (sem self implicito — e gambiarra, nao Java).
	out := rodar(t, `gambiarra fala(eu)
    funciona "salve, " + eu["nome"]
acabou_finalmente
bota p = {"nome": "Erik", "fala": fala}
mostra p.fala(p)`)
	if out != "salve, Erik\n" {
		t.Fatalf("saida %q", out)
	}
}
