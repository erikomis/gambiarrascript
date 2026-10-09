package object

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

const GERADOR_OBJ = "GERADOR"

// Gerador e o valor que uma gambiarra com `rende` devolve quando e chamada:
// o corpo NAO roda na chamada; cada pedido de valor (pra_cada, proximo,
// acabou, lista...) roda o corpo ate o proximo `rende` e pausa ali. Quando o
// corpo acaba (fim, `funciona`, erro), o gerador acabou pra sempre.
//
// Quem roda o corpo e a FonteGerador de cada engine: a VM guarda o frame
// pausado numa sub-VM so dele (sem goroutine); o tree-walker, que nao tem
// como pausar a recursao do Eval, roda o corpo numa goroutine com handoff
// estrito (so um dos dois lados anda por vez). Este embrulho e o mesmo pros
// dois: serializa os pedidos (gerador compartilhado entre goroutines do
// `bora` nao corre), segura o valor espiado pelo acabou() e acusa o gerador
// que pede o proprio proximo valor enquanto roda (seria deadlock).
type Gerador struct {
	Nome  string
	fonte FonteGerador
	fluxo fonteComFluxo // a fonte, se roda o corpo em outra goroutine (nao muda)

	serie uint64 // ordem de criacao (SerieGeradores): o pra_cada sabe se e dono

	mu         sync.Mutex
	rodando    atomic.Bool  // o corpo esta rodando agora (dentro do mu)
	dono       atomic.Int64 // goroutine que pediu (so com concorrencia ligada)
	acabou     bool
	espiado    Object // valor ja puxado pelo acabou(), entregue no proximo pedido
	temEspiado bool
}

// FonteGerador roda o corpo de um gerador. Proximo roda ate o proximo
// `rende`: ok=false e o fim do corpo, com falha != nil (*Erro ou *Sair) se
// ele acabou levantando erro ou chamando sai(). Nunca e chamada de novo
// depois de ok=false, nem de duas goroutines ao mesmo tempo.
type FonteGerador interface {
	Proximo() (valor Object, ok bool, falha Object)
}

// fonteAbandonavel e a fonte que segura recurso fora da memoria (a goroutine
// do tree-walker): Abandona e chamado quando o gerador vira lixo sem ter
// acabado. Nao roda nada do corpo (o finalizer roda em goroutine do runtime,
// em paralelo com o programa).
type fonteAbandonavel interface{ Abandona() }

// fonteFechavel e a fonte que sabe fechar o corpo pausado (`fecha(g)`):
// retoma ele com o sinal de fecha (SinalFecha) no lugar do `rende`, pra os
// `finalmente` pendentes rodarem. rendeu=true: o corpo deu `rende` de novo
// enquanto fechava (na linha); falha: o erro/sai() que escapou (o proprio
// sinal ja vem filtrado, nao e falha). So e chamada com o corpo pausado num
// `rende` (depois do primeiro pedido e antes do fim).
type fonteFechavel interface {
	Fecha() (rendeu bool, linha int, falha Object)
}

// serieGeradores conta os geradores criados no processo. O pra_cada le antes
// de avaliar o cabecalho: gerador com serie maior nasceu ali (e o laco e dono
// dele — fecha na saida antecipada).
var serieGeradores atomic.Uint64

// SerieGeradores e a marca de agora: geradores criados depois tem Serie maior.
func SerieGeradores() uint64 { return serieGeradores.Load() }

// NasceuDepois diz se o gerador foi criado depois da marca.
func (g *Gerador) NasceuDepois(marca uint64) bool { return g.serie > marca }

// KindFechaGerador e o kind do sinal de fecha: o erro que o `fecha(g)` joga no
// `rende` pausado. So o `finalmente` ve ele passar (o `quebrou` nao pega) e
// ele nunca sai do gerador: o Fecha engole.
const KindFechaGerador = "fecha_gerador"

// SinalFecha cria o sinal de fecha (um por fecha: o traco cresce no caminho).
func SinalFecha() *Erro {
	return &Erro{Message: "gerador fechado", Kind: KindFechaGerador}
}

