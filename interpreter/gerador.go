package interpreter

import (
	"runtime"
	"sync"
	"sync/atomic"

	"gambiarrascript/ast"
	"gambiarrascript/object"
)

// Gerador no tree-walker (o valor e o object.Gerador, igual na VM).
//
// O Eval e recursivo: nao da pra pausar o corpo no meio de um `rende` e
// voltar depois sem guardar a pilha do Go. Entao o corpo roda numa goroutine
// propria (a "produtora") com HANDOFF estrito: quem pede um valor manda o
// pedido e fica bloqueado ate o corpo chegar no proximo `rende` (ou acabar);
// o corpo, depois de entregar, fica bloqueado ate o proximo pedido. So um
// lado anda por vez e os canais dao o happens-before entre eles — por isso a
// produtora NAO liga o modo concorrente (object/concorrencia.go): nunca tem
// duas goroutines mexendo em objeto do usuario ao mesmo tempo, igual a VM,
// que roda o corpo na goroutine de quem pede.
//
// Fecha (`fecha(g)` e o pra_cada dono saindo no meio): o pedido vai com
// `fechando` ligado e o `rende` pausado devolve o sinal de fecha
// (object.SinalFecha) no lugar de NADA. Ele sobe como erro — so o `finalmente`
// roda, o `quebrou` deixa passar (evalArruma) — ate o fim do corpo, e a
// produtora termina normal. Um `rende` no caminho (ignorou o fecha) entrega o
// erro e sai com runtime.Goexit.
//
// Vazamento: gerador abandonado no meio (proximo() que ninguem chama de novo,
// `vaza` num pra_cada sobre variavel) deixaria a produtora parada pra sempre no
// `rende`. A fonte nao aponta pro object.Gerador, entao ele vira lixo; o
// finalizer dele (object.NovoGerador) chama Abandona, que acorda a produtora
// pra ela sair com runtime.Goexit — sem rodar mais nada do corpo (nem
// `finalmente`), igual a VM, onde o frame pausado so vira lixo. Sobra um caso:
// gerador pausado guardado num escopo que o PROPRIO corpo enxerga (global, ou
// variavel da funcao que definiu a gambiarra) — a pilha da produtora segura
// esse escopo, que segura o gerador, entao ele nunca vira lixo e a goroutine
// so sai com fecha(g), consumindo ate o fim, ou com o processo.
type geradorTree struct {
	i      *Interpreter
	funcao *object.Funcao
	escopo *object.Environment // ja com os parametros amarrados
	nome   string
	linha  int // linha da chamada que criou o gerador

	pede    chan struct{}
	entrega chan entregaTree
	solta   chan struct{} // fechado pelo Abandona (finalizer)
	soltou  sync.Once

	iniciado bool // so quem pede mexe (com o mu do object.Gerador)
	// fechando: o pedido e o do Fecha. Escrito por quem pede ANTES de mandar
	// no `pede`, lido pela produtora DEPOIS de receber (o canal ordena).
	fechando bool
	produtor atomic.Int64 // id da goroutine produtora (0 = nao nasceu)
	// consumidor: goroutine do ultimo pedido, so anotado com concorrencia
	// ligada (custa ~1us) — pro com_trava achar a corrente de quem espera
	consumidor atomic.Int64
}

type entregaTree struct {
	valor   object.Object
	fim     bool
	falha   object.Object // *Erro ou *Sair no fim com falha
	ignorou int           // fechando, deu rende nesta linha (fim junto)
}

// ctxRende e o que o escopo da chamada do gerador guarda pro `rende` achar.
type ctxRende struct{ g *geradorTree }

func (i *Interpreter) novoGerador(funcao *object.Funcao, escopo *object.Environment, nome string, linha int) *object.Gerador {
	if funcao.Nome != "" {
		nome = funcao.Nome
	}
	g := &geradorTree{
		i: i, funcao: funcao, escopo: escopo, nome: nome, linha: linha,
		pede:    make(chan struct{}),
		entrega: make(chan entregaTree),
		solta:   make(chan struct{}),
	}
	escopo.MarcaGerador(&ctxRende{g: g})
	return object.NovoGerador(nome, g)
}

func (g *geradorTree) Produtor() int64 { return g.produtor.Load() }

