package vm

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// gravador e um Depurador de teste: em cada linha pedida tira um retrato do
// fluxo (pilha, locais e globais de cada quadro, e as expressoes avaliadas no
// quadro 0). Reentrada (gancho disparado pela propria avaliacao) e ignorada.
type gravador struct {
	mu        sync.Mutex
	linhas    map[int]bool
	exprs     []string
	retratos  []string
	acabaram  []int64
	avaliando map[int64]bool
}

func novoGravador(exprs []string, linhas ...int) *gravador {
	g := &gravador{linhas: map[int]bool{}, exprs: exprs, avaliando: map[int64]bool{}}
	for _, l := range linhas {
		g.linhas[l] = true
	}
	return g
}

func renderVars(vs []object.Variavel) string {
	partes := make([]string, 0, len(vs))
	for _, v := range vs {
		if v.Valor.Type() == object.FUNCAO_OBJ {
			// gambiarra tem Inspect diferente nos dois engines
			partes = append(partes, v.Nome+"=<gambiarra>")
			continue
		}
		partes = append(partes, v.Nome+"="+v.Valor.Inspect())
	}
	return strings.Join(partes, " ")
}

func (g *gravador) Linha(s *object.SitioLinha, f object.Fluxo) {
	if !g.linhas[s.Linha] {
		return
	}
	g.mu.Lock()
	if g.avaliando[f.ID()] {
		g.mu.Unlock()
		return
	}
	g.avaliando[f.ID()] = true
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		g.avaliando[f.ID()] = false
		g.mu.Unlock()
	}()

	var b strings.Builder
	fmt.Fprintf(&b, "linha %d fluxo %d prof %d\n", s.Linha, f.ID(), f.Profundidade())
	for k, q := range f.Quadros() {
		fmt.Fprintf(&b, "  #%d %s:%d | locais: %s | globais: %s\n", k, q.Nome, q.Linha,
			renderVars(f.Locais(k)), renderVars(f.Globais(k)))
	}
	for _, e := range g.exprs {
		fmt.Fprintf(&b, "  ve %s => %s\n", e, f.Avalia(0, e).Inspect())
	}
	g.mu.Lock()
	g.retratos = append(g.retratos, b.String())
	g.mu.Unlock()
}

func (g *gravador) FluxoAcabou(id int64) {
	g.mu.Lock()
	g.acabaram = append(g.acabaram, id)
	g.mu.Unlock()
}

func (g *gravador) texto() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return strings.Join(g.retratos, "")
}

func rodaDepTree(t *testing.T, src string, d object.Depurador) string {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse: %v", errs)
	}
	var out bytes.Buffer
	i := interpreter.New(&out)
	i.DefinirDepurador(d)
	res := i.Eval(prog, object.NewEnvironment())
	if object.EhErroLevantado(res) {
		t.Fatalf("tree: %s", res.Inspect())
	}
	return out.String()
}

func rodaDepVM(t *testing.T, src string, d object.Depurador) string {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse: %v", errs)
	}
	c := compiler.New()
	c.Instrumentar = true
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	var out bytes.Buffer
	m := New(c.Bytecode(), &out)
	m.DefinirDepurador(d)
	if err := m.Run(); err != nil {
		t.Fatalf("vm: %v", err)
	}
	return out.String()
}

const progDepura = `bota xs = [1, 2, 3]
treta Ponto
    x
    y
acabou_finalmente
bota p = Ponto{1, 2}
gambiarra soma(a, b)
    bota r = a + b
    funciona r
acabou_finalmente
gambiarra dobra(n)
    bota t = soma(n, n)
    funciona t
acabou_finalmente
mostra dobra(4)
mostra mapeia([5], dobra)
gambiarra fecha(k)
    bota base = 10
    funciona gambiarra(v)
        bota w = v + base + k
        funciona w
    acabou_finalmente
acabou_finalmente
bota f = fecha(100)
mostra f(1)
`

