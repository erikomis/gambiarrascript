package vm

import (
	"fmt"

	"gambiarrascript/code"
	"gambiarrascript/object"
)

// POO na VM (Tier 8): os opcodes de treta/combinado/metodo/literal e a
// chamada de metodo ligado. A regra (promotion, zero-value, satisfacao) e a
// de object/poo.go, a mesma do tree-walker.

// execPOO roda um opcode de POO (todos tem 1 operando de 2 bytes: o indice da
// constante). Fica fora do switch do execDesde: declaracao e literal sao
// raros, e o switch quente nao cresce. false = nao e opcode de POO.
func (vm *VM) execPOO(op code.Opcode, constIdx int) bool {
	switch op {
	case code.OpTreta:
		vm.execTreta(vm.constants[constIdx].(*object.DescTreta))
	case code.OpCombinado:
		vm.execCombinado(vm.constants[constIdx].(*object.DescCombinado))
	case code.OpMetodo:
		vm.execMetodo(vm.constants[constIdx].(*object.Texto).Value)
	case code.OpInstancia:
		vm.execInstancia(vm.constants[constIdx].(*object.DescLiteral))
	default:
		return false
	}
	return true
}

func erroPOO(msg string) {
	panic(VMError{err: &object.Erro{Message: msg, Kind: "runtime"}})
}

// execTreta: pilha [valor de cada campo com puxadinho/padrao...] -> treta.
func (vm *VM) execTreta(desc *object.DescTreta) {
	n := 0
	for _, c := range desc.Campos {
		if c.Embutida || c.TemPadrao {
			n++
		}
	}
	vals := vm.stack[vm.sp-n : vm.sp]
	campos := make([]object.CampoTreta, len(desc.Campos))
	k := 0
	for i, c := range desc.Campos {
		campos[i].Nome = c.Nome
		switch {
		case c.Embutida:
			t, ok := vals[k].(*object.Treta)
			if !ok {
				erroPOO(fmt.Sprintf("puxadinho na treta %s: %s", desc.Nome, object.MsgNaoETreta(c.Texto, vals[k])))
			}
			campos[i].Embutida = t
			k++
		case c.TemPadrao:
			campos[i].Padrao = vals[k]
			k++
		}
	}
	vm.sp -= n
	t, msg := object.NovaTreta(desc.Nome, campos)
	if msg != "" {
		erroPOO(msg)
	}
	vm.push(t)
}

// execCombinado: pilha [combinados embutidos...] -> combinado.
func (vm *VM) execCombinado(desc *object.DescCombinado) {
	n := len(desc.Embutidos)
	embutidos := make([]*object.Combinado, n)
	for i, v := range vm.stack[vm.sp-n : vm.sp] {
		c, ok := v.(*object.Combinado)
		if !ok {
			erroPOO(fmt.Sprintf("no combinado %s: `%s` nao e combinado (e %s)", desc.Nome, desc.Embutidos[i], object.NomeTipo(v)))
		}
		embutidos[i] = c
	}
	vm.sp -= n
	c, msg := object.NovoCombinado(desc.Nome, desc.Metodos, embutidos)
	if msg != "" {
		erroPOO(msg)
	}
	vm.push(c)
}

// execMetodo: pilha [treta, closure] -> pendura o metodo.
func (vm *VM) execMetodo(nome string) {
	fn := vm.pop()
	tv := vm.pop()
	t, ok := tv.(*object.Treta)
	if !ok {
		cf, _ := fn.(*object.CompiledFunction)
		tipo := "?"
		if cf != nil {
			tipo = cf.Name[:len(cf.Name)-len(nome)-1]
		}
		erroPOO(fmt.Sprintf("metodo %s: %s", nome, object.MsgNaoETreta(tipo, tv)))
	}
	if msg := t.DefineMetodo(nome, fn); msg != "" {
		erroPOO(msg)
	}
}

