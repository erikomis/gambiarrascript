// Package depurador e o depurador do GambiarraScript (gs debug): breakpoints,
// passo a passo e inspecao em cima da API Depurador/Fluxo dos dois engines
// (object/gancho.go). Tem dois frontends: o terminal interativo (cli.go) e o
// Debug Adapter Protocol pro VSCode (dap.go).
//
// Modelo: cada fluxo do programa (goroutine: o principal, cada bora, cada
// handler) para sozinho. Quando um fluxo para, a goroutine dele fica presa
// dentro do gancho esperando pedidos (Executa) e a ordem de seguir; os
// outros fluxos continuam rodando. Toda inspecao (pilha, variaveis, avaliar)
// roda NA goroutine do fluxo parado, que e o que o contrato do Fluxo pede.
package depurador

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Motivos de parada (os mesmos nomes do DAP).
const (
	MotivoEntrada    = "entry"
	MotivoBreakpoint = "breakpoint"
	MotivoPasso      = "step"
	MotivoPausa      = "pause"
)

// Breakpoint e um ponto de parada num arquivo. A linha pedida anda pra
// primeira linha executavel dali pra baixo (igual os outros depuradores);
// sem nenhuma, fica nao verificado.
type Breakpoint struct {
	ID         int
	Arquivo    string // caminho como foi pedido (absoluto)
	Pedida     int
	Linha      int // linha onde ficou (0 = nenhuma)
	Verificado bool
	Condicao   string // so para se a expressao der verdadeiro
	Log        string // logpoint: nao para, escreve a mensagem ({expr} e avaliado)
	Msg        string // por que nao ficou verificado
}

// Parada descreve um fluxo que acabou de parar.
type Parada struct {
	Fluxo   int64
	Motivo  string
	Arquivo string
	Linha   int
	BP      int    // id do breakpoint (0 = nao foi breakpoint)
	Detalhe string // ex.: erro na condicao do breakpoint
}

// Ouvinte recebe os eventos da sessao (o frontend). Parou e chamado da
// goroutine do fluxo que parou: nao pode bloquear por muito tempo.
type Ouvinte interface {
	Parou(p Parada)
	FluxoComecou(id int64)
	FluxoAcabou(id int64)
	// Saida: mensagem do depurador pro usuario (logpoint, aviso).
	Saida(texto string)
}

type tipoPasso int

const (
	semPasso tipoPasso = iota
	passoEntra
	passoProximo
	passoSai
)

// fio e o estado de um fluxo na sessao.
type fio struct {
	id        int64
	passo     tipoPasso
	prof      int // profundidade de referencia do passo
	pausar    bool
	parado    bool
	avaliando bool // rodando pedido/condicao: o gancho reentrante e ignorado
	profAgora int  // profundidade na parada
	parada    Parada
	pedidos   chan pedido
}

type pedido struct {
	fn     func(f object.Fluxo)
	feito  chan struct{}
	retoma bool
}

// Sessao implementa object.Depurador.
type Sessao struct {
	ouvinte Ouvinte

	mu             sync.Mutex
	fios           map[int64]*fio
	bps            map[string][]*Breakpoint // arquivo normalizado -> breakpoints
	proxBP         int
	pararNaEntrada bool
	entradaVista   bool
	pausarTodos    bool
	desligado      bool
	norm           map[string]string // cache de caminho -> normalizado
}

// NovaSessao cria a sessao que manda os eventos pro ouvinte.
func NovaSessao(o Ouvinte) *Sessao {
	return &Sessao{ouvinte: o, fios: map[int64]*fio{}, bps: map[string][]*Breakpoint{}, norm: map[string]string{}}
}

// PararNaEntrada faz o fluxo principal parar antes do primeiro statement.
func (s *Sessao) PararNaEntrada(sim bool) {
	s.mu.Lock()
	s.pararNaEntrada = sim
	s.mu.Unlock()
}

