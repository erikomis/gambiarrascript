package interpreter

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gambiarrascript/object"
)

// Timeouts do servidor. ReadHeaderTimeout segura o slowloris (cliente que
// manda o cabecalho a conta-gotas); o corpo tem prazo proprio (prazoCorpo)
// porque um ReadTimeout global tambem mataria as conexoes WebSocket.
const (
	prazoCabecalho  = 10 * time.Second
	prazoOcioso     = 120 * time.Second
	prazoCorpo      = 60 * time.Second
	prazoDesligando = 5 * time.Second
)

// corpo generico do 500: o detalhe vai pro stderr, nunca pro cliente.
const corpoErroInterno = "deu ruim no servidor (erro interno), parca"

type servidorEstado struct {
	// object.Object (nao *object.Funcao): na VM o handler chega como
	// *object.CompiledFunction. Amarrar no tipo do tree-walker fazia `rota()`
	// morrer no engine PADRAO — ninguem conseguia subir servidor com `gs roda`.
	mu         sync.RWMutex
	exatas     map[string]*rotaHTTP
	padroes    []*rotaHTTP
	contaRotas int
	antes      []object.Object
	depois     []object.Object
	cors       *configCors
	pastas     []*pastaEstatica
	i          *Interpreter

	// conexoes WebSocket abertas: o desligamento manda "going away" pra elas
	// (o Shutdown do net/http nao enxerga conexao sequestrada).
	muWS     sync.Mutex
	wsAtivos map[desligavel]struct{}

	// desliga e fechado por PararServidor (testes): faz o escuta desligar
	// com calma, igual ao ctrl+c.
	desliga     chan struct{}
	desligaUmaV sync.Once
}

// desligavel e o que o desligamento sabe fechar (as conexoes WebSocket).
type desligavel interface{ desligando() }

func novoServidorEstado(i *Interpreter) *servidorEstado {
	return &servidorEstado{
		exatas:   map[string]*rotaHTTP{},
		i:        i,
		wsAtivos: map[desligavel]struct{}{},
		desliga:  make(chan struct{}),
	}
}

// ehChamavel diz se o valor da pra chamar como gambiarra: `*object.Funcao` no
// tree-walker, `*object.CompiledFunction` na VM, `*object.Builtin` pros nativos.
func ehChamavel(o object.Object) bool {
	switch o.(type) {
	case *object.Funcao, *object.CompiledFunction, *object.Builtin:
		return true
	}
	return false
}

// chamaAdaptado chama a gambiarra do usuario cortando os argumentos que ela
// nao declarou: `rota("GET", "/saude", gambiarra() funciona "ok" acabou_finalmente)`
// e `rota_ws("/x", gambiarra(ws) ...)` valem sem obrigar a declarar o pedido.
func (s *servidorEstado) chamaAdaptado(fn object.Object, args []object.Object, nome string) object.Object {
	switch f := fn.(type) {
	case *object.Funcao:
		variadico := false
		for _, p := range f.Parametros {
			variadico = variadico || p.Variadico
		}
		if !variadico && len(args) > len(f.Parametros) {
			args = args[:len(f.Parametros)]
		}
	case *object.CompiledFunction:
		if !f.Variadic && len(args) > f.NumArgs {
			args = args[:f.NumArgs]
		}
	}
	return s.i.applyFunction(fn, args, 0, nome)
}

func (s *servidorEstado) builtinRota(args []object.Object) object.Object {
	if len(args) != 3 {
		return erroBuiltin("rota() quer 3 argumentos (metodo, caminho, handler), veio %d", len(args))
	}
	metodo, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("rota(): o metodo tem que ser texto, veio %s", args[0].Type())
	}
	caminho, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("rota(): o caminho tem que ser texto, veio %s", args[1].Type())
	}
	handler := args[2]
	if !ehChamavel(handler) {
		return erroBuiltin("rota(): o handler tem que ser uma gambiarra, veio %s", handler.Type())
	}
	if err := s.registraRota(strings.ToUpper(metodo.Value), caminho.Value, handler, false, nil); err != nil {
		return erroBuiltin("rota(): %v", err)
	}
	return NADA
}

