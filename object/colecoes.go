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
func (l *Lista) Inspect() string {
	elems := l.Visao()
	partes := make([]string, len(elems))
	for i, e := range elems {
		partes[i] = e.Inspect()
	}
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

// ---------------------------------------------------------------- Dicionario

type ParDic struct {
	Chave Object
	Valor Object
}

type Dicionario struct {
	mu    sync.Mutex // so usado com o modo concorrente ligado
	pares map[HashKey]ParDic
	// ordem guarda as chaves na ordem em que ENTRARAM. Sem ela a iteracao usava
	// a ordem do map do Go, que e embaralhada de proposito e muda a cada
	// execucao — `pra_cada k em d` saia diferente toda vez que voce rodava.
	// Igual Python 3.7+ e JS: quem chega primeiro, sai primeiro.
	ordem []HashKey
}

// NovoDicionario cria um dicionario vazio pronto pra receber Bota.
func NovoDicionario() *Dicionario {
	return &Dicionario{pares: map[HashKey]ParDic{}}
}

func (d *Dicionario) Type() ObjectType { return DICIONARIO_OBJ }
func (d *Dicionario) Inspect() string {
	pares := d.Pares()
	partes := make([]string, 0, len(pares))
	for _, par := range pares {
		partes = append(partes, inspectComAspas(par.Chave)+": "+inspectComAspas(par.Valor))
	}
	return "{" + strings.Join(partes, ", ") + "}"
}

// Tamanho devolve quantos pares o dicionario tem agora.
func (d *Dicionario) Tamanho() int {
	if concorrencia.Load() {
		d.mu.Lock()
		n := len(d.pares)
		d.mu.Unlock()
		return n
	}
	return len(d.pares)
}

// Pega busca o par da chave.
func (d *Dicionario) Pega(k HashKey) (ParDic, bool) {
	if concorrencia.Load() {
		d.mu.Lock()
		par, ok := d.pares[k]
		d.mu.Unlock()
		return par, ok
	}
	par, ok := d.pares[k]
	return par, ok
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
		d.bota(k, par)
		d.mu.Unlock()
		return
	}
	d.bota(k, par)
}

func (d *Dicionario) bota(k HashKey, par ParDic) {
	if _, ja := d.pares[k]; !ja {
		d.ordem = append(d.ordem, k)
	}
	d.pares[k] = par
}

// Tira remove a chave do dicionario e da ordem. Devolve true se existia.
func (d *Dicionario) Tira(k HashKey) bool {
	if concorrencia.Load() {
		d.mu.Lock()
		defer d.mu.Unlock()
	}
	if _, ja := d.pares[k]; !ja {
		return false
	}
	delete(d.pares, k)
	for i, o := range d.ordem {
		if o == k {
			d.ordem = append(d.ordem[:i], d.ordem[i+1:]...)
			break
		}
	}
	return true
}

// Chaves devolve uma COPIA das chaves na ordem de insercao.
func (d *Dicionario) Chaves() []HashKey {
	if concorrencia.Load() {
		d.mu.Lock()
		defer d.mu.Unlock()
	}
	out := make([]HashKey, len(d.ordem))
	copy(out, d.ordem)
	return out
}

// Pares devolve uma COPIA dos pares na ordem de insercao (retrato de agora).
// E o jeito de iterar rodando codigo do usuario no meio.
func (d *Dicionario) Pares() []ParDic {
	if concorrencia.Load() {
		d.mu.Lock()
		defer d.mu.Unlock()
	}
	out := make([]ParDic, len(d.ordem))
	for i, k := range d.ordem {
		out[i] = d.pares[k]
	}
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
	for _, k := range d.ordem {
		f(d.pares[k])
	}
}

// ---------------------------------------------------------------- Conjunto

// Conjunto implementa set com chaves do mesmo Dicionario (Chaveavel).
type Conjunto struct {
	mu    sync.Mutex // so usado com o modo concorrente ligado
	itens map[HashKey]Object
}

func NovoConjunto() *Conjunto {
	return &Conjunto{itens: map[HashKey]Object{}}
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
	if _, existe := c.itens[k]; existe {
		return false
	}
	c.itens[k] = v
	return true
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
		_, existe := c.itens[k]
		c.mu.Unlock()
		return existe
	}
	_, existe := c.itens[k]
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
	if _, existe := c.itens[k]; !existe {
		return false
	}
	delete(c.itens, k)
	return true
}

// Tamanho devolve quantos itens o conjunto tem agora.
func (c *Conjunto) Tamanho() int {
	if concorrencia.Load() {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	return len(c.itens)
}

// Valores devolve uma COPIA dos itens (ordem nao garantida).
func (c *Conjunto) Valores() []Object {
	if concorrencia.Load() {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	out := make([]Object, 0, len(c.itens))
	for _, v := range c.itens {
		out = append(out, v)
	}
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
