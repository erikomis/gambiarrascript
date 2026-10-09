package vm

import (
	"testing"
)

// Geradores (`rende`) e o protocolo de iteracao das tretas (itera()):
// mesma saida e mesmo erro nos dois engines.

const geradorConta = `gambiarra conta(n)
    pra_cada i de 1 ate n
        rende i * 10
    acabou_finalmente
acabou_finalmente
`

const geradorNaturais = `gambiarra naturais()
    bota n = 0
    enquanto deu_bom
        rende n
        n += 1
    acabou_finalmente
acabou_finalmente
`

func TestGeradorBasico(t *testing.T) {
	esperaNosDois(t, geradorConta+`bota g = conta(3)
mostra tipo(g)
mostra g
pra_cada x em g
    mostra x
acabou_finalmente
pra_cada x em g
    mostra "de novo " + texto(x)
acabou_finalmente
mostra proximo(g)
mostra proximo(g, "fim")
mostra acabou(g)`, "gerador\n<gerador conta>\n10\n20\n30\nnada\nfim\ndeu_bom\n", "")
}

// Chamar nao roda o corpo; cada valor pedido roda ate o proximo rende.
func TestGeradorPreguicoso(t *testing.T) {
	esperaNosDois(t, `gambiarra g()
    mostra "comecou"
    rende 1
    mostra "meio"
    rende 2
    mostra "fim"
acabou_finalmente
bota x = g()
mostra "chamou"
mostra proximo(x)
mostra "entre"
mostra proximo(x)
mostra proximo(x)
mostra acabou(x)`, "chamou\ncomecou\n1\nentre\nmeio\n2\nfim\nnada\ndeu_bom\n", "")
}

// Gerador infinito + vaza; o mesmo gerador continua de onde parou.
func TestGeradorInfinitoVaza(t *testing.T) {
	esperaNosDois(t, geradorNaturais+geradorConta+`bota g = naturais()
pra_cada x em g
    se_colar x == 3
        vaza
    acabou_finalmente
acabou_finalmente
mostra proximo(g)
pra_cada i, x em naturais()
    se_colar x > 2
        vaza
    acabou_finalmente
    se_colar x == 1
        continua
    acabou_finalmente
    mostra "${i}:${x}"
acabou_finalmente
mostra pega(naturais(), 4)
mostra pega(conta(2), 5)`, "4\n0:0\n2:2\n[0, 1, 2, 3]\n[10, 20]\n", "")
}

// `nada` rendido e valor: proximo(g, padrao) e acabou(g) separam do fim.
func TestGeradorRendeNada(t *testing.T) {
	esperaNosDois(t, `gambiarra g()
    rende nada
    rende 0
acabou_finalmente
bota x = g()
mostra acabou(x)
mostra proximo(x, "fim")
mostra proximo(x, "fim")
mostra proximo(x, "fim")
mostra acabou(x)
mostra lista(g())`, "deu_ruim\nnada\n0\nfim\ndeu_bom\n[nada, 0]\n", "")
}

// acabou() pode rodar o corpo antes (o valor fica guardado pro proximo).
func TestGeradorAcabouAdianta(t *testing.T) {
	esperaNosDois(t, `gambiarra g()
    mostra "rodou"
    rende 7
acabou_finalmente
bota x = g()
mostra acabou(x)
mostra "depois"
mostra proximo(x)`, "rodou\ndeu_ruim\ndepois\n7\n", "")
}

// `funciona` encerra o gerador (o valor e descartado); recursao em cauda
// num gerador nao vira tail call (so encerra).
func TestGeradorFunciona(t *testing.T) {
	esperaNosDois(t, `gambiarra ate_tres()
    bota n = 1
    enquanto deu_bom
        se_colar n > 3
            funciona "ignorado"
        acabou_finalmente
        rende n
        n += 1
    acabou_finalmente
acabou_finalmente
mostra lista(ate_tres())
gambiarra cauda(n)
    rende n
    funciona cauda(n + 1)
acabou_finalmente
mostra lista(cauda(1))`, "[1, 2, 3]\n[1]\n", "")
}

// Parametros sao amarrados na chamada (padrao inclusive, com efeito na hora).
func TestGeradorParametros(t *testing.T) {
	esperaNosDois(t, `gambiarra padrao()
    mostra "padrao rodou"
    funciona 2
acabou_finalmente
gambiarra repete(x, vezes = padrao(), ...resto)
    pra_cada i de 1 ate vezes
        rende [x, resto]
    acabou_finalmente
acabou_finalmente
bota a = 5
bota g = repete(a)
mostra "criou"
bota a = 6
mostra lista(g)
mostra lista(repete("x", 1, 7, 8))`, "padrao rodou\ncriou\n[[5, []], [5, []]]\n[[x, [7, 8]]]\n", "")
}