// builtinRotaWs registra um endpoint WebSocket: rota_ws(caminho, handler,
// [{"origens": [...]}]). O handler roda uma vez por conexao, na goroutine
// dela, e recebe (ws, pedido).
//
// Origem: por padrao so a mesma origem abre o socket. O navegador manda o
// cookie no handshake de qualquer site, entao liberar origem aqui e liberar
// o login do usuario pra esse site (cross-site WebSocket hijacking) — por
// isso o cors() aberto NAO vale pro websocket: tem que listar as origens na
// propria rota_ws, e "*" so com todas as letras.
func (s *servidorEstado) builtinRotaWs(args []object.Object) object.Object {
	if len(args) != 2 && len(args) != 3 {
		return erroBuiltin("rota_ws() quer 2 ou 3 argumentos (caminho, handler, [opcoes]), veio %d", len(args))
	}
	var origens []string
	if len(args) == 3 {
		opcoes, ok := args[2].(*object.Dicionario)
		if !ok {
			return erroBuiltin("rota_ws(): as opcoes tem que ser dicionario, tipo {\"origens\": [\"https://app.com\"]}, veio %s", args[2].Type())
		}
		var erro *object.Erro
		opcoes.Itera(func(par object.ParDic) {
			if erro != nil {
				return
			}
			chave, ok := par.Chave.(*object.Texto)
			if !ok || chave.Value != "origens" {
				erro = erroBuiltin("rota_ws(): opcao desconhecida %s (a que existe: origens)", par.Chave.Inspect())
				return
			}
			origens, erro = listaDeTextos("origens", par.Valor)
		})
		if erro != nil {
			return erro
		}
	}
	caminho, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("rota_ws(): o caminho tem que ser texto, veio %s", args[0].Type())
	}
	if !ehChamavel(args[1]) {
		return erroBuiltin("rota_ws(): o handler tem que ser uma gambiarra, veio %s", args[1].Type())
	}
	if err := s.registraRota("GET", caminho.Value, args[1], true, origens); err != nil {
		return erroBuiltin("rota_ws(): %v", err)
	}
	return NADA
}

// builtinAntes registra um middleware: roda antes de toda rota, na ordem de
// registro. Devolveu nada → segue; devolveu qualquer outra coisa → essa e a
// resposta e a rota nem roda.
func (s *servidorEstado) builtinAntes(args []object.Object) object.Object {
	if len(args) != 1 || !ehChamavel(args[0]) {
		return erroBuiltin("antes() quer 1 gambiarra (pedido), tipo antes(gambiarra(pedido) ... acabou_finalmente)")
	}
	s.mu.Lock()
	s.antes = append(s.antes, args[0])
	s.mu.Unlock()
	return NADA
}

// builtinDepois registra um middleware de saida: recebe (pedido, resposta)
// com a resposta ja normalizada {status, corpo, cabecalhos}. Devolveu nada →
// vale a resposta (inclusive se ele mexeu nela); devolveu outra → troca.
func (s *servidorEstado) builtinDepois(args []object.Object) object.Object {
	if len(args) != 1 || !ehChamavel(args[0]) {
		return erroBuiltin("depois() quer 1 gambiarra (pedido, resposta), tipo depois(gambiarra(pedido, resposta) ... acabou_finalmente)")
	}
	s.mu.Lock()
	s.depois = append(s.depois, args[0])
	s.mu.Unlock()
	return NADA
}

// endereçoDeEscuta traduz o argumento do escuta: numero = porta; texto so com
// digitos = porta; outro texto = endereco completo (":8080", "127.0.0.1:0").
func enderecoDeEscuta(arg object.Object) (string, *object.Erro) {
	switch v := arg.(type) {
	case *object.Numero:
		p := int(v.Value)
		if float64(p) != v.Value || p < 0 || p > 65535 {
			return "", erroBuiltin("nao consegui escutar na porta %s: porta vai de 0 a 65535, parca", v.Inspect())
		}
		return ":" + strconv.Itoa(p), nil
	case *object.Texto:
		t := strings.TrimSpace(v.Value)
		if _, err := strconv.Atoi(t); err == nil {
			return ":" + t, nil
		}
		if t == "" {
			return "", erroBuiltin("escuta(): endereco vazio — passa uma porta (8080) ou um endereco (\":8080\", \"127.0.0.1:0\")")
		}
		return t, nil
	}
	return "", erroBuiltin("escuta(): a porta tem que ser numero ou texto (\":8080\"), veio %s", arg.Type())
}