// Desliga faz a sessao nao parar mais em nada (rodar sem depurar) e solta
// quem esta parado.
func (s *Sessao) Desliga() {
	s.mu.Lock()
	s.desligado = true
	var parados []*fio
	for _, t := range s.fios {
		if t.parado {
			parados = append(parados, t)
		}
	}
	s.mu.Unlock()
	for _, t := range parados {
		t.pedidos <- pedido{retoma: true}
	}
}

// normaliza da a chave de comparacao de um caminho (absoluto, sem symlink:
// /tmp e /private/tmp no macOS sao o mesmo arquivo).
func (s *Sessao) normaliza(p string) string {
	if p == "" {
		return ""
	}
	if n, ok := s.norm[p]; ok {
		return n
	}
	n := p
	if abs, err := filepath.Abs(n); err == nil {
		n = abs
	}
	if real, err := filepath.EvalSymlinks(n); err == nil {
		n = real
	}
	s.norm[p] = n
	return n
}

func (s *Sessao) fioDe(id int64) (*fio, bool) {
	t := s.fios[id]
	if t != nil {
		return t, false
	}
	t = &fio{id: id, pedidos: make(chan pedido)}
	s.fios[id] = t
	return t, true
}

// Linha e o gancho: decide se o fluxo para aqui.
func (s *Sessao) Linha(sitio *object.SitioLinha, f object.Fluxo) {
	id := f.ID()
	s.mu.Lock()
	t, novo := s.fioDe(id)
	if t.avaliando || s.desligado {
		s.mu.Unlock()
		if novo {
			s.ouvinte.FluxoComecou(id)
		}
		return
	}
	motivo := ""
	switch {
	case t.pausar || s.pausarTodos:
		motivo = MotivoPausa
		t.pausar, s.pausarTodos = false, false
	case t.passo == passoEntra:
		motivo = MotivoPasso
	case t.passo == passoProximo && f.Profundidade() <= t.prof:
		motivo = MotivoPasso
	case t.passo == passoSai && f.Profundidade() < t.prof:
		motivo = MotivoPasso
	case id == 1 && s.pararNaEntrada && !s.entradaVista:
		motivo = MotivoEntrada
	}
	if id == 1 {
		s.entradaVista = true
	}
	var cands []Breakpoint
	if motivo == "" {
		for _, bp := range s.bps[s.normaliza(sitio.Arquivo)] {
			if bp.Verificado && bp.Linha == sitio.Linha {
				cands = append(cands, *bp)
			}
		}
	}
	s.mu.Unlock()
	if novo {
		s.ouvinte.FluxoComecou(id)
	}

	p := Parada{Fluxo: id, Motivo: motivo, Arquivo: sitio.Arquivo, Linha: sitio.Linha}
	if motivo == "" && len(cands) > 0 {
		s.marcaAvaliando(t, true)
		for _, bp := range cands {
			para, detalhe := s.confereBP(bp, f)
			if para {
				p.Motivo, p.BP, p.Detalhe = MotivoBreakpoint, bp.ID, detalhe
				break
			}
		}
		s.marcaAvaliando(t, false)
	}
	if p.Motivo == "" {
		return
	}
	s.para(t, f, p)
}

func (s *Sessao) marcaAvaliando(t *fio, v bool) {
	s.mu.Lock()
	t.avaliando = v
	s.mu.Unlock()
}

// confereBP diz se o breakpoint para (condicao) — logpoint escreve e segue.
func (s *Sessao) confereBP(bp Breakpoint, f object.Fluxo) (bool, string) {
	if bp.Condicao != "" {
		v := f.Avalia(0, bp.Condicao)
		if object.EhErroLevantado(v) {
			// condicao quebrada: para e conta o porque (melhor que ignorar)
			return true, "erro na condicao `" + bp.Condicao + "`: " + v.Inspect()
		}
		if !verdadeiro(v) {
			return false, ""
		}
	}
	if bp.Log != "" {
		s.ouvinte.Saida(interpolaLog(bp.Log, f) + "\n")
		return false, ""
	}
	return true, ""
}

