package vm

import (
	"strings"
	"testing"
)

// esperaNosDois roda a fonte nos 2 engines e exige a MESMA saida esperada (e,
// se erroEsp != "", um erro nao capturado contendo esse trecho). Paridade so
// nao basta: os 2 podiam errar igual (fatia compartilhando memoria).
func esperaNosDois(t *testing.T, src, saidaEsp, erroEsp string) {
	t.Helper()
	engines := []struct {
		nome string
		roda func(*testing.T, string) (string, string, string)
	}{{"tree", rodaTWComp}, {"vm", rodaVMComp}}
	for _, e := range engines {
		_, saida, errStr := e.roda(t, src)
		if saida != saidaEsp {
			t.Errorf("[%s] saida errada em\n%s\n  veio:     %q\n  esperado: %q", e.nome, src, saida, saidaEsp)
		}
		if erroEsp == "" && errStr != "" {
			t.Errorf("[%s] erro inesperado em\n%s\n  %s", e.nome, src, errStr)
		}
		if erroEsp != "" && !strings.Contains(errStr, erroEsp) {
			t.Errorf("[%s] erro esperado %q em\n%s\n  veio: %q", e.nome, erroEsp, src, errStr)
		}
	}
}

// ?? com lado esquerdo NAO-nada: a VM descartava o valor (OpPop sobrando) e
// estourava a pilha (index out of range [-1]).
func TestCoalesceNaoNada(t *testing.T) {
	casos := []struct{ src, saida string }{
		{`mostra 3 ?? 4`, "3\n"},
		{`bota a = nada
mostra a ?? 5`, "5\n"},
		{`bota a = 0
mostra a ?? 5`, "0\n"},
		{`bota x = nada
mostra 1 + (x ?? 2)`, "3\n"},
		{`bota x = 7
mostra 1 + (x ?? 2)`, "8\n"},
		{`bota a = nada
bota b = nada
mostra a ?? b ?? 9`, "9\n"},
		{`bota a = nada
mostra a ?? 4 ?? 9`, "4\n"},
		{`mostra 1 ?? nada ?? 9`, "1\n"},
		{`bota m = {"a": {"b": 2}}
mostra m?.a?.b ?? 0
mostra m?.x?.b ?? 0`, "2\n0\n"},
		{`bota d = nada
mostra d?.x ?? "vazio"`, "vazio\n"},
		{`gambiarra f(v)
    bota r = v ?? "padrao"
    funciona r + "!"
acabou_finalmente
mostra f(nada)
mostra f("oi")`, "padrao!\noi!\n"},
		{`gambiarra g(v)
    funciona (v ?? 10) * 2
acabou_finalmente
mostra g(nada)
mostra g(3)`, "20\n6\n"},
		{`bota xs = [nada, 1, nada, 2]
bota out = []
pra_cada v em xs
    adiciona(out, v ?? 0)
acabou_finalmente
mostra out`, "[0, 1, 0, 2]\n"},
		{`bota f = gambiarra(v) funciona v ?? "l" acabou_finalmente
mostra f(nada) + f("k")`, "lk\n"},
		{`3 ?? 4
mostra "ok"`, "ok\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
}

// arruma/quebrou/finalmente dentro de funcao enxergam e escrevem nos params e
// locals da funcao (blocos dividem o escopo da funcao). Na VM o catch/try
// virava um escopo fechado sem frame proprio: "freevar fora do range".
func TestArrumaEnxergaEscopoDaFuncao(t *testing.T) {
	casos := []struct{ src, saida, erro string }{
		{`gambiarra f(x)
    bota y = 2
    arruma
        mostra x + y
    quebrou err
        mostra "caiu: " + erro_msg(err)
    acabou_finalmente
acabou_finalmente
f(1)`, "3\n", ""},
		// escrita no try e no finally vale pra funcao; var nova do try sobrevive
		{`gambiarra f(x)
    bota y = 2
    arruma
        mostra x + y
        bota y = y + 1
        bota z = 10
    quebrou err
        mostra "caiu: " + erro_msg(err)
    finalmente
        mostra "fin " + x + " " + y
        bota y = y + 100
    acabou_finalmente
    funciona [x, y, z]
acabou_finalmente
mostra f(1)`, "3\nfin 1 3\n[1, 103, 10]\n", ""},
		// catch le/escreve locals; nome do erro continua valendo depois
		{`gambiarra f(x)
    bota y = 2
    arruma
        quebra("ops")
    quebrou err
        mostra "caiu: " + erro_msg(err) + " x=" + x + " y=" + y
        bota y = y + x
    acabou_finalmente
    funciona y + tamanho(erro_msg(err))
acabou_finalmente
mostra f(5)`, "caiu: quebra: ops x=5 y=2\n18\n", ""},
		// funcoes aninhadas e lambdas criadas dentro do arruma
		{`gambiarra f(x)
    bota k = 3
    arruma
        gambiarra g(n)
            funciona n * x + k
        acabou_finalmente
        bota h = gambiarra(m) funciona m + x acabou_finalmente
        mostra g(2)
        mostra h(4)
        mostra mapeia([1, 2], gambiarra(v) funciona v * k acabou_finalmente)
    quebrou err
        mostra "caiu: " + erro_msg(err)
    acabou_finalmente
    funciona g(1)
acabou_finalmente
mostra f(10)`, "23\n14\n[3, 6]\n13\n", ""},
		// arruma dentro de lambda
		{`bota lam = gambiarra(a, b)
    bota t = 0
    arruma
        bota t = a / b
    quebrou err
        bota t = "div " + a
    acabou_finalmente
    funciona t
acabou_finalmente
mostra lam(6, 3)
mostra lam(1, 0)`, "2\ndiv 1\n", ""},
		// arruma dentro de laco dentro de funcao
		{`gambiarra soma_ok(xs)
    bota total = 0
    pra_cada v em xs
        arruma
            bota total = total + numero(v)
        quebrou err
            mostra "pulei " + v
        acabou_finalmente
    acabou_finalmente
    pra_cada i de 1 ate 3
        arruma
            bota total = total + i
        finalmente
            bota total = total * 1
        acabou_finalmente
    acabou_finalmente
    funciona total
acabou_finalmente
mostra soma_ok(["1", "x", "3"])`, "pulei x\n10\n", ""},
		// handler de HTTP: de_json dentro do arruma, funciona nos 2 ramos
		{`gambiarra trata(pedido)
    arruma
        bota dados = de_json(pedido["corpo"])
        funciona {"status": 200, "corpo": "oi " + dados["nome"]}
    quebrou err
        funciona {"status": 400, "corpo": "json ruim: " + pedido["corpo"]}
    acabou_finalmente
acabou_finalmente
mostra trata({"corpo": "{\"nome\": \"Ze\"}"})
mostra trata({"corpo": "{quebrado"})["status"]`, "{\"status\": 200, \"corpo\": \"oi Ze\"}\n400\n", ""},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, c.erro)
	}
}

// funciona/vaza/continua saindo de dentro do arruma: o finalmente roda antes
// de sair e o handler do try nao fica orfao (erro depois nao pode cair no
// quebrou de um arruma que ja acabou).
func TestArrumaSaidasAntecipadas(t *testing.T) {
	casos := []struct{ src, saida, erro string }{
		{`gambiarra acha(xs, alvo)
    pra_cada v em xs
        arruma
            se_colar v == alvo
                funciona "achei " + v
            acabou_finalmente
        quebrou err
            mostra "erro"
        finalmente
            mostra "fin " + v
        acabou_finalmente
    acabou_finalmente
    funciona "nao achei"
acabou_finalmente
mostra acha([1, 2, 3], 2)
mostra acha([1], 9)
quebra("fora")`, "fin 1\nfin 2\nachei 2\nfin 1\nnao achei\n", "fora"},
		{`gambiarra f()
    bota out = []
    pra_cada i de 1 ate 5
        arruma
            se_colar i == 2
                continua
            acabou_finalmente
            se_colar i == 4
                vaza
            acabou_finalmente
            adiciona(out, i)
        quebrou err
            mostra "erro"
        finalmente
            adiciona(out, "f" + i)
        acabou_finalmente
    acabou_finalmente
    funciona out
acabou_finalmente
mostra f()
bota n = 0
enquanto n < 3
    bota n = n + 1
    arruma
        se_colar n == 2
            continua
        acabou_finalmente
        mostra "n=" + n
    quebrou err
        mostra "erro"
    acabou_finalmente
acabou_finalmente
pra_cada i de 1 ate 3
    arruma
        vaza
    quebrou err
        mostra "erro"
    acabou_finalmente
acabou_finalmente
mostra "depois"
quebra("fora do laco")`, "[1, f1, f2, 3, f3, f4]\nn=1\nn=3\ndepois\n", "fora do laco"},
		// vaza de dentro do quebrou tambem roda o finalmente
		{`pra_cada i em [1, 2]
    arruma
        quebra("x")
    quebrou err
        vaza
    finalmente
        mostra "fin " + i
    acabou_finalmente
acabou_finalmente
mostra "ok"`, "fin 1\nok\n", ""},
		// arruma aninhado: funciona sai dos dois, finalmentes de dentro pra fora
		{`gambiarra f()
    arruma
        arruma
            funciona "r"
        finalmente
            mostra "dentro"
        acabou_finalmente
    finalmente
        mostra "fora"
    acabou_finalmente
acabou_finalmente
mostra f()`, "dentro\nfora\nr\n", ""},
		// funciona no finalmente ganha do funciona do try
		{`gambiarra h()
    arruma
        funciona 1
    finalmente
        funciona 2
    acabou_finalmente
acabou_finalmente
mostra h()`, "2\n", ""},
		// chamada em cauda dentro do arruma continua protegida pelo try
		{`gambiarra fat(n, acc)
    se_colar n <= 1
        funciona acc
    acabou_finalmente
    arruma
        funciona fat(n - 1, acc * n)
    quebrou err
        funciona -1
    acabou_finalmente
acabou_finalmente
mostra fat(5, 1)
gambiarra quebra_em(n)
    se_colar n == 0
        quebra("zero")
    acabou_finalmente
    arruma
        funciona quebra_em(n - 1)
    quebrou err
        funciona "pegou no " + n
    acabou_finalmente
acabou_finalmente
mostra quebra_em(3)`, "120\npegou no 1\n", ""},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, c.erro)
	}
}

