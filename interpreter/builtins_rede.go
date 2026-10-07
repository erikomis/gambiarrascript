//go:build !js

// Rede baixo nivel: TCP (cliente e servidor) e UDP. Toda conexao implementa
// object.Conexao e mora num *object.Nativo, entao se usa igual a um cano:
// envia(c, texto), recebe(c) (bloqueia; nada quando o outro lado fechou) e
// fecha(c). No navegador (wasm) quem assume e o stub em builtins_rede_js.go.

package interpreter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gambiarrascript/object"
)

const (
	// tamanhoPedacoBruto e o maximo que um recebe() devolve no modo bruto
	// (e o tamanho do buffer de leitura de datagrama UDP — 64 KiB cobre
	// qualquer datagrama).
	tamanhoPedacoBruto = 64 * 1024
	// limiteLinha protege o modo linha de quem manda um rio sem \n.
	limiteLinha = 1024 * 1024
	// esperaHandlers e quanto a parada graciosa espera os handlers terminarem
	// depois de fechar as conexoes.
	esperaHandlers = 5 * time.Second
)

// opcoesRede e o dicionario de opcoes ja validado.
type opcoesRede struct {
	bruto   bool
	timeout time.Duration
	pronto  *object.Cano
	para    *object.Cano
}

// lerOpcoesRede valida o dicionario de opcoes contra as chaves permitidas
// pra cada builtin — chave desconhecida e erro (melhor que ignorar calado).
func lerOpcoesRede(nome string, o object.Object, permitidas ...string) (opcoesRede, *object.Erro) {
	var op opcoesRede
	d, ok := o.(*object.Dicionario)
	if !ok {
		return op, erroBuiltin("%s(): as opcoes tem que ser um dicionario, veio %s", nome, o.Type())
	}
	for _, par := range d.Pares() {
		k, ok := par.Chave.(*object.Texto)
		if !ok || !contemTexto(permitidas, k.Value) {
			return op, erroBuiltin("%s(): opcao %s nao existe aqui (vale: %s)", nome, inspectComAspasRede(par.Chave), strings.Join(permitidas, ", "))
		}
		switch k.Value {
		case "modo":
			t, ok := par.Valor.(*object.Texto)
			if !ok || (t.Value != "linha" && t.Value != "bruto") {
				return op, erroBuiltin("%s(): modo %s nao existe — e \"linha\" (padrao) ou \"bruto\"", nome, inspectComAspasRede(par.Valor))
			}
			op.bruto = t.Value == "bruto"
		case "timeout":
			n, ok := par.Valor.(*object.Numero)
			if !ok || n.Value <= 0 {
				return op, erroBuiltin("%s(): timeout tem que ser numero de segundos maior que zero, veio %s", nome, par.Valor.Inspect())
			}
			op.timeout = time.Duration(n.Value * float64(time.Second))
		case "pronto", "para":
			c, ok := par.Valor.(*object.Cano)
			if !ok {
				return op, erroBuiltin("%s(): a opcao %q tem que ser um cano, veio %s", nome, k.Value, par.Valor.Type())
			}
			if k.Value == "pronto" {
				op.pronto = c
			} else {
				op.para = c
			}
		}
	}
	return op, nil
}

func contemTexto(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func inspectComAspasRede(o object.Object) string {
	if t, ok := o.(*object.Texto); ok {
		return strconv.Quote(t.Value)
	}
	return o.Inspect()
}

// segundos formata a duracao do jeito que o usuario escreveu (1.5 -> "1.5s").
func segundos(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + "s"
}

// enderecoEscuta: numero vira ":porta" (todas as interfaces); texto sem ":"
// tambem e porta; texto com ":" vai do jeito que veio ("127.0.0.1:0").
func enderecoEscuta(nome string, o object.Object) (string, *object.Erro) {
	switch v := o.(type) {
	case *object.Numero:
		return ":" + strconv.Itoa(int(v.Value)), nil
	case *object.Texto:
		if !strings.Contains(v.Value, ":") {
			return ":" + v.Value, nil
		}
		return v.Value, nil
	}
	return "", erroBuiltin("%s(): a porta tem que ser numero ou texto \"host:porta\", veio %s", nome, o.Type())
}

// textoPraMandar: texto vai do jeito que ta; numero/booleano viram texto;
// o resto (lista, dicionario...) e erro com dica — socket so carrega bytes.
func textoPraMandar(v object.Object) (string, error) {
	switch x := v.(type) {
	case *object.Texto:
		return x.Value, nil
	case *object.Numero, *object.Booleano:
		return x.Inspect(), nil
	}
	return "", fmt.Errorf("conexao so manda texto, veio %s — usa pra_json(x) ou texto(x)", v.Type())
}

// traduzErroLeitura: EOF (o outro lado desligou) e conexao fechada por nos
// viram (nil, nil) = nada, igual cano fechado. Timeout ganha mensagem propria.
func traduzErroLeitura(err error, timeout time.Duration) (object.Object, error) {
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return nil, nil
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return nil, fmt.Errorf("ninguem falou nada em %s (timeout)", segundos(timeout))
	}
	return nil, err
}