// opcoesDeEscuta le o segundo argumento do escuta: {"tls": {"cert", "chave"}}.
// Devolve o tls.Config (nil = HTTP puro).
func opcoesDeEscuta(o object.Object) (*tls.Config, *object.Erro) {
	d, ok := o.(*object.Dicionario)
	if !ok {
		return nil, erroBuiltin("escuta(): as opcoes tem que ser dicionario, tipo {\"tls\": {\"cert\": \"cert.pem\", \"chave\": \"chave.pem\"}}, veio %s", o.Type())
	}
	var cfg *tls.Config
	for _, par := range d.Pares() {
		k, ok := par.Chave.(*object.Texto)
		if !ok || k.Value != "tls" {
			return nil, erroBuiltin("escuta(): opcao %s nao existe (a que existe: tls)", chaveComAspas(par.Chave))
		}
		var e *object.Erro
		if cfg, e = configTLSServidor("escuta", par.Valor); e != nil {
			return nil, e
		}
	}
	return cfg, nil
}

func (s *servidorEstado) builtinEscuta(args []object.Object) object.Object {
	if len(args) != 1 && len(args) != 2 {
		return erroBuiltin("escuta() quer 1 ou 2 argumentos (porta, [opcoes]), veio %d", len(args))
	}
	endereco, erro := enderecoDeEscuta(args[0])
	if erro != nil {
		return erro
	}
	// o certificado e lido ANTES de abrir a porta: arquivo faltando e erro
	// na hora, nao um servidor de pe que derruba todo handshake
	var cfgTLS *tls.Config
	if len(args) == 2 {
		if cfgTLS, erro = opcoesDeEscuta(args[1]); erro != nil {
			return erro
		}
	}
	ln, err := net.Listen("tcp", endereco)
	if err != nil {
		return erroBuiltinKind(KindRede, "nao consegui escutar em %q: %v", endereco, err)
	}
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: prazoCabecalho,
		IdleTimeout:       prazoOcioso,
		TLSConfig:         cfgTLS,
		ErrorLog:          log.New(logDoServidor{s.i}, "", 0),
	}
	srv.RegisterOnShutdown(s.fechaWebsockets)

	esquema := "http"
	if cfgTLS != nil {
		esquema = "https"
	}
	s.i.logaErro("servidor de pe em %s://%s (ctrl+c pra parar)\n", esquema, enderecoLegivel(ln.Addr()))

	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sinais)

	fim := make(chan error, 1)
	go func() {
		if cfgTLS != nil {
			// cert ja esta no TLSConfig; o ServeTLS liga o HTTP/2 tambem
			fim <- srv.ServeTLS(ln, "", "")
			return
		}
		fim <- srv.Serve(ln)
	}()

	select {
	case err := <-fim:
		if err != nil && err != http.ErrServerClosed {
			return erroBuiltinKind(KindRede, "o servidor caiu: %v", err)
		}
		return NADA
	case <-sinais:
	case <-s.desliga:
	}
	// segundo ctrl+c volta a matar na hora
	signal.Stop(sinais)
	s.i.logaErro("desligando com calma (ate %ds pros pedidos em andamento)...\n", int(prazoDesligando/time.Second))
	ctx, cancela := context.WithTimeout(context.Background(), prazoDesligando)
	defer cancela()
	if err := srv.Shutdown(ctx); err != nil {
		srv.Close()
		s.i.logaErro("nao deu tempo de todo mundo terminar; derrubei o resto\n")
	}
	s.i.logaErro("servidor desligado, falou!\n")
	return NADA
}

// logDoServidor e o ErrorLog do net/http: manda pro stderr do interpretador
// (com o lock da saida), menos o "TLS handshake error" — cliente falando
// http:// na porta https ou que nao confia no certificado ja ve o erro do
// lado dele; no servidor isso so polui o log.
type logDoServidor struct{ i *Interpreter }

func (l logDoServidor) Write(p []byte) (int, error) {
	if !strings.Contains(string(p), "TLS handshake error") {
		l.i.logaErro("%s", p)
	}
	return len(p), nil
}

// enderecoLegivel troca o "[::]:8080" do Listen por "localhost:8080".
func enderecoLegivel(a net.Addr) string {
	if tcp, ok := a.(*net.TCPAddr); ok && (tcp.IP == nil || tcp.IP.IsUnspecified()) {
		return "localhost:" + strconv.Itoa(tcp.Port)
	}
	return a.String()
}