// finalmente sem quebrou (ou com erro dentro do quebrou): o erro segue
// subindo DEPOIS do finalmente — o finalmente nao engole o erro.
func TestFinalmenteNaoEngoleErro(t *testing.T) {
	casos := []struct{ src, saida, erro string }{
		{`gambiarra f()
    arruma
        quebra("primeiro")
    finalmente
        mostra "limpa"
    acabou_finalmente
    mostra "nao chega"
acabou_finalmente
arruma
    f()
quebrou err
    mostra "pegou " + erro_msg(err)
acabou_finalmente`, "limpa\npegou quebra: primeiro\n", ""},
		{`gambiarra g()
    arruma
        quebra("a")
    quebrou err
        quebra("b")
    finalmente
        mostra "fin g"
    acabou_finalmente
acabou_finalmente
arruma
    g()
quebrou e2
    mostra "pegou " + erro_msg(e2)
acabou_finalmente`, "fin g\npegou quebra: b\n", ""},
		{`arruma
    quebra("solto")
finalmente
    mostra "fin"
acabou_finalmente
mostra "nao chega"`, "fin\n", "solto"},
		// vaza no finalmente descarta o erro pendente (igual Python)
		{`pra_cada i em [1, 2]
    arruma
        quebra("x")
    finalmente
        vaza
    acabou_finalmente
acabou_finalmente
mostra "ok"`, "ok\n", ""},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, c.erro)
	}
}