func traduzErroEscrita(err error, timeout time.Duration) error {
	if errors.Is(err, net.ErrClosed) {
		return errors.New("conexao fechada, nao da pra mandar mais nada")
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("o outro lado nao deu conta de receber em %s (timeout)", segundos(timeout))
	}
	return err
}

// ---------------------------------------------------------------------------
// TCP

// conexaoTCP e um socket TCP com enquadramento por linha (padrao) ou bruto.
// muEnvia garante que dois envia concorrentes nao intercalam bytes; muRecebe
// protege o leitor bufferizado (e o pedaco de linha pendente).
type conexaoTCP struct {
	conn    net.Conn
	leitor  *bufio.Reader
	bruto   bool
	timeout time.Duration

	muEnvia  sync.Mutex
	muRecebe sync.Mutex
	pendente []byte // pedaco de linha lido antes de um timeout
	fechou   sync.Once
}

func novaConexaoTCP(conn net.Conn, op opcoesRede) *object.Nativo {
	c := &conexaoTCP{conn: conn, bruto: op.bruto, timeout: op.timeout}
	if !c.bruto {
		c.leitor = bufio.NewReader(conn)
	}
	return &object.Nativo{Rotulo: "conexao tcp com " + conn.RemoteAddr().String(), Valor: c}
}

func (c *conexaoTCP) Envia(v object.Object) error {
	s, err := textoPraMandar(v)
	if err != nil {
		return err
	}
	if !c.bruto {
		s += "\n"
	}
	c.muEnvia.Lock()
	defer c.muEnvia.Unlock()
	if c.timeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	}
	if _, err := io.WriteString(c.conn, s); err != nil {
		return traduzErroEscrita(err, c.timeout)
	}
	return nil
}

func (c *conexaoTCP) Recebe() (object.Object, error) {
	c.muRecebe.Lock()
	defer c.muRecebe.Unlock()
	if c.timeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	}
	if c.bruto {
		buf := make([]byte, tamanhoPedacoBruto)
		n, err := c.conn.Read(buf)
		if n > 0 {
			// o erro (se veio junto) aparece de novo no proximo Read
			return &object.Texto{Value: string(buf[:n])}, nil
		}
		return traduzErroLeitura(err, c.timeout)
	}
	for {
		pedaco, err := c.leitor.ReadSlice('\n')
		c.pendente = append(c.pendente, pedaco...)
		if err == nil {
			linha := c.pendente
			c.pendente = nil
			linha = linha[:len(linha)-1]
			if n := len(linha); n > 0 && linha[n-1] == '\r' {
				linha = linha[:n-1]
			}
			return &object.Texto{Value: string(linha)}, nil
		}
		if err == bufio.ErrBufferFull {
			if len(c.pendente) > limiteLinha {
				c.pendente = nil
				return nil, errors.New("linha passou de 1 MB sem \\n — isso ai nao e protocolo de linha, tenta {\"modo\": \"bruto\"}")
			}
			continue
		}
		// ultima linha sem \n antes do outro lado desligar: entrega ela; o
		// proximo recebe ja da nada.
		if errors.Is(err, io.EOF) && len(c.pendente) > 0 {
			linha := strings.TrimSuffix(string(c.pendente), "\r")
			c.pendente = nil
			return &object.Texto{Value: linha}, nil
		}
		return traduzErroLeitura(err, c.timeout)
	}
}

func (c *conexaoTCP) Fecha() error {
	c.fechou.Do(func() { c.conn.Close() })
	return nil
}

func (c *conexaoTCP) endereco() string { return c.conn.RemoteAddr().String() }

