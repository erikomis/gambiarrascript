package vm

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// chat de broadcast igual ao examples/chat_ws.gs (sem a pagina).
const fonteChat = `bota conexoes = []
bota sala = trava()
gambiarra espalha(msg)
    pra_cada c em conexoes[0:]
        envia(c, msg)
    acabou_finalmente
acabou_finalmente
antes(gambiarra(pedido)
    se_colar pedido.query["nome"] == "intruso"
        funciona {"status": 401, "corpo": "fora"}
    acabou_finalmente
acabou_finalmente)
rota_ws("/chat/:sala", gambiarra(ws, pedido)
    bota nome = pedido.query["nome"] ?? "anonimo"
    bota na_sala = com_trava(sala, gambiarra()
        adiciona(conexoes, ws)
        funciona tamanho(conexoes)
    acabou_finalmente)
    espalha({"tipo": "entrou", "nome": nome, "sala": pedido.params.sala})
    enquanto deu_bom
        bota m = recebe(ws)
        se_colar m == nada
            vaza
        acabou_finalmente
        espalha({"tipo": "msg", "nome": nome, "texto": m})
    acabou_finalmente
    com_trava(sala, gambiarra() remove(conexoes, ws) acabou_finalmente)
    espalha({"tipo": "saiu", "nome": nome})
acabou_finalmente)
rota_ws("/eco", gambiarra(ws)
    enquanto deu_bom
        bota m = recebe(ws)
        se_colar m == nada
            vaza
        acabou_finalmente
        envia(ws, m)
        envia(ws, [1, {"a": deu_bom}])
        envia(ws, 2.5)
    acabou_finalmente
acabou_finalmente)
rota_ws("/quebra", gambiarra(ws)
    envia(ws, "vou quebrar")
    funciona 1 / 0
acabou_finalmente)`

func wsURL(base, caminho string) string {
	return "ws" + strings.TrimPrefix(base, "http") + caminho
}

// Cliente gs (conecta_ws) contra o servidor gs, nos dois engines dos dois
// lados: saida exata.
func TestWebSocketBroadcastClienteGs(t *testing.T) {
	cliente := `bota a = conecta_ws(URL + "?nome=ana")
mostra "a <- " + recebe(a)
bota b = conecta_ws(URL + "?nome=beto")
mostra "a <- " + recebe(a)
mostra "b <- " + recebe(b)
envia(a, "salve, beto")
mostra "a <- " + recebe(a)
mostra "b <- " + recebe(b)
fecha(b)
mostra "a <- " + recebe(a)
fecha(a)
fecha(a)
mostra "depois de fechar: " + texto(recebe(a))
arruma
    envia(a, "x")
quebrou err
    mostra erro_msg(err)
acabou_finalmente
mostra tipo(a)`
	esperado := `a <- {"tipo":"entrou","nome":"ana","sala":"geral"}
a <- {"tipo":"entrou","nome":"beto","sala":"geral"}
b <- {"tipo":"entrou","nome":"beto","sala":"geral"}
a <- {"tipo":"msg","nome":"ana","texto":"salve, beto"}
b <- {"tipo":"msg","nome":"ana","texto":"salve, beto"}
a <- {"tipo":"saiu","nome":"beto"}
depois de fechar: nada
deu ruim: envia(): essa conexao websocket ja foi fechada (fecha), nao da pra mandar mais nada
nativo
`
	servidorNosDois(t, fonteChat, func(t *testing.T, c ctxServidor) {
		// cada engine do cliente numa sala propria: o "saiu" da ana da rodada
		// anterior e transmitido de forma assincrona e, em maquina lenta (CI),
		// chegava na ana nova da rodada seguinte se a sala fosse a mesma
		clientes := []struct {
			nome string
			roda func(*testing.T, string) (string, string, string)
		}{{"tree", rodaTWComp}, {"vm", rodaVMComp}}
		for _, cli := range clientes {
			sala := "geral" + cli.nome
			src := "bota URL = \"" + wsURL(c.base, "/chat/"+sala) + "\"\n" + cliente
			esp := strings.ReplaceAll(esperado, `"sala":"geral"`, `"sala":"`+sala+`"`)
			_, saida, errStr := cli.roda(t, src)
			if saida != esp || errStr != "" {
				t.Errorf("[cliente %s] saida errada\n  veio:     %q\n  esperado: %q\n  erro: %s", cli.nome, saida, esp, errStr)
			}
		}
	})
}

