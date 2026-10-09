//go:build !js

// Tarefas agendadas:
//
//	a_cada(segundos, gambiarra)   → handle; roda de tantos em tantos segundos
//	depois_de(segundos, gambiarra) → handle; roda uma vez so
//	agenda("cron", gambiarra)      → handle; cron de 5 campos (ver cron.go)
//	cancela(handle)                → verdadeiro se ainda tava ativo
//
// Cada agendamento roda na sua propria goroutine (a gambiarra nao recebe
// argumento). Erro dentro da tarefa vai pro stderr e o agendamento continua.
// Duas execucoes do MESMO agendamento nunca se sobrepoem: se uma demora mais
// que o intervalo, os disparos perdidos sao pulados.
//
// Vida do processo: quando o script principal termina, o `gs roda` espera
// enquanto houver agendamento ativo (EsperaAgendamentos). Ctrl+C (ou SIGTERM)
// cancela tudo, espera as tarefas que estao rodando terminarem e sai. Com
// `escuta` o servidor ja segura o processo; o ctrl+c que derruba o servidor
// cancela os agendamentos tambem. No navegador (wasm) quem responde e o stub
// de builtins_agenda_js.go.

package interpreter

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gambiarrascript/object"
)

// prazoParando e quanto o ctrl+c espera as tarefas em andamento.
const prazoParando = 10 * time.Second

type tarefaAgendada struct {
	id        int64
	rotulo    string
	cancelada chan struct{}
	umaVez    sync.Once
	// proxima devolve o proximo disparo depois de `agora` (zero = acabou)
	proxima func(agora time.Time) time.Time
	unica   bool
	fn      object.Object
}

func (t *tarefaAgendada) cancela() { t.umaVez.Do(func() { close(t.cancelada) }) }

type agendador struct {
	mu     sync.Mutex
	ativos map[int64]*tarefaAgendada
	prox   int64
	mudou  chan struct{} // cutucado (sem bloquear) a cada tarefa que sai
	sai    *object.Sair  // sai(codigo) de dentro de uma tarefa
}

func novoAgendador() *agendador {
	return &agendador{ativos: map[int64]*tarefaAgendada{}, mudou: make(chan struct{}, 1)}
}

func (i *Interpreter) pegaAgendador() *agendador {
	i.muAgenda.Lock()
	defer i.muAgenda.Unlock()
	if i.agenda == nil {
		i.agenda = novoAgendador()
	}
	return i.agenda
}

func (i *Interpreter) agendaTarefa(t *tarefaAgendada) object.Object {
	if !ehChamavel(t.fn) {
		return erroBuiltin("%s: o 2o argumento tem que ser uma gambiarra, veio %s", t.rotulo, object.NomeTipo(t.fn))
	}
	a := i.pegaAgendador()
	a.mu.Lock()
	a.prox++
	t.id = a.prox
	t.rotulo = fmt.Sprintf("#%d %s", t.id, t.rotulo)
	t.cancelada = make(chan struct{})
	a.ativos[t.id] = t
	a.mu.Unlock()

	// liga o modo concorrente ANTES do `go` (ver object/concorrencia.go)
	object.AtivaConcorrencia()
	go i.rodaTarefa(a, t)
	return &object.Nativo{Rotulo: "agendamento " + t.rotulo, Valor: t}
}

func (i *Interpreter) rodaTarefa(a *agendador, t *tarefaAgendada) {
	defer func() {
		a.mu.Lock()
		delete(a.ativos, t.id)
		a.mu.Unlock()
		select {
		case a.mudou <- struct{}{}:
		default:
		}
	}()
	for {
		alvo := t.proxima(time.Now())
		if alvo.IsZero() {
			return
		}
		timer := time.NewTimer(time.Until(alvo))
		select {
		case <-t.cancelada:
			timer.Stop()
			return
		case <-timer.C:
		}
		select { // cancelado bem na hora do disparo: nao roda
		case <-t.cancelada:
			return
		default:
		}
		if !i.executaTarefa(a, t) || t.unica {
			return
		}
	}
}

// executaTarefa roda a gambiarra uma vez. false = parar tudo (sai()).
func (i *Interpreter) executaTarefa(a *agendador, t *tarefaAgendada) (segue bool) {
	defer func() {
		if r := recover(); r != nil {
			i.logaErro("agenda: a tarefa %s deu ruim: %v\n", t.rotulo, r)
			segue = true
		}
	}()
	res := i.applyFunction(t.fn, nil, 0, "<"+t.rotulo+">")
	if s, ok := res.(*object.Sair); ok {
		a.mu.Lock()
		if a.sai == nil {
			a.sai = s
		}
		for _, outra := range a.ativos {
			outra.cancela()
		}
		a.mu.Unlock()
		return false
	}
	if object.EhErroLevantado(res) {
		i.logaErro("agenda: a tarefa %s deu ruim: %s\n", t.rotulo, res.Inspect())
	}
	return true
}