// A pilha, os locais, as globais e a avaliacao batem entre os engines — com
// chamada aninhada, gambiarra chamada por builtin (mapeia: sub-VM no mesmo
// fluxo), closure (local capturado) e o topo do programa.
func TestDepuradorInspecaoParidade(t *testing.T) {
	exprs := []string{"xs[1] + 1", "p.y", "tamanho(xs)"}
	gt := novoGravador(exprs, 8, 15, 21)
	gv := novoGravador(exprs, 8, 15, 21)
	outT := rodaDepTree(t, progDepura, gt)
	outV := rodaDepVM(t, progDepura, gv)
	if outT != outV {
		t.Fatalf("saida diferente:\ntree: %q\nvm:   %q", outT, outV)
	}
	rt, rv := gt.texto(), gv.texto()
	if rt != rv {
		t.Fatalf("retratos diferentes:\n--- tree\n%s--- vm\n%s", rt, rv)
	}
	// o que se espera ver (amostra)
	for _, quer := range []string{
		"linha 8 fluxo 1 prof 3\n  #0 soma:8 | locais: a=4 b=4 | globais: Ponto=<treta Ponto> dobra=<gambiarra> p=Ponto{x: 1, y: 2} soma=<gambiarra> xs=[1, 2, 3]\n  #1 dobra:12 | locais: n=4 |",
		"  #2 <principal>:15 | locais:  |",
		"  ve xs[1] + 1 => 3\n  ve p.y => 2\n  ve tamanho(xs) => 3\n",
		// via mapeia: dobra(5) -> soma no mesmo fluxo
		"  #0 soma:8 | locais: a=5 b=5 |",
		"  #1 dobra:12 | locais: n=5 |",
		"  #2 <principal>:16 |",
		// closure: locais da lambda + capturadas
		"linha 21 fluxo 1 prof 2\n  #0 <anonima>:21 | locais: base=10 k=100 v=1 w=111 |",
	} {
		if !strings.Contains(rt, quer) {
			t.Errorf("faltou %q em:\n%s", quer, rt)
		}
	}
}

// Avaliar no quadro de baixo enxerga os locais daquele quadro; erro de
// avaliacao volta como *Erro (nos dois engines).
func TestDepuradorAvaliaQuadroEErro(t *testing.T) {
	src := `gambiarra g(a)
    bota b = a * 2
    funciona b
acabou_finalmente
gambiarra h(x)
    funciona g(x + 1)
acabou_finalmente
mostra h(1)
`
	for _, eng := range []string{"tree", "vm"} {
		var got []string
		d := &depFunc{fn: func(s *object.SitioLinha, f object.Fluxo) {
			if s.Linha != 2 || len(got) > 0 {
				return
			}
			got = append(got,
				f.Avalia(1, "x * 10").Inspect(),
				f.Avalia(0, "a + 1").Inspect(),
				string(f.Avalia(0, "nao_existe + 1").Type()),
				string(f.Avalia(0, "1 +").Type()),
			)
		}}
		if eng == "tree" {
			rodaDepTree(t, src, d)
		} else {
			rodaDepVM(t, src, d)
		}
		quer := []string{"10", "3", string(object.ERRO_OBJ), string(object.ERRO_OBJ)}
		if strings.Join(got, ",") != strings.Join(quer, ",") {
			t.Errorf("%s: %v, queria %v", eng, got, quer)
		}
	}
}

// Cada bora e um fluxo novo (com id proprio) que acaba quando a gambiarra
// volta; a pilha dele comeca na gambiarra.
func TestDepuradorBoraFluxo(t *testing.T) {
	src := `gambiarra trampo(n)
    bota dobro = n * 2
    funciona dobro
acabou_finalmente
bota fut = bora trampo(21)
mostra espera(fut)
`
	for _, eng := range []string{"tree", "vm"} {
		g := novoGravador(nil, 2)
		if eng == "tree" {
			rodaDepTree(t, src, g)
		} else {
			rodaDepVM(t, src, g)
		}
		rt := g.texto()
		if !strings.Contains(rt, "linha 2 fluxo 2 prof 1\n  #0 trampo:2 | locais: n=21 |") {
			t.Errorf("%s: retrato %q", eng, rt)
		}
		if len(g.acabaram) == 0 || g.acabaram[0] != 2 {
			t.Errorf("%s: fluxos acabados %v", eng, g.acabaram)
		}
	}
}

type depFunc struct {
	fn func(s *object.SitioLinha, f object.Fluxo)
}

func (d *depFunc) Linha(s *object.SitioLinha, f object.Fluxo) { d.fn(s, f) }
func (d *depFunc) FluxoAcabou(int64)                          {}