// fatiar lista devolve copia independente: mexer na fatia nao mexe na
// original (e vice-versa). Os 2 engines compartilhavam o array do Go.
func TestFatiaListaECopia(t *testing.T) {
	casos := []struct{ src, saida string }{
		{`bota xs = [1, 2, 3]
bota ys = xs[:2]
adiciona(ys, 7)
mostra xs
mostra ys`, "[1, 2, 3]\n[1, 2, 7]\n"},
		{`bota xs = [1, 2, 3]
bota ys = xs[:]
bota ys[0] = 9
mostra xs
mostra ys`, "[1, 2, 3]\n[9, 2, 3]\n"},
		{`bota xs = [1, 2, 3, 4]
bota ys = xs[1:3]
bota xs[1] = 0
remove(ys, 3)
mostra xs
mostra ys`, "[1, 0, 3, 4]\n[2]\n"},
		{`bota xs = [3, 1, 2]
bota ys = xs[0:2]
ordena(ys)
inverte(xs)
mostra xs
mostra ys`, "[2, 1, 3]\n[1, 3]\n"},
		{`mostra "abcdef"[1:3]`, "bc\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
}

// variavel do pra_cada vaza pro escopo com o ULTIMO valor iterado (igual
// Python). A VM deixava fim+1.
func TestPraCadaVariavelDepoisDoLaco(t *testing.T) {
	casos := []struct{ src, saida string }{
		{`pra_cada i de 1 ate 3
acabou_finalmente
mostra i`, "3\n"},
		{`gambiarra f()
    pra_cada i de 1 ate 3
    acabou_finalmente
    funciona i
acabou_finalmente
mostra f()`, "3\n"},
		{`pra_cada i de 1 ate 5
    se_colar i == 2
        vaza
    acabou_finalmente
acabou_finalmente
mostra i`, "2\n"},
		{`pra_cada i de 1 ate 3
    continua
acabou_finalmente
mostra i`, "3\n"},
		// faixa vazia nao roda e nao mexe na variavel
		{`bota i = 42
pra_cada i de 5 ate 1
    mostra "nunca"
acabou_finalmente
mostra i`, "42\n"},
		// mexer na variavel no corpo nao muda a contagem
		{`bota n = 0
pra_cada i de 1 ate 3
    bota i = i * 10
    bota n = n + 1
acabou_finalmente
mostra n
mostra i`, "3\n30\n"},
		// o fim e avaliado uma vez so
		{`bota lim = 3
bota n = 0
pra_cada i de 1 ate lim
    bota lim = 10
    bota n = n + 1
acabou_finalmente
mostra n`, "3\n"},
		{`pra_cada x em [1, 2, 9]
acabou_finalmente
mostra x`, "9\n"},
		{`bota x = "antes"
pra_cada x em []
acabou_finalmente
mostra x`, "antes\n"},
		{`pra_cada k, v em {"a": 1, "b": 2}
acabou_finalmente
mostra k + v`, "b2\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
}

// pra_cada ... em ... aninhado: os contadores escondidos do laco de fora e do
// de dentro eram o MESMO slot, entao o de fora parava depois da 1a volta.
func TestPraCadaListaAninhado(t *testing.T) {
	casos := []struct{ src, saida string }{
		{`pra_cada a em [1, 2]
    pra_cada b em [10, 20, 30]
        mostra a + b
    acabou_finalmente
acabou_finalmente`, "11\n21\n31\n12\n22\n32\n"},
		{`gambiarra g()
    bota out = []
    pra_cada a em [1, 2]
        pra_cada k, v em {"x": 3, "y": 4}
            adiciona(out, a * v)
        acabou_finalmente
    acabou_finalmente
    funciona out
acabou_finalmente
mostra g()`, "[3, 4, 6, 8]\n"},
		{`pra_cada i de 1 ate 2
    pra_cada j de 1 ate 2
        pra_cada c em ["p", "q"]
            mostra i + "" + j + c
        acabou_finalmente
    acabou_finalmente
acabou_finalmente`, "11p\n11q\n12p\n12q\n21p\n21q\n22p\n22q\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
}

// vaza dentro de uma funcao declarada dentro de um laco nao pode pular pro
// laco de fora (a VM emitia o jump no bytecode errado e panicava).
func TestVazaEmFuncaoDentroDeLacoNaoPanica(t *testing.T) {
	src := `pra_cada i de 1 ate 3
    gambiarra f()
        vaza
    acabou_finalmente
    f()
acabou_finalmente`
	_, _, errStr := rodaVMComp(t, src)
	if !strings.Contains(errStr, "vaza") {
		t.Fatalf("esperava erro de vaza fora de laco, veio %q", errStr)
	}
}