// Proximo pede um valor e espera a produtora entregar.
func (g *geradorTree) Proximo() (object.Object, bool, object.Object) {
	g.anotaConsumidor()
	if !g.iniciado {
		g.iniciado = true
		go g.roda()
	} else {
		g.pede <- struct{}{}
	}
	r := <-g.entrega
	if r.fim {
		return nil, false, r.falha
	}
	return r.valor, true, nil
}

// Fecha retoma o corpo pausado com o sinal de fecha e espera ele terminar
// (os `finalmente` rodam na produtora, um lado anda por vez como sempre).
// Gerador que nem comecou nao tem goroutine: so acaba.
func (g *geradorTree) Fecha() (bool, int, object.Object) {
	if !g.iniciado {
		return false, 0, nil
	}
	g.anotaConsumidor()
	g.fechando = true
	g.pede <- struct{}{}
	r := <-g.entrega
	if r.ignorou > 0 {
		return true, r.ignorou, nil
	}
	if object.EhSinalFecha(r.falha) {
		return false, 0, nil
	}
	return false, 0, r.falha
}

// Abandona solta a produtora parada num `rende` sem rodar mais nada (chamado
// pelo finalizer do gerador que virou lixo: ninguem mais vai pedir nada).
func (g *geradorTree) Abandona() { g.soltou.Do(func() { close(g.solta) }) }

func (g *geradorTree) anotaConsumidor() {
	if object.ConcorrenciaAtiva() {
		g.consumidor.Store(object.IDGoroutine())
	}
}

// roda e a goroutine produtora: roda o corpo inteiro, entregando um valor a
// cada `rende`, e no fim entrega o fim (com a falha, se teve).
func (g *geradorTree) roda() {
	id := object.IDGoroutine()
	g.produtor.Store(id)
	produtores.Store(id, g)
	nProdutores.Add(1)
	defer func() {
		produtores.Delete(id)
		nProdutores.Add(-1)
	}()
	// panico no corpo vira erro pra quem pediu; o Goexit do Fecha nao passa
	// pelo recover (ninguem ta ouvindo, nao entrega nada)
	defer func() {
		if r := recover(); r != nil {
			g.entrega <- entregaTree{fim: true, falha: newError(g.linha, "panico dentro do gerador %s: %v", g.nome, r)}
		}
	}()
	i := g.i
	var res object.Object
	if i.gancho != nil && i.gancho.dep != nil {
		// depurador: o corpo do gerador e um fluxo proprio (a goroutine dele)
		dep := i.gancho.dep
		fio, gid := dep.entra(i.quadroDaFuncao(g.funcao, g.nome, g.escopo, g.linha))
		defer dep.sai(fio, gid)
	}
	res = i.evalBlock(g.funcao.Body, g.escopo)
	fim := entregaTree{fim: true}
	switch r := res.(type) {
	case *object.Sair:
		fim.falha = r
	case *object.Erro:
		if isError(r) {
			fim.falha = r
		}
	case *object.Vaza:
		fim.falha = newError(r.Line, "deu `vaza` fora de um loop, parca — vaza pra onde?")
	case *object.Continua:
		fim.falha = newError(r.Line, "deu `continua` fora de um loop, parca")
	}
	g.entrega <- fim
}

// evalRende entrega o valor pra quem pediu e espera o proximo pedido.
func (i *Interpreter) evalRende(node *ast.RendeStatement, env *object.Environment) object.Object {
	val := i.Eval(node.Value, env)
	if isError(val) {
		return val
	}
	ctx, ok := env.Gerador().(*ctxRende)
	if !ok {
		return newError(node.Token.Line, "rende fora de gerador")
	}
	g := ctx.g
	if g.fechando {
		// rende enquanto fechava: o fecha vira erro e o corpo para aqui
		g.entrega <- entregaTree{fim: true, ignorou: node.Token.Line}
		runtime.Goexit()
	}
	g.entrega <- entregaTree{valor: val}
	select {
	case <-g.pede:
		if g.fechando {
			return object.SinalFecha()
		}
		return NADA
	case <-g.solta:
		runtime.Goexit()
		return nil
	}
}

// ---------------------------------------------------------------- com_trava

// produtores mapeia a goroutine produtora pro gerador dela, enquanto o corpo
// existe; nProdutores evita a busca quando nao tem gerador nenhum rodando.
var (
	produtores  sync.Map // int64 -> *geradorTree
	nProdutores atomic.Int64
)

