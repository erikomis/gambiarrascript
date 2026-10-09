package vm

import (
	"fmt"

	"gambiarrascript/code"
	"gambiarrascript/object"
)

// Gerador na VM: corrotina de verdade, sem goroutine.
//
// O corpo de um gerador comeca com OpGerador (depois do prologo): a chamada
// abre o frame normal (args, padroes, celulas), e o OpGerador copia os locals
// pra uma sub-VM SO DESTE gerador, com o frame 0 apontando pra instrucao
// seguinte, e retorna o gerador. Cada pedido de valor roda o execDesde dessa
// sub-VM ate o OpRende — que guarda o valor e devolve o controle com o frame
// parado ali (frame.ip, pilha e handlers de arruma ficam na sub-VM) — ou ate
// o frame 0 retornar (acabou). O `rende` so existe no corpo do proprio
// gerador, entao sempre roda no frame 0 da sub-VM: as gambiarras que o corpo
// chama rodam nos frames de cima e ja retornaram quando ele rende.
//
// Por que nao goroutine+canal: a VM ja tem o estado da execucao em dados (pilha
// e frames), entao pausar e so voltar do laco. Gerador abandonado no meio (um
// proximo() que ninguem chama de novo) nao segura nada fora da memoria: a
// sub-VM vira lixo com ele. E o corpo roda na goroutine de quem pede, entao
// nao tem concorrencia nova (o modo concorrente nao liga) nem troca de
// contexto por valor.
//
// Fecha (`fecha(g)`, e o pra_cada dono saindo no meio): o sinal de fecha
// (object.SinalFecha) e levantado como erro no frame 0 parado no `rende` —
// o mesmo desenrola de um erro de verdade, entao os `finalmente` pendentes
// rodam (o quebrou relanca ele com OpRelancaFecha) — e o corpo segue ate o
// fim. Um OpRende no caminho volta do execDesde: ignorou o fecha.
type geradorVM struct {
	vm       *VM
	iniciado bool // ja pediram valor (o corpo ta pausado num rende)
}

// novoGerador guarda o frame que acabou de abrir (chamada do gerador) numa
// sub-VM nova; o corpo comeca em ipCorpo quando pedirem o primeiro valor.
func (vm *VM) novoGerador(frame *Frame, ipCorpo int) *object.Gerador {
	cf := frame.fn
	n := cf.NumLocals
	tam := n + folga(cf)
	if tam < 16 {
		tam = 16
	}
	sub := &VM{
		constants:  vm.constants,
		inst:       vm.inst,
		linhas:     vm.linhas,
		maxStack:   vm.maxStack,
		stack:      make([]object.Object, tam),
		globals:    vm.globals,
		frames:     make([]*Frame, 4), // cresce sob demanda (frameEm)
		subVMs:     vm.subVMs,
		modulos:    vm.modulos,
		builtinIdx: vm.builtinIdx,
		builtins:   vm.builtins,
		porIdx:     vm.porIdx,
		out:        vm.out,
		gancho:     vm.gancho,
		sitios:     vm.sitios,
		dep:        vm.dep,
		depMain:    vm.depMain,
	}
	copy(sub.stack, vm.stack[frame.basePointer:frame.basePointer+n])
	sub.sp = n
	fr0 := sub.frameEm(0)
	fr0.fn = cf
	fr0.ip = ipCorpo
	fr0.basePointer = 0
	fr0.callPos = 0
	sub.framesIdx = 1
	return object.NovoGerador(cf.Name, &geradorVM{vm: sub})
}

