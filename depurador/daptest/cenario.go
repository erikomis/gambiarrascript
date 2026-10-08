package daptest

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ProgramaCenario e o .gs do cenario de ponta a ponta. As linhas importam
// (os breakpoints apontam pra elas).
const ProgramaCenario = `treta Ponto
    x
    y
acabou_finalmente
gambiarra soma(a, b)
    bota r = a + b
    funciona r
acabou_finalmente
gambiarra processa(xs, pt)
    bota total = 0
    pra_cada x em xs
        bota total = total + x
    acabou_finalmente
    bota s = soma(total, pt.x)
    funciona s
acabou_finalmente
gambiarra trampo(n)
    bota dobro = n * 2
    funciona dobro
acabou_finalmente
bota lista = [10, 20, 30]
bota ponto = Ponto{1, 2}
mostra processa(lista, ponto)
bota fut = bora trampo(21)
mostra espera(fut)
`

// t e o pedaco de testing que o cenario usa.
type t interface {
	Helper()
	Fatalf(format string, args ...any)
	Errorf(format string, args ...any)
}

// pede faz o request e exige sucesso.
func pede(tt t, c *Cliente, cmd string, args any) Msg {
	tt.Helper()
	m, err := c.Pede(cmd, args)
	if err != nil {
		tt.Fatalf("%v\nlog: %s", err, strings.Join(c.Log, "\n"))
	}
	if m["success"] != true {
		tt.Fatalf("%s falhou: %v", cmd, m["message"])
	}
	return m
}

func espera(tt t, c *Cliente, nome string, ok func(Msg) bool) Msg {
	tt.Helper()
	m, err := c.Espera(nome, ok)
	if err != nil {
		tt.Fatalf("%v\nlog: %s", err, strings.Join(c.Log, "\n"))
	}
	return m
}

func num(v any) int {
	f, _ := v.(float64)
	return int(f)
}

// Quadro e um stack frame resumido.
type Quadro struct {
	ID    int
	Nome  string
	Linha int
	Path  string
}

func pilha(tt t, c *Cliente, thread int) []Quadro {
	tt.Helper()
	r := pede(tt, c, "stackTrace", map[string]any{"threadId": thread})
	var out []Quadro
	for _, f := range r.Corpo()["stackFrames"].([]any) {
		fm := f.(map[string]any)
		q := Quadro{ID: num(fm["id"]), Nome: fm["name"].(string), Linha: num(fm["line"])}
		if src, ok := fm["source"].(map[string]any); ok {
			q.Path, _ = src["path"].(string)
		}
		out = append(out, q)
	}
	return out
}

func resumoPilha(qs []Quadro) string {
	partes := make([]string, len(qs))
	for k, q := range qs {
		partes[k] = fmt.Sprintf("%s:%d", q.Nome, q.Linha)
	}
	return strings.Join(partes, " ")
}

// variaveis devolve nome -> (valor, ref).
func variaveis(tt t, c *Cliente, ref int) (map[string]string, map[string]int) {
	tt.Helper()
	r := pede(tt, c, "variables", map[string]any{"variablesReference": ref})
	vals, refs := map[string]string{}, map[string]int{}
	for _, v := range r.Corpo()["variables"].([]any) {
		vm := v.(map[string]any)
		nome := vm["name"].(string)
		vals[nome] = vm["value"].(string)
		refs[nome] = num(vm["variablesReference"])
	}
	return vals, refs
}

func escopos(tt t, c *Cliente, frame int) map[string]int {
	tt.Helper()
	r := pede(tt, c, "scopes", map[string]any{"frameId": frame})
	out := map[string]int{}
	for _, s := range r.Corpo()["scopes"].([]any) {
		sm := s.(map[string]any)
		out[sm["name"].(string)] = num(sm["variablesReference"])
	}
	return out
}

func confere(tt t, oque, tem, quer string) {
	tt.Helper()
	if tem != quer {
		tt.Errorf("%s: tem %q, queria %q", oque, tem, quer)
	}
}