// PararServidor desliga com calma todo escuta() rodando nesse interpretador
// (o mesmo caminho do ctrl+c). Usado em teste.
func (i *Interpreter) PararServidor() {
	i.servidor.desligaUmaV.Do(func() { close(i.servidor.desliga) })
}

// logaErro escreve no stderr do interpretador sem embaralhar com outras
// goroutines (mesmo lock do mostra/escreve_erro).
func (i *Interpreter) logaErro(formato string, args ...interface{}) {
	i.muOut.Lock()
	defer i.muOut.Unlock()
	w := i.erroOut
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, formato, args...)
}

// logaErroHandler registra no stderr o erro que estourou num handler (com
// linha e traço), ja que o cliente so ve o 500 generico.
func (s *servidorEstado) logaErroHandler(r *http.Request, onde string, e *object.Erro) {
	var b strings.Builder
	fmt.Fprintf(&b, "servidor: %s %s estourou em %s\n", r.Method, r.URL.Path, onde)
	b.WriteString("  " + e.Message)
	if e.Line > 0 && !strings.Contains(e.Message, "linha") {
		fmt.Fprintf(&b, " (linha %d)", e.Line)
	}
	b.WriteString("\n")
	// o frame do proprio handler entra com linha 0 (chamado do Go, sem
	// call-site no fonte): so polui o traço
	limpo := &object.Erro{}
	for _, f := range e.Stack {
		if f.Line > 0 {
			limpo.Stack = append(limpo.Stack, f)
		}
	}
	if tr := limpo.Traco(); tr != "" {
		b.WriteString("Traço de pilha:\n" + tr)
	}
	s.i.logaErro("%s", b.String())
}

// ServidorHandler expoe o http.Handler do servidor (usado em teste).
func (i *Interpreter) ServidorHandler() http.Handler {
	return i.servidor.Handler()
}

func (s *servidorEstado) Handler() http.Handler {
	// cada requisicao (e cada websocket) roda o handler numa goroutine do
	// net/http: liga o modo concorrente AGORA, antes da primeira (ver
	// object/concorrencia.go)
	object.AtivaConcorrencia()
	return http.HandlerFunc(s.serve)
}

func (s *servidorEstado) serve(w http.ResponseWriter, r *http.Request) {
	// rede de seguranca: panico em qualquer canto vira 500 e o servidor segue.
	defer func() {
		if rec := recover(); rec != nil {
			s.logaErroHandler(r, "panico", &object.Erro{Message: fmt.Sprintf("panico: %v", rec)})
			escreveTexto(w, http.StatusInternalServerError, corpoErroInterno)
		}
	}()

	s.mu.RLock()
	cors := s.cors
	s.mu.RUnlock()
	if cors != nil && cors.aplica(w, r) {
		return // preflight respondido
	}

	rota, params, permitidos := s.acha(r.Method, r.URL.Path, r.URL.EscapedPath())
	if rota == nil {
		if len(permitidos) > 0 {
			w.Header().Set("Allow", strings.Join(permitidos, ", "))
			escreveTexto(w, http.StatusMethodNotAllowed, "metodo "+r.Method+" nao rola nesse caminho, parca (tenta "+strings.Join(permitidos, ", ")+")")
			return
		}
		if pasta := s.achaPasta(r.URL.Path); pasta != nil {
			pasta.serve(w, r)
			return
		}
		escreveTexto(w, http.StatusNotFound, "rota nao encontrada, parca")
		return
	}

	if rota.ws && !pedeUpgradeWS(r) {
		w.Header().Set("Upgrade", "websocket")
		escreveTexto(w, http.StatusUpgradeRequired, "aqui so entra websocket, parca")
		return
	}

	pedido, status, msg := s.montaPedido(w, r, params, !rota.ws)
	if status != 0 {
		escreveTexto(w, status, msg)
		return
	}

	if rota.ws {
		s.atendeWS(w, r, rota, pedido, cors)
		return
	}
	s.atendeRequisicao(w, r, rota, pedido)
}

