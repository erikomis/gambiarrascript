package depurador

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gambiarrascript/object"
)

// ---- gs debug --dap: Debug Adapter Protocol no stdin/stdout ----
//
// Um processo por sessao (o VSCode sobe `gs debug --dap`). O programa roda
// dentro do proprio adapter; a saida dele vira evento `output`. Cada fluxo
// e uma thread do DAP: parar um nao para os outros (allThreadsStopped
// false). Os ids de quadro e de variavel sao handles que valem enquanto o
// fluxo dono continua parado.

type argsLaunch struct {
	Program     string   `json:"program"`
	Args        []string `json:"args"`
	Cwd         string   `json:"cwd"`
	StopOnEntry bool     `json:"stopOnEntry"`
	Engine      string   `json:"engine"`
	NoDebug     bool     `json:"noDebug"`
}

// handleDAP aponta um quadro, um escopo (locais/globais de um quadro) ou um
// valor composto (pra expandir), sempre de um fluxo.
type handleDAP struct {
	fluxo  int64
	quadro int
	escopo string // "", "locais", "globais"
	valor  object.Object
}

type servidorDAP struct {
	w    *escritorDAP
	sess *Sessao

	launch      *argsLaunch
	prog        *Programa
	configurado bool
	iniciado    bool

	hmu     sync.Mutex
	handles map[int]handleDAP
	prox    int
}

// RodaDAP atende uma sessao DAP ate o disconnect (ou o cliente fechar) e
// devolve o codigo de saida do adapter.
func RodaDAP(in io.Reader, out io.Writer) int {
	s := &servidorDAP{w: &escritorDAP{w: out}, handles: map[int]handleDAP{}}
	s.sess = NovaSessao(s)
	l := novoLeitorDAP(in)
	for {
		corpo, err := l.le()
		if err != nil {
			return 0
		}
		var req msgDAP
		if err := json.Unmarshal(corpo, &req); err != nil || req.Type != "request" {
			continue
		}
		if fim := s.atende(&req); fim {
			return 0
		}
	}
}

// atende responde um request; true = acabou a sessao.
func (s *servidorDAP) atende(req *msgDAP) bool {
	switch req.Command {
	case "initialize":
		s.w.resposta(req, map[string]any{
			"supportsConfigurationDoneRequest": true,
			"supportsConditionalBreakpoints":   true,
			"supportsLogPoints":                true,
			"supportsEvaluateForHovers":        true,
			"supportsTerminateRequest":         true,
		})
		s.w.evento("initialized", nil)
	case "launch":
		s.cmdLaunch(req)
	case "setBreakpoints":
		s.cmdSetBreakpoints(req)
	case "setExceptionBreakpoints", "setFunctionBreakpoints":
		s.w.resposta(req, map[string]any{"breakpoints": []any{}})
	case "configurationDone":
		s.w.resposta(req, nil)
		s.configurado = true
		s.inicia()
	case "threads":
		s.cmdThreads(req)
	case "stackTrace":
		s.cmdStackTrace(req)
	case "scopes":
		s.cmdScopes(req)
	case "variables":
		s.cmdVariables(req)
	case "evaluate":
		s.cmdEvaluate(req)
	case "continue", "next", "stepIn", "stepOut":
		s.cmdSegue(req)
	case "pause":
		var a struct {
			ThreadID int64 `json:"threadId"`
		}
		json.Unmarshal(req.Arguments, &a)
		s.sess.Pausa(a.ThreadID)
		s.w.resposta(req, nil)
	case "terminate":
		s.w.resposta(req, nil)
		s.w.evento("terminated", nil)
		return true
	case "disconnect":
		s.w.resposta(req, nil)
		return true
	default:
		s.w.falha(req, "o gs debug nao sabe fazer `"+req.Command+"`")
	}
	return false
}

// ---- launch / configuracao ----

