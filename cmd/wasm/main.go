package main

import (
	"bytes"
	"strings"
	"syscall/js"

	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Versao do runtime WASM — mantida em sincronia visual com cmd/gs/version.go
// quando alterada, recompile o wasm (scripts/build-web).

// avaliar roda o codigo GambiarraScript e devolve {saida, erros}.
func avaliar(code string) map[string]any {
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return map[string]any{"saida": "", "erros": strings.Join(errs, "\n")}
	}

	var buf bytes.Buffer
	interp := interpreter.New(&buf)
	resultado := interp.Eval(prog, object.NewEnvironment())

	erros := ""
	if resultado != nil && resultado.Type() == object.ERRO_OBJ {
		erros = resultado.Inspect()
	}

	return map[string]any{"saida": buf.String(), "erros": erros}
}

// gsEvaluate ponte JS: gsEvaluate(code) -> {saida, erros}
func gsEvaluate(this js.Value, args []js.Value) any {
	if len(args) != 1 {
		return map[string]any{"saida": "", "erros": "gsEvaluate quer 1 argumento (codigo)"}
	}
	code := args[0].String()
	return avaliar(code)
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