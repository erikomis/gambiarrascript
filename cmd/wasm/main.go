//go:build js && wasm

package main

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"syscall/js"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/vm"
)

// Versao do runtime WASM — mantida em sincronia visual com cmd/gs/version.go
// quando alterada, recompile o wasm (scripts/build-web).

// escritorJS repassa cada escrita do programa (mostra & cia) pra uma funcao
// JS, assim o playground mostra a saida ao vivo em vez de so no fim. O mutex
// segura escritas de goroutines do `bora` (no wasm e uma thread so, mas nao
// custa nada).
type escritorJS struct {
	mu sync.Mutex
	fn js.Value
}

func (e *escritorJS) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fn.Invoke(string(p))
	return len(p), nil
}

// avaliar roda o codigo GambiarraScript e devolve {saida, erros}. Se onSaida
// for uma funcao JS, a saida vai sendo entregue nela e `saida` volta vazia.
func avaliar(code string, onSaida js.Value) map[string]any {
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return map[string]any{"saida": "", "erros": strings.Join(errs, "\n")}
	}

	var buf bytes.Buffer
	var out io.Writer = &buf
	if onSaida.Type() == js.TypeFunction {
		out = &escritorJS{fn: onSaida}
	}
	interp := interpreter.New(out)

	// roda na VM, igual `gs roda` — o playground tem que mostrar o mesmo
	// comportamento (e a mesma velocidade) do engine de verdade. Codigo que a
	// VM nao compila cai no tree-walker, que e mais permissivo.
	comp := compiler.New()
	if err := comp.Compile(prog); err == nil {
		maq := vm.NovaComInterp(comp.Bytecode(), out, interp)
		erros := ""
		if err := maq.Run(); err != nil {
			if e := vm.ErroDoRun(err); e != nil {
				erros = e.Inspect()
			} else {
				erros = err.Error()
			}
		}
		return map[string]any{"saida": buf.String(), "erros": erros}
	}

	resultado := interp.Eval(prog, object.NewEnvironment())

	erros := ""
	if resultado != nil && resultado.Type() == object.ERRO_OBJ {
		erros = resultado.Inspect()
	}

	return map[string]any{"saida": buf.String(), "erros": erros}
}

// gsEvaluate ponte JS: gsEvaluate(code, onSaida?) -> {saida, erros}
func gsEvaluate(this js.Value, args []js.Value) any {
	if len(args) < 1 || len(args) > 2 {
		return map[string]any{"saida": "", "erros": "gsEvaluate quer o codigo (e opcionalmente onSaida)"}
	}
	onSaida := js.Undefined()
	if len(args) == 2 {
		onSaida = args[1]
	}
	return avaliar(args[0].String(), onSaida)
}

func main() {
	gs := js.Global().Get("GambiarraScript")
	if !gs.Truthy() {
		gs = js.Global().Get("Object").New()
		js.Global().Set("GambiarraScript", gs)
	}
	gs.Set("evaluate", js.FuncOf(gsEvaluate))

	// sinaliza pro loader que o modulo wasm ta pronto
	ready := js.Global().Get("Object").New()
	ready.Set("ready", true)
	js.Global().Set("__gsWasmReady", ready)

	// mantem o modulo vivo pra as chamadas JS continuarem funcionando
	c := make(chan struct{})
	<-c
}
