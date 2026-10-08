package vm

import "testing"

// multi-catch: varios `quebrou NOME [se CONDICAO]` no mesmo arruma, tentados
// em ordem. Os dois engines tem que dar a mesma saida (e o mesmo erro).

// lanca e um helper comum: levanta erro do tipo pedido.
const lanca = `gambiarra lanca(tipo)
    se_colar tipo == "usuario"
        quebra("caiu")
    acabou_finalmente
    se_colar tipo == "runtime"
        funciona 1 / 0
    acabou_finalmente
    arruma
        quebra("base")
    quebrou base
        envolve_erro(tipo, "falhou " + tipo, base)
    acabou_finalmente
acabou_finalmente
`

func TestMultiCatchEscolheClausula(t *testing.T) {
	src := lanca + `gambiarra trata(tipo)
    arruma
        lanca(tipo)
    quebrou erro se erro_tipo(erro) == "rede"
        funciona "rede: " + erro_msg(erro)
    quebrou erro se erro_tipo(erro) == "jwt"
        funciona "jwt"
    quebrou outro
        funciona "resto: " + erro_tipo(outro)
    acabou_finalmente
acabou_finalmente
mostra trata("rede")
mostra trata("jwt")
mostra trata("usuario")
mostra trata("runtime")`
	esperaNosDois(t, src, "rede: envolve: falhou rede :: quebra: base\njwt\nresto: usuario\nresto: runtime\n", "")
}

func TestMultiCatchPrimeiroQueColaGanha(t *testing.T) {
	// os dois filtros colam: so o primeiro roda
	esperaNosDois(t, `arruma
    quebra("x")
quebrou a se erro_tipo(a) == "usuario"
    mostra "primeiro"
quebrou b se comeca_com(erro_msg(b), "quebra")
    mostra "segundo"
acabou_finalmente
mostra "fim"`, "primeiro\nfim\n", "")
}

func TestMultiCatchNenhumColaPropaga(t *testing.T) {
	// sem pega-resto e nenhum filtro colou: o erro sobe, depois do finalmente
	src := `arruma
    arruma
        quebra("la de dentro")
    quebrou erro se erro_tipo(erro) == "rede"
        mostra "nao"
    finalmente
        mostra "finalmente"
    acabou_finalmente
    mostra "nao chega"
quebrou erro
    mostra "fora: " + erro_msg(erro) + " (" + erro_tipo(erro) + ")"
acabou_finalmente`
	esperaNosDois(t, src, "finalmente\nfora: quebra: la de dentro (usuario)\n", "")

	// sem finalmente e sem arruma de fora: derruba o script
	esperaNosDois(t, `mostra "antes"
arruma
    bota x = 1 / 0
quebrou erro se erro_tipo(erro) == "io"
    mostra "nao"
acabou_finalmente
mostra "nao chega"`, "antes\n", "nao da pra dividir por zero")
}

func TestMultiCatchPropagaComPilha(t *testing.T) {
	// o erro que ninguem pegou atravessa as gambiarras com o traco inteiro
	src := `gambiarra a()
    funciona 1 / 0
acabou_finalmente
gambiarra b()
    arruma
        funciona a()
    quebrou e1 se erro_tipo(e1) == "rede"
        funciona "nao"
    acabou_finalmente
acabou_finalmente
gambiarra c()
    funciona b()
acabou_finalmente
arruma
    c()
quebrou erro
    mostra erro_linha(erro)
    pra_cada f em erro_pilha(erro)
        mostra "em ${f.funcao} (linha ${f.linha})"
    acabou_finalmente
acabou_finalmente`
	esperaNosDois(t, src, "2\nem c (linha 15)\nem b (linha 12)\nem a (linha 6)\n", "")
}

func TestMultiCatchErroNoFiltroSobe(t *testing.T) {
	// erro dentro do filtro sobe no lugar do original (o finalmente roda)
	src := `arruma
    arruma
        quebra("original")
    quebrou erro se erro_tipo(erro) == 1 / 0
        mostra "nao"
    quebrou erro
        mostra "nao tambem"
    finalmente
        mostra "finalmente"
    acabou_finalmente
quebrou fora
    mostra erro_tipo(fora) + ": " + erro_msg(fora)
acabou_finalmente`
	esperaNosDois(t, src, "finalmente\nruntime: deu ruim na linha 4: nao da pra dividir por zero, parca — nem na gambiarra\n", "")

	// filtro que chama builtin com argumento errado
	esperaNosDois(t, `arruma
    quebra("x")
quebrou erro se tamanho(erro) > 0
    mostra "nao"
acabou_finalmente`, "", "tamanho")
}

func TestMultiCatchFinalmenteRodaUmaVez(t *testing.T) {
	casos := []struct{ src, saida, erro string }{
		// pegou no 2o
		{`arruma
    quebra("x")
quebrou a se erro_tipo(a) == "rede"
    mostra "rede"
quebrou b se erro_tipo(b) == "usuario"
    mostra "usuario"
finalmente
    mostra "fin"
acabou_finalmente`, "usuario\nfin\n", ""},
		// sem erro
		{`arruma
    mostra "ok"
quebrou a se erro_tipo(a) == "rede"
    mostra "rede"
quebrou b
    mostra "resto"
finalmente
    mostra "fin"
acabou_finalmente`, "ok\nfin\n", ""},
		// erro dentro da clausula que pegou: finalmente roda e o erro sobe
		{`arruma
    quebra("x")
quebrou a se erro_tipo(a) == "usuario"
    quebra("de novo", a)
finalmente
    mostra "fin"
acabou_finalmente`, "fin\n", "quebra: de novo"},
		// nenhum colou: finalmente uma vez so
		{`arruma
    quebra("x")
quebrou a se erro_tipo(a) == "rede"
    mostra "rede"
quebrou b se erro_tipo(b) == "io"
    mostra "io"
finalmente
    mostra "fin"
acabou_finalmente`, "fin\n", "quebra: x"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, c.erro)
	}
}

