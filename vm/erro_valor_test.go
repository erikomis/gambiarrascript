package vm

import "testing"

// Valor de erro e so um valor: o que o `quebrou` pega pode ser mostrado,
// guardado, passado, devolvido, comparado e posto em lista sem relancar. So
// `quebra(...)` (ou um erro de runtime nao pego) levanta erro. Antes o
// tree-walker relancava o erro num `mostra erro` (o valor de um statement
// era confundido com desvio de fluxo) e a VM relancava o que o
// `erro_causa(e)` devolvia (o erro pego nao ficava marcado como tratado).
func TestErroEValor(t *testing.T) {
	casos := []struct{ nome, src, saida, erro string }{
		{"mostra o erro pego", `arruma
    quebra("x")
quebrou erro
    mostra erro
acabou_finalmente
mostra "fim"`, "quebra: x\nfim\n", ""},
		{"guarda e mostra depois", `arruma
    quebra("x")
quebrou erro
    bota e1 = erro
acabou_finalmente
mostra e1
e1
mostra "fim"`, "quebra: x\nfim\n", ""},
		{"passa, devolve, lista e compara", `arruma
    quebra("x")
quebrou erro
    bota e1 = erro
acabou_finalmente
arruma
    quebra("x")
quebrou erro
    bota e3 = erro
acabou_finalmente
gambiarra f(err)
    funciona err
acabou_finalmente
bota e2 = f(e1)
mostra tipo(e2)
mostra [e1, e2]
mostra {"e": e1}
mostra e1 == e2
mostra e1 == e3
mostra e1 != nada
mostra "fim"`, "erro\n[quebra: x, quebra: x]\n{\"e\": quebra: x}\ndeu_bom\ndeu_ruim\ndeu_bom\nfim\n", ""},
		{"texto e interpolacao", `arruma
    bota r = 1 / 0
quebrou erro
    mostra texto(erro)
    mostra "pegou: ${erro}"
    mostra "pegou: " + erro
    mostra tamanho(erro_msg(erro)) > 0
acabou_finalmente`, "deu ruim na linha 2: nao da pra dividir por zero, parca — nem na gambiarra\npegou: deu ruim na linha 2: nao da pra dividir por zero, parca — nem na gambiarra\npegou: deu ruim na linha 2: nao da pra dividir por zero, parca — nem na gambiarra\ndeu_bom\n", ""},
		{"erro_causa devolve a causa como valor", `arruma
    arruma
        bota r = 1 / 0
    quebrou interno
        quebra("embrulho", interno)
    acabou_finalmente
quebrou erro
    bota c = erro_causa(erro)
    mostra tipo(c)
    mostra c
    mostra erro_causa(c)
    mostra erro_tipo(c)
acabou_finalmente
mostra "fim"`, "erro\ndeu ruim na linha 3: nao da pra dividir por zero, parca — nem na gambiarra\nnada\nruntime\nfim\n", ""},
		{"erro no fim de bloco de laco e funcao", `arruma
    quebra("x")
quebrou erro
    bota e1 = erro
acabou_finalmente
gambiarra g()
    pra_cada i em [1, 2]
        mostra e1
    acabou_finalmente
    se_colar deu_bom
        e1
    acabou_finalmente
acabou_finalmente
mostra g()
mostra "fim"`, "quebra: x\nquebra: x\nnada\nfim\n", ""},
		{"mapeia devolvendo erro", `arruma
    quebra("x")
quebrou erro
    bota e1 = erro
acabou_finalmente
mostra mapeia([1, 2], gambiarra(n) funciona e1 acabou_finalmente)
mostra [e1][0]
gambiarra devolve()
    funciona e1
acabou_finalmente
mostra espera(bora devolve())`, "[quebra: x, quebra: x]\nquebra: x\nquebra: x\n", ""},
		{"quebra com o erro pego como causa relanca", `arruma
    quebra("x")
quebrou erro
    bota e1 = erro
acabou_finalmente
quebra("de novo", e1)`, "", "quebra: de novo"},
		{"pra_json de erro e erro de tipo", `arruma
    quebra("x")
quebrou erro
    arruma
        mostra pra_json(erro)
    quebrou e2
        mostra erro_msg(e2)
    acabou_finalmente
acabou_finalmente`, "deu ruim: nao da pra virar json: ERRO\n", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			esperaNosDois(t, c.src, c.saida, c.erro)
		})
	}
}
