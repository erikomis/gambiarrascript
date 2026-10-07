package object

import (
	"sort"
	"strings"
	"sync"
)

// Colecoes mutaveis (Lista, Dicionario, Conjunto) seguras pra uso concorrente.
//
// Handler do `rota`, `bora` e `paralelo` rodam em goroutines de verdade; antes
// duas delas mexendo no mesmo dicionario matavam o processo com o "concurrent
// map writes" do Go (um fatal, nem o recover pega). A garantia agora e a do
// GIL do Python: cada operacao (le, escreve, adiciona, remove, tamanho,
// iterar) e atomica e nunca corrompe nem derruba o processo. Operacao
// COMPOSTA (`bota d["n"] = d["n"] + 1`) NAO e atomica — pra isso tem `trava()`
// e `com_trava(t, fn)`.
//
// Custo: enquanto o programa tem uma goroutine so, nada trava — cada operacao
// paga so a leitura atomica do modo concorrente (concorrencia.go). Depois que
// o primeiro `bora`/servidor/handler liga o modo, toda operacao trava o mutex
// da propria colecao (sem disputa ele custa um CAS na entrada e outro na
// saida).
//
// Regra pra quem escreve builtin: os campos sao privados de proposito — passe
// sempre pelos metodos. Nunca segure a trava de uma colecao enquanto trava
// outra (Inspect, json, iguais...): tire uma copia (Copia/Pares/Visao), solte e
// trabalhe na copia — senao duas listas que se contem podem dar deadlock. E
// nunca rode codigo do usuario em cima de Visao/Itera: a gambiarra pode ligar
// o modo concorrente no meio e outra goroutine mexer na colecao — use Copia,
// Pares, ParaIterar + Pega.

// ---------------------------------------------------------------- Lista

type Lista struct {
	mu    sync.Mutex // so usado com o modo concorrente ligado
	elems []Object
}

// NovaLista embrulha o slice (sem copiar: a lista passa a ser dona dele).
func NovaLista(elems []Object) *Lista {
	return &Lista{elems: elems}
}

func (l *Lista) Type() ObjectType { return LISTA_OBJ }
func (l *Lista) Inspect() string  { return l.inspect(nil) }

// inspect com emCurso: as colecoes que ja estao sendo impressas mais acima
// (ver "ciclos" no fim do arquivo).
func (l *Lista) inspect(emCurso map[Object]bool) string {
	if emCurso[l] {
		return "[...]"
	}
	elems := l.Visao()
	partes := make([]string, len(elems))
	for i, e := range elems {
		if ehColecaoAninhavel(e) {
			if emCurso == nil {
				emCurso = map[Object]bool{}
			}
			emCurso[l] = true
			partes[i] = inspectDentro(e, emCurso)
		} else {
			partes[i] = e.Inspect()
		}
	}
	delete(emCurso, l)
	return "[" + strings.Join(partes, ", ") + "]"
}

// Tamanho devolve quantos elementos a lista tem agora.
func (l *Lista) Tamanho() int {
	if concorrencia.Load() {
		l.mu.Lock()
		n := len(l.elems)
		l.mu.Unlock()
		return n
	}
	return len(l.elems)
}

// Pega devolve o elemento i (0 <= i < tamanho); ok=false se fora.
func (l *Lista) Pega(i int) (Object, bool) {
	if concorrencia.Load() {
		return l.pegaTravado(i)
	}
	if uint(i) >= uint(len(l.elems)) {
		return nil, false
	}
	return l.elems[i], true
}

func (l *Lista) pegaTravado(i int) (Object, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if uint(i) >= uint(len(l.elems)) {
		return nil, false
	}
	return l.elems[i], true
}

// Indice e o Pega com indice negativo estilo Python (-1 = ultimo).
func (l *Lista) Indice(pos int) (Object, bool) {
	if concorrencia.Load() {
		return l.indiceTravado(pos)
	}
	if pos < 0 {
		pos += len(l.elems)
	}
	if uint(pos) >= uint(len(l.elems)) {
		return nil, false
	}
	return l.elems[pos], true
}

