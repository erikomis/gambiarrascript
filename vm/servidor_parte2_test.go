package vm

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Servidor parte 2 (rotas com parametro, json no pedido, responde_json,
// antes/depois, cors, serve_pasta, erro 500 generico): cada teste sobe o MESMO
// fonte nos dois engines e confere status/corpo/cabecalhos exatos.

// bufTravado e um io.Writer seguro pra varias goroutines (os handlers
// escrevem no stdout/stderr em paralelo).
type bufTravado struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *bufTravado) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *bufTravado) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

type motorTeste struct {
	nome string
	// sobe roda o fonte (registra rotas, nao chama escuta) e devolve o interp.
	sobe func(t *testing.T, fonte string, out, errOut io.Writer) *interpreter.Interpreter
}

var motores = []motorTeste{
	{"tree", func(t *testing.T, fonte string, out, errOut io.Writer) *interpreter.Interpreter {
		t.Helper()
		p := parser.New(lexer.New(fonte))
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatalf("parse: %v", errs)
		}
		i := interpreter.New(out)
		i.DefinirStderr(errOut)
		if res := i.Eval(prog, object.NewEnvironment()); res != nil && res.Type() == object.ERRO_OBJ {
			t.Fatalf("[tree] erro montando servidor: %s", res.Inspect())
		}
		return i
	}},
	{"vm", func(t *testing.T, fonte string, out, errOut io.Writer) *interpreter.Interpreter {
		t.Helper()
		p := parser.New(lexer.New(fonte))
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			t.Fatalf("parse: %v", errs)
		}
		comp := compiler.New()
		if err := comp.Compile(prog); err != nil {
			t.Fatalf("compile: %v", err)
		}
		i := interpreter.New(out)
		i.DefinirStderr(errOut)
		if err := NovaComInterp(comp.Bytecode(), out, i).Run(); err != nil {
			t.Fatalf("[vm] erro montando servidor: %v", err)
		}
		return i
	}},
}

type ctxServidor struct {
	base string
	out  *bufTravado
	err  *bufTravado
}

// servidorNosDois sobe o fonte em cada engine e roda o teste contra ele.
func servidorNosDois(t *testing.T, fonte string, teste func(t *testing.T, c ctxServidor)) {
	t.Helper()
	for _, m := range motores {
		t.Run(m.nome, func(t *testing.T) {
			out, errOut := &bufTravado{}, &bufTravado{}
			i := m.sobe(t, fonte, out, errOut)
			srv := httptest.NewServer(i.ServidorHandler())
			defer srv.Close()
			teste(t, ctxServidor{base: srv.URL, out: out, err: errOut})
		})
	}
}

type respTeste struct {
	status int
	corpo  string
	cab    http.Header
}

func pede(t *testing.T, metodo, url, corpo string, cab ...string) respTeste {
	t.Helper()
	var r io.Reader
	if corpo != "" {
		r = strings.NewReader(corpo)
	}
	req, err := http.NewRequest(metodo, url, r)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(cab); i += 2 {
		req.Header.Add(cab[i], cab[i+1])
	}
	cliente := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := cliente.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return respTeste{resp.StatusCode, string(b), resp.Header}
}

func (r respTeste) confere(t *testing.T, oque string, status int, corpo string) {
	t.Helper()
	if r.status != status || r.corpo != corpo {
		t.Errorf("%s: veio %d %q, esperado %d %q", oque, r.status, r.corpo, status, corpo)
	}
}

func (r respTeste) cabecalho(t *testing.T, oque, nome, esp string) {
	t.Helper()
	if got := strings.Join(r.cab.Values(nome), " | "); got != esp {
		t.Errorf("%s: cabecalho %s = %q, esperado %q", oque, nome, got, esp)
	}
}