func (i *Interpreter) builtinConectaTcp(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("conecta_tcp() quer 1 ou 2 argumentos (\"host:porta\", [opcoes]), veio %d", len(args))
	}
	end, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("conecta_tcp() espera \"host:porta\" (texto), veio %s", args[0].Type())
	}
	var op opcoesRede
	if len(args) == 2 {
		var e *object.Erro
		if op, e = lerOpcoesRede("conecta_tcp", args[1], "modo", "timeout"); e != nil {
			return e
		}
	}
	conn, err := net.DialTimeout("tcp", end.Value, op.timeout)
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return erroBuiltinKind(KindRede, "conecta_tcp(): %s nao atendeu em %s (timeout)", end.Value, segundos(op.timeout))
		}
		return erroBuiltinKind(KindRede, "conecta_tcp(): nao rolou conectar em %s: %v", end.Value, err)
	}
	return novaConexaoTCP(conn, op)
}

// ---------------------------------------------------------------------------
// parada graciosa (servidores)

// esperaParada devolve um canal que fecha quando o servidor tem que parar:
// SIGINT/SIGTERM ou o cano `para` (alguem mandou valor ou fechou). O desliga
// tira o signal.Notify — o segundo Ctrl-C volta a matar o processo na hora.
func esperaParada(para *object.Cano) (<-chan struct{}, func()) {
	parou := make(chan struct{})
	cancela := make(chan struct{})
	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, os.Interrupt, syscall.SIGTERM)
	var paraCh chan object.Object
	if para != nil {
		paraCh = para.Ch
	}
	go func() {
		select {
		case <-sinais:
		case <-paraCh:
		case <-cancela:
		}
		close(parou)
	}()
	var once sync.Once
	return parou, func() {
		once.Do(func() {
			signal.Stop(sinais)
			close(cancela)
		})
	}
}

// avisaPronto manda o endereco real (porta 0 vira a porta sorteada) pro cano
// `pronto` sem travar o servidor se ninguem estiver ouvindo.
func avisaPronto(pronto *object.Cano, endereco string) {
	if pronto == nil {
		return
	}
	go func() {
		defer func() { recover() }() // cano fechado: azar de quem fechou
		pronto.Ch <- &object.Texto{Value: endereco}
	}()
}

// logaErroRede escreve no stderr (com o lock da saida) que um handler quebrou.
func (i *Interpreter) logaErroRede(formato string, args ...interface{}) {
	i.muOut.Lock()
	defer i.muOut.Unlock()
	w := i.erroOut
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, formato+"\n", args...)
}

// rodaHandler chama a gambiarra do usuario protegida contra panic e devolve o
// resultado (erro vira log, nao derruba o servidor).
func (i *Interpreter) rodaHandler(quem, onde string, handler object.Object, args []object.Object) (res object.Object) {
	defer func() {
		if r := recover(); r != nil {
			i.logaErroRede("%s: o handler de %s quebrou feio: %v", quem, onde, r)
			res = NADA
		}
	}()
	res = i.applyFunction(handler, args, 0, "<"+quem+">")
	if e, ok := res.(*object.Erro); ok && !e.Handled {
		i.logaErroRede("%s: o handler de %s quebrou: %s", quem, onde, e.Message)
		return NADA
	}
	return res
}

// esperaGrupo espera o WaitGroup por no maximo `max`.
func esperaGrupo(wg *sync.WaitGroup, max time.Duration) {
	pronto := make(chan struct{})
	go func() { wg.Wait(); close(pronto) }()
	select {
	case <-pronto:
	case <-time.After(max):
	}
}

