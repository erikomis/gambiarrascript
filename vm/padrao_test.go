package vm

import "testing"

// Pattern matching no escolhe/caso e cardapio (enum): mesma saida nos dois
// engines.

func TestPadraoNosDois(t *testing.T) {
	casos := []struct{ nome, src, saida, erro string }{
		{"valor de sempre", `
bota x = 2
escolhe 2
caso 1
    mostra "um"
caso x
    mostra "x"
acabou_finalmente`, "x\n", ""},
		{"lista exata e resto", `
gambiarra f(v)
    escolhe v
    caso []
        funciona "vazia"
    caso [a]
        funciona "um: ${a}"
    caso [a, b]
        funciona "dois: ${a} ${b}"
    caso [primeiro, ...resto]
        funciona "muitos: ${primeiro} + ${resto}"
    se_nao_colar
        funciona "nem lista"
    acabou_finalmente
acabou_finalmente
mostra f([])
mostra f([7])
mostra f([1, 2])
mostra f([1, 2, 3])
mostra f("oi")`, "vazia\num: 7\ndois: 1 2\nmuitos: 1 + [2, 3]\nnem lista\n", ""},
		{"literal e curinga na lista", `
pra_cada v em [[0, 5], [1, 5], [0, 6, 7]]
    escolhe v
    caso [0, _]
        mostra "zero e qualquer"
    caso [_, ..._]
        mostra "outro"
    acabou_finalmente
acabou_finalmente`, "zero e qualquer\noutro\noutro\n", ""},
		{"dicionario subconjunto", `
gambiarra trata(ev)
    escolhe ev
    caso {"tipo": "erro", "msg": m}
        funciona "erro: " + m
    caso {"tipo": "ok", valor}
        funciona "ok: ${valor}"
    caso {}
        funciona "dicionario qualquer"
    acabou_finalmente
    funciona "nada casou"
acabou_finalmente
mostra trata({"tipo": "erro", "msg": "caiu", "extra": 1})
mostra trata({"tipo": "ok", "valor": 42})
mostra trata({"tipo": "ok"})
mostra trata([1])`, "erro: caiu\nok: 42\ndicionario qualquer\nnada casou\n", ""},
		{"treta", `
treta Ponto
    x
    y = 0
acabou_finalmente
treta Outro
    x
acabou_finalmente
gambiarra onde(p)
    escolhe p
    caso Ponto{x: 0, y: 0}
        funciona "origem"
    caso Ponto{x: 0, y}
        funciona "no eixo y em ${y}"
    caso Ponto{x, y}
        funciona "em ${x},${y}"
    se_nao_colar
        funciona "nao e ponto"
    acabou_finalmente
acabou_finalmente
mostra onde(Ponto{0, 0})
mostra onde(Ponto{0, 3})
mostra onde(Ponto{2, 3})
mostra onde(Outro{0})`, "origem\nno eixo y em 3\nem 2,3\nnao e ponto\n", ""},
		{"aninhado e guarda", `
gambiarra f(v)
    escolhe v
    caso [{"n": n}, [a, ...r]]
        funciona "n=${n} a=${a} r=${r}"
    caso [x, y] se x > y
        funciona "desce"
    caso [x, y] se x < y
        funciona "sobe"
    caso [_, _]
        funciona "igual"
    acabou_finalmente
acabou_finalmente
mostra f([3, 1])
mostra f([1, 3])
mostra f([2, 2])
mostra f([{"n": 9}, [1, 2]])`, "desce\nsobe\nigual\nn=9 a=1 r=[2]\n", ""},
		{"guarda em valor comum e alternativas", `
pra_cada n em [1, 2, 3, 4]
    escolhe n
    caso 1, 2 se n % 2 == 0
        mostra "dois"
    caso [x], 3, 4
        mostra "tres ou quatro"
    se_nao_colar
        mostra "resto"
    acabou_finalmente
acabou_finalmente`, "resto\ndois\ntres ou quatro\ntres ou quatro\n", ""},
		{"nomes amarrados sao locais da funcao", `
bota a = "global"
gambiarra f(v)
    escolhe v
    caso [a]
        mostra "dentro: " + a
    acabou_finalmente
    funciona a
acabou_finalmente
mostra f(["local"])
mostra a`, "dentro: local\nlocal\nglobal\n", ""},
		{"amarra antes da guarda", `
bota x = 0
escolhe [5]
caso [x] se x > 10
    mostra "grande"
se_nao_colar
    mostra "x virou ${x}"
acabou_finalmente`, "x virou 5\n", ""},
		{"curinga no topo", `
escolhe 7
caso 1
    mostra "um"
caso _ se deu_bom
    mostra "qualquer"
acabou_finalmente`, "qualquer\n", ""},
		{"tipo do padrao nao e treta", `
bota Coisa = 3
escolhe 1
caso Coisa{x}
    mostra x
acabou_finalmente`, "", "tem que ser treta"},
		{"campo que nao existe", `
treta P
    x
acabou_finalmente
escolhe P{1}
caso P{z: w}
    mostra w
acabou_finalmente`, "", "z"},
		{"treta na ordem", `
treta P
    x
    y
acabou_finalmente
pra_cada p em [P{1, 2}, P{0, 5}]
    escolhe p
    caso P{0, y}
        mostra "zero e ${y}"
    caso P{a, b}
        mostra "${a} e ${b}"
    acabou_finalmente
acabou_finalmente`, "1 e 2\nzero e 5\n", ""},
		{"treta na ordem com campo faltando", `
treta P
    x
    y
acabou_finalmente
escolhe P{1, 2}
caso P{x}
    mostra x
acabou_finalmente`, "", "na ordem tem que passar os 2 campos"},
		{"cardapio basico", `
cardapio Cor
    vermelho
    verde
    azul
acabou_finalmente
bota c = Cor.verde
mostra c
mostra tipo(c)
mostra c.nome
mostra c.indice
mostra c == Cor.verde
mostra c == Cor.azul
mostra Cor.verde == "verde"
pra_cada op em Cor
    mostra op
acabou_finalmente
pra_cada i, op em Cor
    mostra "${i}:${op.nome}"
acabou_finalmente
mostra Cor
mostra tipo(Cor)`, "Cor.verde\nCor\nverde\n1\ndeu_bom\ndeu_ruim\ndeu_ruim\nCor.vermelho\nCor.verde\nCor.azul\n0:vermelho\n1:verde\n2:azul\n<cardapio Cor>\ncardapio\n", ""},
		{"cardapio no escolhe e como chave", `
cardapio Sinal
    verde
    amarelo
    vermelho
acabou_finalmente
gambiarra proximo(s)
    escolhe s
    caso Sinal.verde
        funciona Sinal.amarelo
    caso Sinal.amarelo
        funciona Sinal.vermelho
    se_nao_colar
        funciona Sinal.verde
    acabou_finalmente
acabou_finalmente
mostra proximo(Sinal.verde)
mostra proximo(Sinal.vermelho)
bota tempo = {Sinal.verde: 30, Sinal.vermelho: 20}
mostra tempo[Sinal.verde]
escolhe [Sinal.vermelho, 3]
caso [Sinal.verde, n]
    mostra "verde ${n}"
caso [Sinal.vermelho, n]
    mostra "vermelho ${n}"
acabou_finalmente`, "Sinal.amarelo\nSinal.verde\n30\nvermelho 3\n", ""},
		{"opcao que nao existe", `
cardapio Cor
    azul
acabou_finalmente
mostra Cor.roxo`, "", "o cardapio Cor nao tem roxo"},
		{"campo de opcao que nao existe", `
cardapio Cor
    azul
acabou_finalmente
mostra Cor.azul.cor`, "", "so tem .nome e .indice"},
		{"padrao nao reatribui crava", `
crava x = 1
escolhe [2]
caso [x]
    mostra x
acabou_finalmente`, "", "crava"},
		{"closure enxerga nome amarrado", `
gambiarra f(v)
    escolhe v
    caso {"n": n}
        funciona gambiarra() funciona n * 2 acabou_finalmente
    acabou_finalmente
acabou_finalmente
mostra f({"n": 21})()`, "42\n", ""},
		{"cardapio de modulo-like e iteracao vazia de nada", `
cardapio Dia
    seg
    ter
acabou_finalmente
bota n = 0
pra_cada d em Dia
    n += d.indice + 1
acabou_finalmente
mostra n
mostra Dia.ter != Dia.seg`, "3\ndeu_bom\n", ""},
		{"cardapio no json, conjunto e texto", `
cardapio Cor
    azul
    verde
acabou_finalmente
mostra pra_json({"c": Cor.azul, Cor.verde: [Cor.verde]})
mostra tamanho(conjunto([Cor.azul, Cor.azul, Cor.verde]))
mostra "cor: ${Cor.verde}" + " " + texto(Cor.azul)
mostra [Cor.azul]`, "{\"c\":\"azul\",\"verde\":[\"verde\"]}\n2\ncor: Cor.verde Cor.azul\n[Cor.azul]\n", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			esperaNosDois(t, c.src, c.saida, c.erro)
		})
	}
}