func TestRotaComParametroCuringaE405(t *testing.T) {
	fonte := `rota("GET", "/usuarios/:id", gambiarra(pedido)
    funciona "usuario " + pedido.params.id
acabou_finalmente)
rota("GET", "/usuarios/eu", gambiarra(pedido)
    funciona "sou eu (exata ganha)"
acabou_finalmente)
rota("GET", "/usuarios/:id/posts/:post", gambiarra(pedido)
    funciona pra_json(pedido.params)
acabou_finalmente)
rota("DELETE", "/usuarios/:id", gambiarra(pedido)
    funciona {"status": 204}
acabou_finalmente)
rota("GET", "/arquivos/*caminho", gambiarra(pedido)
    funciona "arquivo [" + pedido.params.caminho + "]"
acabou_finalmente)
rota("GET", "/arquivos/fixo/:x", gambiarra(pedido)
    funciona "param ganha do curinga: " + pedido.params.x
acabou_finalmente)
rota("GET", "/plain", gambiarra(pedido)
    funciona "params vazio: " + pra_json(pedido.params)
acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		pede(t, "GET", c.base+"/usuarios/42", "").confere(t, "param", 200, "usuario 42")
		pede(t, "GET", c.base+"/usuarios/eu", "").confere(t, "exata", 200, "sou eu (exata ganha)")
		pede(t, "GET", c.base+"/usuarios/a%2Fb", "").confere(t, "param escapado", 200, "usuario a/b")
		pede(t, "GET", c.base+"/usuarios/7/posts/9", "").confere(t, "2 params", 200, `{"id":"7","post":"9"}`)
		pede(t, "GET", c.base+"/usuarios/", "").confere(t, "param vazio", 404, "rota nao encontrada, parca")
		pede(t, "GET", c.base+"/usuarios/1/2", "").confere(t, "pedaco a mais", 404, "rota nao encontrada, parca")
		pede(t, "GET", c.base+"/arquivos/css/site.css", "").confere(t, "curinga", 200, "arquivo [css/site.css]")
		pede(t, "GET", c.base+"/arquivos", "").confere(t, "curinga vazio", 200, "arquivo []")
		pede(t, "GET", c.base+"/arquivos/fixo/z", "").confere(t, "especifica", 200, "param ganha do curinga: z")
		pede(t, "GET", c.base+"/plain", "").confere(t, "rota exata", 200, "params vazio: {}")
		pede(t, "DELETE", c.base+"/usuarios/42", "").confere(t, "delete", 204, "")

		r := pede(t, "POST", c.base+"/usuarios/42", "x")
		r.confere(t, "405", 405, "metodo POST nao rola nesse caminho, parca (tenta DELETE, GET, HEAD)")
		r.cabecalho(t, "405", "Allow", "DELETE, GET, HEAD")

		h := pede(t, "HEAD", c.base+"/usuarios/42", "")
		h.confere(t, "HEAD usa o GET", 200, "")
		h.cabecalho(t, "HEAD", "Content-Length", "10")
	})
}

func TestRotaPadraoInvalido(t *testing.T) {
	esperaNosDois(t, `rota("GET", "/a/*resto/b", gambiarra() funciona 1 acabou_finalmente)`, "",
		`rota(): o curinga "*resto" so pode ser o ultimo pedaco do caminho`)
	esperaNosDois(t, `rota("GET", "/a/:/b", gambiarra() funciona 1 acabou_finalmente)`, "",
		`rota(): o parametro ":" ficou sem nome`)
	esperaNosDois(t, `rota("GET", "/a/:id/:id", gambiarra() funciona 1 acabou_finalmente)`, "",
		`rota(): o parametro "id" apareceu duas vezes no caminho`)
}

func TestPedidoJsonIpCookies(t *testing.T) {
	fonte := `bota chamadas = {"n": 0}
rota("POST", "/eco", gambiarra(pedido)
    bota chamadas["n"] = chamadas["n"] + 1
    funciona "json=" + pra_json(pedido.json) + " ip=" + pedido.ip + " cookies=" + pra_json(pedido.cookies)
acabou_finalmente)
rota("GET", "/chamadas", gambiarra() funciona texto(chamadas["n"]) acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		pede(t, "POST", c.base+"/eco", `{"b": [1, 2], "a": "x"}`, "Content-Type", "application/json", "Cookie", "sessao=abc; tema=escuro").
			confere(t, "json", 200, `json={"b":[1,2],"a":"x"} ip=127.0.0.1 cookies={"sessao":"abc","tema":"escuro"}`)
		pede(t, "POST", c.base+"/eco", `{"a": 1}`, "Content-Type", "text/plain").
			confere(t, "nao-json", 200, `json=null ip=127.0.0.1 cookies={}`)
		pede(t, "POST", c.base+"/eco", `{"a": 1}`, "Content-Type", "application/problem+json; charset=utf-8").
			confere(t, "+json", 200, `json={"a":1} ip=127.0.0.1 cookies={}`)
		pede(t, "POST", c.base+"/eco", ``, "Content-Type", "application/json").
			confere(t, "json vazio", 200, `json=null ip=127.0.0.1 cookies={}`)
		r := pede(t, "POST", c.base+"/eco", `{quebrado`, "Content-Type", "application/json")
		r.confere(t, "json quebrado", 400, "esse json do corpo ta quebrado, parca: chave de objeto tem que ser texto entre aspas (posicao 1)")
		pede(t, "GET", c.base+"/chamadas", "").confere(t, "handler nao rodou no 400", 200, "4")
	})
}