// Proximo roda o corpo ate o proximo OpRende (ou o fim). O object.Gerador
// garante uma chamada por vez e nunca chama de novo depois do fim.
func (g *geradorVM) Proximo() (valor object.Object, ok bool, falha object.Object) {
	g.iniciado = true
	sub := g.vm
	if sub.dep != nil {
		// depurador: o corpo roda no fluxo de quem pediu (frames em cima)
		gid := sub.dep.entra(sub)
		defer sub.dep.sai(sub, gid)
	}
	defer func() {
		if r := recover(); r != nil {
			valor, ok = nil, false
			if vme, e := r.(VMError); e {
				if vme.sai != nil {
					falha = vme.sai
				} else {
					falha = vme.err
				}
				return
			}
			falha = &object.Erro{Message: fmt.Sprintf("panico no gerador: %v", r), Kind: "runtime"}
		}
	}()
	if err := sub.execDesde(sub.frames[0], 1); err != nil {
		switch e := err.(type) {
		case erroNaoCapturado:
			return nil, false, e.err // preserva Line/Kind/pilha do erro
		case SaiRequisicao:
			return nil, false, &object.Sair{Codigo: e.Codigo}
		}
		return nil, false, &object.Erro{Message: err.Error(), Kind: "runtime"}
	}
	if sub.framesIdx == 0 {
		return nil, false, nil // o frame 0 retornou: acabou
	}
	v := sub.rendido
	sub.rendido = nil
	return v, true, nil
}

// Fecha retoma o corpo pausado no `rende` com o sinal de fecha (ver o
// comeco do arquivo). O object.Gerador so chama antes do fim, uma vez.
func (g *geradorVM) Fecha() (rendeu bool, linha int, falha object.Object) {
	if !g.iniciado {
		return false, 0, nil // nem comecou: nao tem finalmente pendente
	}
	sub := g.vm
	if sub.dep != nil {
		gid := sub.dep.entra(sub)
		defer sub.dep.sai(sub, gid)
	}
	defer func() {
		if r := recover(); r != nil {
			rendeu, linha = false, 0
			if vme, e := r.(VMError); e {
				if vme.sai != nil {
					falha = vme.sai
				} else {
					falha = vme.err
				}
				return
			}
			falha = &object.Erro{Message: fmt.Sprintf("panico no gerador: %v", r), Kind: "runtime"}
		}
	}()
	e, sai := sub.desenrola(object.SinalFecha(), nil)
	if sai != nil {
		return false, 0, sai
	}
	var err error = erroNaoCapturado{err: e}
	if sub.framesIdx > 0 {
		err = sub.execDesde(sub.currentFrame(), 1)
	}
	switch x := err.(type) {
	case nil:
		if sub.framesIdx == 0 {
			return false, 0, nil // o finalmente engoliu o fecha e o corpo acabou
		}
		fr := sub.frames[0] // parou num OpRende (1 byte): a linha e a dele
		return true, fr.fn.LinhaDoPC(fr.ip - 1), nil
	case erroNaoCapturado:
		if object.EhSinalFecha(x.err) {
			return false, 0, nil
		}
		return false, 0, x.err
	case SaiRequisicao:
		return false, 0, &object.Sair{Codigo: x.Codigo}
	}
	return false, 0, &object.Erro{Message: err.Error(), Kind: "runtime"}
}

// lacoDono e um pra_cada aberto que e dono do gerador que percorre (o gerador
// nasceu no cabecalho dele). tries: quantos handlers de arruma ja estavam
// armados quando o laco comecou — handler armado antes do laco (indice <
// tries) e de fora dele, entao o erro que vai pra ele atravessa o laco.
type lacoDono struct {
	g        *object.Gerador
	frameIdx int
	tries    int
}

// desenrola trata um erro levantado: fecha (de dentro pra fora) os geradores
// dos pra_cada donos que o erro atravessa, depois acha o handler
// (handleVMError). Erro no fecha toma o lugar do que tava subindo (igual um
// erro no finalmente); sai() no fecha volta em sai.
func (vm *VM) desenrola(e *object.Erro, quadro *object.StackFrame) (*object.Erro, *object.Sair) {
	h := len(vm.errStack) - 1
	for len(vm.donos) > 0 && vm.donos[len(vm.donos)-1].tries > h {
		d := vm.donos[len(vm.donos)-1]
		vm.donos[len(vm.donos)-1] = lacoDono{}
		vm.donos = vm.donos[:len(vm.donos)-1]
		switch f := d.g.Fecha().(type) {
		case *object.Sair:
			return e, f
		case *object.Erro:
			e, quadro = f, nil
		}
	}
	vm.handleVMError(e, quadro)
	return e, nil
}