// execInstancia: pilha [treta, valores...] -> instancia do literal.
func (vm *VM) execInstancia(desc *object.DescLiteral) {
	vals := make([]object.Object, desc.N)
	copy(vals, vm.stack[vm.sp-desc.N:vm.sp])
	vm.sp -= desc.N
	tv := vm.pop()
	t, ok := tv.(*object.Treta)
	if !ok {
		erroPOO(fmt.Sprintf("nao da pra fazer %s{...}: %s", desc.Tipo, object.MsgNaoETreta(desc.Tipo, tv)))
	}
	var nomes []string
	if !desc.Posicional {
		nomes = desc.Nomes
	}
	res, msg := object.MontaInstancia(t, nomes, vals, func(fn object.Object) object.Object {
		cf, ok := fn.(*object.CompiledFunction)
		if !ok {
			return &object.Erro{Message: "padrao de campo nao compilado", Kind: "runtime"}
		}
		return vm.chamaCompilada(cf, nil)
	})
	if msg != "" {
		erroPOO(msg)
	}
	switch r := res.(type) {
	case *object.Sair:
		panic(VMError{sai: r})
	case *object.Erro:
		panic(VMError{err: r})
	}
	vm.push(res)
}

// abreMetodo troca `[a1..an, metodo]` no topo da pilha por
// `[receiver, a1..an, fn]` (o receiver vira o primeiro argumento) e devolve
// a funcao e o argc novo. Confere a aridade sem o receiver.
func (vm *VM) abreMetodo(m *object.MetodoLigado, argc int) (*object.CompiledFunction, int) {
	if msg := object.ChecaAridadeMetodo(m, argc); msg != "" {
		erroPOO(msg)
	}
	cf, ok := m.Fn.(*object.CompiledFunction)
	if !ok {
		erroPOO("metodo " + m.Nome + " nao e da VM")
	}
	base := vm.sp - 1 - argc
	vm.garanteEspaco(vm.sp + 1)
	copy(vm.stack[base+1:base+1+argc], vm.stack[base:base+argc])
	vm.stack[base] = m.Receptor
	vm.stack[base+argc+1] = cf
	vm.sp = base + argc + 2
	return cf, argc + 1
}

// chamaMetodoSincrono roda o metodo numa sub-VM (receiver na frente, defaults
// e ...resto arrumados igual o OpCall) e devolve o valor. Erro/sai sobem
// como panic, igual uma chamada normal.
func (vm *VM) chamaMetodoSincrono(m *object.MetodoLigado, args []object.Object) object.Object {
	if msg := object.ChecaAridadeMetodo(m, len(args)); msg != "" {
		erroPOO(msg)
	}
	cf, ok := m.Fn.(*object.CompiledFunction)
	if !ok {
		erroPOO("metodo " + m.Nome + " nao e da VM")
	}
	todos := append([]object.Object{m.Receptor}, args...)
	n := cf.NumArgs
	out := make([]object.Object, 0, n)
	fixos := n
	if cf.Variadic {
		fixos = n - 1
	}
	for i := 0; i < fixos; i++ {
		if i < len(todos) {
			out = append(out, todos[i])
		} else {
			out = append(out, NADA) // o prologo troca pelo default
		}
	}
	if cf.Variadic {
		resto := []object.Object{}
		if len(todos) > fixos {
			resto = append(resto, todos[fixos:]...)
		}
		out = append(out, object.NovaLista(resto))
	}
	switch r := vm.chamaCompilada(cf, out).(type) {
	case *object.Sair:
		panic(VMError{sai: r})
	case *object.Erro:
		if !r.Handled {
			panic(VMError{err: r})
		}
		return r
	default:
		return r
	}
}

// membroVM e o `obj.nome` numa instancia (OpIndex).
func membroVM(inst *object.Instancia, idx object.Object) (object.Object, error) {
	nome, ok := idx.(*object.Texto)
	if !ok {
		return nil, fmt.Errorf("%s", object.MsgCampoNaoTexto(idx))
	}
	v, msg := inst.Membro(nome.Value)
	if msg != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	return v, nil
}

// poeMembroVM e o `bota obj.nome = v` (OpIndexSet).
func poeMembroVM(inst *object.Instancia, idx, val object.Object) error {
	nome, ok := idx.(*object.Texto)
	if !ok {
		return fmt.Errorf("%s", object.MsgCampoNaoTexto(idx))
	}
	if msg := inst.PoeMembro(nome.Value, val); msg != "" {
		return fmt.Errorf("%s", msg)
	}
	return nil
}
