package interpreter

import (
	"strings"
	"sync"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Gancho de linha no tree-walker (contrato em object/gancho.go). Desligado
// custa um teste de nil por statement no evalBlock/evalProgram. Ligado, cada
// programa que roda (o principal e cada modulo, na primeira vez) registra um
// sitio por statement executavel; o laco de statements procura o node no
// mapa e chama o gancho. Statement sintetico (thunk de padrao de campo) nao
// esta no mapa e nao dispara — igual a VM, que so emite OpLinha pros nodes
// do parser.
//
// O mesmo ponto serve o depurador (DefinirDepurador): ai o tree-walker
// tambem rastreia os quadros de cada fluxo (goroutine) — ver depuraTree.
type ganchoLinha struct {
	fn     object.GanchoLinha
	dep    *depuraTree  // nil = sem depurador
	mu     sync.RWMutex // modulo pode ser importado de varias goroutines
	sitios map[ast.Statement]*object.SitioLinha
}

// DefinirGancho liga o gancho de linha (nil desliga). Chama antes do Eval do
// programa: os sitios sao registrados quando cada programa comeca a rodar.
func (i *Interpreter) DefinirGancho(g object.GanchoLinha) {
	if g == nil {
		if i.gancho != nil && i.gancho.dep != nil {
			i.gancho.fn = nil
			return
		}
		i.gancho = nil
		return
	}
	i.garanteGancho().fn = g
}

// DefinirDepurador liga o depurador (nil desliga). Chama antes do Eval, igual
// o DefinirGancho; os dois convivem.
func (i *Interpreter) DefinirDepurador(d object.Depurador) {
	if d == nil {
		if i.gancho != nil {
			i.gancho.dep = nil
			if i.gancho.fn == nil {
				i.gancho = nil
			}
		}
		return
	}
	i.garanteGancho().dep = &depuraTree{d: d, i: i, fios: map[int64]*fioTree{}}
}

func (i *Interpreter) garanteGancho() *ganchoLinha {
	if i.gancho == nil {
		i.gancho = &ganchoLinha{sitios: map[ast.Statement]*object.SitioLinha{}}
	}
	return i.gancho
}

// registraSitios cria os sitios do programa (principal ou modulo).
func (g *ganchoLinha) registraSitios(prog *ast.Program, arquivo string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, s := range ast.StatementsExecutaveis(prog) {
		if _, ok := g.sitios[s]; !ok {
			g.sitios[s] = &object.SitioLinha{Arquivo: arquivo, Linha: ast.LinhaDoStatement(s)}
		}
	}
}

// dispara chama o gancho (e o depurador) se o statement e do usuario. env e
// o escopo onde o statement vai rodar.
func (g *ganchoLinha) dispara(s ast.Statement, env *object.Environment) {
	g.mu.RLock()
	sitio := g.sitios[s]
	g.mu.RUnlock()
	if sitio == nil {
		return
	}
	if g.fn != nil {
		g.fn(sitio)
	}
	if g.dep != nil {
		g.dep.linha(sitio, env)
	}
}

// ---- depurador ----

// depuraTree rastreia, por goroutine, a pilha de quadros do tree-walker (que
// nao tem pilha explicita: cada chamada e recursao do Go). Cada goroutine que
// roda codigo do usuario e um fluxo: o principal nasce no primeiro statement
// (com o quadro <principal> embaixo); bora/handler/paralelo nascem na
// primeira chamada de gambiarra e acabam quando ela volta.
type depuraTree struct {
	d    object.Depurador
	i    *Interpreter
	mu   sync.Mutex
	fios map[int64]*fioTree // goroutine -> fluxo
	prox int64
}

// fioTree e um fluxo. A pilha so e mexida e lida pela goroutine dona (o
// depurador inspeciona pelo Fluxo, que roda la), entao so o mapa trava.
type fioTree struct {
	id      int64
	quadros []*quadroTree // 0 = o de baixo
}

type quadroTree struct {
	nome    string
	arquivo string
	env     *object.Environment
	linha   int  // linha do statement que esta rodando neste quadro
	chamada int  // linha da chamada que abriu o quadro (0 = veio de builtin)
	topo    bool // topo de programa/modulo: sem locais, so globais
}

func (d *depuraTree) novoFio() *fioTree {
	d.prox++
	return &fioTree{id: d.prox}
}

// linha: o statement vai rodar. Atualiza o quadro de cima e chama o depurador.
func (d *depuraTree) linha(sitio *object.SitioLinha, env *object.Environment) {
	gid := object.IDGoroutine()
	d.mu.Lock()
	f := d.fios[gid]
	if f == nil {
		f = d.novoFio()
		f.quadros = append(f.quadros, &quadroTree{nome: "<principal>", arquivo: sitio.Arquivo, topo: true})
		d.fios[gid] = f
	}
	d.mu.Unlock()
	topo := f.quadros[len(f.quadros)-1]
	topo.linha = sitio.Linha
	topo.env = env
	d.d.Linha(sitio, &fluxoTree{d: d, f: f, sitio: sitio})
}

// entra empilha o quadro de uma chamada (gambiarra ou corpo de modulo) no
// fluxo da goroutine atual, criando o fluxo se ela ainda nao tem.
func (d *depuraTree) entra(q *quadroTree) (*fioTree, int64) {
	gid := object.IDGoroutine()
	d.mu.Lock()
	f := d.fios[gid]
	if f == nil {
		f = d.novoFio()
		d.fios[gid] = f
	}
	d.mu.Unlock()
	f.quadros = append(f.quadros, q)
	return f, gid
}

// sai desempilha; fluxo que ficou sem quadro acabou.
func (d *depuraTree) sai(f *fioTree, gid int64) {
	f.quadros[len(f.quadros)-1] = nil
	f.quadros = f.quadros[:len(f.quadros)-1]
	if len(f.quadros) > 0 {
		return
	}
	d.mu.Lock()
	delete(d.fios, gid)
	d.mu.Unlock()
	d.d.FluxoAcabou(f.id)
}

// quadroDaFuncao monta o quadro de uma chamada de gambiarra do usuario.
func (i *Interpreter) quadroDaFuncao(fn *object.Funcao, nome string, escopo *object.Environment, linha int) *quadroTree {
	if fn.Nome != "" {
		nome = fn.Nome
	}
	arq := fn.Env.Modulo()
	if arq == "" {
		arq = i.arquivo
	}
	return &quadroTree{nome: nome, arquivo: arq, env: escopo, chamada: linha}
}

// fluxoTree implementa object.Fluxo em cima do fioTree.
type fluxoTree struct {
	d     *depuraTree
	f     *fioTree
	sitio *object.SitioLinha
}

func (x *fluxoTree) ID() int64         { return x.f.id }
func (x *fluxoTree) Profundidade() int { return len(x.f.quadros) }

// quadro devolve o quadro q (0 = o de cima) ou nil.
func (x *fluxoTree) quadro(q int) *quadroTree {
	k := len(x.f.quadros) - 1 - q
	if q < 0 || k < 0 {
		return nil
	}
	return x.f.quadros[k]
}

func (x *fluxoTree) Quadros() []object.Quadro {
	n := len(x.f.quadros)
	out := make([]object.Quadro, 0, n)
	for q := 0; q < n; q++ {
		qd := x.f.quadros[n-1-q]
		linha := qd.linha
		if q == 0 {
			linha = x.sitio.Linha
		} else if filho := x.f.quadros[n-q]; filho.chamada > 0 {
			// a linha da chamada (igual a VM, que tira do call site)
			linha = filho.chamada
		}
		out = append(out, object.Quadro{Nome: qd.nome, Arquivo: qd.arquivo, Linha: linha})
	}
	return out
}

func (x *fluxoTree) Locais(q int) []object.Variavel {
	qd := x.quadro(q)
	if qd == nil || qd.topo || qd.env == nil {
		return nil
	}
	var out []object.Variavel
	visto := map[string]bool{}
	// o escopo da funcao e os de fora dele ate antes da raiz (closure)
	for env := qd.env; env != nil && env.Externo() != nil; env = env.Externo() {
		for _, nome := range env.Locais() {
			if visto[nome] || strings.HasPrefix(nome, "__") {
				continue
			}
			visto[nome] = true
			if v, ok := env.Get(nome); ok {
				out = append(out, object.Variavel{Nome: nome, Valor: v})
			}
		}
	}
	return object.VariaveisOrdenadas(out)
}

func (x *fluxoTree) Globais(q int) []object.Variavel {
	qd := x.quadro(q)
	if qd == nil || qd.env == nil {
		return nil
	}
	raiz := qd.env
	for raiz.Externo() != nil {
		raiz = raiz.Externo()
	}
	var out []object.Variavel
	for _, nome := range raiz.Locais() {
		if strings.HasPrefix(nome, "__") {
			continue
		}
		if v, ok := raiz.Get(nome); ok {
			out = append(out, object.Variavel{Nome: nome, Valor: v})
		}
	}
	return object.VariaveisOrdenadas(out)
}

// Avalia roda o codigo no escopo do quadro: expressao devolve o valor;
// statement (`bota x = 1`) roda de verdade e muda o escopo.
func (x *fluxoTree) Avalia(q int, fonte string) object.Object {
	qd := x.quadro(q)
	if qd == nil || qd.env == nil {
		return &object.Erro{Message: "esse quadro nao existe", Kind: "depurador"}
	}
	p := parser.New(lexer.New(fonte))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return &object.Erro{Message: errs[0], Kind: "parse"}
	}
	var res object.Object = NADA
	for _, s := range prog.Statements {
		res = x.d.i.Eval(s, qd.env)
		if ret, ok := res.(*object.Retorno); ok {
			return ret.Value
		}
		if object.EhErroLevantado(res) {
			return res
		}
		switch res.(type) {
		case *object.Sair, *object.Vaza, *object.Continua:
			return &object.Erro{Message: "isso nao vale no depurador", Kind: "depurador"}
		}
	}
	return res
}
