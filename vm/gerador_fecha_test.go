package vm

import (
	"runtime"
	"testing"
	"time"
)

// fecha(g) e o fecha automatico do pra_cada dono do gerador: os finalmente
// pendentes do corpo rodam, igual nos dois engines.

const geradorComFim = `gambiarra naturais(nome)
    bota n = 0
    arruma
        enquanto deu_bom
            rende n
            n += 1
        acabou_finalmente
    quebrou erro
        mostra "quebrou nao pega o fecha"
    finalmente
        mostra "fim " + nome + " em " + texto(n)
    acabou_finalmente
acabou_finalmente
`

// fecha(g) roda o finalmente pendente; e idempotente; gerador que nem
// comecou so acaba (nao tem finalmente pendente); depois do fecha acabou.
func TestGeradorFecha(t *testing.T) {
	esperaNosDois(t, geradorComFim+`bota g = naturais("g")
mostra proximo(g)
mostra proximo(g)
mostra fecha(g)
fecha(g)
mostra proximo(g)
mostra acabou(g)
bota novo = naturais("novo")
fecha(novo)
mostra proximo(novo, "acabado")
bota espia = naturais("espia")
proximo(espia)
mostra acabou(espia)
fecha(espia)
mostra lista(espia)`, "0\n1\nfim g em 1\nnada\nnada\ndeu_bom\nacabado\ndeu_ruim\nfim espia em 1\n[]\n", "")
}

// rende no caminho do fecha: o gerador ignorou o fecha (erro com a linha do
// rende); funciona no finalmente engole o fecha (acabou normal); erro no
// finalmente sobe pra quem fechou.
func TestGeradorFechaRendeErroFunciona(t *testing.T) {
	esperaNosDois(t, `gambiarra teimoso()
    arruma
        rende 1
    finalmente
        rende 2
    acabou_finalmente
acabou_finalmente
bota t = teimoso()
proximo(t)
fecha(t)`, "", "deu ruim na linha 5: o gerador teimoso ignorou o fecha (deu rende enquanto fechava)")

	esperaNosDois(t, `gambiarra engole()
    arruma
        rende 1
    finalmente
        mostra "limpou"
        funciona nada
    acabou_finalmente
    mostra "nunca"
acabou_finalmente
bota g = engole()
proximo(g)
mostra fecha(g)
mostra acabou(g)
gambiarra estoura()
    arruma
        rende 1
    finalmente
        quebra("no finalmente")
    acabou_finalmente
acabou_finalmente
bota es = estoura()
proximo(es)
arruma
    fecha(es)
quebrou erro
    mostra erro_msg(erro)
acabou_finalmente
mostra acabou(es)`, "limpou\nnada\ndeu_bom\nquebra: no finalmente\ndeu_bom\n", "")
}

// O gerador que fecha a si mesmo enquanto roda e erro (nunca ia andar).
func TestGeradorFechaEleMesmo(t *testing.T) {
	esperaNosDois(t, `bota g = nada
gambiarra eu()
    fecha(g)
    rende 1
acabou_finalmente
bota g = eu()
proximo(g)`, "", "o gerador eu tentou se fechar enquanto rodava")
}

// pra_cada em gerador criado no cabecalho e dono dele: vaza, funciona e erro
// fecham (o finalmente roda na hora). Gerador em variavel nao fecha: da pra
// continuar depois.
func TestGeradorFechaAutomatico(t *testing.T) {
	esperaNosDois(t, geradorComFim+`pra_cada x em naturais("cabecalho")
    se_colar x == 2
        vaza
    acabou_finalmente
acabou_finalmente
mostra "depois do vaza"
bota g = naturais("variavel")
pra_cada x em g
    se_colar x == 2
        vaza
    acabou_finalmente
acabou_finalmente
mostra "continua em " + texto(proximo(g))
pra_cada i, x em naturais("dois nomes")
    se_colar i == 1
        vaza
    acabou_finalmente
acabou_finalmente
gambiarra primeiro_maior(n)
    arruma
        pra_cada x em naturais("funciona")
            se_colar x > n
                funciona x
            acabou_finalmente
        acabou_finalmente
    finalmente
        mostra "finalmente de fora"
    acabou_finalmente
acabou_finalmente
mostra primeiro_maior(3)
arruma
    pra_cada x em naturais("erro")
        se_colar x == 1
            quebra("estourou no laco")
        acabou_finalmente
    acabou_finalmente
quebrou erro
    mostra erro_msg(erro)
acabou_finalmente
pra_cada x em naturais("pego dentro")
    arruma
        quebra("pego")
    quebrou erro
        se_colar x == 1
            vaza
        acabou_finalmente
    acabou_finalmente
acabou_finalmente
fecha(g)`, "fim cabecalho em 2\ndepois do vaza\ncontinua em 3\nfim dois nomes em 1\nfim funciona em 4\nfinalmente de fora\n4\nfim erro em 1\nquebra: estourou no laco\nfim pego dentro em 1\nfim variavel em 3\n", "")
}

