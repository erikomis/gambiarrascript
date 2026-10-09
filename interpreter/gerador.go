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
// Vazamento: gerador abandonado no meio (`vaza` no pra_cada, proximo() que
// ninguem chama de novo) deixaria a produtora parada pra sempre no `rende`.
// A fonte nao aponta pro object.Gerador, entao ele vira lixo; o finalizer dele
// (object.NovoGerador) chama Fecha, que acorda a produtora pra ela sair com
// runtime.Goexit — sem rodar mais nada do corpo (nem `finalmente`), igual a
// VM, onde o frame pausado so vira lixo.
type geradorTree struct {
	i      *Interpreter
	funcao *object.Funcao
	escopo *object.Environment // ja com os parametros amarrados
	nome   string
	linha  int // linha da chamada que criou o gerador

	pede    chan struct{}
	entrega chan entregaTree
	fecha   chan struct{}
	fechou  sync.Once

	iniciado bool         // so quem pede mexe (com o mu do object.Gerador)
	produtor atomic.Int64 // id da goroutine produtora (0 = nao nasceu)
}

type entregaTree struct {
	valor object.Object
	fim   bool
	falha object.Object // *Erro ou *Sair no fim com falha
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
		fecha:   make(chan struct{}),
	}
	escopo.MarcaGerador(&ctxRende{g: g})
	return object.NovoGerador(nome, g)
}

func (g *geradorTree) Produtor() int64 { return g.produtor.Load() }

// Proximo pede um valor e espera a produtora entregar.
func (g *geradorTree) Proximo() (object.Object, bool, object.Object) {
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

// Fecha solta a produtora parada num `rende` (chamado pelo finalizer do
// gerador que virou lixo, ou seja: ninguem mais vai pedir nada).
func (g *geradorTree) Fecha() { g.fechou.Do(func() { close(g.fecha) }) }

// roda e a goroutine produtora: roda o corpo inteiro, entregando um valor a
// cada `rende`, e no fim entrega o fim (com a falha, se teve).
func (g *geradorTree) roda() {
	g.produtor.Store(object.IDGoroutine())
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
	g.entrega <- entregaTree{valor: val}
	select {
	case <-g.pede:
		return NADA
	case <-g.fecha:
		runtime.Goexit()
		return nil
	}
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
// Com dois nomes vem a posicao (0, 1, ...) e o valor, igual lista.
func (i *Interpreter) praCadaGerador(g *object.Gerador, node *ast.PraCadaListStatement, env *object.Environment) object.Object {
	doisNomes := len(node.Vars) == 2
	for idx := int64(0); ; idx++ {
		v, ok, falha := g.Proximo()
		if falha != nil {
			return falhaDoGerador(falha)
		}
		if !ok {
			return NADA
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
				return res
			case object.VAZA_OBJ:
				return NADA
			}
		}
	}
}

// falhaDoGerador repassa o erro/sai() do corpo do gerador pra quem consumiu,
// igual um erro saindo de uma gambiarra chamada: ja vem com a linha de dentro
// do gerador (onde estourou de verdade) e sobe dali pelo consumidor.
func falhaDoGerador(falha object.Object) object.Object { return falha }
