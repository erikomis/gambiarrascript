package vm

import "testing"

// Escopo de funcao estilo Python, igual o tree-walker: `bota NOME`/`NOME += `
// dentro de gambiarra cria/atualiza um LOCAL (o nome e local na funcao
// inteira). Ler antes de o local ser escrito enxerga o de fora (o escopo que
// envolve, a global, o builtin) — e a regra dinamica do tree-walker: vale o
// escopo mais de dentro em que o nome JA foi botado. Closure enxerga a
// variavel (nao uma copia): atribuicao feita depois de criar a closure
// aparece nela. Na VM o nome era resolvido pela ordem do texto: laco que lia
// antes de escrever lia sempre a global, ramo que nao rodou lia lixo da pilha.
func TestEscopoDeFuncaoIgualNosDois(t *testing.T) {
	casos := []struct{ nome, src, saida, erro string }{
		{"composta em laco", `bota n = 0
gambiarra f()
    pra_cada i em [1, 2, 3]
        n += i
    acabou_finalmente
    funciona n
acabou_finalmente
mostra f()
mostra n`, "6\n0\n", ""},
		{"enquanto com composta", `bota x = 1
gambiarra f()
    enquanto x < 3
        x += 1
    acabou_finalmente
    funciona x
acabou_finalmente
mostra f()
mostra x`, "3\n1\n", ""},
		{"le antes de escrever no laco", `bota x = 1
gambiarra f()
    pra_cada i em [1, 2]
        mostra x
        bota x = 10 + i
    acabou_finalmente
acabou_finalmente
f()
mostra x`, "1\n11\n1\n", ""},
		{"ramo que nao rodou", `bota x = 1
gambiarra f(c)
    se_colar c
        bota x = 5
    acabou_finalmente
    funciona x
acabou_finalmente
mostra f(deu_ruim)
mostra f(deu_bom)
mostra x`, "1\n5\n1\n", ""},
		{"lambda", `bota x = 1
bota f = gambiarra() bota x = 9 funciona x acabou_finalmente
mostra f()
mostra x`, "9\n1\n", ""},
		{"global declarada depois da funcao", `gambiarra f()
    funciona y
acabou_finalmente
bota y = 3
mostra f()`, "3\n", ""},
		{"local nunca botado", `gambiarra f(c)
    se_colar c
        bota y = 1
    acabou_finalmente
    funciona y
acabou_finalmente
mostra f(deu_bom)
mostra f(deu_ruim)`, "1\n", "deu ruim na linha 5: cade o `y`? voce nao botou isso ainda"},
		{"global nunca botada", `se_colar deu_ruim
    bota zz = 1
acabou_finalmente
arruma
    mostra zz
quebrou erro
    mostra erro_msg(erro)
acabou_finalmente`, "deu ruim na linha 5: cade o `zz`? voce nao botou isso ainda\n", ""},
		{"indice muda o objeto compartilhado", `bota xs = [1]
bota d = {"n": 1}
gambiarra g()
    bota xs[0] = 99
    d.n += 1
acabou_finalmente
g()
mostra xs
mostra d.n`, "[99]\n2\n", ""},
		{"bora", `bota x = 1
gambiarra f()
    bota x = 7
    funciona x
acabou_finalmente
mostra espera(bora f())
mostra x`, "7\n1\n", ""},
		{"closure ve reatribuicao posterior", `gambiarra f()
    bota x = 1
    bota g = gambiarra() funciona x acabou_finalmente
    bota x = 2
    funciona g()
acabou_finalmente
mostra f()`, "2\n", ""},
		{"closure escreve no proprio local", `gambiarra fora()
    bota x = 1
    bota d2 = gambiarra() x += 10 funciona x acabou_finalmente
    mostra d2()
    mostra d2()
    mostra x
acabou_finalmente
fora()`, "11\n11\n1\n", ""},
		{"recursao aninhada", `gambiarra fora()
    gambiarra fat(n)
        se_colar n <= 1
            funciona 1
        acabou_finalmente
        funciona n * fat(n - 1)
    acabou_finalmente
    funciona fat(5)
acabou_finalmente
mostra fora()`, "120\n", ""},
		{"aninhadas se chamando", `gambiarra fora()
    gambiarra a(n)
        funciona b(n) + 1
    acabou_finalmente
    gambiarra b(n)
        funciona n * 2
    acabou_finalmente
    funciona a(5)
acabou_finalmente
mostra fora()`, "11\n", ""},
		{"closures no laco veem o ultimo valor", `gambiarra f()
    bota fs = []
    pra_cada i em [1, 2, 3]
        adiciona(fs, gambiarra() funciona i acabou_finalmente)
    acabou_finalmente
    funciona mapeia(fs, gambiarra(g) funciona g() acabou_finalmente)
acabou_finalmente
mostra f()`, "[3, 3, 3]\n", ""},
		{"cadeia: de fora sem valor cai na global", `bota x = "global"
gambiarra fora(c)
    se_colar c
        bota x = "fora"
    acabou_finalmente
    gambiarra dentro()
        funciona x
    acabou_finalmente
    funciona dentro()
acabou_finalmente
mostra fora(deu_ruim)
mostra fora(deu_bom)`, "global\nfora\n", ""},
		{"builtin sombreado depois de lido", `gambiarra f()
    bota a = max(1, 2)
    bota max = 10
    funciona a + max
acabou_finalmente
mostra f()
bota r = []
pra_cada k em [1, 2]
    adiciona(r, tipo(max))
    bota max = k
acabou_finalmente
mostra r`, "12\n[funcao, numero]\n", ""},
		{"param capturado e default", `gambiarra f(a, b = a * 2)
    bota g = gambiarra() funciona a + b acabou_finalmente
    bota a = 100
    funciona g()
acabou_finalmente
mostra f(1)
mostra f(1, 1)`, "102\n101\n", ""},
		{"tail call com closure", `gambiarra conta(n, fs = [])
    se_colar n == 0
        funciona mapeia(fs, gambiarra(g) funciona g() acabou_finalmente)
    acabou_finalmente
    bota k = n * 10
    adiciona(fs, gambiarra() funciona k acabou_finalmente)
    funciona conta(n - 1, fs)
acabou_finalmente
mostra conta(3)`, "[30, 20, 10]\n", ""},
		{"celula lida por goroutine enquanto a dona escreve", `gambiarra f()
    bota n = 0
    gambiarra le()
        funciona n
    acabou_finalmente
    bota fs = []
    pra_cada i de 1 ate 20
        adiciona(fs, bora le())
        n += 1
    acabou_finalmente
    pra_cada x em fs
        espera(x)
    acabou_finalmente
    funciona n
acabou_finalmente
mostra f()`, "20\n", ""},
		{"quebrou e pra_cada viram locais", `bota erro = "g"
bota i = "g"
gambiarra f()
    pra_cada i em [1]
    acabou_finalmente
    arruma
        quebra("a")
    quebrou erro
    acabou_finalmente
    funciona i
acabou_finalmente
mostra f()
mostra erro
mostra i`, "1\ng\ng\n", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			esperaNosDois(t, c.src, c.saida, c.erro)
		})
	}
}
