package depurador

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"gambiarrascript/object"
)

// ---- gs debug no terminal ----
//
// O programa comeca parado na entrada. Os comandos (com apelido curto):
//
//	para/b [arq:]LINHA [se COND]   poe breakpoint (sem argumento lista)
//	tira/d N                       tira o breakpoint N
//	segue/c, proximo/n, entra/s, sai/o
//	pilha/bt, quadro/f N, ve/p EXPR, vars/v, lista/l, fluxos/th
//	ajuda/h, fim/q
//
// Enter vazio repete o ultimo passo (n/s/o). Ctrl+C com o programa rodando
// pausa o proximo statement que rodar.

// OpcoesCLI ajusta o terminal.
type OpcoesCLI struct {
	// CtrlCPausa liga o tratamento do Ctrl+C (so no terminal de verdade; os
	// testes deixam desligado).
	CtrlCPausa bool
}

// saidaTravada serializa as escritas (programa e depurador escrevem no mesmo
// terminal, de goroutines diferentes).
type saidaTravada struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *saidaTravada) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// entradaLinhas le a entrada uma linha por vez e reparte entre o depurador
// (no prompt) e o programa (pergunta/le_linhas), que dividem o terminal.
type entradaLinhas struct {
	linhas chan string
	resto  []byte
}

func novaEntradaLinhas(r io.Reader) *entradaLinhas {
	e := &entradaLinhas{linhas: make(chan string)}
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			e.linhas <- sc.Text()
		}
		close(e.linhas)
	}()
	return e
}

// Read e o lado do programa.
func (e *entradaLinhas) Read(p []byte) (int, error) {
	if len(e.resto) == 0 {
		l, ok := <-e.linhas
		if !ok {
			return 0, io.EOF
		}
		e.resto = []byte(l + "\n")
	}
	n := copy(p, e.resto)
	e.resto = e.resto[n:]
	return n, nil
}

type cli struct {
	sess    *Sessao
	prog    *Programa
	out     io.Writer
	in      *entradaLinhas
	paradas chan Parada
	fim     chan int

	fluxo  int64
	quadro int
	ultimo string
	fontes map[string][]string
}

// RodaCLI roda o depurador interativo no terminal e devolve o codigo de
// saida do programa (ou 0 se o usuario saiu com `fim`).
func RodaCLI(cfg Config, in io.Reader, out io.Writer, op OpcoesCLI) int {
	so := &saidaTravada{w: out}
	ent := novaEntradaLinhas(in)
	cfg.Saida, cfg.Erro, cfg.Entrada = so, so, ent
	prog, err := Prepara(cfg)
	if err != nil {
		fmt.Fprintln(so, err.Error())
		return 1
	}
	c := &cli{prog: prog, out: so, in: ent, paradas: make(chan Parada, 16), fim: make(chan int, 1), fontes: map[string][]string{}}
	c.sess = NovaSessao(c)
	c.sess.PararNaEntrada(true)

	var sinais chan os.Signal
	if op.CtrlCPausa {
		sinais = make(chan os.Signal, 1)
		signal.Notify(sinais, os.Interrupt)
		defer signal.Stop(sinais)
	}

	fmt.Fprintf(so, "gs debug: %s (engine %s). `ajuda` lista os comandos.\n", filepath.Base(prog.Arquivo()), motorNome(cfg.Motor))
	go func() { c.fim <- prog.Roda(c.sess) }()
	for {
		select {
		case p := <-c.paradas:
			if sair := c.prompt(p, sinais); sair {
				fmt.Fprintln(so, "fui. (programa abortado)")
				return 0
			}
		case codigo := <-c.fim:
			fmt.Fprintf(so, "programa terminou (codigo %d)\n", codigo)
			return codigo
		case <-sinais:
			fmt.Fprintln(so, "pausando...")
			c.sess.Pausa(0)
		}
	}
}

func motorNome(m string) string {
	if m == "tree" {
		return "tree-walker"
	}
	return "vm"
}