// execFecha roda os opcodes do fecha de gerador (raros: um por laco). false =
// nao e um deles.
func (vm *VM) execFecha(op code.Opcode) bool {
	switch op {
	case code.OpIterMarca:
		vm.push(vm.num.Int(int64(object.SerieGeradores())))
	case code.OpIterFim:
		vm.fimDoLaco(vm.pop())
	case code.OpRelancaFecha:
		if e, ok := vm.stack[vm.sp-1].(*object.Erro); ok && object.EhSinalFecha(e) {
			vm.sp--
			e.Handled = false
			panic(VMError{err: e})
		}
	default:
		return false
	}
	return true
}

// fimDoLaco e o fim de um pra_cada (acabou, vaza ou funciona): se ele e dono
// do gerador seq, fecha (no gerador que acabou nao faz nada).
func (vm *VM) fimDoLaco(seq object.Object) {
	g, ok := seq.(*object.Gerador)
	if !ok || len(vm.donos) == 0 {
		return
	}
	d := vm.donos[len(vm.donos)-1]
	if d.g != g || d.frameIdx != vm.framesIdx {
		return
	}
	vm.donos[len(vm.donos)-1] = lacoDono{}
	vm.donos = vm.donos[:len(vm.donos)-1]
	if falha := g.Fecha(); falha != nil {
		panicFalha(falha)
	}
}

// cresceFrames aumenta o array de slots de frame ate caber idx (a sub-VM de
// gerador nasce pequena). O teto continua o MaxFrames do empurraFrame.
func (vm *VM) cresceFrames(idx int) {
	novo := 2 * len(vm.frames)
	for novo <= idx {
		novo *= 2
	}
	if novo > MaxFrames {
		novo = MaxFrames
	}
	frames := make([]*Frame, novo)
	copy(frames, vm.frames)
	vm.frames = frames
}

// ---------------------------------------------------------------- iteracao

// iterSeq e o comeco do pra_cada: empilha orig, seq e tamanho (ver
// compilePraCadaList). Treta percorre o que o itera() dela devolve.
func (vm *VM) iterSeq(it object.Object, marca uint64, fn *object.CompiledFunction, ip int) {
	if inst, ok := it.(*object.Instancia); ok {
		it = vm.iteraTreta(inst, fn.LinhaDoPC(ip))
	}
	switch c := it.(type) {
	case *object.Lista:
		// com concorrencia o laco percorre um retrato tirado agora
		seq := c.ParaIterar()
		vm.push(c)
		vm.push(seq)
		vm.push(vm.num.Int(int64(seq.Tamanho())))
	case *object.Dicionario:
		chaves := make([]object.Object, 0, c.Tamanho())
		c.Itera(func(par object.ParDic) { chaves = append(chaves, par.Chave) })
		vm.push(c)
		vm.push(object.NovaLista(chaves))
		vm.push(vm.num.Int(int64(len(chaves))))
	case *object.Conjunto:
		// retrato dos itens na ordem de insercao; com dois nomes vem
		// (indice, item), igual lista
		itens := c.Valores()
		vm.push(c)
		vm.push(object.NovaLista(itens))
		vm.push(vm.num.Int(int64(len(itens))))
	case *object.Cardapio:
		// as opcoes na ordem da declaracao (igual lista)
		opcoes := c.ListaOpcoes()
		vm.push(c)
		vm.push(object.NovaLista(opcoes))
		vm.push(vm.num.Int(int64(len(opcoes))))
	case *object.Gerador:
		if c.NasceuDepois(marca) {
			// nasceu no cabecalho (ou no itera()): o laco e dono
			vm.donos = append(vm.donos, lacoDono{g: c, frameIdx: vm.framesIdx, tries: len(vm.errStack)})
		}
		vm.push(c)
		vm.push(c)
		vm.push(vm.num.Int(0)) // sem tamanho: pede ate acabar
	default:
		panic(VMError{err: &object.Erro{Message: object.MsgNaoIteravel(it), Kind: "runtime"}})
	}
}