// Lambda, closure, metodo e dois geradores independentes da mesma gambiarra.
func TestGeradorLambdaClosureMetodo(t *testing.T) {
	esperaNosDois(t, `bota base = 100
bota soma = gambiarra(n)
    pra_cada i de 1 ate n
        rende base + i
    acabou_finalmente
acabou_finalmente
bota g = soma(2)
bota base = 200
mostra lista(g)
treta Contador
    limite
acabou_finalmente
gambiarra (c Contador) numeros()
    pra_cada i de 1 ate c.limite
        rende i
    acabou_finalmente
acabou_finalmente
bota c = Contador{3}
bota a = c.numeros()
bota b = c.numeros()
mostra proximo(a)
mostra proximo(a)
mostra proximo(b)
mostra a`, "[201, 202]\n1\n2\n1\n<gerador Contador.numeros>\n", "")
}

// Gerador consumindo gerador (pipeline) e recursao (percorre arvore).
func TestGeradorAninhadoERecursivo(t *testing.T) {
	esperaNosDois(t, geradorNaturais+`gambiarra pares(g)
    pra_cada x em g
        se_colar x % 2 == 0
            rende x
        acabou_finalmente
    acabou_finalmente
acabou_finalmente
gambiarra primeiros(g, n)
    pra_cada i, x em g
        se_colar i >= n
            vaza
        acabou_finalmente
        rende x
    acabou_finalmente
acabou_finalmente
mostra lista(primeiros(pares(naturais()), 4))
gambiarra folhas(no)
    se_colar tipo(no) != "lista"
        rende no
        funciona nada
    acabou_finalmente
    pra_cada filho em no
        pra_cada f em folhas(filho)
            rende f
        acabou_finalmente
    acabou_finalmente
acabou_finalmente
mostra lista(folhas([1, [2, [3, 4]], [[5]]]))`, "[0, 2, 4, 6]\n[1, 2, 3, 4, 5]\n", "")
}

// mapeia/filtra/reduz consomem o gerador inteiro; lista() materializa.
func TestGeradorBuiltinsDeLista(t *testing.T) {
	esperaNosDois(t, geradorConta+`mostra mapeia(conta(3), gambiarra(x) funciona x + 1 acabou_finalmente)
mostra filtra(conta(4), gambiarra(x) funciona x > 15 acabou_finalmente)
mostra reduz(conta(3), gambiarra(a, x) funciona a + x acabou_finalmente, 0)
mostra lista(conta(0))
mostra lista([1, 2])
mostra lista(conjunto([3, 3, 4]))
mostra lista({"a": 1, "b": 2})`, "[11, 21, 31]\n[20, 30, 40]\n60\n[]\n[1, 2]\n[3, 4]\n[a, b]\n", "")
}

func TestGeradorBuiltinsErros(t *testing.T) {
	esperaNosDois(t, `mostra proximo([1])`, "", "proximo() espera um gerador, veio lista")
	esperaNosDois(t, geradorConta+`mostra pega(conta(2), -1)`, "", "pega() quer quantos")
	esperaNosDois(t, `mostra lista(3)`, "", "lista() quer gerador, lista, conjunto, dicionario, cardapio ou treta com itera(), veio numero")
	esperaNosDois(t, `mostra tamanho(lista(3))`, "", "veio numero")
}

// Erro dentro do gerador sobe no ponto do consumo com a linha de dentro do
// gerador; da pra pegar com arruma, e o gerador acabou depois do erro.
func TestGeradorErroPropaga(t *testing.T) {
	src := `gambiarra quebra_no_dois()
    rende 1
    bota x = 1 / 0
    rende 2
acabou_finalmente
bota g = quebra_no_dois()
arruma
    pra_cada v em g
        mostra v
    acabou_finalmente
quebrou erro
    mostra "pegou: " + erro_msg(erro)
acabou_finalmente
mostra proximo(g, "acabou")
arruma
    mostra proximo(quebra_no_dois())
    mostra proximo(proximo(quebra_no_dois()))
quebrou erro
    mostra erro_linha(erro)
acabou_finalmente
pra_cada v em quebra_no_dois()
    mostra v
acabou_finalmente`
	esperaNosDois(t, src, "1\npegou: deu ruim na linha 3: nao da pra dividir por zero, parca — nem na gambiarra\nacabou\n1\n0\n1\n", "deu ruim na linha 3")
}

