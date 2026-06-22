package interpreter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// servidorDeTeste roda um .gs que registra rotas (sem chamar escuta) e devolve
// um httptest.Server usando o Handler do interpretador.
func servidorDeTeste(t *testing.T, fonte string) *httptest.Server {
	t.Helper()
	prog := parser.New(lexer.New(fonte)).ParseProgram()
	i := New(io.Discard)
	if res := i.Eval(prog, object.NewEnvironment()); isError(res) {
		t.Fatalf("erro montando servidor: %s", res.Inspect())
	}
	return httptest.NewServer(i.ServidorHandler())
}

func TestServidorGetTexto(t *testing.T) {
	srv := servidorDeTeste(t, `gambiarra ola(pedido)
    funciona "salve, " + pedido["caminho"]
acabou_finalmente
rota("GET", "/oi", ola)`)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/oi")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status: got %d, esperado 200", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "salve, /oi" {
		t.Fatalf("corpo: got %q", string(b))
	}
}

func TestServidor404(t *testing.T) {
	srv := servidorDeTeste(t, `gambiarra ola(pedido)
    funciona "oi"
acabou_finalmente
rota("GET", "/oi", ola)`)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/naoexiste")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("esperava 404, got %d", resp.StatusCode)
	}
}

func TestServidorPostDictResposta(t *testing.T) {
	srv := servidorDeTeste(t, `gambiarra eco(pedido)
    funciona {"status": 201, "corpo": pedido["corpo"]}
acabou_finalmente
rota("POST", "/eco", eco)`)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/eco", "text/plain", strings.NewReader("oi servidor"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status: got %d, esperado 201", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "oi servidor" {
		t.Fatalf("corpo: got %q", string(b))
	}
}

func TestServidorHandlerErro500EContinuaVivo(t *testing.T) {
	srv := servidorDeTeste(t, `gambiarra quebra(pedido)
    funciona 1 / 0
acabou_finalmente
gambiarra ok(pedido)
    funciona "ok"
acabou_finalmente
rota("GET", "/quebra", quebra)
rota("GET", "/ok", ok)`)
	defer srv.Close()

	r1, _ := http.Get(srv.URL + "/quebra")
	if r1.StatusCode != 500 {
		t.Fatalf("handler com erro deveria dar 500, got %d", r1.StatusCode)
	}
	r2, _ := http.Get(srv.URL + "/ok")
	if r2.StatusCode != 200 {
		t.Fatalf("servidor deveria seguir vivo (200), got %d", r2.StatusCode)
	}
}

func TestServidorQueryECabecalhos(t *testing.T) {
	srv := servidorDeTeste(t, `gambiarra ver(pedido)
    funciona pedido["query"]["nome"] + "|" + pedido["cabecalhos"]["X-Teste"]
acabou_finalmente
rota("GET", "/ver", ver)`)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/ver?nome=erik", nil)
	req.Header.Set("X-Teste", "valor")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "erik|valor" {
		t.Fatalf("corpo: got %q", string(b))
	}
}

func TestEscutaPortaInvalida(t *testing.T) {
	i := New(io.Discard)
	res := i.servidor.builtinEscuta([]object.Object{&object.Numero{Value: 999999}})
	if res.Type() != object.ERRO_OBJ {
		t.Fatalf("porta invalida deveria dar erro, got %s", res.Type())
	}
}