func TestRespostaJsonAutomaticaECookies(t *testing.T) {
	fonte := `rota("GET", "/lista", gambiarra() funciona [1, "dois", deu_bom, nada] acabou_finalmente)
rota("GET", "/dic", gambiarra() funciona {"status": "ok", "itens": 2} acabou_finalmente)
rota("GET", "/vazio", gambiarra() funciona {} acabou_finalmente)
rota("POST", "/criado", gambiarra() funciona responde_json({"id": 7}, 201) acabou_finalmente)
rota("GET", "/corpo_dic", gambiarra() funciona {"status": 202, "corpo": {"fila": 3}} acabou_finalmente)
rota("GET", "/cookie", gambiarra()
    funciona {"corpo": "biscoito", "cabecalhos": {"Set-Cookie": ["a=1; Path=/", "b=2; HttpOnly"], "X-Num": 5}}
acabou_finalmente)
rota("GET", "/numero", gambiarra() funciona 42 acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		r := pede(t, "GET", c.base+"/lista", "")
		r.confere(t, "lista", 200, `[1,"dois",true,null]`)
		r.cabecalho(t, "lista", "Content-Type", "application/json; charset=utf-8")
		pede(t, "GET", c.base+"/dic", "").confere(t, "dic de dado", 200, `{"status":"ok","itens":2}`)
		pede(t, "GET", c.base+"/vazio", "").confere(t, "dic vazio", 200, `{}`)
		r = pede(t, "POST", c.base+"/criado", "")
		r.confere(t, "responde_json", 201, `{"id":7}`)
		r.cabecalho(t, "responde_json", "Content-Type", "application/json; charset=utf-8")
		r = pede(t, "GET", c.base+"/corpo_dic", "")
		r.confere(t, "corpo dicionario", 202, `{"fila":3}`)
		r.cabecalho(t, "corpo dicionario", "Content-Type", "application/json; charset=utf-8")
		r = pede(t, "GET", c.base+"/cookie", "")
		r.confere(t, "cookie", 200, "biscoito")
		r.cabecalho(t, "cookie", "Set-Cookie", "a=1; Path=/ | b=2; HttpOnly")
		r.cabecalho(t, "cookie", "X-Num", "5")
		pede(t, "GET", c.base+"/numero", "").confere(t, "numero", 500, "deu ruim no servidor (erro interno), parca")
		if !strings.Contains(c.err.String(), "o handler devolveu algo que nao da pra responder: NUMERO") {
			t.Errorf("stderr sem o motivo do 500: %q", c.err.String())
		}
	})
}

func TestRespondeJsonNosDois(t *testing.T) {
	esperaNosDois(t, `bota r = responde_json({"a": [1, 2]})
mostra r.status
mostra r.corpo
mostra r.cabecalhos
mostra responde_json("oi", 404).corpo`,
		"200\n{\"a\":[1,2]}\n{\"Content-Type\": \"application/json; charset=utf-8\"}\n\"oi\"\n", "")
	esperaNosDois(t, `responde_json(1, 99)`, "", "responde_json(): o status tem que ser numero inteiro de 100 a 599, veio 99")
	esperaNosDois(t, `responde_json(gambiarra() funciona 1 acabou_finalmente)`, "", "nao da pra virar json: FUNCAO")
}

func TestMiddlewareAntesEDepois(t *testing.T) {
	fonte := `antes(gambiarra(pedido)
    se_colar pedido.cabecalhos["Authorization"] != "Bearer ok"
        funciona responde_json({"erro": "sem token"}, 401)
    acabou_finalmente
    bota pedido["usuario"] = "ze"
acabou_finalmente)
antes(gambiarra(pedido)
    bota pedido["ordem"] = (pedido["ordem"] ?? "") + "2"
acabou_finalmente)
depois(gambiarra(pedido, resposta)
    mostra pedido.metodo + " " + pedido.caminho + " -> " + texto(resposta.status)
acabou_finalmente)
depois(gambiarra(pedido, resposta)
    bota resposta["cabecalhos"]["X-Visto"] = "sim"
acabou_finalmente)
rota("GET", "/eu", gambiarra(pedido)
    funciona "oi " + pedido.usuario + " ordem " + pedido.ordem
acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		r := pede(t, "GET", c.base+"/eu", "")
		r.confere(t, "sem token", 401, `{"erro":"sem token"}`)
		r.cabecalho(t, "sem token", "X-Visto", "sim")
		r = pede(t, "GET", c.base+"/eu", "", "Authorization", "Bearer ok")
		r.confere(t, "com token", 200, "oi ze ordem 2")
		r.cabecalho(t, "com token", "X-Visto", "sim")
		pede(t, "GET", c.base+"/naotem", "").confere(t, "404 nao passa no middleware", 404, "rota nao encontrada, parca")
		if got := c.out.String(); got != "GET /eu -> 401\nGET /eu -> 200\n" {
			t.Errorf("log do depois: %q", got)
		}
	})
}