func (s *servidorDAP) cmdLaunch(req *msgDAP) {
	var a argsLaunch
	if err := json.Unmarshal(req.Arguments, &a); err != nil {
		s.w.falha(req, "argumentos do launch invalidos: "+err.Error())
		return
	}
	if a.Program == "" {
		s.w.falha(req, "faltou o `program` (o .gs pra depurar) no launch.json")
		return
	}
	if a.Cwd != "" {
		if err := os.Chdir(a.Cwd); err != nil {
			s.w.falha(req, "nao consegui entrar em "+a.Cwd+": "+err.Error())
			return
		}
	}
	prog, err := Prepara(Config{
		Arquivo: a.Program,
		Args:    a.Args,
		Motor:   a.Engine,
		Saida:   &saidaDAP{w: s.w, cat: "stdout"},
		Erro:    &saidaDAP{w: s.w, cat: "stderr"},
		// o stdin do adapter e o canal do DAP: o programa le vazio
		Entrada: strings.NewReader(""),
	})
	if err != nil {
		s.w.falha(req, err.Error())
		return
	}
	s.launch, s.prog = &a, prog
	s.w.resposta(req, nil)
	s.inicia()
}

// inicia roda o programa quando o launch e o configurationDone chegaram.
func (s *servidorDAP) inicia() {
	if s.iniciado || s.prog == nil || !s.configurado {
		return
	}
	s.iniciado = true
	s.sess.PararNaEntrada(s.launch.StopOnEntry)
	if s.launch.NoDebug {
		s.sess.Desliga()
	}
	go func() {
		codigo := s.prog.Roda(s.sess)
		s.w.evento("exited", map[string]any{"exitCode": codigo})
		s.w.evento("terminated", nil)
	}()
}

func (s *servidorDAP) cmdSetBreakpoints(req *msgDAP) {
	var a struct {
		Source struct {
			Path string `json:"path"`
		} `json:"source"`
		Breakpoints []struct {
			Line         int    `json:"line"`
			Condition    string `json:"condition"`
			LogMessage   string `json:"logMessage"`
			HitCondition string `json:"hitCondition"`
		} `json:"breakpoints"`
	}
	if err := json.Unmarshal(req.Arguments, &a); err != nil {
		s.w.falha(req, err.Error())
		return
	}
	pedidos := make([]PedidoBP, len(a.Breakpoints))
	for k, b := range a.Breakpoints {
		pedidos[k] = PedidoBP{Linha: b.Line, Condicao: b.Condition, Log: b.LogMessage}
	}
	bps := s.sess.DefineBreakpoints(a.Source.Path, pedidos)
	out := make([]map[string]any, len(bps))
	for k, bp := range bps {
		m := map[string]any{"id": bp.ID, "verified": bp.Verificado, "source": fonteDAP(bp.Arquivo)}
		if bp.Verificado {
			m["line"] = bp.Linha
		} else {
			m["line"] = bp.Pedida
			m["message"] = bp.Msg
		}
		out[k] = m
	}
	s.w.resposta(req, map[string]any{"breakpoints": out})
}

func fonteDAP(caminho string) map[string]any {
	return map[string]any{"name": filepath.Base(caminho), "path": caminho}
}

// ---- threads e pilha ----

func (s *servidorDAP) cmdThreads(req *msgDAP) {
	fluxos := s.sess.Fluxos()
	ths := make([]map[string]any, 0, len(fluxos)+1)
	tem1 := false
	for _, f := range fluxos {
		tem1 = tem1 || f.ID == 1
		ths = append(ths, map[string]any{"id": f.ID, "name": f.Nome})
	}
	if !tem1 {
		// antes de rodar (ou depois do principal acabar) o VSCode ainda quer
		// ver a thread principal
		ths = append([]map[string]any{{"id": 1, "name": NomeFluxo(1)}}, ths...)
	}
	s.w.resposta(req, map[string]any{"threads": ths})
}

func (s *servidorDAP) novoHandle(h handleDAP) int {
	s.hmu.Lock()
	defer s.hmu.Unlock()
	s.prox++
	s.handles[s.prox] = h
	return s.prox
}

func (s *servidorDAP) handle(id int) (handleDAP, bool) {
	s.hmu.Lock()
	defer s.hmu.Unlock()
	h, ok := s.handles[id]
	return h, ok
}