// Fechar um gerador que percorre outro criado no proprio cabecalho fecha os
// dois (o de dentro primeiro); a treta cujo itera() cria o gerador tambem e
// dona; o laco de dentro do funciona fecha antes do finalmente de fora.
func TestGeradorFechaEmCadeia(t *testing.T) {
	esperaNosDois(t, geradorComFim+`gambiarra pares()
    arruma
        pra_cada x em naturais("dentro")
            se_colar x % 2 == 0
                rende x
            acabou_finalmente
        acabou_finalmente
    finalmente
        mostra "fim pares"
    acabou_finalmente
acabou_finalmente
pra_cada p em pares()
    se_colar p == 4
        vaza
    acabou_finalmente
acabou_finalmente
treta Contador
    n
acabou_finalmente
gambiarra (c Contador) itera()
    funciona naturais("itera")
acabou_finalmente
pra_cada x em Contador{3}
    se_colar x == 1
        vaza
    acabou_finalmente
acabou_finalmente`, "fim dentro em 4\nfim pares\nfim itera em 1\n", "")
}

// Erro no fecha automatico toma o lugar do que tava saindo (igual erro no
// finalmente) e da pra pegar em volta do laco.
func TestGeradorFechaAutomaticoComErro(t *testing.T) {
	esperaNosDois(t, `gambiarra chato()
    arruma
        rende 1
        rende 2
    finalmente
        quebra("fechando")
    acabou_finalmente
acabou_finalmente
arruma
    pra_cada x em chato()
        vaza
    acabou_finalmente
quebrou erro
    mostra erro_msg(erro)
acabou_finalmente
arruma
    pra_cada x em chato()
        quebra("do laco")
    acabou_finalmente
quebrou erro
    mostra erro_msg(erro)
acabou_finalmente`, "quebra: fechando\nquebra: fechando\n", "")
}

// com_trava segurado por quem consome e pedido de novo no corpo do gerador:
// a VM roda o corpo na goroutine de quem pede (reentrada); o tree-walker,
// que roda numa goroutine propria, travava pra sempre — agora acusa igual,
// inclusive com um gerador consumido dentro de outro.
func TestGeradorComTravaDeQuemConsome(t *testing.T) {
	esperaNosDois(t, `bota t = trava()
gambiarra pega_trava()
    com_trava(t, gambiarra()
        mostra "nunca"
    acabou_finalmente)
    rende 1
acabou_finalmente
gambiarra repassa()
    pra_cada x em pega_trava()
        rende x
    acabou_finalmente
acabou_finalmente
arruma
    com_trava(t, gambiarra()
        funciona proximo(pega_trava())
    acabou_finalmente)
quebrou erro
    mostra contem(erro_msg(erro), "nao e reentrante")
acabou_finalmente
arruma
    com_trava(t, gambiarra()
        funciona lista(repassa())
    acabou_finalmente)
quebrou erro
    mostra contem(erro_msg(erro), "nao e reentrante")
acabou_finalmente
mostra com_trava(t, gambiarra() funciona "livre" acabou_finalmente)`, "deu_bom\ndeu_bom\nlivre\n", "")
}

// Fecha (explicito ou automatico) solta a goroutine do tree-walker na hora,
// inclusive do gerador guardado numa global (que nunca vira lixo: a pilha da
// produtora segura o escopo global). Na VM nao tem goroutine nenhuma.
func TestGeradorFechaSoltaGoroutine(t *testing.T) {
	src := geradorComFim + `bota guardados = []
pra_cada k de 1 ate 100
    bota g = naturais("g")
    proximo(g)
    adiciona(guardados, g)
    pra_cada x em naturais("laco")
        vaza
    acabou_finalmente
acabou_finalmente
mostra tamanho(guardados)
pra_cada g em guardados
    fecha(g)
acabou_finalmente`
	for _, e := range []struct {
		nome string
		roda func(*testing.T, string) (string, string, string)
	}{{"tree", rodaTWComp}, {"vm", rodaVMComp}} {
		antes := esperaGoroutines(runtime.NumGoroutine())
		_, saida, errStr := e.roda(t, src)
		if errStr != "" || len(saida) == 0 {
			t.Fatalf("[%s] saida %q erro %q", e.nome, saida, errStr)
		}
		// sem GC nenhum: o fecha ja soltou (a produtora termina logo depois
		// de entregar o fim, entao da um respiro curto)
		if depois := esperaGoroutinesSemGC(antes + 2); depois > antes+2 {
			t.Fatalf("[%s] goroutines presas depois do fecha: antes %d, depois %d", e.nome, antes, depois)
		}
	}
}

// esperaGoroutinesSemGC espera o numero de goroutines voltar pro limite sem
// chamar o coletor (o que solta tem que soltar sozinho).
func esperaGoroutinesSemGC(limite int) int {
	n := runtime.NumGoroutine()
	for k := 0; k < 100 && n > limite; k++ {
		time.Sleep(10 * time.Millisecond)
		n = runtime.NumGoroutine()
	}
	return n
}
