package depurador

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gambiarrascript/object"
)

// ouvinteTeste junta as paradas num canal.
type ouvinteTeste struct {
	paradas chan Parada
	mu      sync.Mutex
	saida   strings.Builder
}

func (o *ouvinteTeste) Parou(p Parada)     { o.paradas <- p }
func (o *ouvinteTeste) FluxoComecou(int64) {}
func (o *ouvinteTeste) FluxoAcabou(int64)  {}
func (o *ouvinteTeste) Saida(t string) {
	o.mu.Lock()
	o.saida.WriteString(t)
	o.mu.Unlock()
}

// sessaoTeste roda o programa principal (com os modulos) parado na entrada.
type sessaoTeste struct {
	t    *testing.T
	s    *Sessao
	o    *ouvinteTeste
	fim  chan int
	dir  string
	out  *bytes.Buffer
	prog *Programa
}

func novaSessaoTeste(t *testing.T, eng string, arquivos map[string]string) *sessaoTeste {
	t.Helper()
	dir := t.TempDir()
	for nome, fonte := range arquivos {
		if err := os.WriteFile(filepath.Join(dir, nome), []byte(fonte), 0644); err != nil {
			t.Fatal(err)
		}
	}
	st := &sessaoTeste{t: t, o: &ouvinteTeste{paradas: make(chan Parada, 16)}, fim: make(chan int, 1), dir: dir, out: &bytes.Buffer{}}
	prog, err := Prepara(Config{Arquivo: filepath.Join(dir, "main.gs"), Motor: eng, Saida: st.out, Erro: st.out, Entrada: strings.NewReader("")})
	if err != nil {
		t.Fatal(err)
	}
	st.prog = prog
	st.s = NovaSessao(st.o)
	st.s.PararNaEntrada(true)
	go func() { st.fim <- prog.Roda(st.s) }()
	return st
}

// para espera a proxima parada e devolve "arquivo:linha quadros".
func (st *sessaoTeste) espera() Parada {
	st.t.Helper()
	select {
	case p := <-st.o.paradas:
		return p
	case c := <-st.fim:
		st.t.Fatalf("programa acabou (codigo %d) esperando parada; saida %q", c, st.out.String())
	case <-time.After(10 * time.Second):
		st.t.Fatalf("timeout esperando parada")
	}
	return Parada{}
}

// onde descreve a pilha do fluxo parado: "nome@arq:linha ...".
func (st *sessaoTeste) onde(p Parada) string {
	st.t.Helper()
	var qs []object.Quadro
	if err := st.s.Executa(p.Fluxo, func(f object.Fluxo) { qs = f.Quadros() }); err != nil {
		st.t.Fatal(err)
	}
	partes := make([]string, len(qs))
	for k, q := range qs {
		partes[k] = fmt.Sprintf("%s@%s:%d", q.Nome, filepath.Base(q.Arquivo), q.Linha)
	}
	return strings.Join(partes, " ")
}

func (st *sessaoTeste) avalia(p Parada, q int, expr string) string {
	st.t.Helper()
	var r object.Object
	if err := st.s.Executa(p.Fluxo, func(f object.Fluxo) { r = f.Avalia(q, expr) }); err != nil {
		st.t.Fatal(err)
	}
	return r.Inspect()
}

func (st *sessaoTeste) fimCom() int {
	st.t.Helper()
	select {
	case c := <-st.fim:
		return c
	case p := <-st.o.paradas:
		st.t.Fatalf("parou de novo em %s:%d (%s), esperava o fim", filepath.Base(p.Arquivo), p.Linha, p.Motivo)
	case <-time.After(10 * time.Second):
		st.t.Fatalf("timeout esperando o fim")
	}
	return -1
}

func (st *sessaoTeste) ok(err error) {
	st.t.Helper()
	if err != nil {
		st.t.Fatal(err)
	}
}

