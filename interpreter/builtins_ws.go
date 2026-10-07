//go:build !js

// WebSocket (servidor via rota_ws e cliente via conecta_ws) em cima do
// github.com/coder/websocket. Fica fora do build wasm: o Accept nao existe em
// GOOS=js e o Dial de la usa a API do navegador (sem cabecalhos). No
// navegador o stub em builtins_ws_js.go assume.

package interpreter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"gambiarrascript/object"
)

// Ajustes do WebSocket. Variaveis (nao const) pra teste poder encurtar.
var (
	wsIntervaloPing = 30 * time.Second
	wsPrazoPong     = 15 * time.Second
	wsPrazoEnvio    = 10 * time.Second
	wsLimiteLeitura = int64(1 << 20) // 1 MiB por mensagem
)

// wsConexao e a object.Conexao de um WebSocket: envia/recebe/fecha da
// linguagem caem aqui.
type wsConexao struct {
	conn    *websocket.Conn
	ctx     context.Context
	cancela context.CancelFunc

	// envio serializado: o broadcast chama envia(c, msg) na mesma conexao a
	// partir de varios handlers ao mesmo tempo.
	muEnvio sync.Mutex
	// leitura tambem: o coder/websocket nao aceita dois Read juntos.
	muLeitura sync.Mutex

	fechadaPorNos atomic.Bool // fecha(ws) chamado: envia depois disso e erro
	caiu          atomic.Bool // o outro lado sumiu: envia vira no-op
	lendo         atomic.Bool // tem recebe() bloqueado (o pong so chega lendo)
	fechaUmaVez   sync.Once
}

func novaWsConexao(conn *websocket.Conn) *wsConexao {
	ctx, cancela := context.WithCancel(context.Background())
	conn.SetReadLimit(wsLimiteLeitura)
	c := &wsConexao{conn: conn, ctx: ctx, cancela: cancela}
	go c.mantemViva()
	return c
}

// mantemViva manda ping de tempos em tempos enquanto alguem esta lendo. Se o
// pong nao volta no prazo, o outro lado morreu sem avisar (wifi caiu, NAT
// esqueceu): derruba a conexao e o recebe() devolve nada. Sem ninguem lendo
// nao da pra ver o pong, entao nem tenta.
func (c *wsConexao) mantemViva() {
	t := time.NewTicker(wsIntervaloPing)
	defer t.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-t.C:
		}
		if !c.lendo.Load() {
			continue
		}
		ctx, cancela := context.WithTimeout(c.ctx, wsPrazoPong)
		err := c.conn.Ping(ctx)
		cancela()
		if err != nil && c.lendo.Load() && c.ctx.Err() == nil {
			c.morreu()
			return
		}
	}
}

// morreu marca a conexao como caida e solta tudo que estava preso nela.
func (c *wsConexao) morreu() {
	c.caiu.Store(true)
	c.conn.CloseNow()
	c.cancela()
}

// serializaWs: texto vai como esta; lista/dicionario viram JSON (mesmo
// formato do pra_json); numero/booleano viram o texto deles.
func serializaWs(v object.Object) ([]byte, error) {
	switch val := v.(type) {
	case *object.Texto:
		return []byte(val.Value), nil
	case *object.Lista, *object.Dicionario:
		var buf bytes.Buffer
		if e := escreveJson(&buf, val); e != nil {
			return nil, errors.New(e.Message)
		}
		return buf.Bytes(), nil
	case *object.Numero, *object.Booleano:
		return []byte(val.Inspect()), nil
	}
	return nil, fmt.Errorf("websocket so manda texto, numero, booleano, lista ou dicionario — veio %s", v.Type())
}

func (c *wsConexao) Envia(v object.Object) error {
	if c.fechadaPorNos.Load() {
		return errors.New("essa conexao websocket ja foi fechada (fecha), nao da pra mandar mais nada")
	}
	dados, err := serializaWs(v)
	if err != nil {
		return err
	}
	if c.caiu.Load() {
		return nil // o outro lado ja foi embora: a mensagem se perde
	}
	c.muEnvio.Lock()
	defer c.muEnvio.Unlock()
	ctx, cancela := context.WithTimeout(c.ctx, wsPrazoEnvio)
	defer cancela()
	if err := c.conn.Write(ctx, websocket.MessageText, dados); err != nil {
		if c.fechadaPorNos.Load() {
			return errors.New("essa conexao websocket ja foi fechada (fecha), nao da pra mandar mais nada")
		}
		// cliente lento ou caido: derruba ele em vez de travar o broadcast
		c.morreu()
	}
	return nil
}

// Recebe bloqueia ate a proxima mensagem. Qualquer erro de leitura (fechou,
// caiu, servidor desligando) vira nada: no coder/websocket erro de leitura
// ja derruba a conexao, nao tem o que tentar de novo.
func (c *wsConexao) Recebe() (object.Object, error) {
	c.muLeitura.Lock()
	defer c.muLeitura.Unlock()
	if c.caiu.Load() || c.fechadaPorNos.Load() {
		return nil, nil
	}
	c.lendo.Store(true)
	_, dados, err := c.conn.Read(c.ctx)
	c.lendo.Store(false)
	if err != nil {
		c.caiu.Store(true)
		c.cancela()
		return nil, nil
	}
	return &object.Texto{Value: string(dados)}, nil
}

func (c *wsConexao) Fecha() error {
	c.fechaCom(websocket.StatusNormalClosure, "")
	return nil
}