// O erro levantado no gerador (quebra) e o erro de dentro de uma gambiarra
// que o corpo chama chegam iguais; o traco aponta pras chamadas de dentro.
func TestGeradorErroQuebraEPilha(t *testing.T) {
	src := `gambiarra falha()
    quebra("estourou")
acabou_finalmente
gambiarra g()
    rende 1
    falha()
acabou_finalmente
gambiarra consome()
    funciona lista(g())
acabou_finalmente
arruma
    consome()
quebrou erro
    mostra erro_msg(erro)
    mostra erro_pilha(erro)
acabou_finalmente`
	esperaNosDois(t, src, "quebra: estourou\n[{\"funcao\": \"consome\", \"linha\": 12}, {\"funcao\": \"lista\", \"linha\": 9}, {\"funcao\": \"falha\", \"linha\": 6}, {\"funcao\": \"quebra\", \"linha\": 2}]\n", "")
}

// rende dentro de arruma: o erro depois do rende e pego dentro do gerador;
// vaza/continua dentro de laco do corpo; sai() no gerador sai do programa.
func TestGeradorArrumaLacos(t *testing.T) {
	esperaNosDois(t, `gambiarra g()
    arruma
        rende "a"
        bota x = [][3]
        rende "nunca"
    quebrou erro
        rende "pegou"
    finalmente
        rende "fim"
    acabou_finalmente
    pra_cada i de 1 ate 10
        se_colar i == 2
            continua
        acabou_finalmente
        se_colar i == 4
            vaza
        acabou_finalmente
        rende i
    acabou_finalmente
acabou_finalmente
mostra lista(g())`, "[a, pegou, fim, 1, 3]\n", "")
}

// rende fora de gambiarra e erro de parse (nos dois, o mesmo parser).
func TestGeradorRendeForaDeGambiarra(t *testing.T) {
	esperaNosDois(t, "rende 1", "", "rende fora de gambiarra")
}

// O gerador que pede o proprio valor enquanto roda daria deadlock: e erro.
func TestGeradorChamaEleMesmo(t *testing.T) {
	esperaNosDois(t, `bota g = nada
gambiarra eu()
    rende proximo(g)
acabou_finalmente
bota g = eu()
mostra proximo(g)`, "", "pediu o proprio proximo valor enquanto rodava")
}

// Gerador criado e consumido em goroutines do bora.
func TestGeradorComBora(t *testing.T) {
	esperaNosDois(t, geradorConta+`bota g = conta(100)
gambiarra soma_metade()
    bota s = 0
    pra_cada i de 1 ate 50
        s += proximo(g)
    acabou_finalmente
    funciona s
acabou_finalmente
bota a = bora soma_metade()
bota b = bora soma_metade()
mostra espera(a) + espera(b)
bota f = bora conta(3)
mostra lista(espera(f))`, "50500\n[10, 20, 30]\n", "")
}

// ---------------------------------------------------------------- treta itera

const tretaCaixa = `treta Caixa
    itens
acabou_finalmente
gambiarra (c Caixa) itera()
    funciona c.itens
acabou_finalmente
`

func TestTretaIteraLista(t *testing.T) {
	esperaNosDois(t, tretaCaixa+`bota c = Caixa{[3, 4]}
pra_cada x em c
    mostra x
acabou_finalmente
pra_cada i, x em c
    mostra "${i}=${x}"
acabou_finalmente
mostra lista(c)
bota d = Caixa{{"a": 1, "b": 2}}
pra_cada k, v em d
    mostra "${k}:${v}"
acabou_finalmente
pra_cada k em d
    mostra k
acabou_finalmente
pra_cada i, x em Caixa{conjunto([9, 8])}
    mostra "${i}/${x}"
acabou_finalmente`, "3\n4\n0=3\n1=4\n[3, 4]\na:1\nb:2\na\nb\n0/9\n1/8\n", "")
}

func TestTretaIteraGerador(t *testing.T) {
	esperaNosDois(t, `treta Intervalo
    inicio
    fim
acabou_finalmente
gambiarra (r Intervalo) itera()
    bota i = r.inicio
    enquanto i <= r.fim
        rende i
        i += 1
    acabou_finalmente
acabou_finalmente
treta Pilha
    Intervalo
acabou_finalmente
pra_cada i, x em Intervalo{2, 4}
    mostra "${i}:${x}"
acabou_finalmente
pra_cada x em Pilha{Intervalo{7, 8}}
    mostra x
acabou_finalmente
mostra lista(Intervalo{1, 3})`, "0:2\n1:3\n2:4\n7\n8\n[1, 2, 3]\n", "")
}