// iterProx e uma volta do pra_cada: tira [orig] seq it tam e empilha o(s)
// valor(es) da volta. false = acabou (nao empilha nada).
func (vm *VM) iterProx(nomes int) bool {
	tam := vm.stack[vm.sp-1].(*object.Numero).Int
	i := vm.stack[vm.sp-2].(*object.Numero).Int
	seq := vm.stack[vm.sp-3]
	vm.sp -= 3
	var orig object.Object
	if nomes == 2 {
		vm.sp--
		orig = vm.stack[vm.sp]
	}
	switch s := seq.(type) {
	case *object.Lista:
		if i >= tam {
			return false
		}
		item, ok := s.Pega(int(i))
		if !ok {
			return false // o corpo do laco encolheu a lista
		}
		if nomes != 2 {
			vm.push(item)
			return true
		}
		if d, ok := orig.(*object.Dicionario); ok {
			// item e a chave; o valor e lido agora (chave removida no meio
			// do laco vale nada)
			var valor object.Object = NADA
			if ch, ok := item.(object.Chaveavel); ok {
				if par, existe := d.Pega(ch.ChaveHash()); existe {
					valor = par.Valor
				}
			}
			vm.push(item)
			vm.push(valor)
			return true
		}
		vm.push(vm.num.Int(i))
		vm.push(item)
		return true
	case *object.Gerador:
		v, ok, falha := s.Proximo()
		if falha != nil {
			panicFalha(falha)
		}
		if !ok {
			return false
		}
		if nomes == 2 {
			vm.push(vm.num.Int(i))
		}
		vm.push(v)
		return true
	}
	panic(VMError{err: &object.Erro{Message: object.MsgNaoIteravel(seq), Kind: "runtime"}})
}

// iteraTreta chama o itera() da instancia e confere o que ele devolveu. O
// quadro do metodo entra no traco do erro igual no tree-walker.
func (vm *VM) iteraTreta(inst *object.Instancia, linha int) object.Object {
	m, msg := object.MetodoItera(inst)
	if msg != "" {
		panic(VMError{err: &object.Erro{Message: msg, Kind: "runtime"}})
	}
	cf, ok := m.Fn.(*object.CompiledFunction)
	if !ok {
		erroPOO("metodo " + m.Nome + " nao e da VM")
	}
	if msg := object.ChecaAridadeMetodo(m, 0); msg != "" {
		erroPOO(msg)
	}
	res := vm.chamaMetodoSincronoCru(m, cf)
	if e, ok := res.(*object.Erro); ok && !e.Handled {
		e.Stack = append([]object.StackFrame{{Funcao: m.Nome, Line: linha}}, e.Stack...)
		panic(VMError{err: e})
	}
	if s, ok := res.(*object.Sair); ok {
		panic(VMError{sai: s})
	}
	if msg := object.ConfereItera(inst, res); msg != "" {
		panic(VMError{err: &object.Erro{Message: msg, Kind: "runtime"}})
	}
	return res
}

// chamaMetodoSincronoCru roda o metodo sem argumento (so o receiver; os
// padroes viram nada pro prologo trocar) e devolve o resultado sem panicar.
func (vm *VM) chamaMetodoSincronoCru(m *object.MetodoLigado, cf *object.CompiledFunction) object.Object {
	args := make([]object.Object, cf.NumArgs)
	args[0] = m.Receptor
	for k := 1; k < cf.NumArgs; k++ {
		args[k] = NADA
	}
	if cf.Variadic && cf.NumArgs > 1 {
		args[cf.NumArgs-1] = object.NovaLista([]object.Object{})
	}
	return vm.chamaCompilada(cf, args)
}

// panicFalha sobe o erro/sai() que o corpo do gerador levantou, como se
// tivesse saido de uma gambiarra chamada aqui.
func panicFalha(falha object.Object) {
	switch f := falha.(type) {
	case *object.Sair:
		panic(VMError{sai: f})
	case *object.Erro:
		panic(VMError{err: f})
	}
	panic(VMError{err: &object.Erro{Message: fmt.Sprintf("falha estranha no gerador: %s", falha.Inspect()), Kind: "runtime"}})
}