const progFat = `gambiarra fat(n)
    se_colar n <= 1
        funciona 1
    acabou_finalmente
    bota r = n * fat(n - 1)
    funciona r
acabou_finalmente
bota a = fat(3)
mostra a
`

// proximo passa por cima da recursao (fica no mesmo quadro ou no de quem
// chamou); depois de voltar da gambiarra para no statement seguinte do topo.
func TestPassoProximoRecursao(t *testing.T) {
	for _, eng := range []string{"vm", "tree"} {
		t.Run(eng, func(t *testing.T) {
			st := novaSessaoTeste(t, eng, map[string]string{"main.gs": progFat})
			p := st.espera()
			if p.Motivo != MotivoEntrada || p.Linha != 1 {
				t.Fatalf("entrada: %+v", p)
			}
			bp := st.s.AdicionaBreakpoint(st.prog.Arquivo(), PedidoBP{Linha: 5})
			st.ok(st.s.Continua(1))
			p = st.espera()
			if p.Motivo != MotivoBreakpoint || p.BP != bp.ID {
				t.Fatalf("breakpoint: %+v", p)
			}
			if got := st.onde(p); got != "fat@main.gs:5 <principal>@main.gs:8" {
				t.Errorf("pilha no bp: %s", got)
			}
			st.s.RemoveBreakpoint(bp.ID)
			st.ok(st.s.Proximo(1))
			p = st.espera()
			if got := st.onde(p); got != "fat@main.gs:6 <principal>@main.gs:8" {
				t.Errorf("proximo por cima da recursao: %s", got)
			}
			if r := st.avalia(p, 0, "r"); r != "6" {
				t.Errorf("r = %s", r)
			}
			st.ok(st.s.Proximo(1))
			p = st.espera()
			if got := st.onde(p); got != "<principal>@main.gs:9" {
				t.Errorf("proximo saindo da gambiarra: %s", got)
			}
			st.ok(st.s.Proximo(1))
			if c := st.fimCom(); c != 0 {
				t.Errorf("codigo %d", c)
			}
			if st.out.String() != "6\n" {
				t.Errorf("saida %q", st.out.String())
			}
		})
	}
}

// entra desce em cada chamada; sai volta pro statement seguinte de quem
// chamou (aqui, o resto da conta no quadro de cima).
func TestPassoEntraESai(t *testing.T) {
	for _, eng := range []string{"vm", "tree"} {
		t.Run(eng, func(t *testing.T) {
			st := novaSessaoTeste(t, eng, map[string]string{"main.gs": progFat})
			st.espera() // entrada, linha 1
			st.ok(st.s.Proximo(1))
			p := st.espera()
			if p.Linha != 8 {
				t.Fatalf("queria a linha 8, parou na %d", p.Linha)
			}
			passos := []struct {
				cmd  func(int64) error
				quer string
			}{
				{st.s.Entra, "fat@main.gs:2 <principal>@main.gs:8"},
				{st.s.Entra, "fat@main.gs:5 <principal>@main.gs:8"},
				{st.s.Entra, "fat@main.gs:2 fat@main.gs:5 <principal>@main.gs:8"},
				{st.s.Sai, "fat@main.gs:6 <principal>@main.gs:8"},
				{st.s.Sai, "<principal>@main.gs:9"},
			}
			for k, ps := range passos {
				st.ok(ps.cmd(1))
				p = st.espera()
				if p.Motivo != MotivoPasso {
					t.Fatalf("passo %d: motivo %s", k, p.Motivo)
				}
				if got := st.onde(p); got != ps.quer {
					t.Errorf("passo %d: %s, queria %s", k, got, ps.quer)
				}
			}
			// sai no topo do programa: roda ate o fim
			st.ok(st.s.Sai(1))
			st.fimCom()
		})
	}
}