func TestTretaIteraErros(t *testing.T) {
	esperaNosDois(t, `treta Caixa
    itens
acabou_finalmente
pra_cada x em Caixa{[1]}
    mostra x
acabou_finalmente`, "", "deu ruim na linha 4: a treta Caixa nao tem itera(), nao da pra percorrer")
	esperaNosDois(t, `treta Caixa
    itens
acabou_finalmente
gambiarra (c Caixa) itera()
    funciona 42
acabou_finalmente
pra_cada x em Caixa{[1]}
    mostra x
acabou_finalmente`, "", "deu ruim na linha 7: o itera() da treta Caixa devolveu numero, e tem que ser lista, dicionario, conjunto, cardapio ou gerador")
	esperaNosDois(t, `treta Caixa
    itens
acabou_finalmente
gambiarra (c Caixa) itera()
    funciona c.itens[5]
acabou_finalmente
arruma
    pra_cada x em Caixa{[1]}
        mostra x
    acabou_finalmente
quebrou erro
    mostra erro_linha(erro)
    mostra erro_pilha(erro)
acabou_finalmente`, "5\n[{\"funcao\": \"Caixa.itera\", \"linha\": 8}]\n", "")
	esperaNosDois(t, `pra_cada x em 3
    mostra x
acabou_finalmente`, "", "so funciona com lista, dicionario, conjunto, cardapio, gerador ou treta com itera(), e isso ai e numero")
}

// O laco percorre um retrato da lista do comeco (igual nos dois).
func TestPraCadaListaEncolhe(t *testing.T) {
	esperaNosDois(t, `bota xs = [1, 2, 3, 4]
pra_cada x em xs
    mostra x
    remove(xs, 0)
acabou_finalmente
bota ys = [1]
pra_cada y em ys
    adiciona(ys, y + 1)
acabou_finalmente
mostra ys`, "1\n2\n3\n4\n[1, 2]\n", "")
}

// A sub-VM do gerador nasce com poucos frames e cresce: recursao funda la
// dentro funciona.
func TestGeradorRecursaoNoCorpo(t *testing.T) {
	esperaNosDois(t, `gambiarra soma(n)
    se_colar n == 0
        funciona 0
    acabou_finalmente
    bota r = soma(n - 1)
    funciona n + r
acabou_finalmente
gambiarra g(n)
    rende soma(n)
acabou_finalmente
mostra lista(g(500))
mostra proximo(g(1000))`, "[125250]\n500500\n", "")
}

// Gerador com pattern matching (escolhe/caso) e cardapio: o gerador rende
// dentro de um caso com padrao, o cardapio e percorrido dentro do gerador, e
// uma gambiarra do usuario chamada proximo sombreia o builtin.
func TestGeradorComPadraoECardapio(t *testing.T) {
	esperaNosDois(t, `cardapio Forma
    circulo
    quadrado
acabou_finalmente
gambiarra formas()
    pra_cada i, f em Forma
        rende [f, i + 1]
    acabou_finalmente
acabou_finalmente
gambiarra descreve(itens)
    pra_cada v em itens
        escolhe v
        caso [Forma.circulo, r]
            rende "circulo de raio ${r}"
        caso [primeiro, ...resto] se tamanho(resto) > 1
            rende "lista comprida ${primeiro}"
        caso [f, lado]
            rende "${f.nome} de lado ${lado}"
        se_nao_colar
            rende "sei la"
        acabou_finalmente
    acabou_finalmente
acabou_finalmente
pra_cada d em descreve(formas())
    mostra d
acabou_finalmente
mostra lista(descreve([[1, 2, 3], 7]))
escolhe proximo(formas())
caso [Forma.circulo, n]
    mostra "primeiro e circulo, ${n}"
acabou_finalmente
mostra lista(Forma)
treta Caixa
    nada_aqui
acabou_finalmente
gambiarra (c Caixa) itera()
    funciona Forma
acabou_finalmente
pra_cada i, f em Caixa{1}
    mostra "${i}=${f}"
acabou_finalmente
gambiarra proximo(s)
    funciona "meu proximo " + s
acabou_finalmente
mostra proximo("x")`, "circulo de raio 1\nquadrado de lado 2\n[lista comprida 1, sei la]\nprimeiro e circulo, 1\n[Forma.circulo, Forma.quadrado]\n0=Forma.circulo\n1=Forma.quadrado\nmeu proximo x\n", "")
}