func pedeUpgradeWS(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func escreveTexto(w http.ResponseWriter, status int, corpo string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	io.WriteString(w, corpo)
}

// rodaAntes roda os middlewares de entrada. Devolve a resposta que cortou o
// caminho (ou nil pra seguir pra rota).
func (s *servidorEstado) rodaAntes(w http.ResponseWriter, r *http.Request, rota *rotaHTTP, pedido *object.Dicionario) *respostaHTTP {
	s.mu.RLock()
	antes := s.antes
	s.mu.RUnlock()
	for _, mw := range antes {
		res := s.chamaAdaptado(mw, []object.Object{pedido}, "<antes>")
		if _, ehNada := res.(*object.Nada); ehNada {
			continue
		}
		return s.normaliza(r, "antes() de "+rota.rotulo(), res)
	}
	return nil
}

// atendeRequisicao roda antes → handler → depois e escreve a resposta. O
// net/http ja entrega cada request numa goroutine propria, entao as
// requisicoes rodam em paralelo de verdade.
func (s *servidorEstado) atendeRequisicao(w http.ResponseWriter, r *http.Request, rota *rotaHTTP, pedido *object.Dicionario) {
	resp := s.rodaAntes(w, r, rota, pedido)
	if resp == nil {
		res := s.chamaAdaptado(rota.handler, []object.Object{pedido}, rota.rotulo())
		resp = s.normaliza(r, rota.rotulo(), res)
	}

	s.mu.RLock()
	depois := s.depois
	s.mu.RUnlock()
	for _, mw := range depois {
		dic := resp.dicionario()
		res := s.chamaAdaptado(mw, []object.Object{pedido, dic}, "<depois>")
		if _, ehNada := res.(*object.Nada); ehNada {
			// pode ter mexido na resposta no lugar (bota resposta["status"] = ...)
			res = dic
		}
		resp = s.normaliza(r, "depois() de "+rota.rotulo(), res)
	}
	resp.escreve(w)
}

// montaPedido monta o dicionario `pedido`. Status != 0 = responde direto com
// esse status/mensagem (corpo ilegivel, json quebrado).
func (s *servidorEstado) montaPedido(w http.ResponseWriter, r *http.Request, params [][2]string, leCorpo bool) (*object.Dicionario, int, string) {
	var corpo []byte
	if leCorpo {
		// prazo so pra ler o corpo (ResponseController: Go 1.20+). Em writer
		// que nao suporta (httptest.Recorder) o erro e ignorado.
		rc := http.NewResponseController(w)
		_ = rc.SetReadDeadline(time.Now().Add(prazoCorpo))
		var err error
		corpo, err = io.ReadAll(r.Body)
		_ = rc.SetReadDeadline(time.Time{})
		if err != nil {
			return nil, http.StatusBadRequest, "nao consegui ler o corpo do pedido, parca"
		}
	}

	var jsonCorpo object.Object = NADA
	if ehTipoJSON(r.Header.Get("Content-Type")) && len(bytes.TrimSpace(corpo)) > 0 {
		v, err := parseJson(string(corpo))
		if err != nil {
			return nil, http.StatusBadRequest, "esse json do corpo ta quebrado, parca: " + err.Error()
		}
		jsonCorpo = v
	}

	dic := object.NovoDicionario()
	dicBota(dic, "metodo", &object.Texto{Value: r.Method})
	dicBota(dic, "caminho", &object.Texto{Value: r.URL.Path})
	dicBota(dic, "corpo", &object.Texto{Value: string(corpo)})
	dicBota(dic, "cabecalhos", dicDeMultimap(r.Header))
	dicBota(dic, "query", dicDeMultimap(r.URL.Query()))
	ps := object.NovoDicionario()
	for _, p := range params {
		dicBota(ps, p[0], &object.Texto{Value: p[1]})
	}
	dicBota(dic, "params", ps)
	dicBota(dic, "json", jsonCorpo)
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ip = host
	}
	dicBota(dic, "ip", &object.Texto{Value: ip})
	cookies := object.NovoDicionario()
	for _, c := range r.Cookies() {
		if _, ja := dicPega(cookies, c.Name); !ja {
			dicBota(cookies, c.Name, &object.Texto{Value: c.Value})
		}
	}
	dicBota(dic, "cookies", cookies)
	return dic, 0, ""
}

