package vm

import (
	"fmt"
	"strings"
	"sync"

	"gambiarrascript/ast"
	"gambiarrascript/compiler"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Depurador na VM (contrato em object/gancho.go). So funciona com bytecode
// instrumentado (compiler.Instrumentar): o OpLinha e o ponto de parada e as
// CompiledFunction trazem os nomes dos locais (InfoDepuracao). Sem depurador
// nada disto roda: o OpLinha nem existe no bytecode normal, e o resto e um
// teste de nil fora do laco quente (Run, bora, chamaCompilada).
//
// Fluxo = goroutine. Cada VM roda numa goroutine so, mas uma goroutine pode
// ter varias VMs empilhadas: a gambiarra chamada por builtin (mapeia...) ou o
// corpo de modulo roda numa sub-VM em cima da VM de quem chamou. O registro
// por goroutine (cadeias) liga a sub-VM nova a VM de baixo (vm.pai) — a
// pilha do fluxo e a soma dos frames da cadeia.
type depuraVM struct {
	d         object.Depurador
	principal *object.InfoDepuracao // arquivo e globais do fluxo principal

	// porCodigo acha a InfoDepuracao de uma closure: o OpClosure cria uma
	// CompiledFunction nova (sem copiar o Depura, pra nao mexer no caminho
	// quente) que divide o Bytecode com a constante — a chave e o 1o byte.
	porCodigo map[*byte]*object.InfoDepuracao

	mu      sync.Mutex
	cadeias map[int64]*VM // goroutine -> VM de cima
	prox    int64
}

// DefinirDepurador liga o depurador (nil desliga). Chama antes do Run, com o
// bytecode compilado com compiler.Instrumentar. Os clones herdam.
func (vm *VM) DefinirDepurador(d object.Depurador) {
	if d == nil {
		vm.dep = nil
		return
	}
	dep := &depuraVM{d: d, principal: vm.depMain, cadeias: map[int64]*VM{}, porCodigo: map[*byte]*object.InfoDepuracao{}}
	for _, k := range vm.constants {
		var cf *object.CompiledFunction
		switch c := k.(type) {
		case *object.CompiledFunction:
			cf = c
		case *object.Modulo:
			cf = c.Corpo
		}
		if cf != nil && cf.Depura != nil && len(cf.Bytecode) > 0 {
			dep.porCodigo[&cf.Bytecode[0]] = cf.Depura
		}
	}
	vm.dep = dep
}

// info devolve os nomes de depuracao da gambiarra (nil se nao tem).
func (d *depuraVM) info(fn *object.CompiledFunction) *object.InfoDepuracao {
	if fn.Depura != nil {
		return fn.Depura
	}
	if d == nil || len(fn.Bytecode) == 0 {
		return nil
	}
	return d.porCodigo[&fn.Bytecode[0]]
}

// entra registra a VM como a de cima da goroutine atual. A de baixo (se
// tem) vira o pai e empresta o fluxo; sem, o fluxo e novo.
func (d *depuraVM) entra(v *VM) int64 {
	gid := object.IDGoroutine()
	d.mu.Lock()
	pai := d.cadeias[gid]
	v.pai = pai
	if pai != nil {
		v.fluxoID = pai.fluxoID
	} else {
		d.prox++
		v.fluxoID = d.prox
	}
	d.cadeias[gid] = v
	d.mu.Unlock()
	return gid
}

// sai desfaz o entra; a VM de baixo da cadeia acabando, o fluxo acabou.
func (d *depuraVM) sai(v *VM, gid int64) {
	pai, id := v.pai, v.fluxoID
	d.mu.Lock()
	if pai != nil {
		d.cadeias[gid] = pai
	} else {
		delete(d.cadeias, gid)
	}
	d.mu.Unlock()
	v.pai = nil
	if pai == nil {
		d.d.FluxoAcabou(id)
	}
}

// fluxoVM implementa object.Fluxo em cima da VM que disparou o OpLinha (a de
// cima da cadeia do fluxo).
type fluxoVM struct {
	vm    *VM
	sitio *object.SitioLinha
}

func (f *fluxoVM) ID() int64 { return f.vm.fluxoID }

func (f *fluxoVM) Profundidade() int {
	n := 0
	for v := f.vm; v != nil; v = v.pai {
		n += v.framesIdx
	}
	return n
}

// refQuadro aponta um frame de uma VM da cadeia.
type refQuadro struct {
	vm  *VM
	idx int
}

// refs lista os frames do fluxo, do de cima pro de baixo.
func (f *fluxoVM) refs() []refQuadro {
	var out []refQuadro
	for v := f.vm; v != nil; v = v.pai {
		for j := v.framesIdx - 1; j >= 0; j-- {
			out = append(out, refQuadro{v, j})
		}
	}
	return out
}

func (f *fluxoVM) ref(q int) (refQuadro, bool) {
	rs := f.refs()
	if q < 0 || q >= len(rs) {
		return refQuadro{}, false
	}
	return rs[q], true
}

func (f *fluxoVM) Quadros() []object.Quadro {
	rs := f.refs()
	out := make([]object.Quadro, len(rs))
	for q, r := range rs {
		fr := r.vm.frames[r.idx]
		var linha int
		switch {
		case q == 0:
			linha = f.sitio.Linha
		case r.idx < r.vm.framesIdx-1:
			// chamou o frame de cima nesta mesma VM: a linha do call site
			linha = fr.fn.LinhaDoPC(r.vm.frames[r.idx+1].callPos)
		default:
			// chamou um builtin que abriu a sub-VM de cima: o ip parou na
			// instrucao da chamada
			linha = fr.fn.LinhaDoPC(fr.ip)
		}
		nome := fr.fn.Name
		if nome == "<main>" {
			nome = "<principal>"
		}
		arq := ""
		if dep := f.vm.dep.info(fr.fn); dep != nil {
			arq = dep.Arquivo
		}
		out[q] = object.Quadro{Nome: nome, Arquivo: arq, Linha: linha}
	}
	return out
}

func (f *fluxoVM) Locais(q int) []object.Variavel {
	r, ok := f.ref(q)
	if !ok {
		return nil
	}
	fr := r.vm.frames[r.idx]
	dep := f.vm.dep.info(fr.fn)
	if dep == nil {
		return nil
	}
	var out []object.Variavel
	visto := map[string]bool{}
	for slot, nome := range dep.Locais {
		if nome == "" || fr.basePointer+slot >= len(r.vm.stack) {
			continue
		}
		v := valorLivre(r.vm.stack[fr.basePointer+slot])
		if v == nil {
			continue // ainda nao botou
		}
		visto[nome] = true
		out = append(out, object.Variavel{Nome: nome, Valor: v})
	}
	for k, nome := range dep.Livres {
		if visto[nome] || k >= len(fr.fn.Free) {
			continue
		}
		if v := valorLivre(fr.fn.Free[k]); v != nil {
			visto[nome] = true
			out = append(out, object.Variavel{Nome: nome, Valor: v})
		}
	}
	return object.VariaveisOrdenadas(out)
}

func (f *fluxoVM) globais(r refQuadro) *object.TabelaGlobais {
	if dep := f.vm.dep.info(r.vm.frames[r.idx].fn); dep != nil {
		return dep.Globais
	}
	return nil
}

func (f *fluxoVM) Globais(q int) []object.Variavel {
	r, ok := f.ref(q)
	if !ok {
		return nil
	}
	tab := f.globais(r)
	if tab == nil {
		return nil
	}
	var out []object.Variavel
	for k, nome := range tab.Nomes {
		slot := tab.Slots[k]
		if slot >= len(r.vm.globals) {
			continue
		}
		var v object.Object
		if object.ConcorrenciaAtiva() {
			v = pegaGlobalTravado(r.vm.globals, slot)
		} else {
			v = r.vm.globals[slot]
		}
		if v != nil {
			out = append(out, object.Variavel{Nome: nome, Valor: v})
		}
	}
	return object.VariaveisOrdenadas(out)
}

// Avalia compila a expressao contra o quadro (compiler.CompilaAvaliacao: os
// locais entram como parametros, por valor; as globais pelos slots de
// verdade) e roda numa VM avulsa que divide globais, builtins e modulos com
// o programa. So expressao: statement (`bota`) da erro — no tree-walker
// funciona.
func (f *fluxoVM) Avalia(q int, fonte string) (res object.Object) {
	r, ok := f.ref(q)
	if !ok {
		return erroDep("esse quadro nao existe")
	}
	p := parser.New(lexer.New(fonte))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return &object.Erro{Message: errs[0], Kind: "parse"}
	}
	if len(prog.Statements) != 1 {
		return erroDep("na VM o depurador so avalia UMA expressao")
	}
	es, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		return erroDep("na VM o depurador so avalia expressao (statement, tipo `bota`, so no --tree)")
	}
	locais := f.Locais(q)
	nomes := make([]string, len(locais))
	valores := make([]object.Object, len(locais))
	for k, v := range locais {
		nomes[k], valores[k] = v.Nome, v.Valor
	}
	av, err := compiler.CompilaAvaliacao(es.Expression, nomes, r.vm.constants, f.globais(r), len(r.vm.globals))
	if err != nil {
		return erroDep(err.Error())
	}
	defer func() {
		if p := recover(); p != nil {
			res = erroDep(fmt.Sprintf("panico avaliando: %v", p))
		}
	}()
	bc := av.Bytecode
	ev := &VM{
		constants:  bc.Constants,
		inst:       bc.Instructions,
		linhas:     bc.Linhas,
		maxStack:   bc.MaxStack,
		stack:      make([]object.Object, StackInicial),
		globals:    r.vm.globals,
		frames:     novosFrames(),
		subVMs:     &sync.Pool{},
		modulos:    r.vm.modulos,
		builtinIdx: r.vm.builtinIdx,
		builtins:   r.vm.builtins,
		out:        r.vm.out,
	}
	if err := ev.Run(); err != nil {
		if e := ErroDoRun(err); e != nil {
			return e
		}
		return erroDep(err.Error())
	}
	cf, ok := ev.LastPoppedStackElem().(*object.CompiledFunction)
	if !ok {
		return erroDep("avaliacao nao gerou a gambiarra esperada")
	}
	res = ev.chamaCompilada(cf, valores)
	if _, ok := res.(*object.Sair); ok {
		return erroDep("sai() nao vale no depurador")
	}
	return res
}

// erroDep: erro do depurador. Erro de compilacao da expressao vem com
// "linha N: " (da expressao, nao do programa) — sai fora.
func erroDep(msg string) *object.Erro {
	if strings.HasPrefix(msg, "linha ") {
		if _, resto, ok := strings.Cut(msg, ": "); ok {
			msg = resto
		}
	}
	return &object.Erro{Message: msg, Kind: "depurador"}
}