func TestMultiCatchSinaisDeControle(t *testing.T) {
	// funciona/vaza/continua dentro das clausulas, com finalmente
	src := lanca + `gambiarra f(tipo)
    arruma
        lanca(tipo)
    quebrou err se erro_tipo(err) == "volta"
        funciona "voltou"
    quebrou err
        funciona "resto"
    finalmente
        mostra "fin " + tipo
    acabou_finalmente
acabou_finalmente
mostra f("volta")
mostra f("outro")
bota tipos = ["pula", "ok", "para", "nunca"]
pra_cada t em tipos
    arruma
        lanca(t)
    quebrou err se erro_tipo(err) == "pula"
        continua
    quebrou err se erro_tipo(err) == "para"
        vaza
    quebrou err
        mostra "pegou " + t
    finalmente
        mostra "fin " + t
    acabou_finalmente
acabou_finalmente
mostra "depois"`
	esperaNosDois(t, src, "fin volta\nvoltou\nfin outro\nresto\nfin pula\npegou ok\nfin ok\nfin para\ndepois\n", "")
}

func TestMultiCatchRelancaEEnvolve(t *testing.T) {
	src := `gambiarra carrega()
    arruma
        le_arquivo("/nao/existe/mesmo.json")
    quebrou erro se erro_tipo(erro) == "io"
        envolve_erro("config", "sem config", erro)
    acabou_finalmente
acabou_finalmente
arruma
    carrega()
quebrou erro se erro_tipo(erro) == "config"
    mostra "config: " + erro_tipo(erro_causa(erro))
    arruma
        quebra("relancado", erro)
    quebrou de_novo se erro_tipo(de_novo) == "usuario"
        mostra erro_tipo(erro_causa(de_novo))
    acabou_finalmente
quebrou erro
    mostra "nao"
acabou_finalmente`
	esperaNosDois(t, src, "config: io\nconfig\n", "")
}

func TestMultiCatchNomeEFiltroVariados(t *testing.T) {
	casos := []struct{ src, saida string }{
		// o nome fica amarrado depois (escopo de funcao), mesmo de clausula que nao colou
		{`gambiarra f()
    arruma
        quebra("x")
    quebrou a se nao a
        mostra "nao"
    quebrou b
        mostra "b"
    acabou_finalmente
    funciona erro_msg(a) + " / " + erro_msg(b)
acabou_finalmente
mostra f()`, "b\nquebra: x / quebra: x\n"},
		// filtro truthy sem comparacao: o proprio erro e verdadeiro
		{`arruma
    quebra("x")
quebrou e1 se e1
    mostra "pegou"
acabou_finalmente`, "pegou\n"},
		// filtro le variavel de fora e chama gambiarra/lambda
		{`bota quero = "usuario"
bota eh = gambiarra(x, t) funciona erro_tipo(x) == t acabou_finalmente
arruma
    quebra("x")
quebrou e1 se eh(e1, "rede")
    mostra "rede"
quebrou e1 se eh(e1, quero) e contem(erro_msg(e1), "x")
    mostra "certo"
acabou_finalmente`, "certo\n"},
		// mesmo nome em todas, arruma aninhado dentro da clausula
		{`arruma
    arruma
        quebra("dentro")
    quebrou e1 se erro_tipo(e1) == "rede"
        mostra "nao"
    acabou_finalmente
quebrou e1 se erro_tipo(e1) == "usuario"
    arruma
        bota z = [1][5]
    quebrou e2 se erro_tipo(e2) == "runtime"
        mostra "runtime dentro"
    acabou_finalmente
    mostra erro_msg(e1)
acabou_finalmente`, "runtime dentro\nquebra: dentro\n"},
		// clausula dentro de gambiarra lendo param e local
		{`gambiarra g(limite)
    bota n = 0
    arruma
        quebra("x")
    quebrou err se limite > 5
        bota n = 1
    quebrou err
        bota n = 2
    acabou_finalmente
    funciona n
acabou_finalmente
mostra g(10)
mostra g(1)`, "1\n2\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
}

func TestMultiCatchCravaNoNome(t *testing.T) {
	esperaNosDois(t, `crava E = 1
arruma
    quebra("x")
quebrou a se erro_tipo(a) == "rede"
    mostra "nao"
quebrou E
    mostra "nao"
acabou_finalmente`, "", "`E` foi cravada")
}

func TestMultiCatchQuebrouSemFiltroNoMeio(t *testing.T) {
	esperaNosDois(t, `arruma
    quebra("x")
quebrou a
    mostra "a"
quebrou b se erro_tipo(b) == "rede"
    mostra "b"
acabou_finalmente`, "", "quebrou sem filtro tem que ser o ultimo")
}