// ---- Ouvinte ----

func (c *cli) Parou(p Parada)            { c.paradas <- p }
func (c *cli) FluxoComecou(int64)        {}
func (c *cli) FluxoAcabou(int64)         {}
func (c *cli) Saida(texto string)        { fmt.Fprint(c.out, texto) }
func (c *cli) printf(f string, a ...any) { fmt.Fprintf(c.out, f, a...) }

// ---- prompt ----

// prompt atende o fluxo parado ate um comando de seguir. Devolve true se o
// usuario mandou sair.
func (c *cli) prompt(p Parada, sinais chan os.Signal) bool {
	c.fluxo, c.quadro = p.Fluxo, 0
	c.anunciaParada(p)
	for {
		c.printf("(gs) ")
		var linha string
		var ok bool
		select {
		case linha, ok = <-c.in.linhas:
			if !ok {
				c.printf("\n")
				return true
			}
		case codigo := <-c.fim:
			// o principal acabou com este fluxo parado: o processo acaba
			c.printf("\nprograma terminou (codigo %d)\n", codigo)
			c.fim <- codigo
			return false
		case <-sinais:
			c.printf("\n(o programa ja ta parado; `fim` sai)\n")
			continue
		}
		linha = strings.TrimSpace(linha)
		if linha == "" {
			linha = c.ultimo
		}
		if linha == "" {
			continue
		}
		cmd, arg, _ := strings.Cut(linha, " ")
		arg = strings.TrimSpace(arg)
		switch cmd {
		case "segue", "c", "continua":
			return c.seguir(c.sess.Continua, "")
		case "proximo", "n":
			return c.seguir(c.sess.Proximo, linha)
		case "entra", "s":
			return c.seguir(c.sess.Entra, linha)
		case "sai", "o":
			return c.seguir(c.sess.Sai, linha)
		case "para", "b", "break":
			c.cmdPara(arg)
		case "tira", "d":
			c.cmdTira(arg)
		case "pilha", "bt":
			c.cmdPilha()
		case "quadro", "f":
			c.cmdQuadro(arg)
		case "ve", "p":
			c.cmdVe(arg)
		case "vars", "v":
			c.cmdVars()
		case "lista", "l":
			c.cmdLista()
		case "fluxos", "th":
			c.cmdFluxos()
		case "ajuda", "h", "?":
			c.printf("%s", textoAjuda)
		case "fim", "q", "tchau":
			return true
		default:
			c.printf("comando desconhecido: %s (digita `ajuda`)\n", cmd)
		}
	}
}

// seguir manda o fluxo andar. Erro (fluxo nao parado) fica no prompt.
func (c *cli) seguir(f func(int64) error, lembra string) bool {
	if err := f(c.fluxo); err != nil {
		c.printf("%v\n", err)
		return false
	}
	c.ultimo = lembra
	// volta pro laco de fora, que espera a proxima parada (ou o fim)
	return false
}

const textoAjuda = `comandos (apelido curto entre parenteses):
  para (b) [arq.gs:]LINHA [se COND]  poe breakpoint; sem argumento, lista
  tira (d) N                         tira o breakpoint N
  segue (c)                          continua ate o proximo breakpoint
  proximo (n)                        proxima linha (passa por cima de chamada)
  entra (s)                          proxima linha, entrando na chamada
  sai (o)                            roda ate a gambiarra atual voltar
  pilha (bt)                         mostra a pilha de chamadas
  quadro (f) N                       escolhe o quadro N da pilha (pro ve/vars/lista)
  ve (p) EXPR                        avalia a expressao no quadro
  vars (v)                           locais e globais do quadro
  lista (l)                          mostra o codigo em volta da linha
  fluxos (th)                        lista os fluxos (principal, bora...)
  ajuda (h)                          esta ajuda
  fim (q)                            aborta o programa e sai
enter vazio repete o ultimo proximo/entra/sai.
`

