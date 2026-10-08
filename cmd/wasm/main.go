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

// saidaRastreada lembra o ultimo byte escrito na saida, pra entrada saber se
// o cursor ta no meio de uma linha (ex.: logo depois do prompt de pergunta).
type saidaRastreada struct {
	mu     sync.Mutex
	w      io.Writer
	ultimo byte
}

func (s *saidaRastreada) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(p) > 0 {
		s.ultimo = p[len(p)-1]
	}
	return s.w.Write(p)
}

// quebraLinha poe um \n se a saida parou no meio de uma linha
func (s *saidaRastreada) quebraLinha() {
	s.mu.Lock()
	meio := s.ultimo != 0 && s.ultimo != '\n'
	s.mu.Unlock()
	if meio {
		io.WriteString(s, "\n")
	}
}

// entradaComEco e o stdin do playground: o texto da caixa "Entrada (stdin)",
// entregue uma linha por Read. Como o bufio.Reader do interpretador so pede
// mais dado quando o buffer dele acaba, cada linha so sai daqui quando o
// programa vai consumir ela de fato — e nessa hora ela e ecoada na saida, do
// jeito que o terminal mostra o que voce digita. Assim
// `pergunta("teu nome: ")` aparece como "teu nome: Jurandir" na transcricao.
// Depois da ultima linha vem io.EOF, igual stdin fechado no nativo (pergunta
// devolve texto vazio); so que aqui, se a saida parou no meio de uma linha
// (o prompt), ela e quebrada — como o Enter/ctrl+d no terminal.
type entradaComEco struct {
	mu    sync.Mutex
	resto string
	eco   *saidaRastreada
}

func (e *entradaComEco) Read(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.resto == "" {
		e.eco.quebraLinha()
		return 0, io.EOF
	}
	linha := e.resto
	if i := strings.IndexByte(linha, '\n'); i >= 0 {
		linha = linha[:i+1]
	}
	if len(linha) > len(p) {
		linha = linha[:len(p)]
	}
	e.resto = e.resto[len(linha):]
	// eco sempre termina a linha, mesmo a ultima sem \n, pra proxima saida
	// do programa nao grudar no que foi "digitado"
	eco := linha
	if e.resto == "" && !strings.HasSuffix(eco, "\n") {
		eco += "\n"
	}
	io.WriteString(e.eco, eco)
	return copy(p, linha), nil
}

// avaliar roda o codigo GambiarraScript e devolve {saida, erros}. Se onSaida
// for uma funcao JS, a saida vai sendo entregue nela e `saida` volta vazia.
// `entrada` vira o stdin do programa (pergunta, le_linhas, le_tudo).
func avaliar(code string, onSaida js.Value, entrada string) map[string]any {
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return map[string]any{"saida": "", "erros": strings.Join(errs, "\n")}
	}

	var buf bytes.Buffer
	var destino io.Writer = &buf
	if onSaida.Type() == js.TypeFunction {
		destino = &escritorJS{fn: onSaida}
	}
	saida := &saidaRastreada{w: destino}
	var out io.Writer = saida
	interp := interpreter.New(out)
	interp.DefinirStdin(&entradaComEco{resto: entrada, eco: saida})

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
	if object.EhErroLevantado(resultado) {
		erros = resultado.Inspect()
	}

	return map[string]any{"saida": buf.String(), "erros": erros}
}

// gsEvaluate ponte JS: gsEvaluate(code, onSaida?, entrada?) -> {saida, erros}
func gsEvaluate(this js.Value, args []js.Value) any {
	if len(args) < 1 || len(args) > 3 {
		return map[string]any{"saida": "", "erros": "gsEvaluate quer o codigo (e opcionalmente onSaida e entrada)"}
	}
	onSaida := js.Undefined()
	if len(args) >= 2 {
		onSaida = args[1]
	}
	entrada := ""
	if len(args) == 3 && args[2].Type() == js.TypeString {
		entrada = args[2].String()
	}
	return avaliar(args[0].String(), onSaida, entrada)
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