func (c *wsConexao) fechaCom(status websocket.StatusCode, motivo string) {
	c.fechaUmaVez.Do(func() {
		c.fechadaPorNos.Store(true)
		if !c.caiu.Load() {
			c.conn.Close(status, motivo)
		}
		c.conn.CloseNow()
		c.cancela()
	})
}

// desligando: o servidor vai desligar, avisa o cliente com "going away".
func (c *wsConexao) desligando() {
	c.fechaCom(websocket.StatusGoingAway, "servidor desligando")
}

func (s *servidorEstado) fechaWebsockets() {
	s.muWS.Lock()
	ativos := make([]desligavel, 0, len(s.wsAtivos))
	for c := range s.wsAtivos {
		ativos = append(ativos, c)
	}
	s.muWS.Unlock()
	var wg sync.WaitGroup
	for _, c := range ativos {
		wg.Add(1)
		go func(c desligavel) {
			defer wg.Done()
			c.desligando()
		}(c)
	}
	wg.Wait()
}

// atendeWS: middlewares antes → handshake → handler(ws, pedido) na goroutine
// do proprio request (uma por conexao) → fecha quando o handler volta.
func (s *servidorEstado) atendeWS(w http.ResponseWriter, r *http.Request, rota *rotaHTTP, pedido *object.Dicionario, cors *configCors) {
	if resp := s.rodaAntes(w, r, rota, pedido); resp != nil {
		resp.escreve(w) // middleware barrou (401...): nem faz o handshake
		return
	}
	opcoes := &websocket.AcceptOptions{}
	// origem: a lista da propria rota_ws manda; sem ela, uma lista explicita
	// do cors() serve; o resto (inclusive cors() aberto) fica so na mesma
	// origem. Ver builtinRotaWs: liberar origem aqui libera o cookie do usuario.
	switch {
	case len(rota.wsOrigens) == 1 && rota.wsOrigens[0] == "*":
		opcoes.InsecureSkipVerify = true
	case len(rota.wsOrigens) > 0:
		opcoes.OriginPatterns = padroesDeOrigem(rota.wsOrigens)
	case cors != nil && !cors.todas:
		opcoes.OriginPatterns = cors.listaOrig
	}
	conn, err := websocket.Accept(w, r, opcoes)
	if err != nil {
		return // o Accept ja respondeu (403 de origem, 400 de handshake torto)
	}
	c := novaWsConexao(conn)
	s.muWS.Lock()
	s.wsAtivos[c] = struct{}{}
	s.muWS.Unlock()
	defer func() {
		s.muWS.Lock()
		delete(s.wsAtivos, c)
		s.muWS.Unlock()
	}()
	defer func() {
		if rec := recover(); rec != nil {
			s.logaErroHandler(r, rota.rotulo(), &object.Erro{Message: fmt.Sprintf("panico: %v", rec)})
			c.fechaCom(websocket.StatusInternalError, "deu ruim no servidor")
		}
	}()

	ws := &object.Nativo{Rotulo: "websocket", Valor: c}
	res := s.chamaAdaptado(rota.handler, []object.Object{ws, pedido}, rota.rotulo())
	if e, ok := res.(*object.Erro); ok {
		s.logaErroHandler(r, rota.rotulo(), e)
		c.fechaCom(websocket.StatusInternalError, "deu ruim no servidor")
		return
	}
	c.Fecha()
}

// builtinConectaWs: conecta_ws(url, [{"cabecalhos": {...}, "timeout": s}])
// abre um WebSocket cliente; usa com envia/recebe/fecha.
func builtinConectaWs(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("conecta_ws() quer 1 ou 2 argumentos (url, [opcoes]), veio %d", len(args))
	}
	url, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("conecta_ws() espera a url como texto (ws://...), veio %s", args[0].Type())
	}
	cab := map[string]string{}
	prazo := timeoutPadraoHTTP
	if len(args) == 2 {
		opcoes, ok := args[1].(*object.Dicionario)
		if !ok {
			return erroBuiltin("conecta_ws() espera um dicionario de opcoes, veio %s", args[1].Type())
		}
		var erro *object.Erro
		if cab, erro = opcaoCabecalhos(opcoes); erro != nil {
			return erro
		}
		if t, erro := opcaoTimeout(opcoes); erro != nil {
			return erro
		} else if t > 0 {
			prazo = t
		}
	}
	h := http.Header{}
	for k, v := range cab {
		h.Set(k, v)
	}
	ctx, cancela := context.WithTimeout(context.Background(), prazo)
	defer cancela()
	conn, resp, err := websocket.Dial(ctx, url.Value, &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		extra := ""
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
			extra = fmt.Sprintf(" (o servidor respondeu %d)", resp.StatusCode)
		}
		return erroBuiltinKind(KindRede, "conecta_ws(): nao consegui conectar em %q%s: %v", url.Value, extra, err)
	}
	return &object.Nativo{Rotulo: "websocket", Valor: novaWsConexao(conn)}
}

// padroesDeOrigem normaliza a lista do rota_ws pro OriginPatterns da lib:
// "https://app.com" compara esquema+host, "app.com" so o host.
func padroesDeOrigem(origens []string) []string {
	saida := make([]string, 0, len(origens))
	for _, o := range origens {
		saida = append(saida, strings.TrimSuffix(o, "/"))
	}
	return saida
}