func verdadeiro(v object.Object) bool {
	switch x := v.(type) {
	case *object.Nada:
		return false
	case *object.Booleano:
		return x.Value
	}
	return true
}

// interpolaLog troca cada {expr} da mensagem do logpoint pelo valor.
func interpolaLog(msg string, f object.Fluxo) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(msg, '{')
		if i < 0 {
			b.WriteString(msg)
			break
		}
		j := strings.IndexByte(msg[i:], '}')
		if j < 0 {
			b.WriteString(msg)
			break
		}
		b.WriteString(msg[:i])
		b.WriteString(f.Avalia(0, msg[i+1:i+j]).Inspect())
		msg = msg[i+j+1:]
	}
	return b.String()
}

// para segura o fluxo aqui ate alguem mandar seguir, rodando os pedidos de
// inspecao na goroutine dele.
func (s *Sessao) para(t *fio, f object.Fluxo, p Parada) {
	prof := f.Profundidade()
	s.mu.Lock()
	t.passo = semPasso
	t.parado = true
	t.profAgora = prof
	t.parada = p
	s.mu.Unlock()
	s.ouvinte.Parou(p)
	for pd := range t.pedidos {
		if pd.fn != nil {
			s.marcaAvaliando(t, true)
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.ouvinte.Saida(fmt.Sprintf("depurador: panico inspecionando: %v\n", r))
					}
				}()
				pd.fn(f)
			}()
			s.marcaAvaliando(t, false)
			close(pd.feito)
		}
		if pd.retoma {
			break
		}
	}
	s.mu.Lock()
	t.parado = false
	s.mu.Unlock()
}

// FluxoAcabou: o fluxo terminou (bora/handler voltou).
func (s *Sessao) FluxoAcabou(id int64) {
	s.mu.Lock()
	_, existia := s.fios[id]
	delete(s.fios, id)
	s.mu.Unlock()
	if existia {
		s.ouvinte.FluxoAcabou(id)
	}
}

// ---- comandos do frontend ----

// ErrNaoParado: o fluxo pedido nao esta parado (ou nao existe).
var ErrNaoParado = fmt.Errorf("esse fluxo nao ta parado")

func (s *Sessao) parado(id int64) (*fio, error) {
	t := s.fios[id]
	if t == nil || !t.parado {
		return nil, ErrNaoParado
	}
	return t, nil
}

// EstaParado diz se o fluxo esta parado esperando comando.
func (s *Sessao) EstaParado(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.parado(id)
	return err == nil
}

// Executa roda fn na goroutine do fluxo parado e espera acabar.
func (s *Sessao) Executa(id int64, fn func(f object.Fluxo)) error {
	s.mu.Lock()
	t, err := s.parado(id)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	feito := make(chan struct{})
	t.pedidos <- pedido{fn: fn, feito: feito}
	<-feito
	return nil
}

func (s *Sessao) segue(id int64, passo tipoPasso) error {
	s.mu.Lock()
	t, err := s.parado(id)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	t.passo = passo
	t.prof = t.profAgora
	t.parado = false
	s.mu.Unlock()
	t.pedidos <- pedido{retoma: true}
	return nil
}

// Continua solta o fluxo ate o proximo breakpoint (ou pausa).
func (s *Sessao) Continua(id int64) error { return s.segue(id, semPasso) }

// Proximo para no proximo statement do mesmo quadro ou de quem chamou.
func (s *Sessao) Proximo(id int64) error { return s.segue(id, passoProximo) }

// Entra para no proximo statement, em qualquer quadro (entra na chamada).
func (s *Sessao) Entra(id int64) error { return s.segue(id, passoEntra) }

// Sai para no proximo statement depois que o quadro atual voltar.
func (s *Sessao) Sai(id int64) error { return s.segue(id, passoSai) }

// Pausa pede pro fluxo parar no proximo statement (id 0 = o primeiro fluxo
// que rodar um statement).
func (s *Sessao) Pausa(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := s.fios[id]; id != 0 && t != nil {
		t.pausar = true
		return
	}
	s.pausarTodos = true
}

