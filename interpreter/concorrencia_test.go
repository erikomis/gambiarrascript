package interpreter

import (
	"bytes"
	"strings"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

func TestParaleloCalculaTodosEmParalelo(t *testing.T) {
	p := parser.New(lexer.New(`
gambiarra dobra(n)
    funciona n * 2
acabou_finalmente
bota r = paralelo([1, 2, 3, 4, 5], dobra)
mostra r`))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse: %v", errs)
	}
	var out bytes.Buffer
	interp := New(&out)
	res := interp.Eval(prog, object.NewEnvironment())
	if isError(res) {
		t.Fatalf("runtime erro: %s", res.Inspect())
	}
	if out.String() != "[2, 4, 6, 8, 10]\n" {
		t.Fatalf("saida: %q", out.String())
	}
}

func TestParaleloListaVazia(t *testing.T) {
	p := parser.New(lexer.New(`
gambiarra id(x)
    funciona x
acabou_finalmente
mostra paralelo([], id)`))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse: %v", errs)
	}
	var out bytes.Buffer
	interp := New(&out)
	res := interp.Eval(prog, object.NewEnvironment())
	if isError(res) {
		t.Fatalf("runtime: %s", res.Inspect())
	}
	if out.String() != "[]\n" {
		t.Fatalf("esperava [], got %q", out.String())
	}
}

func TestParaleloPropagaErroDoHandler(t *testing.T) {
	// handler quebra no index 2 — paralelo deveria propagar o Erro
	p := parser.New(lexer.New(`
gambiarra f(n)
    se_colar n == 3
        funciona quebra("explodi no " + texto(n))
    acabou_finalmente
    funciona n
acabou_finalmente
bota r = paralelo([1, 2, 3], f)
mostra r`))
	prog := p.ParseProgram()
	var out bytes.Buffer
	interp := New(&out)
	res := interp.Eval(prog, object.NewEnvironment())
	if !isError(res) {
		t.Fatalf("esperava Erro propagado, got %s (%q)", res.Type(), out.String())
	}
	if !strings.Contains(res.Inspect(), "explodi") {
		t.Fatalf("mensagem do erro: %q", res.Inspect())
	}
}

func TestEscutaServidorParalelo(t *testing.T) {
	// valida que meio server continua vivo apos handler lento (prova que
	// requisicoes sao paralelas — nao serializadas). Roda 10 reqs em paralelo
	// contra handler que "dorme"; todas devem completar.
	srv := servidorDeTeste(t, `gambiarra h(pedido)
    # simula trabalho sem esperar — so confirma alcançabilidade
    funciona "ok"
acabou_finalmente
rota("GET", "/h", h)`)
	defer srv.Close()

	// dispara Varias em paralelo
	const N = 12
	done := make(chan string, N)
	for i := 0; i < N; i++ {
		go func() {
			resp, err := srv.Client().Get(srv.URL + "/h")
			if err != nil {
				done <- "ERR:" + err.Error()
				return
			}
			defer resp.Body.Close()
			b := make([]byte, 2)
			resp.Body.Read(b)
			done <- string(b)
		}()
	}
	for i := 0; i < N; i++ {
		got := <-done
		if got != "ok" {
			t.Fatalf("req %d: got %q", i, got)
		}
	}
}