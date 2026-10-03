package vm

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

// TestRotaFuncionaNaVM: na VM o handler chega como *object.CompiledFunction, e
// `rota()` so aceitava *object.Funcao (o tipo do tree-walker). Resultado: dava
// "o handler tem que ser uma gambiarra, veio FUNCAO" no engine PADRAO — nao
// dava pra subir servidor nenhum com `gs roda`.
func TestRotaFuncionaNaVM(t *testing.T) {
	fonte := `gambiarra ola(pedido)
    funciona "salve, " + pedido["caminho"]
acabou_finalmente
gambiarra eco(pedido)
    funciona {"status": 201, "corpo": pra_json({"metodo": pedido["metodo"]})}
acabou_finalmente
rota("GET", "/oi", ola)
rota("POST", "/eco", eco)`

	prog := parser.New(lexer.New(fonte)).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	interp := interpreter.New(io.Discard)
	maq := NovaComInterp(comp.Bytecode(), io.Discard, interp)
	if err := maq.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	srv := httptest.NewServer(interp.ServidorHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/oi")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	corpo, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(corpo) != "salve, /oi" {
		t.Fatalf("GET /oi: status %d corpo %q", resp.StatusCode, corpo)
	}

	resp2, err := http.Post(srv.URL+"/eco", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	corpo2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != 201 || string(corpo2) != `{"metodo":"POST"}` {
		t.Fatalf("POST /eco: status %d corpo %q", resp2.StatusCode, corpo2)
	}
}

// TestArgumentosNaVM: `argumentos()` voltava lista vazia na VM porque o
// caminho da VM montava um interpreter proprio, sem os args do script.
func TestArgumentosNaVM(t *testing.T) {
	prog := parser.New(lexer.New(`mostra argumentos()`)).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	var saida escritor
	interp := interpreter.New(&saida)
	interp.DefinirArgumentos([]string{"oi", "tropa"})
	maq := NovaComInterp(comp.Bytecode(), &saida, interp)
	if err := maq.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := saida.String(); got != "[oi, tropa]\n" {
		t.Fatalf("argumentos() na VM deu %q, queria %q", got, "[oi, tropa]\n")
	}
}

type escritor struct{ b []byte }

func (e *escritor) Write(p []byte) (int, error) { e.b = append(e.b, p...); return len(p), nil }
func (e *escritor) String() string              { return string(e.b) }

// TestHandlerComArrumaNaVM: handler que faz de_json(pedido["corpo"]) dentro de
// arruma. Na VM o try/catch nao enxergava o param `pedido` ("freevar fora do
// range") e todo pedido caia no quebrou.
func TestHandlerComArrumaNaVM(t *testing.T) {
	fonte := `gambiarra cria(pedido)
    arruma
        bota dados = de_json(pedido["corpo"])
        funciona {"status": 201, "corpo": "oi " + dados["nome"]}
    quebrou err
        funciona {"status": 400, "corpo": "json ruim"}
    acabou_finalmente
acabou_finalmente
rota("POST", "/cria", cria)`

	prog := parser.New(lexer.New(fonte)).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	interp := interpreter.New(io.Discard)
	maq := NovaComInterp(comp.Bytecode(), io.Discard, interp)
	if err := maq.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	srv := httptest.NewServer(interp.ServidorHandler())
	defer srv.Close()

	casos := []struct {
		corpo, esp string
		status     int
	}{
		{`{"nome": "Ze"}`, "oi Ze", 201},
		{`{quebrado`, "json ruim", 400},
	}
	for _, c := range casos {
		// text/plain de proposito: com application/json o servidor ja barra
		// json quebrado com 400 antes do handler (pedido["json"]), e aqui o
		// que interessa e o arruma do handler.
		resp, err := http.Post(srv.URL+"/cria", "text/plain", strings.NewReader(c.corpo))
		if err != nil {
			t.Fatal(err)
		}
		corpo, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != c.status || string(corpo) != c.esp {
			t.Fatalf("POST %s: status %d corpo %q, queria %d %q", c.corpo, resp.StatusCode, corpo, c.status, c.esp)
		}
	}
}