// esquece invalida os handles do fluxo (ele voltou a rodar ou acabou).
func (s *servidorDAP) esquece(fluxo int64) {
	s.hmu.Lock()
	defer s.hmu.Unlock()
	for id, h := range s.handles {
		if h.fluxo == fluxo {
			delete(s.handles, id)
		}
	}
}

func (s *servidorDAP) cmdStackTrace(req *msgDAP) {
	var a struct {
		ThreadID   int64 `json:"threadId"`
		StartFrame int   `json:"startFrame"`
		Levels     int   `json:"levels"`
	}
	json.Unmarshal(req.Arguments, &a)
	var qs []object.Quadro
	if err := s.sess.Executa(a.ThreadID, func(f object.Fluxo) { qs = f.Quadros() }); err != nil {
		s.w.falha(req, err.Error())
		return
	}
	total := len(qs)
	fim := total
	if a.Levels > 0 && a.StartFrame+a.Levels < fim {
		fim = a.StartFrame + a.Levels
	}
	frames := []map[string]any{}
	for k := a.StartFrame; k < fim; k++ {
		q := qs[k]
		id := s.novoHandle(handleDAP{fluxo: a.ThreadID, quadro: k})
		fr := map[string]any{"id": id, "name": q.Nome, "line": q.Linha, "column": 1}
		if q.Arquivo != "" {
			fr["source"] = fonteDAP(q.Arquivo)
		}
		frames = append(frames, fr)
	}
	s.w.resposta(req, map[string]any{"stackFrames": frames, "totalFrames": total})
}

func (s *servidorDAP) cmdScopes(req *msgDAP) {
	var a struct {
		FrameID int `json:"frameId"`
	}
	json.Unmarshal(req.Arguments, &a)
	h, ok := s.handle(a.FrameID)
	if !ok {
		s.w.falha(req, "quadro nao existe mais (o fluxo seguiu)")
		return
	}
	var nLocais int
	if err := s.sess.Executa(h.fluxo, func(f object.Fluxo) { nLocais = len(f.Locais(h.quadro)) }); err != nil {
		s.w.falha(req, err.Error())
		return
	}
	locais := map[string]any{"name": "Locais", "presentationHint": "locals", "expensive": false,
		"variablesReference": s.novoHandle(handleDAP{fluxo: h.fluxo, quadro: h.quadro, escopo: "locais"})}
	globais := map[string]any{"name": "Globais", "expensive": false,
		"variablesReference": s.novoHandle(handleDAP{fluxo: h.fluxo, quadro: h.quadro, escopo: "globais"})}
	scopes := []any{locais, globais}
	if nLocais == 0 {
		// topo do programa/modulo: o que importa sao as globais (o VSCode
		// abre o primeiro escopo)
		scopes = []any{globais, locais}
	}
	s.w.resposta(req, map[string]any{"scopes": scopes})
}

// variavelDAP monta uma variavel (com handle pra expandir se for composta).
func (s *servidorDAP) variavelDAP(fluxo int64, nome string, v object.Object) map[string]any {
	ref := 0
	if TemFilhos(v) {
		ref = s.novoHandle(handleDAP{fluxo: fluxo, valor: v})
	}
	return map[string]any{"name": nome, "value": Mostra(v), "type": Tipo(v), "variablesReference": ref}
}

func (s *servidorDAP) cmdVariables(req *msgDAP) {
	var a struct {
		Ref int `json:"variablesReference"`
	}
	json.Unmarshal(req.Arguments, &a)
	h, ok := s.handle(a.Ref)
	if !ok {
		s.w.falha(req, "variavel nao existe mais (o fluxo seguiu)")
		return
	}
	var vs []object.Variavel
	switch h.escopo {
	case "locais", "globais":
		if err := s.sess.Executa(h.fluxo, func(f object.Fluxo) {
			if h.escopo == "locais" {
				vs = f.Locais(h.quadro)
			} else {
				vs = f.Globais(h.quadro)
			}
		}); err != nil {
			s.w.falha(req, err.Error())
			return
		}
	default:
		vs = Filhos(h.valor)
	}
	out := make([]map[string]any, len(vs))
	for k, v := range vs {
		out[k] = s.variavelDAP(h.fluxo, v.Nome, v.Valor)
	}
	s.w.resposta(req, map[string]any{"variables": out})
}