func TestWebSocketEcoEJsonClienteGo(t *testing.T) {
	servidorNosDois(t, fonteChat, func(t *testing.T, c ctxServidor) {
		ctx, cancela := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancela()
		conn, _, err := websocket.Dial(ctx, wsURL(c.base, "/eco"), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.CloseNow()
		if err := conn.Write(ctx, websocket.MessageText, []byte("oi")); err != nil {
			t.Fatal(err)
		}
		var veio []string
		for range 3 {
			_, b, err := conn.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			veio = append(veio, string(b))
		}
		if got := strings.Join(veio, " | "); got != `oi | [1,{"a":true}] | 2.5` {
			t.Errorf("eco: %q", got)
		}
		conn.Close(websocket.StatusNormalClosure, "")

		// middleware barra o handshake
		_, resp, err := websocket.Dial(ctx, wsURL(c.base, "/chat/x?nome=intruso"), nil)
		if err == nil || resp == nil || resp.StatusCode != 401 {
			t.Errorf("antes() devia barrar o websocket com 401, veio %v %v", resp, err)
		}

		// erro no handler: fecha com 1011 e loga
		q, _, err := websocket.Dial(ctx, wsURL(c.base, "/quebra"), nil)
		if err != nil {
			t.Fatal(err)
		}
		_, b, _ := q.Read(ctx)
		_, _, err = q.Read(ctx)
		if string(b) != "vou quebrar" || websocket.CloseStatus(err) != websocket.StatusInternalError {
			t.Errorf("quebra: %q %v", b, err)
		}
		time.Sleep(50 * time.Millisecond)
		if !strings.Contains(c.err.String(), "estourou em <rota_ws /quebra>") {
			t.Errorf("erro do handler ws nao foi pro stderr: %q", c.err.String())
		}

		// GET normal numa rota_ws
		r := pede(t, "GET", c.base+"/eco", "")
		r.confere(t, "sem upgrade", 426, "aqui so entra websocket, parca")
	})
}

// Muitos clientes mandando ao mesmo tempo: o envia na mesma conexao vem de
// varias goroutines (um handler por cliente) — sem a trava de escrita o
// coder/websocket embaralharia os frames.
func TestWebSocketBroadcastConcorrente(t *testing.T) {
	const clientes, porCliente = 8, 10
	servidorNosDois(t, fonteChat, func(t *testing.T, c ctxServidor) {
		ctx, cancela := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancela()
		conns := make([]*websocket.Conn, clientes)
		for i := range conns {
			conn, _, err := websocket.Dial(ctx, wsURL(c.base, fmt.Sprintf("/chat/s?nome=c%d", i)), nil)
			if err != nil {
				t.Fatal(err)
			}
			conn.SetReadLimit(1 << 20)
			conns[i] = conn
			// espera os "entrou" ate o dele (garante que ta na lista)
			for {
				_, b, err := conn.Read(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(b), fmt.Sprintf(`"nome":"c%d"`, i)) {
					break
				}
			}
		}
		// cada um ja recebeu os "entrou" de quem veio depois: drena por contagem
		var wg sync.WaitGroup
		contagem := make([]int, clientes)
		for i, conn := range conns {
			wg.Add(1)
			go func(i int, conn *websocket.Conn) {
				defer wg.Done()
				esperadas := clientes * porCliente
				for contagem[i] < esperadas {
					_, b, err := conn.Read(ctx)
					if err != nil {
						t.Errorf("cliente %d: %v (recebeu %d)", i, err, contagem[i])
						return
					}
					if strings.Contains(string(b), `"tipo":"msg"`) {
						contagem[i]++
					}
				}
			}(i, conn)
		}
		for i, conn := range conns {
			wg.Add(1)
			go func(i int, conn *websocket.Conn) {
				defer wg.Done()
				for j := range porCliente {
					if err := conn.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf("m%d-%d", i, j))); err != nil {
						t.Errorf("write: %v", err)
						return
					}
				}
			}(i, conn)
		}
		wg.Wait()
		for i, n := range contagem {
			if n != clientes*porCliente {
				t.Errorf("cliente %d recebeu %d mensagens, esperado %d", i, n, clientes*porCliente)
			}
		}
		for _, conn := range conns {
			conn.Close(websocket.StatusNormalClosure, "")
		}
	})
}