// InfoFluxo resume um fluxo vivo.
type InfoFluxo struct {
	ID     int64
	Nome   string
	Parado bool
	Parada Parada
}

// NomeFluxo e o nome mostrado pro fluxo.
func NomeFluxo(id int64) string {
	if id == 1 {
		return "principal"
	}
	return fmt.Sprintf("fluxo %d", id)
}

// Fluxos lista os fluxos vivos, pelo id.
func (s *Sessao) Fluxos() []InfoFluxo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]InfoFluxo, 0, len(s.fios))
	for id, t := range s.fios {
		out = append(out, InfoFluxo{ID: id, Nome: NomeFluxo(id), Parado: t.parado, Parada: t.parada})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

// ---- breakpoints ----

// PedidoBP e um breakpoint pedido pelo frontend.
type PedidoBP struct {
	Linha    int
	Condicao string
	Log      string
}

// LinhasExecutaveis le e parseia o arquivo e devolve as linhas onde comeca
// algum statement executavel (a mesma regra dos engines).
func LinhasExecutaveis(arquivo string) ([]int, error) {
	fonte, err := os.ReadFile(arquivo)
	if err != nil {
		return nil, err
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("o arquivo tem erro de parse: %s", errs[0])
	}
	return ast.LinhasExecutaveis(prog), nil
}

// resolve acha a linha executavel pro breakpoint.
func resolve(bp *Breakpoint, linhas []int, errArq error) {
	if errArq != nil {
		bp.Msg = errArq.Error()
		return
	}
	k := sort.SearchInts(linhas, bp.Pedida)
	if k >= len(linhas) {
		bp.Msg = fmt.Sprintf("nenhuma linha executavel da %d pra baixo", bp.Pedida)
		return
	}
	bp.Linha = linhas[k]
	bp.Verificado = true
}

// DefineBreakpoints troca TODOS os breakpoints do arquivo pelos pedidos
// (semantica do setBreakpoints do DAP) e devolve eles na mesma ordem.
func (s *Sessao) DefineBreakpoints(arquivo string, pedidos []PedidoBP) []*Breakpoint {
	linhas, errArq := LinhasExecutaveis(arquivo)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Breakpoint, len(pedidos))
	for k, pd := range pedidos {
		s.proxBP++
		bp := &Breakpoint{ID: s.proxBP, Arquivo: arquivo, Pedida: pd.Linha, Condicao: pd.Condicao, Log: pd.Log}
		resolve(bp, linhas, errArq)
		out[k] = bp
	}
	s.bps[s.normaliza(arquivo)] = out
	return out
}

// AdicionaBreakpoint poe mais um breakpoint no arquivo (CLI).
func (s *Sessao) AdicionaBreakpoint(arquivo string, pd PedidoBP) *Breakpoint {
	linhas, errArq := LinhasExecutaveis(arquivo)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proxBP++
	bp := &Breakpoint{ID: s.proxBP, Arquivo: arquivo, Pedida: pd.Linha, Condicao: pd.Condicao, Log: pd.Log}
	resolve(bp, linhas, errArq)
	n := s.normaliza(arquivo)
	s.bps[n] = append(s.bps[n], bp)
	return bp
}

// RemoveBreakpoint tira o breakpoint pelo id.
func (s *Sessao) RemoveBreakpoint(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for arq, lista := range s.bps {
		for k, bp := range lista {
			if bp.ID == id {
				s.bps[arq] = append(lista[:k:k], lista[k+1:]...)
				return true
			}
		}
	}
	return false
}

// Breakpoints lista todos, pelo id.
func (s *Sessao) Breakpoints() []Breakpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Breakpoint
	for _, lista := range s.bps {
		for _, bp := range lista {
			out = append(out, *bp)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

// TemBreakpoint diz se tem breakpoint verificado na linha do arquivo.
func (s *Sessao) TemBreakpoint(arquivo string, linha int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, bp := range s.bps[s.normaliza(arquivo)] {
		if bp.Verificado && bp.Linha == linha {
			return true
		}
	}
	return false
}