// Breakpoint num modulo importado: a pilha passa pelo arquivo do modulo e as
// globais do quadro sao as do modulo; o corpo do modulo e o quadro <modulo>.
func TestBreakpointEmModulo(t *testing.T) {
	arqs := map[string]string{
		"main.gs": `importa "util.gs" como u
mostra u.dobro(5)
`,
		"util.gs": `bota fator = 2
gambiarra dobro(x)
    funciona x * fator
acabou_finalmente
`,
	}
	for _, eng := range []string{"vm", "tree"} {
		t.Run(eng, func(t *testing.T) {
			st := novaSessaoTeste(t, eng, arqs)
			st.espera()
			util := filepath.Join(st.dir, "util.gs")
			st.s.AdicionaBreakpoint(util, PedidoBP{Linha: 1})
			st.s.AdicionaBreakpoint(util, PedidoBP{Linha: 3})
			st.ok(st.s.Continua(1))
			p := st.espera()
			if got := st.onde(p); got != "<modulo>@util.gs:1 <principal>@main.gs:1" {
				t.Errorf("corpo do modulo: %s", got)
			}
			st.ok(st.s.Continua(1))
			p = st.espera()
			if got := st.onde(p); got != "dobro@util.gs:3 <principal>@main.gs:2" {
				t.Errorf("gambiarra do modulo: %s", got)
			}
			var globais, locais []object.Variavel
			st.ok(st.s.Executa(p.Fluxo, func(f object.Fluxo) { globais, locais = f.Globais(0), f.Locais(0) }))
			if got := renderVarsT(globais); got != "dobro fator=2" {
				t.Errorf("globais do modulo: %s", got)
			}
			if got := renderVarsT(locais); got != "x=5" {
				t.Errorf("locais: %s", got)
			}
			if r := st.avalia(p, 0, "x * fator + 1"); r != "11" {
				t.Errorf("avalia: %s", r)
			}
			st.ok(st.s.Continua(1))
			st.fimCom()
			if st.out.String() != "10\n" {
				t.Errorf("saida %q", st.out.String())
			}
		})
	}
}

func renderVarsT(vs []object.Variavel) string {
	partes := make([]string, len(vs))
	for k, v := range vs {
		if v.Valor.Type() == object.FUNCAO_OBJ {
			partes[k] = v.Nome
			continue
		}
		partes[k] = v.Nome + "=" + v.Valor.Inspect()
	}
	return strings.Join(partes, " ")
}

// Breakpoint onde nao tem linha executavel da dali pra baixo fica nao
// verificado; linha vazia anda pra proxima executavel.
func TestResolveBreakpoint(t *testing.T) {
	dir := t.TempDir()
	arq := filepath.Join(dir, "x.gs")
	os.WriteFile(arq, []byte("bota a = 1\n\n# comentario\nmostra a\n"), 0644)
	s := NovaSessao(&ouvinteTeste{paradas: make(chan Parada, 1)})
	bps := s.DefineBreakpoints(arq, []PedidoBP{{Linha: 2}, {Linha: 4}, {Linha: 9}})
	got := fmt.Sprintf("%v@%d %v@%d %v@%d", bps[0].Verificado, bps[0].Linha, bps[1].Verificado, bps[1].Linha, bps[2].Verificado, bps[2].Linha)
	if got != "true@4 true@4 false@0" {
		t.Errorf("resolucao: %s", got)
	}
	if bps[2].Msg == "" {
		t.Errorf("nao verificado sem motivo")
	}
	// setBreakpoints troca todos do arquivo
	s.DefineBreakpoints(arq, []PedidoBP{{Linha: 1}})
	if n := len(s.Breakpoints()); n != 1 {
		t.Errorf("sobraram %d breakpoints", n)
	}
	// arquivo que nao existe
	if bp := s.AdicionaBreakpoint(filepath.Join(dir, "nao.gs"), PedidoBP{Linha: 1}); bp.Verificado {
		t.Errorf("breakpoint em arquivo inexistente verificado")
	}
}