// builtinEscutaTcp: escuta_tcp(porta_ou_endereco, handler, [opcoes]). Cada
// conexao aceita roda handler(conexao) na propria goroutine e e fechada
// quando o handler volta. Bloqueia ate SIGINT/SIGTERM ou o cano `para`;
// ai fecha a porta, fecha as conexoes abertas (o recebe delas da nada),
// espera os handlers e devolve nada.
func (i *Interpreter) builtinEscutaTcp(args []object.Object) object.Object {
	if len(args) < 2 || len(args) > 3 {
		return erroBuiltin("escuta_tcp() quer 2 ou 3 argumentos (porta, handler, [opcoes]), veio %d", len(args))
	}
	endereco, e := enderecoEscuta("escuta_tcp", args[0])
	if e != nil {
		return e
	}
	handler := args[1]
	if !ehChamavel(handler) {
		return erroBuiltin("escuta_tcp(): o handler tem que ser uma gambiarra(conexao), veio %s", handler.Type())
	}
	var op opcoesRede
	if len(args) == 3 {
		if op, e = lerOpcoesRede("escuta_tcp", args[2], "modo", "timeout", "pronto", "para"); e != nil {
			return e
		}
	}
	// cada conexao/datagrama roda o handler na propria goroutine: liga o modo
	// concorrente ANTES (ver object/concorrencia.go)
	object.AtivaConcorrencia()
	ln, err := net.Listen("tcp", endereco)
	if err != nil {
		return erroBuiltinKind(KindRede, "escuta_tcp(): nao consegui escutar em %s: %v", endereco, err)
	}
	parou, desliga := esperaParada(op.para)
	defer desliga()

	var (
		mu      sync.Mutex
		abertas = map[*conexaoTCP]bool{}
		wg      sync.WaitGroup
		parando bool
	)
	go func() {
		<-parou
		mu.Lock()
		parando = true
		ln.Close()
		for c := range abertas {
			c.Fecha()
		}
		mu.Unlock()
	}()
	avisaPronto(op.pronto, ln.Addr().String())

	for {
		conn, err := ln.Accept()
		if err != nil {
			mu.Lock()
			p := parando
			mu.Unlock()
			if p {
				break
			}
			i.logaErroRede("escuta_tcp: accept deu ruim: %v", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		nat := novaConexaoTCP(conn, op)
		c := nat.Valor.(*conexaoTCP)
		mu.Lock()
		if parando {
			mu.Unlock()
			c.Fecha()
			break
		}
		abertas[c] = true
		wg.Add(1)
		mu.Unlock()
		go func() {
			defer func() {
				c.Fecha()
				mu.Lock()
				delete(abertas, c)
				mu.Unlock()
				wg.Done()
			}()
			i.rodaHandler("escuta_tcp", c.endereco(), handler, []object.Object{nat})
		}()
	}
	esperaGrupo(&wg, esperaHandlers)
	return NADA
}

// builtinEndereco: endereco(conexao) -> "ip:porta" do outro lado.
func builtinEndereco(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("endereco() quer 1 argumento (conexao), veio %d", len(args))
	}
	if n, ok := args[0].(*object.Nativo); ok {
		switch c := n.Valor.(type) {
		case *conexaoTCP:
			return &object.Texto{Value: c.endereco()}
		case *conexaoUDP:
			return &object.Texto{Value: c.endereco()}
		}
	}
	return erroBuiltin("endereco() espera uma conexao tcp/udp, veio %s", args[0].Type())
}

// ---------------------------------------------------------------------------
// UDP

// conexaoUDP e um socket UDP "conectado" (conecta_udp): cada envia e um
// datagrama com exatamente o texto, cada recebe e o proximo datagrama.
type conexaoUDP struct {
	conn    net.Conn
	timeout time.Duration
	muEnvia sync.Mutex
	fechou  sync.Once
}

func (c *conexaoUDP) Envia(v object.Object) error {
	s, err := textoPraMandar(v)
	if err != nil {
		return err
	}
	c.muEnvia.Lock()
	defer c.muEnvia.Unlock()
	if c.timeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	}
	if _, err := io.WriteString(c.conn, s); err != nil {
		return traduzErroEscrita(err, c.timeout)
	}
	return nil
}

func (c *conexaoUDP) Recebe() (object.Object, error) {
	if c.timeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	}
	buf := make([]byte, tamanhoPedacoBruto)
	n, err := c.conn.Read(buf)
	if err != nil {
		return traduzErroLeitura(err, c.timeout)
	}
	return &object.Texto{Value: string(buf[:n])}, nil
}

func (c *conexaoUDP) Fecha() error {
	c.fechou.Do(func() { c.conn.Close() })
	return nil
}

func (c *conexaoUDP) endereco() string { return c.conn.RemoteAddr().String() }