// EhSinalFecha diz se o erro e o sinal de fecha de gerador.
func EhSinalFecha(o Object) bool {
	e, ok := o.(*Erro)
	return ok && e.Kind == KindFechaGerador
}

// fonteComFluxo e a fonte que roda o corpo em outra goroutine: Produtor diz
// qual (0 = nenhuma ainda), pra acusar o gerador que pede o proprio valor.
type fonteComFluxo interface{ Produtor() int64 }

// NovoGerador embrulha a fonte. Se ela segura goroutine, o gerador
// abandonado no meio (um `vaza` no pra_cada, um proximo() que ninguem chama
// de novo) solta a goroutine quando o coletor de lixo recolher o gerador: a
// fonte nao aponta pro embrulho, entao ele fica coletavel.
func NovoGerador(nome string, fonte FonteGerador) *Gerador {
	g := &Gerador{Nome: nome, fonte: fonte, serie: serieGeradores.Add(1)}
	if f, ok := fonte.(fonteComFluxo); ok {
		g.fluxo = f
	}
	if f, ok := fonte.(fonteAbandonavel); ok {
		// finalizer (go.mod e 1.23, sem runtime.AddCleanup): so enxerga a
		// fonte, que nao aponta de volta pro g — o g sem dono e recolhido
		runtime.SetFinalizer(g, func(*Gerador) { f.Abandona() })
	}
	return g
}

func (g *Gerador) Type() ObjectType { return GERADOR_OBJ }
func (g *Gerador) Inspect() string {
	if g.Nome == "" {
		return "<gerador>"
	}
	return "<gerador " + g.Nome + ">"
}

// Proximo devolve o proximo valor rendido. ok=false: o gerador acabou (agora
// ou antes); falha != nil so na vez em que o corpo acabou com erro ou sai().
func (g *Gerador) Proximo() (valor Object, ok bool, falha Object) {
	if falha := g.trava("pediu o proprio proximo valor"); falha != nil {
		return nil, false, falha
	}
	defer g.solta()
	if g.temEspiado {
		v := g.espiado
		g.espiado, g.temEspiado = nil, false
		return v, true, nil
	}
	return g.puxa()
}

// Acabou diz se o gerador nao tem mais valor. Pra saber, pode ter que rodar
// o corpo ate o proximo `rende` — o valor fica guardado e sai no proximo
// pedido (os efeitos do corpo, tipo um mostra, acontecem ja aqui).
func (g *Gerador) Acabou() (bool, Object) {
	if falha := g.trava("pediu o proprio proximo valor"); falha != nil {
		return false, falha
	}
	defer g.solta()
	if g.temEspiado {
		return false, nil
	}
	v, ok, falha := g.puxa()
	if falha != nil {
		return true, falha
	}
	if !ok {
		return true, nil
	}
	g.espiado, g.temEspiado = v, true
	return false, nil
}

// Fecha encerra o gerador (`fecha(g)`, e o pra_cada dono dele saindo no
// meio): se o corpo ta pausado num `rende`, retoma com o sinal de fecha pra
// os `finalmente` pendentes rodarem (o `quebrou` nao pega o sinal). Depois
// disso o gerador acabou. Idempotente; gerador que nem comecou so acaba.
// Devolve a falha: erro (ou sai()) que escapou do corpo fechando, ou o erro
// de quem deu `rende` enquanto fechava ("ignorou o fecha").
func (g *Gerador) Fecha() Object {
	if falha := g.trava("tentou se fechar"); falha != nil {
		return falha
	}
	defer g.solta()
	g.espiado, g.temEspiado = nil, false
	if g.acabou {
		return nil
	}
	g.acabou = true
	fonte := g.fonte
	g.fonte = fonteVazia{}
	f, ok := fonte.(fonteFechavel)
	if !ok {
		return nil
	}
	g.rodando.Store(true)
	rendeu, linha, falha := f.Fecha()
	g.rodando.Store(false)
	if rendeu {
		return &Erro{
			Message: fmt.Sprintf("deu ruim na linha %d: o gerador %s ignorou o fecha (deu rende enquanto fechava)", linha, g.nomeOuAnonimo()),
			Line:    linha,
			Kind:    "runtime",
		}
	}
	return falha
}