// escuta("127.0.0.1:0"): mostra no stderr o endereco de verdade e desliga com
// calma esperando o pedido em andamento terminar. Nos dois engines.
func TestEscutaEnderecoEDesligaComCalma(t *testing.T) {
	fonte := `rota("GET", "/lento", gambiarra()
    espera_ms(300)
    funciona "terminei"
acabou_finalmente)
escuta("127.0.0.1:0")
mostra "escuta voltou"`
	for _, m := range motores {
		t.Run(m.nome, func(t *testing.T) {
			pr, pw := io.Pipe()
			out := &bufTravado{}
			var interp *interpreter.Interpreter
			pronto := make(chan *interpreter.Interpreter, 1)
			fim := make(chan struct{})
			go func() {
				defer close(fim)
				interp = rodaComGancho(t, m.nome, fonte, out, pw, pronto)
			}()
			i := <-pronto
			linha, err := bufio.NewReader(pr).ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			go io.Copy(io.Discard, pr)
			re := regexp.MustCompile(`^servidor de pe em (http://127\.0\.0\.1:\d+) \(ctrl\+c pra parar\)\n$`)
			sub := re.FindStringSubmatch(linha)
			if sub == nil {
				t.Fatalf("linha do escuta: %q", linha)
			}
			resp := make(chan respTeste, 1)
			go func() { resp <- pede(t, "GET", sub[1]+"/lento", "") }()
			time.Sleep(100 * time.Millisecond) // pedido ja em andamento
			i.PararServidor()
			select {
			case r := <-resp:
				r.confere(t, "pedido em andamento", 200, "terminei")
			case <-time.After(5 * time.Second):
				t.Fatal("pedido em andamento nao terminou")
			}
			select {
			case <-fim:
			case <-time.After(6 * time.Second):
				t.Fatal("escuta nao voltou depois do desligamento")
			}
			_ = interp
			if got := out.String(); got != "escuta voltou\n" {
				t.Errorf("saida: %q", got)
			}
		})
	}
}

// rodaComGancho roda o fonte inteiro (que chama escuta) e entrega o interp
// antes de rodar, pro teste conseguir chamar PararServidor.
func rodaComGancho(t *testing.T, motor, fonte string, out, errOut io.Writer, pronto chan<- *interpreter.Interpreter) *interpreter.Interpreter {
	p := parser.New(lexer.New(fonte))
	prog := p.ParseProgram()
	i := interpreter.New(out)
	i.DefinirStderr(errOut)
	pronto <- i
	if motor == "tree" {
		if res := i.Eval(prog, object.NewEnvironment()); res != nil && res.Type() == object.ERRO_OBJ {
			t.Errorf("tree: %s", res.Inspect())
		}
		return i
	}
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Errorf("compile: %v", err)
		return i
	}
	if err := NovaComInterp(comp.Bytecode(), out, i).Run(); err != nil {
		t.Errorf("vm: %v", err)
	}
	return i
}

func TestEscutaEnderecoInvalido(t *testing.T) {
	esperaNosDois(t, `escuta(70000)`, "", "porta vai de 0 a 65535")
	esperaNosDois(t, `escuta(deu_bom)`, "", "escuta(): a porta tem que ser numero ou texto")
	esperaNosDois(t, `escuta("nao:e:endereco")`, "", `nao consegui escutar em "nao:e:endereco"`)
}

// busca: HEAD/OPTIONS, opcao json, corpo_base64 na ida e na volta.
func TestBuscaParte2NosDois(t *testing.T) {
	binario := []byte{0x89, 'P', 'N', 'G', 0xff, 0x00}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bin":
			w.Header().Set("Content-Type", "image/png")
			w.Write(binario)
		default:
			b, _ := io.ReadAll(r.Body)
			w.Header().Set("X-Metodo", r.Method)
			fmt.Fprintf(w, "%s %s %s", r.Method, r.Header.Get("Content-Type"), base64.StdEncoding.EncodeToString(b))
		}
	}))
	defer srv.Close()
	src := `bota U = "` + srv.URL + `"
bota r = busca(U + "/bin")
mostra r.corpo_base64
mostra tem(r, "corpo_base64")
mostra tem(busca(U + "/eco"), "corpo_base64")
mostra busca(U + "/eco", {"metodo": "POST", "json": {"a": [1, nada]}}).corpo
mostra busca(U + "/eco", {"metodo": "PUT", "json": 1, "cabecalhos": {"content-type": "application/vnd+json"}}).corpo
mostra busca(U + "/eco", {"metodo": "POST", "corpo_base64": "iVBORw=="}).corpo
bota h = busca(U + "/eco", {"metodo": "HEAD"})
mostra h.status + " [" + h.corpo + "] " + h.cabecalhos["X-Metodo"]
mostra busca(U + "/eco", {"metodo": "options"}).corpo
arruma
    busca(U, {"corpo": "a", "json": 1})
quebrou err
    mostra erro_msg(err)
acabou_finalmente
arruma
    busca(U, {"corpo_base64": "%%%"})
quebrou err
    mostra erro_msg(err)
acabou_finalmente`
	esp := `iVBOR/8A
deu_bom
deu_ruim
POST application/json; charset=utf-8 eyJhIjpbMSxudWxsXX0=
PUT application/vnd+json MQ==
POST  iVBORw==
200 [] HEAD
` + "OPTIONS  " + `
deu ruim: busca(): escolhe um so entre "corpo", "corpo_base64" e "json", veio corpo e json
deu ruim: busca(): "corpo_base64" nao e base64 valido, parca: illegal base64 data at input byte 0
`
	esperaNosDois(t, src, esp, "")
}