func TestCors(t *testing.T) {
	livre := `cors()
rota("GET", "/x", gambiarra() funciona "x" acabou_finalmente)`
	servidorNosDois(t, livre, func(t *testing.T, c ctxServidor) {
		r := pede(t, "OPTIONS", c.base+"/x", "", "Origin", "https://a.com", "Access-Control-Request-Method", "PUT", "Access-Control-Request-Headers", "content-type")
		r.confere(t, "preflight", 204, "")
		r.cabecalho(t, "preflight", "Access-Control-Allow-Origin", "*")
		r.cabecalho(t, "preflight", "Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		r.cabecalho(t, "preflight", "Access-Control-Allow-Headers", "content-type")
		r.cabecalho(t, "preflight", "Access-Control-Max-Age", "600")
		r = pede(t, "GET", c.base+"/x", "", "Origin", "https://a.com")
		r.confere(t, "simples", 200, "x")
		r.cabecalho(t, "simples", "Access-Control-Allow-Origin", "*")
		r = pede(t, "GET", c.base+"/x", "")
		r.cabecalho(t, "sem Origin", "Access-Control-Allow-Origin", "")
	})

	restrito := `cors({"origens": ["https://app.com"], "metodos": ["get", "post"], "cabecalhos": ["Authorization"], "credenciais": deu_bom, "max_idade": 60})
rota("GET", "/x", gambiarra() funciona "x" acabou_finalmente)`
	servidorNosDois(t, restrito, func(t *testing.T, c ctxServidor) {
		r := pede(t, "OPTIONS", c.base+"/x", "", "Origin", "https://app.com", "Access-Control-Request-Method", "POST")
		r.confere(t, "preflight liberado", 204, "")
		r.cabecalho(t, "preflight liberado", "Access-Control-Allow-Origin", "https://app.com")
		r.cabecalho(t, "preflight liberado", "Access-Control-Allow-Credentials", "true")
		r.cabecalho(t, "preflight liberado", "Access-Control-Allow-Methods", "GET, POST")
		r.cabecalho(t, "preflight liberado", "Access-Control-Allow-Headers", "Authorization")
		r.cabecalho(t, "preflight liberado", "Access-Control-Max-Age", "60")
		pede(t, "OPTIONS", c.base+"/x", "", "Origin", "https://mal.com", "Access-Control-Request-Method", "POST").
			confere(t, "preflight barrado", 403, "origem https://mal.com nao ta liberada no cors(), parca")
		r = pede(t, "GET", c.base+"/x", "", "Origin", "https://mal.com")
		r.confere(t, "origem barrada", 200, "x")
		r.cabecalho(t, "origem barrada", "Access-Control-Allow-Origin", "")
	})

	esperaNosDois(t, `cors({"origem": ["x"]})`, "", `cors(): opcao desconhecida "origem"`)
}

func TestServePasta(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "public")
	escreve := func(rel, conteudo string) {
		t.Helper()
		p := filepath.Join(pub, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escreve("index.html", "<h1>oi</h1>")
	escreve("css/site.css", "body{}")
	escreve("app.js", "1")
	escreve("dados.json", "{}")
	escreve("sem_index/x.txt", "x")
	escreve(".env", "SEGREDO=1")
	if err := os.WriteFile(filepath.Join(dir, "segredo.txt"), []byte("nao"), 0o644); err != nil {
		t.Fatal(err)
	}
	// symlink pra fora da pasta nao pode vazar
	_ = os.Symlink(filepath.Join(dir, "segredo.txt"), filepath.Join(pub, "atalho.txt"))

	fonte := `serve_pasta("/publico", "` + filepath.ToSlash(pub) + `")
rota("GET", "/publico/api", gambiarra() funciona "rota ganha" acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		r := pede(t, "GET", c.base+"/publico/", "")
		r.confere(t, "index", 200, "<h1>oi</h1>")
		r.cabecalho(t, "index", "Content-Type", "text/html; charset=utf-8")
		r = pede(t, "GET", c.base+"/publico", "")
		r.confere(t, "sem barra", 301, "<a href=\"/publico/\">Moved Permanently</a>.\n\n")
		r.cabecalho(t, "sem barra", "Location", "/publico/")
		r = pede(t, "GET", c.base+"/publico/css/site.css", "")
		r.confere(t, "css", 200, "body{}")
		r.cabecalho(t, "css", "Content-Type", "text/css; charset=utf-8")
		r = pede(t, "GET", c.base+"/publico/dados.json", "")
		r.cabecalho(t, "json", "Content-Type", "application/json")
		pede(t, "GET", c.base+"/publico/api", "").confere(t, "rota ganha", 200, "rota ganha")
		naoAchei := "arquivo nao encontrado, parca"
		pede(t, "GET", c.base+"/publico/sem_index/", "").confere(t, "pasta sem index nao lista", 404, naoAchei)
		pede(t, "GET", c.base+"/publico/.env", "").confere(t, "dotfile", 404, naoAchei)
		pede(t, "GET", c.base+"/publico/../segredo.txt", "").confere(t, "traversal", 404, naoAchei)
		pede(t, "GET", c.base+"/publico/%2e%2e/segredo.txt", "").confere(t, "traversal escapado", 404, naoAchei)
		pede(t, "GET", c.base+"/publico/atalho.txt", "").confere(t, "symlink pra fora", 404, naoAchei)
		pede(t, "GET", c.base+"/publico/nao.txt", "").confere(t, "nao existe", 404, naoAchei)
		r = pede(t, "POST", c.base+"/publico/app.js", "")
		r.confere(t, "post", 405, "pasta estatica so atende GET e HEAD, parca")
		r.cabecalho(t, "post", "Allow", "GET, HEAD")
		pede(t, "GET", c.base+"/outro", "").confere(t, "fora do prefixo", 404, "rota nao encontrada, parca")
	})

	esperaNosDois(t, `serve_pasta("/x", "/nao/existe/mesmo")`, "", `serve_pasta(): a pasta "/nao/existe/mesmo" nao existe, parca`)
}

func TestErroNoHandlerVira500GenericoELoga(t *testing.T) {
	fonte := `gambiarra divide(pedido)
    bota x = 0
    funciona 10 / x
acabou_finalmente
rota("GET", "/quebra", divide)
gambiarra ajuda()
    quebra("segredo interno")
acabou_finalmente
rota("GET", "/quebra_user", gambiarra()
    funciona ajuda()
acabou_finalmente)
rota("GET", "/ok", gambiarra() funciona "ok" acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		pede(t, "GET", c.base+"/quebra", "").confere(t, "500", 500, "deu ruim no servidor (erro interno), parca")
		pede(t, "GET", c.base+"/quebra_user", "").confere(t, "500 sem vazar", 500, "deu ruim no servidor (erro interno), parca")
		pede(t, "GET", c.base+"/ok", "").confere(t, "segue vivo", 200, "ok")
		log := c.err.String()
		for _, esp := range []string{
			"servidor: GET /quebra estourou em <rota GET /quebra>\n",
			"linha 3",
			"nao da pra dividir por zero",
			"servidor: GET /quebra_user estourou em <rota GET /quebra_user>\n",
			"segredo interno",
			"Traço de pilha:\n",
			"em ajuda (linha 10)",
		} {
			if !strings.Contains(log, esp) {
				t.Errorf("stderr sem %q:\n%s", esp, log)
			}
		}
	})
}