// travaComQuemConsome diz se o com_trava(t) chamado daqui nunca ia andar
// porque a trava ta com quem consome este gerador. No tree-walker o corpo do
// gerador roda na goroutine produtora, mas logicamente e o MESMO fluxo de
// quem pediu o valor (que ta parado esperando): pegar de novo a trava dele e
// a reentrada que a VM (corpo na goroutine de quem pede) ja acusa. Sem
// concorrencia ligada so um fluxo anda por vez, entao trava ocupada e trava
// de alguem da corrente parada; com concorrencia segue a corrente de
// consumidores (anotada a cada pedido) atras da dona.
func travaComQuemConsome(t *object.Trava) bool {
	if nProdutores.Load() == 0 {
		return false
	}
	dona := t.Dono()
	if dona == 0 {
		return false
	}
	v, ok := produtores.Load(object.IDGoroutine())
	if !ok {
		return false
	}
	if !object.ConcorrenciaAtiva() {
		return true
	}
	g := v.(*geradorTree)
	for passo := 0; passo < 1000; passo++ { // anotacao velha nao vira laco eterno
		c := g.consumidor.Load()
		if c == 0 {
			return false
		}
		if c == dona {
			return true
		}
		prox, ok := produtores.Load(c)
		if !ok {
			return false
		}
		g = prox.(*geradorTree)
	}
	return false
}

// ---------------------------------------------------------------- iteracao

// iteravel resolve o que o pra_cada (e o lista()) percorre: instancia de
// treta vira o que o itera() dela devolve; o resto passa direto.
func (i *Interpreter) iteravel(v object.Object, linha int) object.Object {
	inst, ok := v.(*object.Instancia)
	if !ok {
		return v
	}
	m, msg := object.MetodoItera(inst)
	if msg != "" {
		return newError(linha, "%s", msg)
	}
	res := i.applyFunction(m, nil, linha, m.Nome)
	if isError(res) {
		return res
	}
	if _, ok := res.(*object.Sair); ok {
		return res
	}
	if msg := object.ConfereItera(inst, res); msg != "" {
		return newError(linha, "%s", msg)
	}
	return res
}

// praCadaGerador e o pra_cada em cima de um gerador: pede um valor por volta.
// Com dois nomes vem a posicao (0, 1, ...) e o valor, igual lista. dono: o
// gerador nasceu no cabecalho do laco (ninguem mais tem ele), entao saindo no
// meio (vaza, funciona, erro) o laco fecha ele — como um `finalmente {
// fecha(g) }` em volta: erro do fecha toma o lugar do que tava saindo. sai()
// nao fecha (o programa ta acabando; a VM nem desenrola).
func (i *Interpreter) praCadaGerador(g *object.Gerador, dono bool, node *ast.PraCadaListStatement, env *object.Environment) object.Object {
	res, noMeio := i.voltasGerador(g, node, env)
	if !noMeio || !dono {
		return res
	}
	if _, ok := res.(*object.Sair); ok {
		return res
	}
	if falha := g.Fecha(); falha != nil {
		return falha
	}
	return res
}

// voltasGerador roda as voltas. noMeio: o laco saiu antes do gerador acabar
// (vaza, funciona, erro ou sai() do corpo do laco).
func (i *Interpreter) voltasGerador(g *object.Gerador, node *ast.PraCadaListStatement, env *object.Environment) (res object.Object, noMeio bool) {
	doisNomes := len(node.Vars) == 2
	for idx := int64(0); ; idx++ {
		v, ok, falha := g.Proximo()
		if falha != nil {
			return falhaDoGerador(falha), false
		}
		if !ok {
			return NADA, false
		}
		if doisNomes {
			env.Set(node.Vars[0].Value, object.NumInt(idx))
			env.Set(node.Vars[1].Value, v)
		} else {
			env.Set(node.Vars[0].Value, v)
		}
		res := i.evalBlock(node.Body, env)
		if res != nil {
			switch res.Type() {
			case object.ERRO_OBJ, object.RETORNO_OBJ, object.SAIR_OBJ:
				return res, true
			case object.VAZA_OBJ:
				return NADA, true
			}
		}
	}
}

// falhaDoGerador repassa o erro/sai() do corpo do gerador pra quem consumiu,
// igual um erro saindo de uma gambiarra chamada: ja vem com a linha de dentro
// do gerador (onde estourou de verdade) e sobe dali pelo consumidor.
func falhaDoGerador(falha object.Object) object.Object { return falha }