// ehTipoJSON: application/json e os +json (application/problem+json...).
func ehTipoJSON(ct string) bool {
	if ct == "" {
		return false
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return mt == "application/json" || strings.HasSuffix(mt, "+json")
}

// dicDeMultimap converte um map[string][]string (header/query) num dicionario
// texto->texto, juntando multiplos valores com ", ".
func dicDeMultimap(m map[string][]string) *object.Dicionario {
	dic := object.NovoDicionario()
	for _, nome := range chavesOrdenadas(m) {
		dicBota(dic, nome, &object.Texto{Value: strings.Join(m[nome], ", ")})
	}
	return dic
}

// dicBota / dicPega: acesso por chave-texto num dicionario da linguagem.
func dicBota(d *object.Dicionario, chave string, valor object.Object) {
	k := &object.Texto{Value: chave}
	d.Bota(k.ChaveHash(), object.ParDic{Chave: k, Valor: valor})
}

func dicPega(d *object.Dicionario, chave string) (object.Object, bool) {
	par, ok := d.Pega((&object.Texto{Value: chave}).ChaveHash())
	if !ok {
		return nil, false
	}
	return par.Valor, true
}

// ---------------------------------------------------------------------------
// Resposta

// respostaHTTP e a resposta ja decidida, independente do que o handler
// devolveu (texto, nada, lista, dicionario de resposta...).
type respostaHTTP struct {
	status     int
	cabecalhos http.Header
	corpo      []byte
}

const tipoJSON = "application/json; charset=utf-8"

func respostaErroInterno() *respostaHTTP {
	h := http.Header{}
	h.Set("Content-Type", "text/plain; charset=utf-8")
	return &respostaHTTP{status: http.StatusInternalServerError, cabecalhos: h, corpo: []byte(corpoErroInterno)}
}

// respostaJSON serializa o valor com o mesmo escritor do pra_json.
func respostaJSON(v object.Object, status int) (*respostaHTTP, *object.Erro) {
	var buf bytes.Buffer
	if e := escreveJson(&buf, v); e != nil {
		return nil, e
	}
	h := http.Header{}
	h.Set("Content-Type", tipoJSON)
	return &respostaHTTP{status: status, cabecalhos: h, corpo: buf.Bytes()}, nil
}

// normaliza transforma o que o handler devolveu numa respostaHTTP:
//
//	texto              → 200 com o texto
//	nada               → 200 vazio
//	lista              → 200 com JSON
//	dic de resposta    → {"status", "corpo", "cabecalhos"} (so essas chaves)
//	outro dicionario   → 200 com JSON
//	erro               → 500 generico (detalhe no stderr)
func (s *servidorEstado) normaliza(r *http.Request, onde string, res object.Object) *respostaHTTP {
	switch v := res.(type) {
	case *object.Texto:
		return &respostaHTTP{status: http.StatusOK, cabecalhos: http.Header{}, corpo: []byte(v.Value)}
	case *object.Nada:
		return &respostaHTTP{status: http.StatusOK, cabecalhos: http.Header{}}
	case *object.Erro:
		s.logaErroHandler(r, onde, v)
		return respostaErroInterno()
	case *object.Lista:
		resp, e := respostaJSON(v, http.StatusOK)
		if e != nil {
			s.logaErroHandler(r, onde, e)
			return respostaErroInterno()
		}
		return resp
	case *object.Dicionario:
		if !ehDicResposta(v) {
			resp, e := respostaJSON(v, http.StatusOK)
			if e != nil {
				s.logaErroHandler(r, onde, e)
				return respostaErroInterno()
			}
			return resp
		}
		resp, e := respostaDoDic(v)
		if e != nil {
			s.logaErroHandler(r, onde, e)
			return respostaErroInterno()
		}
		return resp
	}
	s.logaErroHandler(r, onde, erroBuiltin("o handler devolveu algo que nao da pra responder: %s (devolve texto, nada, lista ou dicionario)", res.Type()))
	return respostaErroInterno()
}

// ehDicResposta: dicionario so com as chaves status/corpo/cabecalhos (status
// numero, cabecalhos dicionario) e o formato de resposta; qualquer outro
// dicionario e dado e vira JSON. `{"status": "ok"}` e dado (status texto).
func ehDicResposta(d *object.Dicionario) bool {
	pares := d.Pares()
	if len(pares) == 0 {
		return false
	}
	for _, par := range pares {
		k, ok := par.Chave.(*object.Texto)
		if !ok {
			return false
		}
		switch k.Value {
		case "status":
			if _, ok := par.Valor.(*object.Numero); !ok {
				return false
			}
		case "cabecalhos":
			if _, ok := par.Valor.(*object.Dicionario); !ok {
				return false
			}
		case "corpo":
		default:
			return false
		}
	}
	return true
}

func respostaDoDic(d *object.Dicionario) (*respostaHTTP, *object.Erro) {
	resp := &respostaHTTP{status: http.StatusOK, cabecalhos: http.Header{}}
	if v, ok := dicPega(d, "cabecalhos"); ok {
		if cab, ok := v.(*object.Dicionario); ok {
			cab.Itera(func(p object.ParDic) {
				nome, ok := p.Chave.(*object.Texto)
				if !ok {
					return
				}
				switch val := p.Valor.(type) {
				case *object.Texto:
					resp.cabecalhos.Set(nome.Value, val.Value)
				case *object.Numero, *object.Booleano:
					resp.cabecalhos.Set(nome.Value, val.Inspect())
				case *object.Lista:
					// varios valores = varias linhas (Set-Cookie de 2 cookies)
					resp.cabecalhos.Del(nome.Value)
					for _, e := range val.Copia() {
						if t, ok := e.(*object.Texto); ok {
							resp.cabecalhos.Add(nome.Value, t.Value)
						}
					}
				}
			})
		}
	}
	if v, ok := dicPega(d, "status"); ok {
		if n, ok := v.(*object.Numero); ok {
			st := int(n.Value)
			if st < 100 || st > 599 {
				st = http.StatusInternalServerError
			}
			resp.status = st
		}
	}
	if v, ok := dicPega(d, "corpo"); ok {
		switch c := v.(type) {
		case *object.Texto:
			resp.corpo = []byte(c.Value)
		case *object.Nada:
		case *object.Lista, *object.Dicionario:
			var buf bytes.Buffer
			if e := escreveJson(&buf, c); e != nil {
				return nil, e
			}
			resp.corpo = buf.Bytes()
			if resp.cabecalhos.Get("Content-Type") == "" {
				resp.cabecalhos.Set("Content-Type", tipoJSON)
			}
		default:
			resp.corpo = []byte(c.Inspect())
		}
	}
	return resp, nil
}

// dicionario e a cara da resposta que o depois() enxerga.
func (resp *respostaHTTP) dicionario() *object.Dicionario {
	d := object.NovoDicionario()
	dicBota(d, "status", object.NumInt(int64(resp.status)))
	dicBota(d, "corpo", &object.Texto{Value: string(resp.corpo)})
	cab := object.NovoDicionario()
	nomes := make([]string, 0, len(resp.cabecalhos))
	for n := range resp.cabecalhos {
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)
	for _, n := range nomes {
		vs := resp.cabecalhos[n]
		if len(vs) == 1 {
			dicBota(cab, n, &object.Texto{Value: vs[0]})
			continue
		}
		l := object.NovaLista(make([]object.Object, 0, len(vs)))
		for _, v := range vs {
			l.Adiciona(&object.Texto{Value: v})
		}
		dicBota(cab, n, l)
	}
	dicBota(d, "cabecalhos", cab)
	return d
}

func (resp *respostaHTTP) escreve(w http.ResponseWriter) {
	for nome, vs := range resp.cabecalhos {
		for _, v := range vs {
			w.Header().Add(nome, v)
		}
	}
	w.WriteHeader(resp.status)
	w.Write(resp.corpo)
}

// builtinRespondeJson: responde_json(valor, [status]) → o dicionario de
// resposta pronto, com Content-Type de JSON e o corpo do pra_json.
func builtinRespondeJson(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("responde_json() quer 1 ou 2 argumentos (valor, [status]), veio %d", len(args))
	}
	status := int64(200)
	if len(args) == 2 {
		n, ok := args[1].(*object.Numero)
		if !ok || float64(int(n.Value)) != n.Value || n.Value < 100 || n.Value > 599 {
			return erroBuiltin("responde_json(): o status tem que ser numero inteiro de 100 a 599, veio %s", args[1].Inspect())
		}
		status = int64(n.Value)
	}
	var buf bytes.Buffer
	if e := escreveJson(&buf, args[0]); e != nil {
		return e
	}
	d := object.NovoDicionario()
	dicBota(d, "status", object.NumInt(status))
	dicBota(d, "corpo", &object.Texto{Value: buf.String()})
	cab := object.NovoDicionario()
	dicBota(cab, "Content-Type", &object.Texto{Value: tipoJSON})
	dicBota(d, "cabecalhos", cab)
	return d
}