// puxa roda a fonte uma vez (com g.mu na mao).
func (g *Gerador) puxa() (Object, bool, Object) {
	if g.acabou {
		return nil, false, nil
	}
	g.rodando.Store(true)
	v, ok, falha := g.fonte.Proximo()
	g.rodando.Store(false)
	if !ok {
		g.acabou = true
		g.fonte = fonteVazia{} // solta a sub-VM/goroutine (e o que elas seguram)
	}
	return v, ok, falha
}

// trava pega o mu. Se ele ja ta ocupado com o corpo rodando, pode ser o
// proprio corpo pedindo o proximo valor dele mesmo: isso nunca andaria
// (deadlock), entao vira erro. Sem concorrencia ligada so roda gambiarra numa
// goroutine por vez, entao mu ocupado com o corpo rodando E esse caso; com
// concorrencia, confere pelo id da goroutine (so neste caminho de disputa).
func (g *Gerador) trava(acao string) Object {
	if g.mu.TryLock() {
		g.marcaDono()
		return nil
	}
	if g.rodando.Load() && g.chamouEleMesmo() {
		return &Erro{Message: fmt.Sprintf("o gerador %s %s enquanto rodava: isso nunca ia andar", g.nomeOuAnonimo(), acao), Kind: "runtime"}
	}
	g.mu.Lock()
	g.marcaDono()
	return nil
}

func (g *Gerador) solta() {
	if concorrencia.Load() {
		g.dono.Store(0)
	}
	g.mu.Unlock()
}

func (g *Gerador) marcaDono() {
	if concorrencia.Load() {
		g.dono.Store(idGoroutine())
	}
}

func (g *Gerador) chamouEleMesmo() bool {
	if !concorrencia.Load() {
		return true
	}
	eu := idGoroutine()
	if g.dono.Load() == eu {
		return true
	}
	// tree-walker: o corpo roda na goroutine da fonte, nao na de quem pediu
	return g.fluxo != nil && g.fluxo.Produtor() == eu
}

func (g *Gerador) nomeOuAnonimo() string {
	if g.Nome == "" {
		return "<anonima>"
	}
	return g.Nome
}

// fonteVazia e a fonte de um gerador que ja acabou.
type fonteVazia struct{}

func (fonteVazia) Proximo() (Object, bool, Object) { return nil, false, nil }

// ---------------------------------------------------------------- iteracao

// MsgNaoIteravel e o erro do pra_cada em valor que nao da pra percorrer.
func MsgNaoIteravel(v Object) string {
	return fmt.Sprintf("pra_cada ... em ... so funciona com lista, dicionario, conjunto, cardapio, gerador ou treta com itera(), e isso ai e %s", NomeTipo(v))
}

// MetodoItera devolve o `itera()` da instancia (proprio ou promovido de uma
// treta embutida), ou a mensagem de erro.
func MetodoItera(inst *Instancia) (*MetodoLigado, string) {
	m, msg := inst.Membro("itera")
	if ml, ok := m.(*MetodoLigado); ok && msg == "" {
		return ml, ""
	}
	return nil, fmt.Sprintf("a treta %s nao tem itera(), nao da pra percorrer", inst.Tipo.Nome)
}

// ConfereItera checa o que o itera() devolveu: tem que ser coisa que o
// pra_cada percorre direto (nao outra treta, pra nao virar corrente).
func ConfereItera(inst *Instancia, v Object) string {
	switch v.(type) {
	case *Lista, *Dicionario, *Conjunto, *Cardapio, *Gerador:
		return ""
	}
	return fmt.Sprintf("o itera() da treta %s devolveu %s, e tem que ser lista, dicionario, conjunto, cardapio ou gerador", inst.Tipo.Nome, NomeTipo(v))
}
