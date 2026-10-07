package vm

import "testing"

// POO no modelo do Go (Tier 8): treta, metodo, combinado, puxadinho. Tudo
// roda nos 2 engines com a MESMA saida (esperaNosDois).

const pooPonto = `treta Ponto
    x
    y = 0
acabou_finalmente

gambiarra (p Ponto) distancia()
    funciona raiz(p.x * p.x + p.y * p.y)
acabou_finalmente

gambiarra (p Ponto) move(dx, dy = 0)
    bota p.x = p.x + dx
    p.y += dy
acabou_finalmente
`

func TestPOOTretaLiteralEMostra(t *testing.T) {
	casos := []struct{ src, saida string }{
		{pooPonto + `mostra Ponto{x: 3, y: 4}`, "Ponto{x: 3, y: 4}\n"},
		{pooPonto + `mostra Ponto{3, 4}`, "Ponto{x: 3, y: 4}\n"},
		// campo faltando: padrao declarado, senao nada
		{pooPonto + `mostra Ponto{y: 9}
mostra Ponto{}`, "Ponto{x: nada, y: 9}\nPonto{x: nada, y: 0}\n"},
		// texto sai com aspas (igual dicionario); virgula no fim vale
		{`treta P
    nome
acabou_finalmente
mostra P{nome: "ana",}
mostra "${P{"bia"}}"`, "P{nome: \"ana\"}\nP{nome: \"bia\"}\n"},
		// literal em varias linhas e dentro de expressao
		{pooPonto + `bota p = Ponto{
    x: 1,
    y: 2
}
mostra Ponto{1, 2}.distancia() == p.distancia()
se_colar p == Ponto{1, 2}
    mostra "igual"
acabou_finalmente`, "deu_bom\nigual\n"},
		{pooPonto + `mostra tipo(Ponto{1, 2})
mostra tipo(Ponto)
mostra tipo(Ponto{}.distancia)`, "Ponto\ntreta\nfuncao\n"},
		{pooPonto + `mostra Ponto`, "<treta Ponto>\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
}

func TestPOOMetodos(t *testing.T) {
	casos := []struct{ src, saida string }{
		{pooPonto + `bota p = Ponto{3, 4}
mostra p.distancia()
p.move(1, 1)
mostra p
p.move(2)
mostra p`, "5\nPonto{x: 4, y: 5}\nPonto{x: 6, y: 5}\n"},
		// por referencia: quem guarda a instancia ve a mudanca do metodo
		{pooPonto + `bota p = Ponto{0, 0}
bota xs = [p]
gambiarra empurra(q)
    q.move(10)
acabou_finalmente
empurra(p)
mostra xs[0].x`, "10\n"},
		// metodo como valor (receiver grudado), em builtin de ordem superior
		{pooPonto + `bota f = Ponto{3, 4}.distancia
mostra f()
bota ps = [Ponto{3, 4}, Ponto{6, 8}]
mostra mapeia(ps, gambiarra(q) funciona q.distancia() acabou_finalmente)
bota d = Ponto{0, 0}.move
d(5)
treta Conta
    total = 0
acabou_finalmente
gambiarra (c Conta) soma(n)
    c.total += n
    funciona c.total
acabou_finalmente
bota c = Conta{}
mapeia([1, 2, 3], c.soma)
mostra c.total`, "5\n[5, 10]\n6\n"},
		// metodo chamando metodo, varargs, recursao, spread
		{`treta Lista2
    itens = []
acabou_finalmente
gambiarra (l Lista2) poe(...xs)
    pra_cada x em xs
        adiciona(l.itens, x)
    acabou_finalmente
    funciona l
acabou_finalmente
gambiarra (l Lista2) conta(n = 0)
    se_colar n == tamanho(l.itens)
        funciona n
    acabou_finalmente
    funciona l.conta(n + 1)
acabou_finalmente
bota l = Lista2{}
l.poe(1, 2).poe(...[3, 4])
mostra l.conta()
mostra Lista2{}.itens`, "4\n[]\n"},
		// redeclarar metodo troca (igual gambiarra)
		{`treta A
acabou_finalmente
gambiarra (a A) f()
    funciona 1
acabou_finalmente
gambiarra (a A) f()
    funciona 2
acabou_finalmente
mostra A{}.f()`, "2\n"},
		// `bora` com metodo
		{pooPonto + `bota p = Ponto{3, 4}
bota fut = bora p.distancia()
mostra espera(fut)`, "5\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
}

func TestPOOPadraoRodaPorInstancia(t *testing.T) {
	src := `bota contador = 0
gambiarra proximo()
    bota contador = contador + 1
    funciona contador
acabou_finalmente
treta Pilha
    itens = []
    nome = "p" + texto(1 + 1)
    vazia = deu_bom
acabou_finalmente
bota a = Pilha{}
bota b = Pilha{}
adiciona(a.itens, 1)
mostra a.itens
mostra b.itens
mostra b.nome
mostra a.vazia
treta No
    valor
    prox = nada
acabou_finalmente
bota n = No{valor: 1}
bota n.prox = n
mostra n
mostra n == n`
	esperaNosDois(t, src, "[1]\n[]\np2\ndeu_bom\nNo{valor: 1, prox: No{...}}\ndeu_bom\n", "")
}

func TestPOOIgualdade(t *testing.T) {
	src := pooPonto + `treta Outro
    x
    y
acabou_finalmente
mostra Ponto{1, 2} == Ponto{1, 2}
mostra Ponto{1, 2} != Ponto{1, 3}
mostra Ponto{1, 2} == Outro{1, 2}
mostra Ponto{[1], {"a": 2}} == Ponto{[1], {"a": 2}}
mostra Ponto{1, 2} == {"x": 1, "y": 2}
escolhe Ponto{1, 2}
caso Ponto{2, 1}
    mostra "nao"
caso Ponto{1, 2}
    mostra "sim"
acabou_finalmente`
	esperaNosDois(t, src, "deu_bom\ndeu_bom\ndeu_ruim\ndeu_bom\ndeu_ruim\nsim\n", "")
}

const pooAnimais = `treta Animal
    nome = "anonimo"
    patas = 4
acabou_finalmente
gambiarra (a Animal) fala()
    funciona a.nome + " faz barulho"
acabou_finalmente
gambiarra (a Animal) renomeia(novo)
    bota a.nome = novo
acabou_finalmente

treta Cachorro
    Animal
    raca
acabou_finalmente
gambiarra (c Cachorro) fala()
    funciona c.nome + " late"
acabou_finalmente
`

func TestPOOPuxadinho(t *testing.T) {
	casos := []struct{ src, saida string }{
		// zero-value do puxadinho = a treta embutida zerada
		{pooAnimais + `mostra Cachorro{raca: "vira"}`,
			"Cachorro{Animal: Animal{nome: \"anonimo\", patas: 4}, raca: \"vira\"}\n"},
		// promotion de campo e metodo; o mais raso ganha (fala do Cachorro)
		{pooAnimais + `bota c = Cachorro{Animal{"rex", 3}, "vira"}
mostra c.nome
mostra c.patas
mostra c.fala()
mostra c.Animal.fala()
c.renomeia("toto")
mostra c.Animal.nome
bota c.patas = 5
mostra c.Animal.patas
bota c.Animal.nome = "bidu"
mostra c.nome`, "rex\n3\nrex late\nrex faz barulho\ntoto\n5\nbidu\n"},
		// nomeado com o campo do puxadinho; puxadinho dentro de puxadinho
		{pooAnimais + `treta Policial
    Cachorro
    distintivo = 7
acabou_finalmente
bota p = Policial{Cachorro: Cachorro{Animal: Animal{nome: "k9"}}}
mostra p.nome
mostra p.fala()
mostra p.distintivo
mostra p.Cachorro.Animal.patas`, "k9\nk9 late\n7\n4\n"},
		// pra_json achata o puxadinho (igual encoding/json do Go)
		{pooAnimais + `mostra pra_json(Cachorro{Animal{"rex", 4}, "vira"})
treta Pt
    x
    y
acabou_finalmente
mostra pra_json(Pt{1, [2, 3]})`, "{\"nome\":\"rex\",\"patas\":4,\"raca\":\"vira\"}\n{\"x\":1,\"y\":[2,3]}\n"},
		// desestruturacao pelos campos
		{pooAnimais + `bota {nome, raca, sumiu} = Cachorro{Animal{"rex", 4}, "vira"}
mostra nome + " " + raca
mostra sumiu`, "rex vira\nnada\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
}

func TestPOOCombinadoETypeSwitch(t *testing.T) {
	src := `combinado Forma
    area()
    nome()
acabou_finalmente
combinado Escritor
    escreve(texto)
acabou_finalmente
combinado FormaEscrita
    Forma
    Escritor
acabou_finalmente
combinado Qualquer
acabou_finalmente

treta Quadrado
    lado
acabou_finalmente
gambiarra (q Quadrado) area()
    funciona q.lado * q.lado
acabou_finalmente
gambiarra (q Quadrado) nome()
    funciona "quadrado"
acabou_finalmente

treta Circulo
    r
acabou_finalmente
gambiarra (c Circulo) area()
    funciona 3 * c.r * c.r
acabou_finalmente

treta Moldura
    Quadrado
acabou_finalmente
gambiarra (m Moldura) escreve(t)
    funciona t
acabou_finalmente

bota q = Quadrado{2}
mostra satisfaz(q, Forma)
mostra satisfaz(Circulo{1}, Forma)
mostra satisfaz(Moldura{Quadrado{3}}, Forma)
mostra satisfaz(Moldura{Quadrado{3}}, FormaEscrita)
mostra satisfaz(q, FormaEscrita)
mostra satisfaz(5, Forma)
mostra satisfaz(5, Qualquer)
mostra satisfaz(q, Quadrado)
mostra satisfaz(Moldura{}, Quadrado)
mostra como_tipo(q, Forma).area()
mostra tipo(Forma)
mostra Forma

gambiarra descreve(v)
    escolhe tipo(v)
    caso "Quadrado"
        funciona "quadrado de lado " + texto(v.lado)
    caso "Circulo"
        funciona "circulo de raio " + texto(v.r)
    se_nao_colar
        funciona "sei la: " + tipo(v)
    acabou_finalmente
acabou_finalmente
pra_cada f em [q, Circulo{5}, Moldura{}, 7]
    mostra descreve(f)
acabou_finalmente`
	saida := "deu_bom\ndeu_ruim\ndeu_bom\ndeu_bom\ndeu_ruim\ndeu_ruim\ndeu_bom\ndeu_bom\ndeu_ruim\n4\ncombinado\n<combinado Forma>\n" +
		"quadrado de lado 2\ncirculo de raio 5\nsei la: Moldura\nsei la: numero\n"
	esperaNosDois(t, src, saida, "")

	// satisfacao confere quantos parametros o metodo aceita
	esperaNosDois(t, `combinado Escritor
    escreve(texto)
acabou_finalmente
treta A
acabou_finalmente
gambiarra (a A) escreve()
    funciona 1
acabou_finalmente
treta B
acabou_finalmente
gambiarra (b B) escreve(t, extra = 0)
    funciona 1
acabou_finalmente
treta C
acabou_finalmente
gambiarra (c C) escreve(...ts)
    funciona 1
acabou_finalmente
mostra satisfaz(A{}, Escritor)
mostra satisfaz(B{}, Escritor)
mostra satisfaz(C{}, Escritor)`, "deu_ruim\ndeu_bom\ndeu_bom\n", "")
}

func TestPOOErros(t *testing.T) {
	casos := []struct{ src, erro string }{
		{pooPonto + `mostra Ponto{z: 1}`, "treta Ponto nao tem campo `z`"},
		{pooPonto + `mostra Ponto{1}`, "Ponto{...} na ordem quer os 2 campo(s) (x, y), veio 1"},
		{pooPonto + `mostra Ponto{x: 1, x: 2}`, "campo `x` repetido no Ponto{...}"},
		{pooPonto + `mostra Ponto{1, 2}.z`, "treta Ponto nao tem campo nem metodo `z`"},
		{pooPonto + `bota p = Ponto{1, 2}
bota p.z = 3`, "treta Ponto nao tem campo `z` (campo novo so declarando na treta)"},
		{pooPonto + `bota p = Ponto{1, 2}
bota p.distancia = 3`, "`distancia` e metodo da treta Ponto, nao campo"},
		{pooPonto + `mostra Ponto{1, 2}[0]`, "campo de treta e por nome (texto), veio numero"},
		{pooPonto + `Ponto{1, 2}.move()`, "o metodo Ponto.move quer entre 1 e 2 parametro(s), voce mandou 0"},
		{pooPonto + `Ponto{1, 2}.distancia(1)`, "o metodo Ponto.distancia quer 0 parametro(s), voce mandou 1"},
		{pooPonto + `gambiarra (p Ponto) x()
    funciona 1
acabou_finalmente`, "a treta Ponto ja tem o campo `x`: metodo nao pode ter o mesmo nome de campo"},
		{`bota Ponto = 3
mostra Ponto{1}`, "nao da pra fazer Ponto{...}: `Ponto` nao e treta (e numero)"},
		{`treta A
    x
    x
acabou_finalmente`, "campo `x` repetido na treta A"},
		{`bota B = 1
treta A
    B
acabou_finalmente`, "puxadinho na treta A: `B` nao e treta (e numero)"},
		{`bota F = 1
gambiarra (f F) m()
acabou_finalmente`, "metodo m: `F` nao e treta (e numero)"},
		{pooAnimais + `mostra Cachorro{nome: "rex"}`, "`nome` e campo promovido do puxadinho Animal"},
		{pooAnimais + `mostra Cachorro{Animal: 3}`, "o campo Animal de Cachorro e o puxadinho da treta Animal: so aceita Animal{...}, veio numero"},
		{pooAnimais + `bota c = Cachorro{}
bota c.Animal = nada`, "so aceita Animal{...}, veio nada"},
		// ambiguo: o mesmo nome em dois puxadinhos na mesma profundidade
		{`treta A
    nome = "a"
acabou_finalmente
treta B
    nome = "b"
acabou_finalmente
treta C
    A
    B
acabou_finalmente
mostra C{}.nome`, "`nome` e ambiguo em C: tem em A e B na mesma profundidade"},
		{`combinado F
    area()
acabou_finalmente
treta Q
acabou_finalmente
como_tipo(Q{}, F)`, "como_tipo: Q nao satisfaz F: falta o metodo area"},
		{`treta Q
acabou_finalmente
treta R
acabou_finalmente
como_tipo(R{}, Q)`, "como_tipo: esperava Q, veio R"},
		{`satisfaz(1, 2)`, "o Tipo tem que ser treta ou combinado, veio numero"},
		{`combinado F
    a()
acabou_finalmente
combinado G
    a(x)
    F
acabou_finalmente`, "o combinado G tem o metodo `a` duas vezes com numero de parametros diferente"},
		{`treta P
    xs = quebra("padrao quebrou")
acabou_finalmente
P{}`, "padrao quebrou"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, "", c.erro)
	}
	// o que imprimiu antes do erro vale
	esperaNosDois(t, `treta A
    nome = "a"
acabou_finalmente
treta B
    nome = "b"
acabou_finalmente
treta C
    A
    B
acabou_finalmente
bota c = C{}
mostra c.A.nome + c.B.nome
mostra c.nome`, "ab\n", "ambiguo")
}

// arruma pega erro de POO como qualquer outro
func TestPOOErroCapturavel(t *testing.T) {
	esperaNosDois(t, pooPonto+`arruma
    mostra Ponto{1, 2}.nada_disso
quebrou err
    mostra erro_msg(err)
acabou_finalmente`, "deu ruim na linha 15: treta Ponto nao tem campo nem metodo `nada_disso`\n", "")
}

func TestPOOModulo(t *testing.T) {
	geo := `treta Ponto
    x
    y
acabou_finalmente
gambiarra (p Ponto) soma()
    funciona p.x + p.y
acabou_finalmente
gambiarra nova_ponto(x, y)
    funciona Ponto{x, y}
acabou_finalmente
`
	esperaModulos(t, map[string]string{
		"geo.gs": geo,
		"main.gs": `importa "geo.gs" como geo
bota p = geo.Ponto{x: 1, y: 2}
mostra p.soma()
mostra geo.nova_ponto(3, 4)
treta Ponto3
    geo.Ponto
    z = 0
acabou_finalmente
mostra Ponto3{}.soma == nada
bota q = Ponto3{Ponto: geo.Ponto{1, 1}, z: 1}
mostra q.soma()
`,
	}, "main.gs", "3\nPonto{x: 3, y: 4}\ndeu_ruim\n2\n", "")
	esperaModulos(t, map[string]string{
		"geo.gs": geo,
		"main.gs": `importa "geo.gs"
mostra Ponto{5, 5}.soma()
`,
	}, "main.gs", "10\n", "")
}

// Instancia mexida por varias goroutines: cada leitura/escrita de campo e
// atomica (igual as colecoes); a soma composta usa com_trava. Rode com -race.
func TestPOOConcorrencia(t *testing.T) {
	src := `treta Placar
    pontos = 0
    log = []
acabou_finalmente
gambiarra (p Placar) marca(n)
    bota p.pontos = p.pontos + n
    adiciona(p.log, n)
acabou_finalmente
gambiarra (p Placar) le()
    funciona p.pontos
acabou_finalmente
bota pl = Placar{}
bota t = trava()
gambiarra trabalha(i)
    pra_cada k de 1 ate 50
        com_trava(t, gambiarra() pl.marca(1) acabou_finalmente)
        pl.le()
        bota visto = pl.log
    acabou_finalmente
    funciona i
acabou_finalmente
bota futs = []
pra_cada i de 1 ate 8
    adiciona(futs, bora trabalha(i))
acabou_finalmente
espera(futs)
mostra pl.pontos
mostra tamanho(pl.log)
gambiarra (p Placar) zera()
    bota p.pontos = 0
acabou_finalmente
pl.zera()
mostra pl.pontos`
	esperaNosDois(t, src, "400\n400\n0\n", "")
}

// Escopo de funcao (estilo Python) com POO: gambiarra declarada antes da
// treta enxerga ela; closure dentro de literal/metodo ve a variavel, nao uma
// copia; padrao de campo le global declarada depois.
func TestPOOEscopo(t *testing.T) {
	src := `gambiarra cria(a)
    funciona Ponto{x: a, y: a * 2}
acabou_finalmente
treta Ponto
    x
    y
    rotulo = prefixo + "!"
acabou_finalmente
bota prefixo = "pt"
mostra cria(2)
gambiarra f()
    bota n = 1
    bota g = gambiarra() funciona Ponto{x: n, y: n} acabou_finalmente
    bota n = 5
    funciona g()
acabou_finalmente
mostra f()
gambiarra (p Ponto) contador()
    bota conta = gambiarra()
        p.x += 1
        funciona p.x
    acabou_finalmente
    conta()
    funciona conta()
acabou_finalmente
mostra Ponto{x: 0}.contador()`
	esperaNosDois(t, src, "Ponto{x: 2, y: 4, rotulo: \"pt!\"}\nPonto{x: 5, y: 5, rotulo: \"pt!\"}\n2\n", "")

	// local do metodo que nao foi botado nessa chamada le a global (nao o
	// lixo da chamada anterior)
	esperaNosDois(t, `bota total = 100
treta C
acabou_finalmente
gambiarra (c C) f(flag)
    se_colar flag
        bota total = 1
    acabou_finalmente
    funciona total
acabou_finalmente
bota c = C{}
mostra c.f(deu_bom)
mostra c.f(deu_ruim)`, "1\n100\n", "")

	// `funciona g()` (tail call) com g = metodo ligado, com padrao e ...resto
	esperaNosDois(t, `treta A
    n = 2
acabou_finalmente
gambiarra (a A) soma(x = 10, ...ys)
    funciona a.n + x + tamanho(ys)
acabou_finalmente
bota g = A{}.soma
gambiarra g2()
    funciona g2()
acabou_finalmente
gambiarra h()
    funciona h()
acabou_finalmente
bota orig_g2 = g2
bota orig_h = h
bota g2 = g
bota h = A{n: 5}.soma
mostra orig_g2()
mostra orig_h()
mostra g(1, 9, 9)`, "12\n15\n5\n", "")
}
