package object

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gambiarrascript/ast"
)

type ObjectType string

const (
	NUMERO_OBJ   = "NUMERO"
	TEXTO_OBJ    = "TEXTO"
	BOOLEANO_OBJ = "BOOLEANO"
	NADA_OBJ     = "NADA"
	LISTA_OBJ    = "LISTA"
	FUNCAO_OBJ   = "FUNCAO"
	RETORNO_OBJ  = "RETORNO"
	ERRO_OBJ     = "ERRO"
	VAZA_OBJ     = "VAZA"
	CONTINUA_OBJ = "CONTINUA"
	SAIR_OBJ     = "SAIR"

	BUILTIN_OBJ    = "BUILTIN"
	DICIONARIO_OBJ = "DICIONARIO"

	NATIVO_OBJ = "NATIVO"

	// concorrencia
	FUTURO_OBJ = "FUTURO"
	CANO_OBJ   = "CANO"

	// colecoes extras
	CONJUNTO_OBJ = "CONJUNTO" // Set: chaves unicas
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

// FormatNumero imprime inteiros sem casa decimal e o resto com precisao minima.
func FormatNumero(f float64) string {
	if !math.IsInf(f, 0) && !math.IsNaN(f) && f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// Numero guarda um numero da linguagem. Quando EhInt e true ele representa um
// inteiro exato em Int (e Value e so um espelho aproximado pra builtins de mat);
// caso contrario o valor real e o float64 em Value. Inteiros exatos evitam a
// perda de precisao do float64 acima de 2^53 (ex.: somas/contagens gigantes).
type Numero struct {
	Value float64 // sempre preenchido (espelho de Int quando EhInt)
	Int   int64   // valor exato quando EhInt
	EhInt bool
}

// NumInt cria um numero inteiro exato.
func NumInt(i int64) *Numero { return &Numero{Value: float64(i), Int: i, EhInt: true} }

// NumFloat cria um numero de ponto flutuante.
func NumFloat(f float64) *Numero { return &Numero{Value: f} }

// RangeMax e o limite de elementos de um range `..` — evita estourar a memoria
// com algo tipo `1..999999999999`.
const RangeMax = 100_000_000

// RangeInts monta os elementos de um range inteiro inclusivo lo..hi. Cresce
// (lo<=hi) ou decresce (lo>hi). Devolve (nil, false) se passar de RangeMax.
func RangeInts(lo, hi int64) ([]Object, bool) {
	n := hi - lo
	if n < 0 {
		n = -n
	}
	if n+1 > RangeMax {
		return nil, false
	}
	elems := make([]Object, 0, n+1)
	if lo <= hi {
		for v := lo; v <= hi; v++ {
			elems = append(elems, NumInt(v))
		}
	} else {
		for v := lo; v >= hi; v-- {
			elems = append(elems, NumInt(v))
		}
	}
	return elems, true
}

func (n *Numero) Type() ObjectType { return NUMERO_OBJ }
func (n *Numero) Inspect() string {
	if n.EhInt {
		return strconv.FormatInt(n.Int, 10)
	}
	return FormatNumero(n.Value)
}

type Texto struct{ Value string }

func (t *Texto) Type() ObjectType { return TEXTO_OBJ }
func (t *Texto) Inspect() string  { return t.Value }

type Booleano struct{ Value bool }

func (b *Booleano) Type() ObjectType { return BOOLEANO_OBJ }
func (b *Booleano) Inspect() string {
	if b.Value {
		return "deu_bom"
	}
	return "deu_ruim"
}

type Nada struct{}

func (n *Nada) Type() ObjectType { return NADA_OBJ }
func (n *Nada) Inspect() string  { return "nada" }

type Lista struct{ Elements []Object }

func (l *Lista) Type() ObjectType { return LISTA_OBJ }
func (l *Lista) Inspect() string {
	partes := make([]string, len(l.Elements))
	for i, e := range l.Elements {
		partes[i] = e.Inspect()
	}
	return "[" + strings.Join(partes, ", ") + "]"
}

type Funcao struct {
	Parametros []*ast.Parametro
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Funcao) Type() ObjectType { return FUNCAO_OBJ }
func (f *Funcao) Inspect() string {
	nomes := make([]string, len(f.Parametros))
	for i, p := range f.Parametros {
		nomes[i] = p.String()
	}
	return "gambiarra(" + strings.Join(nomes, ", ") + ")"
}

// LinhaPC mapeia um offset do bytecode (PC) pra linha do codigo-fonte. A
// tabela e esparsa: o compiler so grava uma entrada quando a linha muda; a
// linha de um ip qualquer e a da ultima entrada com PC <= ip.
type LinhaPC struct {
	PC    int
	Linha int
}

// CompiledFunction e a representacao de uma funcao na VM: bytecode + numArgs +
// numLocals (slots pra params e locals no frame) + freeVars capturadas.
// Reaproveita o mesmo FUNCAO_OBJ pra nao ter que adicionar outro ObjectType e
// quebrar scripts que checam Inspect/Type. Na arvore (tree-walker) o campo
// Compiled e nil; na VM usamos so os campos Bytecode/NumArgs/NumLocals/Free.
type CompiledFunction struct {
	Name      string
	NumArgs   int
	MinArgs   int // args requeridos (sem default e sem varargs); 0 = nenhum
	NumLocals int
	Bytecode  []byte
	Free      []Object
	Linhas    []LinhaPC // tabela pc->linha pra erros com posicao
	Variadic  bool       // true: ultimo param e ...resto (coleta args extras)
}

func (f *CompiledFunction) Type() ObjectType { return FUNCAO_OBJ }
func (f *CompiledFunction) Inspect() string {
	return "gambiarra<" + f.Name + ">(vm)"
}

// LinhaDoPC devolve a linha do fonte pro offset ip (0 = desconhecida).
// Scan linear: a tabela e pequena e isso so roda em caminho de erro.
func (f *CompiledFunction) LinhaDoPC(ip int) int {
	linha := 0
	for _, e := range f.Linhas {
		if e.PC > ip {
			break
		}
		linha = e.Linha
	}
	return linha
}

type Retorno struct{ Value Object }

func (r *Retorno) Type() ObjectType { return RETORNO_OBJ }
func (r *Retorno) Inspect() string  { return r.Value.Inspect() }

// StackFrame e um nivel da pilha de chamadas num traço de erro.
type StackFrame struct {
	Funcao string // nome da gambiarra chamada (ou "<topo>" / "<anonima>")
	Line   int    // linha da chamada no codigo-fonte
}

// Erro e o valor de erro da linguagem. Message continua sendo o texto que o
// usuario ve (compativel com concatenacao: Inspect devolve Message), mas agora
// carrega tambem Line, Kind, Stack e Cause pra inspecao programatica.
type Erro struct {
	Message string       // texto completo que o usuario ve
	Line    int          // linha de origem (0 = desconhecida, p.ex. builtin)
	Kind    string       // "runtime", "builtin", "io", "rede", "parse", "usuario"
	Stack   []StackFrame // traço de pilha, do mais externo pro mais interno
	Cause   *Erro        // erro original (para encadeamento / wrap)
	Handled bool         // true depois que um `arruma ... quebrou` capturou;
	// significa "ja foi tratado", nao deve voltar a
	// propagar — deixa o usuario logar/inspecionar.
}

func (e *Erro) Type() ObjectType { return ERRO_OBJ }

// Inspect devolve so Message — assim `erro + "x"` continua funcionando como
// antes e a saida nao muda retroativamente.
func (e *Erro) Inspect() string { return e.Message }

// Traco devolve o traço de pilha formatado em multi-linhas, pra mostrar em
// diagnosticos e na builtin erro_pilha.
func (e *Erro) Traco() string {
	if len(e.Stack) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range e.Stack {
		if f.Funcao == "" {
			f.Funcao = "<anonima>"
		}
		fmt.Fprintf(&b, "  em %s (linha %d)\n", f.Funcao, f.Line)
	}
	return b.String()
}

type Vaza struct{ Line int }

func (v *Vaza) Type() ObjectType { return VAZA_OBJ }
func (v *Vaza) Inspect() string  { return "vaza" }

type Continua struct{ Line int }

func (c *Continua) Type() ObjectType { return CONTINUA_OBJ }
func (c *Continua) Inspect() string  { return "continua" }

// Sair e o objeto de controle do builtin `sai(codigo)`: desenrola tudo (blocos,
// loops, funcoes) ate o topo, onde o runner encerra o processo com o codigo.
// Diferente de Vaza/Continua (que param no loop), Sair nao para em lugar nenhum.
type Sair struct{ Codigo int }

func (s *Sair) Type() ObjectType { return SAIR_OBJ }
func (s *Sair) Inspect() string  { return "sai" }

// IndiceNormalizado resolve indice negativo (estilo Python: -1 = ultimo) e
// checa limites. Devolve (indice real, true) se valido; (0, false) se fora.
// Usado por lista e texto nos dois engines pra indexacao/atribuicao.
func IndiceNormalizado(pos, tamanho int) (int, bool) {
	if pos < 0 {
		pos += tamanho
	}
	if pos < 0 || pos >= tamanho {
		return 0, false
	}
	return pos, true
}

// NormalizarFatia resolve os indices de uma fatia [inicio:fim] (nil = omitido),
// suportando indices negativos (estilo Python). Devolve (lo, hi) validos pra
// usar direto num slice Go (c.Elements[lo:hi]). Clampa nos limites [0, tamanho].
func NormalizarFatia(inicio, fim *Numero, tamanho int) (int, int) {
	lo := 0
	if inicio != nil {
		lo = int(inicio.Value)
		if lo < 0 {
			lo += tamanho
		}
		if lo < 0 {
			lo = 0
		}
		if lo > tamanho {
			lo = tamanho
		}
	}
	hi := tamanho
	if fim != nil {
		hi = int(fim.Value)
		if hi < 0 {
			hi += tamanho
		}
		if hi < 0 {
			hi = 0
		}
		if hi > tamanho {
			hi = tamanho
		}
	}
	if lo > hi {
		lo = hi
	}
	return lo, hi
}

type BuiltinFunc func(args []Object) Object

type Builtin struct {
	Nome string
	Fn   BuiltinFunc
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin " + b.Nome }

// HashKey e a chave canonica usada no mapa interno do dicionario.
type HashKey struct {
	Tipo  ObjectType
	Valor string
}

// Chaveavel e implementado pelos tipos que podem ser chave de dicionario.
type Chaveavel interface {
	ChaveHash() HashKey
}

func (t *Texto) ChaveHash() HashKey { return HashKey{Tipo: TEXTO_OBJ, Valor: t.Value} }
func (n *Numero) ChaveHash() HashKey {
	if n.EhInt {
		return HashKey{Tipo: NUMERO_OBJ, Valor: strconv.FormatInt(n.Int, 10)}
	}
	return HashKey{Tipo: NUMERO_OBJ, Valor: FormatNumero(n.Value)}
}
func (b *Booleano) ChaveHash() HashKey { return HashKey{Tipo: BOOLEANO_OBJ, Valor: b.Inspect()} }

type ParDic struct {
	Chave Object
	Valor Object
}

type Dicionario struct {
	Pares map[HashKey]ParDic
}

func (d *Dicionario) Type() ObjectType { return DICIONARIO_OBJ }
func (d *Dicionario) Inspect() string {
	partes := make([]string, 0, len(d.Pares))
	for _, par := range d.Pares {
		partes = append(partes, inspectComAspas(par.Chave)+": "+inspectComAspas(par.Valor))
	}
	return "{" + strings.Join(partes, ", ") + "}"
}

// Nativo e um handle opaco que embrulha um valor Go (ex.: uma conexao de banco).
type Nativo struct {
	Rotulo string
	Valor  interface{}
}

func (n *Nativo) Type() ObjectType { return NATIVO_OBJ }
func (n *Nativo) Inspect() string  { return "<nativo: " + n.Rotulo + ">" }

// Futuro e o valor devolvido por `bora fn(args)`: representa uma chamada
// concorrente em andamento. `Valor` so e preenchido quando a goroutine termina;
// ate la `Pronto` e falso. Usa-se `espera(futuro)` pra bloquear ate resolver.
type Futuro struct {
	// pronto e fechado quando a goroutine termina; apos isso Valor e estavel.
	pronto chan struct{}
	val    Object // valor devolvido pela fn (pode ser *Erro)
	once   Once
}

// Once e uma casca leve em cima de sync.Once pra evitar importar sync aqui
// (mantem o pacote object independente).
type Once struct {
	done chan struct{}
}

func NovaOnce() Once {
	return Once{done: make(chan struct{})}
}

func (o *Once) Do(f func()) {
	select {
	case <-o.done:
		// ja rodou — ignora
		return
	default:
		f()
		close(o.done)
	}
}

// Aguarda bloqueia a goroutine ate o futuro resolver e devolve o valor.
// Multiplas chamadas concorrentes sao seguras.
func (f *Futuro) Aguarda() Object {
	<-f.pronto
	return f.val
}

// Resolve completa o futuro com o valor. So a primeira chamada tem efeito;
// as demais sao silenciadas (defensive contra goroutine que chama duas vezes).
func (f *Futuro) Resolve(v Object) {
	f.once.Do(func() {
		f.val = v
		close(f.pronto)
	})
}

// NovoFuturo cria um Futuro vazio (nao resolvido).
func NovoFuturo() *Futuro {
	return &Futuro{
		pronto: make(chan struct{}),
		once:   NovaOnce(),
	}
}

func (f *Futuro) Type() ObjectType { return FUTURO_OBJ }
func (f *Futuro) Inspect() string {
	select {
	case <-f.pronto:
		return "<futuro resolvido: " + f.val.Inspect() + ">"
	default:
		return "<futuro em andamento>"
	}
}

// Cano e o canal de mensagens (estilo Go channel) da linguagem. Permite
// producer/consumer entre goroutines. Pode ser bufferizado (capacidade > 0)
// ou sincrono (capacidade 0 — envia bloqueia ate ter receptor).
type Cano struct {
	Ch    chan Object
	Cap   int // capacidade pedida (0 = unbuffered)
	fecha Once
}

// NovoCano cria um canal com a capacidade dada.
func NovoCano(capacidade int) *Cano {
	if capacidade < 0 {
		capacidade = 0
	}
	return &Cano{
		Ch:    make(chan Object, capacidade),
		Cap:   capacidade,
		fecha: NovaOnce(),
	}
}

// Fecha o canal (idempotente).
func (c *Cano) Fechar() {
	c.fecha.Do(func() {
		close(c.Ch)
	})
}

func (c *Cano) Type() ObjectType { return CANO_OBJ }
func (c *Cano) Inspect() string {
	if c.Cap == 0 {
		return "<cano sincrono>"
	}
	return "<cano buffer=" + strconv.Itoa(c.Cap) + ">"
}

// inspectComAspas envolve textos em aspas (estilo JSON) e usa Inspect no resto.
func inspectComAspas(o Object) string {
	if t, ok := o.(*Texto); ok {
		return `"` + t.Value + `"`
	}
	return o.Inspect()
}

// Conjunto implementa set com chaves do mesmo Dicionario (Chaveavel).
type Conjunto struct {
	Items map[HashKey]Object
}

func NovoConjunto() *Conjunto {
	return &Conjunto{Items: map[HashKey]Object{}}
}

// Adiciona insere v no conjunto. Devolve true se era novo.
func (c *Conjunto) Adiciona(v Object) bool {
	ch, ok := v.(Chaveavel)
	if !ok {
		return false
	}
	k := ch.ChaveHash()
	if _, existe := c.Items[k]; existe {
		return false
	}
	c.Items[k] = v
	return true
}

// Contem devolve true se v esta no conjunto.
func (c *Conjunto) Contem(v Object) bool {
	ch, ok := v.(Chaveavel)
	if !ok {
		return false
	}
	_, existe := c.Items[ch.ChaveHash()]
	return existe
}

// Remove tira v do conjunto. Devolve true se existia.
func (c *Conjunto) Remove(v Object) bool {
	ch, ok := v.(Chaveavel)
	if !ok {
		return false
	}
	k := ch.ChaveHash()
	if _, existe := c.Items[k]; !existe {
		return false
	}
	delete(c.Items, k)
	return true
}

func (c *Conjunto) Type() ObjectType { return CONJUNTO_OBJ }
func (c *Conjunto) Inspect() string {
	partes := make([]string, 0, len(c.Items))
	for _, v := range c.Items {
		partes = append(partes, inspectComAspas(v))
	}
	if len(partes) == 0 {
		return "conjunto()"
	}
	return "{" + strings.Join(partes, ", ") + "}"
}