func (l *Lista) indiceTravado(pos int) (Object, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if pos < 0 {
		pos += len(l.elems)
	}
	if uint(pos) >= uint(len(l.elems)) {
		return nil, false
	}
	return l.elems[pos], true
}

// Poe troca o elemento na posicao (aceita negativo). ok=false se fora.
func (l *Lista) Poe(pos int, v Object) bool {
	if concorrencia.Load() {
		return l.poeTravado(pos, v)
	}
	if pos < 0 {
		pos += len(l.elems)
	}
	if uint(pos) >= uint(len(l.elems)) {
		return false
	}
	l.elems[pos] = v
	return true
}

func (l *Lista) poeTravado(pos int, v Object) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if pos < 0 {
		pos += len(l.elems)
	}
	if uint(pos) >= uint(len(l.elems)) {
		return false
	}
	l.elems[pos] = v
	return true
}

// Adiciona poe v no fim da lista.
func (l *Lista) Adiciona(v Object) {
	if concorrencia.Load() {
		l.mu.Lock()
		l.elems = append(l.elems, v)
		l.mu.Unlock()
		return
	}
	l.elems = append(l.elems, v)
}

// AdicionaVarios poe todos os vs no fim, de uma vez so (atomico).
func (l *Lista) AdicionaVarios(vs []Object) {
	if concorrencia.Load() {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	l.elems = append(l.elems, vs...)
}

// Copia devolve um slice NOVO com os elementos de agora (um retrato).
func (l *Lista) Copia() []Object {
	if concorrencia.Load() {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	out := make([]Object, len(l.elems))
	copy(out, l.elems)
	return out
}

// Visao devolve os elementos pra LEITURA rapida: sem concorrencia e o proprio
// slice (sem copiar), com concorrencia e um retrato. Use so em codigo que nao
// roda gambiarra do usuario no meio (a gambiarra pode ligar a concorrencia e
// outra goroutine mexer na lista) e nunca escreva no slice devolvido. Pra
// iterar chamando codigo do usuario, use Copia ou ParaIterar + Pega.
func (l *Lista) Visao() []Object {
	if concorrencia.Load() {
		return l.Copia()
	}
	return l.elems
}

// ParaIterar devolve o que um `pra_cada` (ou mapeia/filtra...) deve percorrer
// com Pega: a propria lista sem concorrencia (o laco ve o que o corpo
// adiciona, como sempre foi), ou um RETRATO tirado agora com concorrencia —
// assim outra goroutine adicionando/removendo no meio do laco nao faz o laco
// pular, repetir nem estourar indice. Se a concorrencia ligar NO MEIO do laco
// (o corpo deu `bora`), o Pega passa a travar sozinho: continua seguro.
func (l *Lista) ParaIterar() *Lista {
	if concorrencia.Load() {
		return NovaLista(l.Copia())
	}
	return l
}

// Substitui troca todo o conteudo da lista pelo slice dado (a lista vira dona).
func (l *Lista) Substitui(elems []Object) {
	if concorrencia.Load() {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	l.elems = elems
}

// Ordena ordena no lugar (estavel) com a trava segura durante a ordenacao
// inteira. `menor` NAO pode rodar codigo do usuario nem travar outra colecao
// — pra comparador do usuario, ordene uma Copia e devolva com Substitui.
func (l *Lista) Ordena(menor func(a, b Object) bool) {
	if concorrencia.Load() {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	e := l.elems
	sort.SliceStable(e, func(i, j int) bool { return menor(e[i], e[j]) })
}

// Inverte inverte a ordem dos elementos no lugar.
func (l *Lista) Inverte() {
	if concorrencia.Load() {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	e := l.elems
	for i, j := 0, len(e)-1; i < j; i, j = i+1, j-1 {
		e[i], e[j] = e[j], e[i]
	}
}

// RemoveObjeto tira a primeira ocorrencia do PROPRIO objeto alvo (identidade,
// nao igualdade). Quem quer remover por valor acha o alvo numa Copia antes —
// assim a comparacao por valor (que pode travar colecoes aninhadas) roda sem
// a trava desta lista.
func (l *Lista) RemoveObjeto(alvo Object) bool {
	if concorrencia.Load() {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	for i, e := range l.elems {
		if e == alvo {
			copy(l.elems[i:], l.elems[i+1:])
			l.elems[len(l.elems)-1] = nil // nao segura o ultimo vivo
			l.elems = l.elems[:len(l.elems)-1]
			return true
		}
	}
	return false
}

// FatiaLista devolve uma lista NOVA com os elementos [inicio:fim]. Copia o
// trecho: fatiar direto o slice do Go dividia o array com a original, e um
// adiciona na fatia sobrescrevia elemento da original.
func FatiaLista(l *Lista, inicio, fim *Numero) *Lista {
	if concorrencia.Load() {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	lo, hi := NormalizarFatia(inicio, fim, len(l.elems))
	elems := make([]Object, hi-lo)
	copy(elems, l.elems[lo:hi])
	return NovaLista(elems)
}

// ---------------------------------------------------------------- ordenado

// ordenado e o miolo do Dicionario e do Conjunto: valores por chave lembrando
// a ordem em que ENTRARAM, com pega/bota/tira O(1). Sem a ordem a iteracao
// usava a do map do Go, que e embaralhada de proposito e muda a cada execucao
// — `pra_cada k em d` (e o `mostra` de um conjunto) saia diferente toda vez.
// Igual Python 3.7+ e JS: quem chega primeiro, sai primeiro.
//
// Tirar deixa um buraco no slice (vivo=false) em vez de arrastar o resto pra
// tras; quando os buracos passam da metade o slice e compactado, entao o custo
// amortizado continua O(1). Nao trava nada: quem usa (Dicionario/Conjunto) e
// que segura a trava dele.
type ordenado[V any] struct {
	pos     map[HashKey]int // chave -> indice em slots
	slots   []slot[V]
	buracos int
}

type slot[V any] struct {
	k    HashKey
	v    V
	vivo bool
}

func novoOrdenado[V any]() ordenado[V] {
	return ordenado[V]{pos: map[HashKey]int{}}
}

func (o *ordenado[V]) tamanho() int { return len(o.pos) }

func (o *ordenado[V]) pega(k HashKey) (V, bool) {
	if i, ok := o.pos[k]; ok {
		return o.slots[i].v, true
	}
	var zero V
	return zero, false
}

// bota insere ou atualiza. Atualizar chave que ja existe NAO muda o lugar dela
// na ordem (igual Python/JS). Devolve true se a chave era nova.
func (o *ordenado[V]) bota(k HashKey, v V) bool {
	if i, ok := o.pos[k]; ok {
		o.slots[i].v = v
		return false
	}
	o.pos[k] = len(o.slots)
	o.slots = append(o.slots, slot[V]{k: k, v: v, vivo: true})
	return true
}

// tira remove a chave. Devolve true se existia.
func (o *ordenado[V]) tira(k HashKey) bool {
	i, ok := o.pos[k]
	if !ok {
		return false
	}
	delete(o.pos, k)
	o.slots[i] = slot[V]{} // buraco (e solta o valor pro GC)
	o.buracos++
	switch {
	case len(o.pos) == 0:
		o.slots = o.slots[:0]
		o.buracos = 0
	case o.buracos > 16 && o.buracos*2 > len(o.slots):
		o.compacta()
	}
	return true
}

// compacta tira os buracos no lugar, mantendo a ordem dos vivos.
func (o *ordenado[V]) compacta() {
	vivos := o.slots[:0]
	for _, s := range o.slots {
		if s.vivo {
			o.pos[s.k] = len(vivos)
			vivos = append(vivos, s)
		}
	}
	clear(o.slots[len(vivos):])
	o.slots = vivos
	o.buracos = 0
}

// cada roda f em cada valor vivo, na ordem de entrada. f nao pode mexer aqui.
func (o *ordenado[V]) cada(f func(HashKey, V)) {
	for _, s := range o.slots {
		if s.vivo {
			f(s.k, s.v)
		}
	}
}

// ---------------------------------------------------------------- Dicionario

type ParDic struct {
	Chave Object
	Valor Object
}

type Dicionario struct {
	mu    sync.Mutex // so usado com o modo concorrente ligado
	pares ordenado[ParDic]
}

// NovoDicionario cria um dicionario vazio pronto pra receber Bota.
func NovoDicionario() *Dicionario {
	return &Dicionario{pares: novoOrdenado[ParDic]()}
}

func (d *Dicionario) Type() ObjectType { return DICIONARIO_OBJ }
func (d *Dicionario) Inspect() string  { return d.inspect(nil) }

func (d *Dicionario) inspect(emCurso map[Object]bool) string {
	if emCurso[d] {
		return "{...}"
	}
	pares := d.Pares()
	partes := make([]string, 0, len(pares))
	for _, par := range pares {
		var v string
		if ehColecaoAninhavel(par.Valor) {
			if emCurso == nil {
				emCurso = map[Object]bool{}
			}
			emCurso[d] = true
			v = inspectDentro(par.Valor, emCurso)
		} else {
			v = inspectComAspas(par.Valor)
		}
		partes = append(partes, inspectComAspas(par.Chave)+": "+v)
	}
	delete(emCurso, d)
	return "{" + strings.Join(partes, ", ") + "}"
}

// Tamanho devolve quantos pares o dicionario tem agora.
func (d *Dicionario) Tamanho() int {
	if concorrencia.Load() {
		d.mu.Lock()
		n := d.pares.tamanho()
		d.mu.Unlock()
		return n
	}
	return d.pares.tamanho()
}

// Pega busca o par da chave.
func (d *Dicionario) Pega(k HashKey) (ParDic, bool) {
	if concorrencia.Load() {
		d.mu.Lock()
		par, ok := d.pares.pega(k)
		d.mu.Unlock()
		return par, ok
	}
	return d.pares.pega(k)
}

// PegaTexto e o atalho pra chave texto (`d["nome"]`) usado pelos builtins.
func (d *Dicionario) PegaTexto(nome string) (Object, bool) {
	par, ok := d.Pega(HashKey{Tipo: TEXTO_OBJ, Valor: nome})
	return par.Valor, ok
}

// Bota insere ou atualiza um par mantendo a ordem de insercao. Sobrescrever
// chave que ja existe NAO muda o lugar dela na ordem (igual Python/JS).
func (d *Dicionario) Bota(k HashKey, par ParDic) {
	if concorrencia.Load() {
		d.mu.Lock()
		d.pares.bota(k, par)
		d.mu.Unlock()
		return
	}
	d.pares.bota(k, par)
}

// Tira remove a chave do dicionario (O(1) amortizado). Devolve true se
// existia. Se a chave voltar depois, ela entra no FIM da ordem.
func (d *Dicionario) Tira(k HashKey) bool {
	if concorrencia.Load() {
		d.mu.Lock()
		defer d.mu.Unlock()
	}
	return d.pares.tira(k)
}

// Chaves devolve uma COPIA das chaves na ordem de insercao.
func (d *Dicionario) Chaves() []HashKey {
	if concorrencia.Load() {
		d.mu.Lock()
		defer d.mu.Unlock()
	}
	out := make([]HashKey, 0, d.pares.tamanho())
	d.pares.cada(func(k HashKey, _ ParDic) { out = append(out, k) })
	return out
}

// Pares devolve uma COPIA dos pares na ordem de insercao (retrato de agora).
// E o jeito de iterar rodando codigo do usuario no meio.
func (d *Dicionario) Pares() []ParDic {
	if concorrencia.Load() {
		d.mu.Lock()
		defer d.mu.Unlock()
	}
	out := make([]ParDic, 0, d.pares.tamanho())
	d.pares.cada(func(_ HashKey, par ParDic) { out = append(out, par) })
	return out
}

// Itera roda f em cada par, na ordem de insercao. Com concorrencia itera um
// retrato (Pares); sem, vai direto, sem copiar — por isso f NAO pode rodar
// codigo do usuario nem mexer neste dicionario.
func (d *Dicionario) Itera(f func(ParDic)) {
	if concorrencia.Load() {
		for _, par := range d.Pares() {
			f(par)
		}
		return
	}
	d.pares.cada(func(_ HashKey, par ParDic) { f(par) })
}

// ---------------------------------------------------------------- Conjunto

// Conjunto implementa set com chaves do mesmo Dicionario (Chaveavel). Lembra a
// ordem de insercao igual o dicionario: `mostra` e `pra_cada` saem sempre na
// mesma ordem (antes era a ordem embaralhada do map do Go).
type Conjunto struct {
	mu    sync.Mutex // so usado com o modo concorrente ligado
	itens ordenado[Object]
}

func NovoConjunto() *Conjunto {
	return &Conjunto{itens: novoOrdenado[Object]()}
}

// Adiciona insere v no conjunto. Devolve true se era novo.
func (c *Conjunto) Adiciona(v Object) bool {
	ch, ok := v.(Chaveavel)
	if !ok {
		return false
	}
	k := ch.ChaveHash()
	if concorrencia.Load() {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	if _, existe := c.itens.pega(k); existe {
		return false // ja tinha: fica o primeiro, no lugar dele
	}
	return c.itens.bota(k, v)
}

// Contem devolve true se v esta no conjunto.
func (c *Conjunto) Contem(v Object) bool {
	ch, ok := v.(Chaveavel)
	if !ok {
		return false
	}
	k := ch.ChaveHash()
	if concorrencia.Load() {
		c.mu.Lock()
		_, existe := c.itens.pega(k)
		c.mu.Unlock()
		return existe
	}
	_, existe := c.itens.pega(k)
	return existe
}

// Remove tira v do conjunto. Devolve true se existia.
func (c *Conjunto) Remove(v Object) bool {
	ch, ok := v.(Chaveavel)
	if !ok {
		return false
	}
	k := ch.ChaveHash()
	if concorrencia.Load() {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	return c.itens.tira(k)
}

// Tamanho devolve quantos itens o conjunto tem agora.
func (c *Conjunto) Tamanho() int {
	if concorrencia.Load() {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	return c.itens.tamanho()
}

// Valores devolve uma COPIA dos itens, na ordem de insercao.
func (c *Conjunto) Valores() []Object {
	if concorrencia.Load() {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	out := make([]Object, 0, c.itens.tamanho())
	c.itens.cada(func(_ HashKey, v Object) { out = append(out, v) })
	return out
}

func (c *Conjunto) Type() ObjectType { return CONJUNTO_OBJ }
func (c *Conjunto) Inspect() string {
	vals := c.Valores()
	partes := make([]string, 0, len(vals))
	for _, v := range vals {
		partes = append(partes, inspectComAspas(v))
	}
	if len(partes) == 0 {
		return "conjunto()"
	}
	return "{" + strings.Join(partes, ", ") + "}"
}

// ---------------------------------------------------------------- ciclos

// Estrutura que contem ela mesma (`adiciona(xs, xs)`, `bota d["eu"] = d`)
// fazia o Inspect descer pra sempre e estourar a pilha do Go — fatal, nem o
// recover pega, derrubava o processo. Agora a colecao que ja esta sendo
// impressa mais acima no caminho vira a marca [...] ou {...}, igual o Python.
//
// emCurso e por PONTEIRO e guarda so o caminho atual: a mesma lista aparecendo
// duas vezes lado a lado (`[s, s]`, sem ciclo) sai inteira nas duas. O map so
// e criado quando aparece colecao dentro de colecao — lista de numero nao paga
// nada. As colecoes sao lidas por retrato (Visao/Pares), entao nenhuma trava
// fica presa enquanto a de dentro e impressa.

// ehColecaoAninhavel diz se o valor pode (direta ou indiretamente) conter
// colecao — e portanto fechar um ciclo. Conjunto so guarda chave (texto,
// numero, booleano), entao nunca fecha.
func ehColecaoAninhavel(o Object) bool {
	switch o.(type) {
	case *Lista, *Dicionario:
		return true
	}
	return false
}

// inspectDentro imprime uma lista/dicionario que esta dentro de outra colecao.
func inspectDentro(o Object, emCurso map[Object]bool) string {
	switch c := o.(type) {
	case *Lista:
		return c.inspect(emCurso)
	case *Dicionario:
		return c.inspect(emCurso)
	}
	return inspectComAspas(o)
}
