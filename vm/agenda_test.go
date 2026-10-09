package vm

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// saidaTravada: as tarefas agendadas escrevem de outra goroutine.
type saidaTravada struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *saidaTravada) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *saidaTravada) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

type resultadoAgenda struct {
	saida, erros, erro string
	sai                *object.Sair
}

// rodaComAgenda roda a fonte no motor e depois espera os agendamentos, igual
// o `gs roda` faz.
func rodaComAgenda(t *testing.T, motor, fonte string) resultadoAgenda {
	t.Helper()
	p := parser.New(lexer.New(fonte))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	out, errOut := &saidaTravada{}, &saidaTravada{}
	interp := interpreter.New(out)
	interp.DefinirStderr(errOut)
	var r resultadoAgenda
	if motor == "vm" {
		comp := compiler.New()
		if err := comp.Compile(prog); err != nil {
			t.Fatalf("compile: %v", err)
		}
		if err := NovaComInterp(comp.Bytecode(), out, interp).Run(); err != nil {
			r.erro = err.Error()
		}
	} else if res := interp.Eval(prog, object.NewEnvironment()); object.EhErroLevantado(res) {
		r.erro = res.Inspect()
	}
	r.sai = interp.EsperaAgendamentos()
	r.saida, r.erros = out.String(), errOut.String()
	return r
}

func TestAgendaACadaParidade(t *testing.T) {
	fonte := `
bota c = {"n": 0}
bota h = a_cada(0.02, gambiarra()
    bota c["n"] = c["n"] + 1
    mostra "tique " + c["n"]
    se_colar c["n"] == 2
        quebra("falhou de proposito")
    acabou_finalmente
    se_colar c["n"] == 3
        mostra cancela(h)
        mostra cancela(h)
    acabou_finalmente
acabou_finalmente)
mostra h
`
	for _, motor := range []string{"tree", "vm"} {
		r := rodaComAgenda(t, motor, fonte)
		quer := "<nativo: agendamento #1 (a_cada 0.02s)>\ntique 1\ntique 2\ntique 3\ndeu_bom\ndeu_ruim\n"
		if r.erro != "" || r.saida != quer {
			t.Errorf("%s: saida %q (erro %q), queria %q", motor, r.saida, r.erro, quer)
		}
		if !strings.Contains(r.erros, "a tarefa #1 (a_cada 0.02s) deu ruim") || !strings.Contains(r.erros, "falhou de proposito") {
			t.Errorf("%s: erro da tarefa nao foi pro stderr: %q", motor, r.erros)
		}
		if r.sai != nil {
			t.Errorf("%s: sai inesperado", motor)
		}
	}
}

func TestAgendaDepoisDeECancelaParidade(t *testing.T) {
	fonte := `
depois_de(0.1, gambiarra() mostra "b" acabou_finalmente)
depois_de(0.02, gambiarra() mostra "a" acabou_finalmente)
bota nunca = depois_de(0.03, gambiarra() mostra "nunca" acabou_finalmente)
mostra cancela(nunca)
mostra cancela(nunca)
bota cron = agenda("@minuto", gambiarra() mostra "minuto" acabou_finalmente)
mostra cancela(cron)
arruma
    agenda("0 0 30 2 *", gambiarra() mostra "x" acabou_finalmente)
quebrou err
    mostra erro_msg(err)
acabou_finalmente
arruma
    agenda("*/0 * * * *", gambiarra() mostra "x" acabou_finalmente)
quebrou err
    mostra erro_msg(err)
acabou_finalmente
arruma
    a_cada(0, gambiarra() mostra "x" acabou_finalmente)
quebrou err
    mostra erro_msg(err)
acabou_finalmente
arruma
    cancela(42)
quebrou err
    mostra contem(erro_msg(err), "agendamento")
acabou_finalmente
mostra "inicio"
`
	quer := "deu_bom\ndeu_ruim\ndeu_bom\n" +
		"deu ruim: agenda: \"0 0 30 2 *\" nunca acontece (confere dia e mes)\n" +
		"deu ruim: agenda: cron: passo \"0\" invalido no campo minuto\n" +
		"deu ruim: a_cada: os segundos tem que ser maior que zero, veio 0\n" +
		"deu_bom\ninicio\na\nb\n"
	for _, motor := range []string{"tree", "vm"} {
		r := rodaComAgenda(t, motor, fonte)
		if r.erro != "" || r.saida != quer {
			t.Errorf("%s: saida %q (erro %q), queria %q", motor, r.saida, r.erro, quer)
		}
	}
}

func TestAgendaSaiDeDentroDaTarefa(t *testing.T) {
	fonte := `
a_cada(0.01, gambiarra() mostra "rodando" acabou_finalmente)
depois_de(0.05, gambiarra() sai(3) acabou_finalmente)
`
	for _, motor := range []string{"tree", "vm"} {
		r := rodaComAgenda(t, motor, fonte)
		if r.sai == nil || r.sai.Codigo != 3 {
			t.Errorf("%s: queria sai(3), veio %v (saida %q, erro %q)", motor, r.sai, r.saida, r.erro)
		}
		if !strings.Contains(r.saida, "rodando") {
			t.Errorf("%s: o a_cada nem rodou: %q", motor, r.saida)
		}
	}
}

func TestFormataLeDataParidade(t *testing.T) {
	casos := []string{
		`formata_data("2026-10-08T14:05:09Z", "dd/mm/aaaa hh:mi:ss")`,
		`formata_data("2026-10-08T14:05:09Z", "dddd, dd 'de' mmmm 'de' aaaa")`,
		`formata_data("2026-03-01T03:00:00Z", "ddd dd/mmm/aa 'as' hh'h'", "America/Sao_Paulo")`,
		`formata_data(0, "aaaa-mm-dd hh:mi:ss ''UTC''")`,
		`le_data("08/10/2026", "dd/mm/aaaa")`,
		`le_data("8/3/26 7:05", "dd/mm/aa hh:mi")`,
		`le_data("08 de MARCO de 2026", "dd 'de' mmmm 'de' aaaa")`,
		`le_data("sábado, 10/out/2026 22:30:15", "dddd, dd/mmm/aaaa hh:mi:ss", "America/Sao_Paulo")`,
		`le_data("31/02/2026", "dd/mm/aaaa")`,
		`le_data("08/10/2026 lixo", "dd/mm/aaaa")`,
		`formata_data(le_data("25/12/2024", "dd/mm/aaaa"), "dddd")`,
		`formata_tempo("02/01/2006", "2024-12-25T10:00:00Z")`,
	}
	quer := []string{
		"08/10/2026 14:05:09",
		"quinta-feira, 08 de outubro de 2026",
		"dom 01/mar/26 as 00h",
		"1970-01-01 00:00:00 'UTC'",
		"2026-10-08T00:00:00Z",
		"2026-03-08T07:05:00Z",
		"2026-03-08T00:00:00Z",
		"2026-10-10T22:30:15-03:00",
		"", "",
		"quarta-feira",
		"25/12/2024",
	}
	for k, c := range casos {
		comparaEngines(t, "mostra "+c)
		_, saida, erro := rodaTWComp(t, "mostra "+c)
		if quer[k] == "" {
			if erro == "" {
				t.Errorf("%s: queria erro, veio %q", c, saida)
			}
			continue
		}
		if strings.TrimSpace(saida) != quer[k] || erro != "" {
			t.Errorf("%s: veio %q (erro %q), queria %q", c, saida, erro, quer[k])
		}
	}
}