// quadros pega a pilha do fluxo atual.
func (c *cli) quadros() []object.Quadro {
	var qs []object.Quadro
	if err := c.sess.Executa(c.fluxo, func(f object.Fluxo) { qs = f.Quadros() }); err != nil {
		c.printf("%v\n", err)
	}
	return qs
}

func (c *cli) anunciaParada(p Parada) {
	qs := c.quadros()
	onde := ""
	if len(qs) > 0 {
		onde = " (em " + qs[0].Nome + ")"
	}
	prefixo := ""
	if p.Fluxo != 1 {
		prefixo = "[" + NomeFluxo(p.Fluxo) + "] "
	}
	pos := fmt.Sprintf("%s:%d", filepath.Base(p.Arquivo), p.Linha)
	switch p.Motivo {
	case MotivoEntrada:
		c.printf("%sparado na entrada: %s%s\n", prefixo, pos, onde)
	case MotivoBreakpoint:
		c.printf("%sparou no breakpoint %d: %s%s\n", prefixo, p.BP, pos, onde)
	case MotivoPausa:
		c.printf("%spausado: %s%s\n", prefixo, pos, onde)
	default:
		c.printf("%s%s%s\n", prefixo, pos, onde)
	}
	if p.Detalhe != "" {
		c.printf("  (%s)\n", p.Detalhe)
	}
	if l, ok := c.linhaFonte(p.Arquivo, p.Linha); ok {
		c.linhaCodigo("", p.Linha, l)
	}
}

// linhaCodigo imprime uma linha do fonte ("  12 | codigo"), sem espaco
// sobrando no fim quando a linha e vazia.
func (c *cli) linhaCodigo(marca string, n int, texto string) {
	c.printf("%s\n", strings.TrimRight(fmt.Sprintf("%s%4d | %s", marca, n, texto), " "))
}

func (c *cli) linhaFonte(arq string, n int) (string, bool) {
	ls := c.fonte(arq)
	if n < 1 || n > len(ls) {
		return "", false
	}
	return ls[n-1], true
}

func (c *cli) fonte(arq string) []string {
	if ls, ok := c.fontes[arq]; ok {
		return ls
	}
	b, err := os.ReadFile(arq)
	var ls []string
	if err == nil {
		ls = strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	}
	c.fontes[arq] = ls
	return ls
}

// cmdPara: `para 12`, `para util.gs:3`, `para 12 se x > 3`.
func (c *cli) cmdPara(arg string) {
	if arg == "" {
		bps := c.sess.Breakpoints()
		if len(bps) == 0 {
			c.printf("nenhum breakpoint\n")
		}
		for _, bp := range bps {
			c.printf("  %s\n", descreveBP(bp))
		}
		return
	}
	alvo, cond, _ := strings.Cut(arg, " se ")
	alvo = strings.TrimSpace(alvo)
	arq := c.prog.Arquivo()
	if k := strings.LastIndexByte(alvo, ':'); k >= 0 {
		arq = alvo[:k]
		if !filepath.IsAbs(arq) {
			arq = filepath.Join(filepath.Dir(c.prog.Arquivo()), arq)
		}
		alvo = alvo[k+1:]
	}
	n, err := strconv.Atoi(alvo)
	if err != nil || n < 1 {
		c.printf("linha invalida: %q (ex.: para 12, para util.gs:3, para 12 se x > 3)\n", alvo)
		return
	}
	bp := c.sess.AdicionaBreakpoint(arq, PedidoBP{Linha: n, Condicao: strings.TrimSpace(cond)})
	c.printf("%s\n", descreveBP(*bp))
}

func descreveBP(bp Breakpoint) string {
	base := filepath.Base(bp.Arquivo)
	if !bp.Verificado {
		return fmt.Sprintf("breakpoint %d em %s:%d nao vale: %s", bp.ID, base, bp.Pedida, bp.Msg)
	}
	s := fmt.Sprintf("breakpoint %d em %s:%d", bp.ID, base, bp.Linha)
	if bp.Linha != bp.Pedida {
		s += fmt.Sprintf(" (a linha %d nao roda nada; foi pra %d)", bp.Pedida, bp.Linha)
	}
	if bp.Condicao != "" {
		s += " se " + bp.Condicao
	}
	return s
}

