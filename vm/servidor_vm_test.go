package vm

import (
	"io"
	"net/http"
	"net/http/httptest"
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