func builtinConectaUdp(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("conecta_udp() quer 1 ou 2 argumentos (\"host:porta\", [opcoes]), veio %d", len(args))
	}
	end, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("conecta_udp() espera \"host:porta\" (texto), veio %s", args[0].Type())
	}
	var op opcoesRede
	if len(args) == 2 {
		var e *object.Erro
		if op, e = lerOpcoesRede("conecta_udp", args[1], "timeout"); e != nil {
			return e
		}
	}
	conn, err := net.Dial("udp", end.Value)
	if err != nil {
		return erroBuiltinKind(KindRede, "conecta_udp(): nao rolou mirar em %s: %v", end.Value, err)
	}
	c := &conexaoUDP{conn: conn, timeout: op.timeout}
	return &object.Nativo{Rotulo: "conexao udp com " + conn.RemoteAddr().String(), Valor: c}
}

// builtinEnviaUdp: envia_udp("host:porta", texto) manda UM datagrama e
// esquece (UDP nao tem confirmacao). Pra esperar resposta use conecta_udp.
func builtinEnviaUdp(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("envia_udp() quer 2 argumentos (\"host:porta\", texto), veio %d", len(args))
	}
	end, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("envia_udp() espera \"host:porta\" (texto), veio %s", args[0].Type())
	}
	s, err := textoPraMandar(args[1])
	if err != nil {
		return erroBuiltin("envia_udp(): %s", err)
	}
	conn, err := net.Dial("udp", end.Value)
	if err != nil {
		return erroBuiltinKind(KindRede, "envia_udp(): nao rolou mirar em %s: %v", end.Value, err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, s); err != nil {
		return erroBuiltinKind(KindRede, "envia_udp(): %v", err)
	}
	return NADA
}

// builtinEscutaUdp: escuta_udp(porta_ou_endereco, handler, [opcoes]). Cada
// datagrama roda handler(mensagem, remetente) na propria goroutine; se o
// handler devolver texto (ou numero), ele volta como resposta pro remetente.
// Para igual o escuta_tcp (SIGINT/SIGTERM ou cano `para`).
func (i *Interpreter) builtinEscutaUdp(args []object.Object) object.Object {
	if len(args) < 2 || len(args) > 3 {
		return erroBuiltin("escuta_udp() quer 2 ou 3 argumentos (porta, handler, [opcoes]), veio %d", len(args))
	}
	endereco, e := enderecoEscuta("escuta_udp", args[0])
	if e != nil {
		return e
	}
	handler := args[1]
	if !ehChamavel(handler) {
		return erroBuiltin("escuta_udp(): o handler tem que ser uma gambiarra(mensagem, remetente), veio %s", handler.Type())
	}
	var op opcoesRede
	if len(args) == 3 {
		if op, e = lerOpcoesRede("escuta_udp", args[2], "pronto", "para"); e != nil {
			return e
		}
	}
	// cada conexao/datagrama roda o handler na propria goroutine: liga o modo
	// concorrente ANTES (ver object/concorrencia.go)
	object.AtivaConcorrencia()
	pc, err := net.ListenPacket("udp", endereco)
	if err != nil {
		return erroBuiltinKind(KindRede, "escuta_udp(): nao consegui escutar em %s: %v", endereco, err)
	}
	parou, desliga := esperaParada(op.para)
	defer desliga()
	var (
		mu      sync.Mutex
		parando bool
		wg      sync.WaitGroup
	)
	go func() {
		<-parou
		mu.Lock()
		parando = true
		mu.Unlock()
		pc.Close()
	}()
	avisaPronto(op.pronto, pc.LocalAddr().String())

	buf := make([]byte, tamanhoPedacoBruto)
	for {
		n, de, err := pc.ReadFrom(buf)
		if err != nil {
			mu.Lock()
			p := parando
			mu.Unlock()
			if p || errors.Is(err, net.ErrClosed) {
				break
			}
			i.logaErroRede("escuta_udp: leitura deu ruim: %v", err)
			continue
		}
		msg := &object.Texto{Value: string(buf[:n])}
		remetente := de.String()
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := i.rodaHandler("escuta_udp", remetente, handler, []object.Object{msg, &object.Texto{Value: remetente}})
			if _, nada := res.(*object.Nada); nada || res == nil {
				return
			}
			resposta, err := textoPraMandar(res)
			if err != nil {
				i.logaErroRede("escuta_udp: a resposta pra %s nao rolou: %v", remetente, err)
				return
			}
			if _, err := pc.WriteTo([]byte(resposta), de); err != nil && !errors.Is(err, net.ErrClosed) {
				i.logaErroRede("escuta_udp: a resposta pra %s nao rolou: %v", remetente, err)
			}
		}()
	}
	esperaGrupo(&wg, esperaHandlers)
	return NADA
}