func (c *cli) cmdTira(arg string) {
	n, err := strconv.Atoi(arg)
	if err != nil {
		c.printf("uso: tira N (o numero do breakpoint)\n")
		return
	}
	if c.sess.RemoveBreakpoint(n) {
		c.printf("breakpoint %d tirado\n", n)
	} else {
		c.printf("nao tem breakpoint %d\n", n)
	}
}

func (c *cli) cmdPilha() {
	for k, q := range c.quadros() {
		marca := "  "
		if k == c.quadro {
			marca = "=>"
		}
		c.printf("%s #%d %s  %s:%d\n", marca, k, q.Nome, filepath.Base(q.Arquivo), q.Linha)
	}
}

func (c *cli) cmdQuadro(arg string) {
	n, err := strconv.Atoi(arg)
	qs := c.quadros()
	if err != nil || n < 0 || n >= len(qs) {
		c.printf("quadro invalido (tem %d: 0 a %d)\n", len(qs), len(qs)-1)
		return
	}
	c.quadro = n
	q := qs[n]
	c.printf("#%d %s  %s:%d\n", n, q.Nome, filepath.Base(q.Arquivo), q.Linha)
	if l, ok := c.linhaFonte(q.Arquivo, q.Linha); ok {
		c.linhaCodigo("", q.Linha, l)
	}
}

func (c *cli) cmdVe(arg string) {
	if arg == "" {
		c.printf("uso: ve EXPRESSAO\n")
		return
	}
	var res object.Object
	if err := c.sess.Executa(c.fluxo, func(f object.Fluxo) { res = f.Avalia(c.quadro, arg) }); err != nil {
		c.printf("%v\n", err)
		return
	}
	if object.EhErroLevantado(res) {
		c.printf("erro: %s\n", res.Inspect())
		return
	}
	c.printf("%s\n", Mostra(res))
}

func (c *cli) cmdVars() {
	var locais, globais []object.Variavel
	if err := c.sess.Executa(c.fluxo, func(f object.Fluxo) {
		locais, globais = f.Locais(c.quadro), f.Globais(c.quadro)
	}); err != nil {
		c.printf("%v\n", err)
		return
	}
	if len(locais) > 0 {
		c.printf("locais:\n")
		for _, v := range locais {
			c.printf("  %s = %s\n", v.Nome, Mostra(v.Valor))
		}
	}
	c.printf("globais:\n")
	if len(globais) == 0 {
		c.printf("  (nenhuma)\n")
	}
	for _, v := range globais {
		c.printf("  %s = %s\n", v.Nome, Mostra(v.Valor))
	}
}

func (c *cli) cmdLista() {
	qs := c.quadros()
	if c.quadro >= len(qs) {
		return
	}
	q := qs[c.quadro]
	ls := c.fonte(q.Arquivo)
	ini, fim := q.Linha-5, q.Linha+5
	if ini < 1 {
		ini = 1
	}
	if fim > len(ls) {
		fim = len(ls)
	}
	for n := ini; n <= fim; n++ {
		marca := "  "
		if n == q.Linha {
			marca = "=>"
		}
		bp := " "
		if c.sess.TemBreakpoint(q.Arquivo, n) {
			bp = "*"
		}
		c.linhaCodigo(marca+bp, n, ls[n-1])
	}
}

func (c *cli) cmdFluxos() {
	for _, f := range c.sess.Fluxos() {
		marca := " "
		if f.ID == c.fluxo {
			marca = "*"
		}
		estado := "rodando"
		if f.Parado {
			estado = fmt.Sprintf("parado em %s:%d", filepath.Base(f.Parada.Arquivo), f.Parada.Linha)
		}
		c.printf("%s %d %s (%s)\n", marca, f.ID, f.Nome, estado)
	}
}