func (s *servidorDAP) cmdEvaluate(req *msgDAP) {
	var a struct {
		Expression string `json:"expression"`
		FrameID    int    `json:"frameId"`
		Context    string `json:"context"`
	}
	json.Unmarshal(req.Arguments, &a)
	var h handleDAP
	if a.FrameID != 0 {
		var ok bool
		if h, ok = s.handle(a.FrameID); !ok {
			s.w.falha(req, "quadro nao existe mais (o fluxo seguiu)")
			return
		}
	} else {
		// sem quadro (console sem nada selecionado): o primeiro fluxo parado
		achou := false
		for _, f := range s.sess.Fluxos() {
			if f.Parado {
				h, achou = handleDAP{fluxo: f.ID}, true
				break
			}
		}
		if !achou {
			s.w.falha(req, "pausa o programa pra avaliar")
			return
		}
	}
	var res object.Object
	if err := s.sess.Executa(h.fluxo, func(f object.Fluxo) { res = f.Avalia(h.quadro, a.Expression) }); err != nil {
		s.w.falha(req, err.Error())
		return
	}
	if object.EhErroLevantado(res) {
		s.w.falha(req, res.Inspect())
		return
	}
	v := s.variavelDAP(h.fluxo, "", res)
	s.w.resposta(req, map[string]any{"result": v["value"], "type": v["type"], "variablesReference": v["variablesReference"]})
}

func (s *servidorDAP) cmdSegue(req *msgDAP) {
	var a struct {
		ThreadID int64 `json:"threadId"`
	}
	json.Unmarshal(req.Arguments, &a)
	if !s.sess.EstaParado(a.ThreadID) {
		s.w.falha(req, ErrNaoParado.Error())
		return
	}
	s.esquece(a.ThreadID)
	// responde ANTES de soltar: o stopped seguinte nao pode chegar antes da
	// resposta (o cliente marcaria a thread como rodando depois de parada)
	if req.Command == "continue" {
		s.w.resposta(req, map[string]any{"allThreadsContinued": false})
	} else {
		s.w.resposta(req, nil)
	}
	var err error
	switch req.Command {
	case "continue":
		err = s.sess.Continua(a.ThreadID)
	case "next":
		err = s.sess.Proximo(a.ThreadID)
	case "stepIn":
		err = s.sess.Entra(a.ThreadID)
	case "stepOut":
		err = s.sess.Sai(a.ThreadID)
	}
	if err != nil {
		s.Saida("depurador: " + err.Error() + "\n")
	}
}

// ---- Ouvinte: eventos da sessao ----

func (s *servidorDAP) Parou(p Parada) {
	body := map[string]any{"reason": p.Motivo, "threadId": p.Fluxo, "allThreadsStopped": false}
	if p.BP != 0 {
		body["hitBreakpointIds"] = []int{p.BP}
	}
	if p.Detalhe != "" {
		body["text"] = p.Detalhe
		body["description"] = p.Detalhe
	}
	s.w.evento("stopped", body)
}

func (s *servidorDAP) FluxoComecou(id int64) {
	s.w.evento("thread", map[string]any{"reason": "started", "threadId": id})
}

func (s *servidorDAP) FluxoAcabou(id int64) {
	s.esquece(id)
	s.w.evento("thread", map[string]any{"reason": "exited", "threadId": id})
}

func (s *servidorDAP) Saida(texto string) {
	s.w.evento("output", map[string]any{"category": "console", "output": texto})
}

// saidaDAP manda o que o programa escreve como evento output.
type saidaDAP struct {
	w   *escritorDAP
	cat string
}

func (o *saidaDAP) Write(p []byte) (int, error) {
	o.w.evento("output", map[string]any{"category": o.cat, "output": string(p)})
	return len(p), nil
}