func paradaEm(motivo string, thread int) func(Msg) bool {
	return func(m Msg) bool {
		b := m.Corpo()
		return b["reason"] == motivo && (thread == 0 || num(b["threadId"]) == thread)
	}
}

// Cenario roda a sessao completa contra um adapter ja ligado ao cliente:
// launch com stopOnEntry, breakpoints (um que anda pra proxima linha
// executavel, um num bora, um invalido), continue ate o breakpoint, pilha,
// escopos e variaveis (lista e treta expandidas), next/stepIn/stepOut,
// evaluate, breakpoint dentro do bora (outra thread) e ate o fim.
func Cenario(tt t, c *Cliente, arquivo, engine string) {
	tt.Helper()
	init := pede(tt, c, "initialize", map[string]any{"adapterID": "gambiarrascript", "linesStartAt1": true})
	if init.Corpo()["supportsConfigurationDoneRequest"] != true {
		tt.Errorf("initialize sem supportsConfigurationDoneRequest: %v", init.Corpo())
	}
	espera(tt, c, "initialized", nil)
	pede(tt, c, "launch", map[string]any{"program": arquivo, "stopOnEntry": true, "engine": engine})

	bps := pede(tt, c, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": arquivo},
		"breakpoints": []any{map[string]any{"line": 13}, map[string]any{"line": 18}, map[string]any{"line": 40}},
	}).Corpo()["breakpoints"].([]any)
	var bpLinhas []string
	for _, b := range bps {
		bm := b.(map[string]any)
		bpLinhas = append(bpLinhas, fmt.Sprintf("%v@%d", bm["verified"], num(bm["line"])))
	}
	// 13 e o acabou_finalmente do laco: anda pra 14; 40 passa do fim
	confere(tt, "breakpoints", strings.Join(bpLinhas, " "), "true@14 true@18 false@40")
	bpProcessa := num(bps[0].(map[string]any)["id"])

	pede(tt, c, "configurationDone", nil)
	espera(tt, c, "stopped", paradaEm("entry", 1))
	ths := pede(tt, c, "threads", nil).Corpo()["threads"].([]any)
	if len(ths) == 0 || num(ths[0].(map[string]any)["id"]) != 1 {
		tt.Errorf("threads: %v", ths)
	}
	confere(tt, "pilha na entrada", resumoPilha(pilha(tt, c, 1)), "<principal>:1")

	// continue -> breakpoint em processa
	pede(tt, c, "continue", map[string]any{"threadId": 1})
	st := espera(tt, c, "stopped", paradaEm("breakpoint", 1))
	if ids, _ := st.Corpo()["hitBreakpointIds"].([]any); len(ids) != 1 || num(ids[0]) != bpProcessa {
		tt.Errorf("hitBreakpointIds: %v (queria %d)", st.Corpo()["hitBreakpointIds"], bpProcessa)
	}
	qs := pilha(tt, c, 1)
	confere(tt, "pilha no breakpoint", resumoPilha(qs), "processa:14 <principal>:23")
	if filepath.Base(qs[0].Path) != filepath.Base(arquivo) {
		tt.Errorf("source.path do quadro: %q", qs[0].Path)
	}
	sc := escopos(tt, c, qs[0].ID)
	vals, refs := variaveis(tt, c, sc["Locais"])
	confere(tt, "total", vals["total"], "60")
	confere(tt, "x", vals["x"], "30")
	confere(tt, "xs", vals["xs"], "[10, 20, 30]")
	confere(tt, "pt", vals["pt"], "Ponto{x: 1, y: 2}")
	if refs["total"] != 0 || refs["xs"] == 0 || refs["pt"] == 0 {
		tt.Errorf("variablesReference: %v", refs)
	}
	filhos, _ := variaveis(tt, c, refs["xs"])
	confere(tt, "xs expandida", fmt.Sprint(filhos), "map[[0]:10 [1]:20 [2]:30]")
	campos, _ := variaveis(tt, c, refs["pt"])
	confere(tt, "pt expandido", fmt.Sprint(campos), "map[x:1 y:2]")
	globs, _ := variaveis(tt, c, sc["Globais"])
	confere(tt, "global lista", globs["lista"], "[10, 20, 30]")
	if _, ok := globs["processa"]; !ok {
		tt.Errorf("globais sem processa: %v", globs)
	}
	// topo do programa: escopo de globais vem primeiro
	scTopo := pede(tt, c, "scopes", map[string]any{"frameId": qs[1].ID}).Corpo()["scopes"].([]any)
	confere(tt, "1o escopo do topo", scTopo[0].(map[string]any)["name"].(string), "Globais")

	// stepIn entra no soma; next anda uma linha; stepOut volta pro processa
	pede(tt, c, "stepIn", map[string]any{"threadId": 1})
	espera(tt, c, "stopped", paradaEm("step", 1))
	confere(tt, "depois do stepIn", resumoPilha(pilha(tt, c, 1)), "soma:6 processa:14 <principal>:23")
	pede(tt, c, "next", map[string]any{"threadId": 1})
	espera(tt, c, "stopped", paradaEm("step", 1))
	qs = pilha(tt, c, 1)
	confere(tt, "depois do next", resumoPilha(qs), "soma:7 processa:14 <principal>:23")
	somaVals, _ := variaveis(tt, c, escopos(tt, c, qs[0].ID)["Locais"])
	confere(tt, "r no soma", somaVals["r"], "61")
	pede(tt, c, "stepOut", map[string]any{"threadId": 1})
	espera(tt, c, "stopped", paradaEm("step", 1))
	qs = pilha(tt, c, 1)
	confere(tt, "depois do stepOut", resumoPilha(qs), "processa:15 <principal>:23")

	// evaluate no quadro (repl e hover) e erro
	ev := pede(tt, c, "evaluate", map[string]any{"expression": "s * 2 + pt.y", "frameId": qs[0].ID, "context": "repl"})
	confere(tt, "evaluate", ev.Corpo()["result"].(string), "124")
	ev = pede(tt, c, "evaluate", map[string]any{"expression": "xs", "frameId": qs[0].ID, "context": "hover"})
	if num(ev.Corpo()["variablesReference"]) == 0 {
		tt.Errorf("evaluate de lista sem variablesReference")
	}
	ev = pede(tt, c, "evaluate", map[string]any{"expression": "lista[0] + 1", "frameId": qs[1].ID, "context": "watch"})
	confere(tt, "evaluate no quadro de baixo", ev.Corpo()["result"].(string), "11")
	if r, _ := c.Pede("evaluate", map[string]any{"expression": "nao_existe_isso + 1", "frameId": qs[0].ID}); r["success"] != false {
		tt.Errorf("evaluate com erro deu sucesso: %v", r)
	}

	// continue: imprime 61 e o bora para no breakpoint em outra thread
	pede(tt, c, "continue", map[string]any{"threadId": 1})
	stBora := espera(tt, c, "stopped", paradaEm("breakpoint", 0))
	tb := num(stBora.Corpo()["threadId"])
	if tb == 1 {
		tt.Fatalf("breakpoint do bora parou na thread principal")
	}
	espera(tt, c, "thread", func(m Msg) bool { return m.Corpo()["reason"] == "started" && num(m.Corpo()["threadId"]) == tb })
	qsBora := pilha(tt, c, tb)
	confere(tt, "pilha do bora", resumoPilha(qsBora), "trampo:18")
	boraVals, _ := variaveis(tt, c, escopos(tt, c, qsBora[0].ID)["Locais"])
	confere(tt, "n no bora", boraVals["n"], "21")
	achou := false
	for _, th := range pede(tt, c, "threads", nil).Corpo()["threads"].([]any) {
		achou = achou || num(th.(map[string]any)["id"]) == tb
	}
	if !achou {
		tt.Errorf("thread do bora fora do threads")
	}
	pede(tt, c, "continue", map[string]any{"threadId": tb})

	ex := espera(tt, c, "exited", nil)
	confere(tt, "exitCode", fmt.Sprint(num(ex.Corpo()["exitCode"])), "0")
	espera(tt, c, "terminated", nil)
	confere(tt, "stdout", c.Saida("stdout"), "61\n42\n")
	pede(tt, c, "disconnect", nil)
}