func segundosAgenda(nome string, o object.Object) (time.Duration, *object.Erro) {
	n, ok := o.(*object.Numero)
	if !ok {
		return 0, erroBuiltin("%s: o 1o argumento (segundos) tem que ser numero, veio %s", nome, object.NomeTipo(o))
	}
	seg := n.Value
	if n.EhInt {
		seg = float64(n.Int)
	}
	d := time.Duration(seg * float64(time.Second))
	if d <= 0 {
		return 0, erroBuiltin("%s: os segundos tem que ser maior que zero, veio %s", nome, n.Inspect())
	}
	return d, nil
}

func (i *Interpreter) builtinACada(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("a_cada() quer 2 argumentos (segundos, gambiarra), veio %d", len(args))
	}
	d, e := segundosAgenda("a_cada", args[0])
	if e != nil {
		return e
	}
	// alvo fixo (inicio + n*intervalo): nao escorrega com o tempo de execucao
	alvo := time.Now()
	return i.agendaTarefa(&tarefaAgendada{
		rotulo: "(a_cada " + args[0].Inspect() + "s)",
		fn:     args[1],
		proxima: func(agora time.Time) time.Time {
			alvo = alvo.Add(d)
			if !alvo.After(agora) { // atrasou: pula os disparos perdidos
				alvo = agora.Add(d - agora.Sub(alvo)%d)
			}
			return alvo
		},
	})
}

func (i *Interpreter) builtinDepoisDe(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("depois_de() quer 2 argumentos (segundos, gambiarra), veio %d", len(args))
	}
	d, e := segundosAgenda("depois_de", args[0])
	if e != nil {
		return e
	}
	alvo := time.Now().Add(d)
	return i.agendaTarefa(&tarefaAgendada{
		rotulo:  "(depois_de " + args[0].Inspect() + "s)",
		fn:      args[1],
		unica:   true,
		proxima: func(time.Time) time.Time { return alvo },
	})
}

func (i *Interpreter) builtinAgenda(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("agenda() quer 2 argumentos (\"cron\", gambiarra), veio %d", len(args))
	}
	txt, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("agenda: o 1o argumento tem que ser texto (cron tipo \"*/5 * * * *\"), veio %s", object.NomeTipo(args[0]))
	}
	expr, err := parseCron(txt.Value)
	if err != nil {
		return erroBuiltin("agenda: %v", err)
	}
	if expr.proximo(time.Now()).IsZero() {
		return erroBuiltin("agenda: %q nunca acontece (confere dia e mes)", txt.Value)
	}
	return i.agendaTarefa(&tarefaAgendada{
		rotulo:  fmt.Sprintf("(agenda %q)", txt.Value),
		fn:      args[1],
		proxima: func(agora time.Time) time.Time { return expr.proximo(agora.Local()) },
	})
}

func (i *Interpreter) builtinCancela(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("cancela() quer 1 argumento (o que a_cada/depois_de/agenda devolveu), veio %d", len(args))
	}
	n, ok := args[0].(*object.Nativo)
	var t *tarefaAgendada
	if ok {
		t, ok = n.Valor.(*tarefaAgendada)
	}
	if !ok {
		return erroBuiltin("cancela() espera um agendamento (o que a_cada/depois_de/agenda devolveu), veio %s", object.NomeTipo(args[0]))
	}
	a := i.pegaAgendador()
	a.mu.Lock()
	_, ativo := a.ativos[t.id]
	a.mu.Unlock()
	select {
	case <-t.cancelada:
		ativo = false
	default:
	}
	t.cancela()
	if ativo {
		return DEU_BOM
	}
	return DEU_RUIM
}

// AgendamentosAtivos conta quantos agendamentos ainda estao de pe.
func (i *Interpreter) AgendamentosAtivos() int {
	a := i.pegaAgendador()
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.ativos)
}

// CancelaAgendamentos cancela tudo (sem esperar quem esta rodando).
func (i *Interpreter) CancelaAgendamentos() {
	a := i.pegaAgendador()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, t := range a.ativos {
		t.cancela()
	}
}

// EsperaAgendamentos segura o processo enquanto houver agendamento ativo.
// Chamado pelo `gs roda` depois que o script principal termina. Ctrl+C ou
// SIGTERM cancela tudo e espera (ate prazoParando) as tarefas em andamento.
// Devolve o sai(codigo) que alguma tarefa pediu, se pediu.
func (i *Interpreter) EsperaAgendamentos() *object.Sair {
	a := i.pegaAgendador()
	if i.AgendamentosAtivos() == 0 {
		return a.saida()
	}
	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sinais)
	i.logaErro("agendamentos rodando (ctrl+c pra parar)\n")
	for i.AgendamentosAtivos() > 0 {
		select {
		case <-a.mudou:
		case <-sinais:
			// segundo ctrl+c volta a matar na hora
			signal.Stop(sinais)
			i.logaErro("parando os agendamentos (esperando quem ta rodando terminar)...\n")
			i.CancelaAgendamentos()
			prazo := time.After(prazoParando)
			for i.AgendamentosAtivos() > 0 {
				select {
				case <-a.mudou:
				case <-prazo:
					i.logaErro("nao deu tempo de todo mundo terminar; derrubei o resto\n")
					return a.saida()
				}
			}
			i.logaErro("agendamentos parados, falou!\n")
			return a.saida()
		}
	}
	return a.saida()
}

func (a *agendador) saida() *object.Sair {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sai
}
