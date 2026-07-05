package vm

import (
	"fmt"
	"io"
	"math"
	"strings"

	"gambiarrascript/code"
	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/object"
)

const (
	StackSize  = 16384
	MaxFrames  = 1024
	MaxGlobals = 65536
)

var (
	DEU_BOM  = &object.Booleano{Value: true}
	DEU_RUIM = &object.Booleano{Value: false}
	NADA     = &object.Nada{}
)

type Frame struct {
	fn          *object.CompiledFunction
	ip          int
	basePointer int
	// callPos e o offset do OpCall no bytecode do frame PAI — usado pra
	// resolver a linha do call site ao montar o traço de pilha (lazy, so em
	// caminho de erro). 0 no frame raiz.
	callPos int
}

// tryHandler e uma entrada da pilha de arruma/quebrou: o endereco do catch e
// a PROFUNDIDADE de frames (framesIdx) em que o OpTry rodou — necessario pra
// desempilhar frames ate o dono do try quando o erro estoura em funcao
// chamada dentro do bloco.
type tryHandler struct {
	catchAddr int
	frameIdx  int
}

type VM struct {
	constants []object.Object
	inst      code.Instructions
	linhas    []object.LinhaPC // tabela pc->linha do fluxo principal
	stack     []object.Object
	sp        int
	globals   []object.Object
	frames    []*Frame
	framesIdx int

	// erros: pilha de handlers de arruma/quebrou. Throw pega o mais interno.
	errStack []tryHandler

	out io.Writer

	builtinIdx map[string]int
	builtins   map[string]*object.Builtin
}

func New(bytecode *compiler.Bytecode, out io.Writer) *VM {
	bidx := map[string]int{}
	for i, n := range compiler.BuiltinNomes() {
		bidx[n] = i
	}
	interp := interpreter.New(out)
	vm := &VM{
		constants:  bytecode.Constants,
		inst:       bytecode.Instructions,
		linhas:     bytecode.Linhas,
		stack:      make([]object.Object, StackSize),
		globals:    make([]object.Object, MaxGlobals),
		frames:     make([]*Frame, MaxFrames),
		builtinIdx: bidx,
		builtins:   interp.BuiltinsVisiveis(),
		out:        out,
	}
	// gancho: os builtins de ordem superior (mapeia, filtra, reduz...) vem do
	// interpreter e chamam applyFunction — que delega pra ca quando a funcao
	// do usuario e uma CompiledFunction (bytecode).
	interp.ChamaCompilada = vm.chamaCompilada
	return vm
}

// chamaCompilada executa uma CompiledFunction de forma SINCRONA numa VM
// clonada (compartilha globals/constants/builtins). Erros de runtime viram
// *object.Erro (os builtins ja propagam via isError).
func (vm *VM) chamaCompilada(cf *object.CompiledFunction, args []object.Object) (res object.Object) {
	if len(args) != cf.NumArgs {
		return &object.Erro{
			Message: fmt.Sprintf("essa gambiarra quer %d parametro(s), voce mandou %d", cf.NumArgs, len(args)),
			Kind:    "runtime",
		}
	}
	clone := vm.clone()
	for i, a := range args {
		clone.stack[i] = a
	}
	// reserva os slots de locals (igual OpCall)
	clone.sp = cf.NumLocals
	if clone.sp < len(args) {
		clone.sp = len(args)
	}
	clone.frames[0] = &Frame{fn: cf, ip: 0, basePointer: 0}
	clone.framesIdx = 1
	defer func() {
		if r := recover(); r != nil {
			if vme, ok := r.(VMError); ok {
				res = vme.err
				return
			}
			res = &object.Erro{Message: fmt.Sprintf("panico na gambiarra: %v", r), Kind: "runtime"}
		}
	}()
	if err := clone.execFrame(clone.currentFrame()); err != nil {
		if enc, ok := err.(erroNaoCapturado); ok {
			return enc.err // preserva Line/Kind do erro original
		}
		return &object.Erro{Message: err.Error(), Kind: "runtime"}
	}
	// apos OpReturn/OpReturnNada o valor fica em stack[sp]
	clone.sp--
	return clone.stack[clone.sp]
}

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

func (vm *VM) push(o object.Object) { vm.stack[vm.sp] = o; vm.sp++ }
func (vm *VM) pop() object.Object   { vm.sp--; return vm.stack[vm.sp] }

func (vm *VM) currentFrame() *Frame { return vm.frames[vm.framesIdx-1] }
func (vm *VM) pushFrame(f *Frame)   { vm.frames[vm.framesIdx] = f; vm.framesIdx++ }
func (vm *VM) popFrame() *Frame     { vm.framesIdx--; return vm.frames[vm.framesIdx] }

// clone devolve uma VM nova pronta pra rodar em goroutine: compartilha
// constants/globals/builtins/out com a original (igual o tree-walker, que
// compartilha o mesmo Environment), mas tem stack/frames proprios.
func (vm *VM) clone() *VM {
	return &VM{
		constants:  vm.constants,
		inst:       vm.inst,
		linhas:     vm.linhas,
		stack:      make([]object.Object, StackSize),
		sp:         0,
		globals:    vm.globals, // slice compartilhado — pagadores por concorrencia
		frames:     make([]*Frame, MaxFrames),
		builtinIdx: vm.builtinIdx,
		builtins:   vm.builtins,
		out:        vm.out,
	}
}

// execBoraCall dispara a chamada (callee + args ja empilhados) numa goroutine
// em VM separada e empurra o *Futuro correspondente na pilha da VM atual.
// args layout: stack[sp-1-argc .. sp-2] sao args; stack[sp-1] e o callee.
func (vm *VM) execBoraCall(argc int) {
	callee := vm.stack[vm.sp-1]
	args := make([]object.Object, argc)
	copy(args, vm.stack[vm.sp-1-argc:vm.sp-1])
	vm.sp -= argc + 1

	fut := object.NovoFuturo()
	switch fn := callee.(type) {
	case *object.CompiledFunction:
		clone := vm.clone()
		// monta o frame inicial: args entram como locals a partir de bp=0
		for i, a := range args {
			clone.stack[i] = a
		}
		// reserva os slots de locals (igual OpCall) pra pilha de trabalho nao
		// pisar em cima de local do corpo.
		clone.sp = fn.NumLocals
		if clone.sp < len(args) {
			clone.sp = len(args)
		}
		frame := &Frame{fn: fn, ip: 0, basePointer: 0}
		clone.frames[0] = frame
		clone.framesIdx = 1
		go func(c *VM, f *object.Futuro) {
			defer func() {
				if r := recover(); r != nil {
					if vme, ok := r.(VMError); ok {
						f.Resolve(vme.err)
						return
					}
					f.Resolve(&object.Erro{Message: fmt.Sprintf("panico dentro do `bora`: %v", r), Kind: "runtime"})
				}
			}()
			if err := c.execFrame(c.currentFrame()); err != nil {
				if enc, ok := err.(erroNaoCapturado); ok {
					f.Resolve(enc.err) // preserva Line/Kind do erro original
					return
				}
				f.Resolve(&object.Erro{Message: err.Error(), Kind: "runtime"})
				return
			}
			// apos OpReturn, valor fica em stack[sp]
			c.sp--
			f.Resolve(c.stack[c.sp])
		}(clone, fut)
	case *object.Builtin:
		go func(f *object.Futuro, b *object.Builtin, argv []object.Object) {
			defer func() {
				if r := recover(); r != nil {
					f.Resolve(&object.Erro{Message: fmt.Sprintf("panico dentro do `bora`: %v", r), Kind: "runtime"})
				}
			}()
			f.Resolve(b.Fn(argv))
		}(fut, fn, args)
	default:
		panic(VMError{err: &object.Erro{Message: fmt.Sprintf("bora: nao da pra chamar %s", callee.Type()), Kind: "runtime"}})
	}
	vm.push(fut)
}

// Run executa o bytecode. frame e ip reciclados entre chamadas via execFrame.
func (vm *VM) Run() error {
	main := &object.CompiledFunction{Name: "<main>", Bytecode: vm.inst, NumLocals: 0, Linhas: vm.linhas}
	vm.frames[0] = &Frame{fn: main, ip: 0, basePointer: 0}
	vm.framesIdx = 1

	err := vm.execFrame(vm.frames[0])
	// Programa que termina via `funciona` no top-level sai por OpReturn: o valor
	// fica em stack[sp-1] (push) e o frame principal e desempilhado (framesIdx=0).
	// O fim normal (fallthrough/OpPop) deixa o valor em stack[sp], onde
	// LastPoppedStackElem le. Ajusta o sp pra os dois casos convergirem.
	if err == nil && vm.framesIdx == 0 {
		vm.sp--
	}
	return err
}

// erroNaoCapturado e o Go error devolvido quando um erro de runtime da VM
// estoura sem handler. Carrega o *object.Erro original (com Line/Kind/...)
// pra quem chamou (chamaCompilada, execBoraCall) nao perder a posicao.
type erroNaoCapturado struct{ err *object.Erro }

func (e erroNaoCapturado) Error() string { return e.err.Message }

// ErroDoRun extrai o *object.Erro de um erro devolvido por Run (nil se o
// erro nao veio do runtime do script). O CLI usa pra imprimir o traço de
// pilha igual o tree-walker.
func ErroDoRun(err error) *object.Erro {
	if enc, ok := err.(erroNaoCapturado); ok {
		return enc.err
	}
	return nil
}

// execFrame executa a partir de um frame ate o retorno dele (ou OpHalt).
// Erros propagam como VMError ate achar um handler (OpTry) ou ate o top
// (Run devolve como Go error).
func (vm *VM) execFrame(frame *Frame) error {
	return vm.execDesde(frame, vm.framesIdx)
}

// execDesde e o loop ITERATIVO da VM: OpCall/OpReturn trocam o frame local
// sem recursao Go, e a execucao devolve o controle quando framesIdx cair
// abaixo de baseIdx (o frame que iniciou a invocacao retornou). O baseIdx e
// repassado no resume pos-catch — um catch dentro de funcao continua depois
// no chamador, nao para no retorno da funcao.
func (vm *VM) execDesde(frame *Frame, baseIdx int) (errRet error) {
	// Recupera panics transformando em VMError — toda construcao de erro
	// runtime usa panic(VMError{...}) por simplicidade.
	defer func() {
		if r := recover(); r != nil {
			if vme, ok := r.(VMError); ok {
				// amarra a linha do fonte (tabela pc->linha) e formata a
				// mensagem igual o tree-walker ("deu ruim na linha N: ...").
				// So pra erro cru de runtime: builtins ja vem formatados.
				// `frame` e capturado por referencia: aponta pro frame que
				// estava rodando na hora do panic (o loop reatribui a var).
				// Erro cru de runtime (sem linha e sem prefixo) ganha a linha e o
				// prefixo aqui. Erros que ja vem formatados de um builtin (ex.:
				// funcao chamada dentro de reduz/mapeia) comecam com "deu ruim"
				// e NAO devem ser re-prefixados — senao vira "deu ruim ... deu ruim".
				if vme.err.Line == 0 && vme.err.Kind == "runtime" && !strings.HasPrefix(vme.err.Message, "deu ruim") {
					if l := frame.fn.LinhaDoPC(frame.ip); l > 0 {
						vme.err.Line = l
						vme.err.Message = fmt.Sprintf("deu ruim na linha %d: %s", l, vme.err.Message)
					}
				}
				vm.handleVMError(vme.err)
				// apos handle: ou temos handler (continua) ou propaga
				errRet = nil
				// se ainda ha erro pendente (sem handler), sinalizamos
				if vm.framesIdx == 0 {
					// top-level sem handler: erro fatal
					errRet = erroNaoCapturado{err: vme.err}
					return
				}
				// handler achado — resume no frame que registrou o try (o
				// unwinding ja desempilhou os intermediarios), MANTENDO o
				// baseIdx original desta invocacao.
				errRet = vm.execDesde(vm.currentFrame(), baseIdx)
				return
			}
			panic(r)
		}
	}()

	fn := frame.fn
	ip := frame.ip
	for ip < len(fn.Bytecode) {
		frame.ip = ip // sync pro recover saber onde o erro estourou
		op := code.Opcode(fn.Bytecode[ip])
		switch op {
		case code.OpConstant:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			vm.push(vm.constants[idx])
		case code.OpPop:
			vm.pop()
			ip++
		case code.OpHalt:
			return nil
		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod,
			code.OpBAnd, code.OpBOr, code.OpBXor, code.OpLShift, code.OpRShift:
			vm.execBinario(op)
			ip++
		case code.OpTrue:
			vm.push(DEU_BOM)
			ip++
		case code.OpFalse:
			vm.push(DEU_RUIM)
			ip++
		case code.OpNada:
			vm.push(NADA)
			ip++
		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan, code.OpGreaterEqual, code.OpMenor, code.OpMenorEqual:
			vm.execComparacao(op)
			ip++
		case code.OpMinus:
			vm.execMinus()
			ip++
		case code.OpNao:
			vm.push(boolNativo(!ehVerdade(vm.pop())))
			ip++
		case code.OpBNot:
			o := vm.pop()
			n, ok := o.(*object.Numero)
			if !ok || !n.EhInt {
				panic(VMError{err: &object.Erro{Message: "~ espera inteiro", Kind: "runtime"}})
			}
			vm.push(object.NumInt(^n.Int))
			ip++
		case code.OpMostra:
			fmt.Fprintln(vm.out, vm.pop().Inspect())
			ip++
		case code.OpGetGlobal:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			vm.push(vm.globals[idx])
		case code.OpSetGlobal:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			vm.globals[idx] = vm.pop()
		case code.OpJump:
			pos := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip = pos
		case code.OpJumpIfFalse:
			pos := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			val := vm.pop()
			if !ehVerdade(val) {
				ip = pos
			} else {
				ip += 3
			}
		case code.OpJumpIfTrue:
			pos := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			val := vm.pop()
			if ehVerdade(val) {
				ip = pos
			} else {
				ip += 3
			}
		case code.OpArray:
			n := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			elems := make([]object.Object, n)
			copy(elems, vm.stack[vm.sp-n:vm.sp])
			vm.sp -= n
			vm.push(&object.Lista{Elements: elems})
		case code.OpHash:
			n := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			pares := map[object.HashKey]object.ParDic{}
			base := vm.sp - 2*n
			for i := 0; i < n; i++ {
				chave := vm.stack[base+2*i]
				valor := vm.stack[base+2*i+1]
				c, ok := chave.(object.Chaveavel)
				if !ok {
					panic(VMError{err: &object.Erro{Message: "chave de dicionario inaceitavel: " + string(chave.Type()), Kind: "runtime"}})
				}
				pares[c.ChaveHash()] = object.ParDic{Chave: chave, Valor: valor}
			}
			vm.sp = base
			vm.push(&object.Dicionario{Pares: pares})
		case code.OpIndex:
			idx := vm.pop()
			cont := vm.pop()
			r, perr := vmIndex(cont, idx)
			if perr != nil {
				panic(VMError{err: &object.Erro{Message: perr.Error(), Kind: "runtime"}})
			}
			vm.push(r)
			ip++
		case code.OpIndexSet:
			val := vm.pop()
			idx := vm.pop()
			cont := vm.pop()
			if perr := vmIndexSet(cont, idx, val); perr != nil {
				panic(VMError{err: &object.Erro{Message: perr.Error(), Kind: "runtime"}})
			}
			// `bota d[k] = v` e statement: nao deixa valor na pilha (igual o
			// caminho `bota nome = v`, que o OpSetGlobal/Local consome).
			ip++
		case code.OpRange:
			hi := vm.pop()
			lo := vm.pop()
			ln, lok := lo.(*object.Numero)
			hn, hok := hi.(*object.Numero)
			if !lok || !ln.EhInt || !hok || !hn.EhInt {
				panic(VMError{err: &object.Erro{Message: "range .. quer inteiros dos dois lados", Kind: "runtime"}})
			}
			elems, ok := object.RangeInts(ln.Int, hn.Int)
			if !ok {
				panic(VMError{err: &object.Erro{Message: fmt.Sprintf("range .. de %d..%d e gigante demais", ln.Int, hn.Int), Kind: "runtime"}})
			}
			vm.push(&object.Lista{Elements: elems})
			ip++
		case code.OpIndexOuNada:
			idx := vm.pop()
			cont := vm.pop()
			switch cc := cont.(type) {
			case *object.Lista:
				if n, ok := idx.(*object.Numero); ok && n.EhInt && n.Int >= 0 && int(n.Int) < len(cc.Elements) {
					vm.push(cc.Elements[n.Int])
				} else {
					vm.push(NADA)
				}
			case *object.Dicionario:
				if ch, ok := idx.(object.Chaveavel); ok {
					if par, existe := cc.Pares[ch.ChaveHash()]; existe {
						vm.push(par.Valor)
					} else {
						vm.push(NADA)
					}
				} else {
					vm.push(NADA)
				}
			default:
				panic(VMError{err: &object.Erro{Message: fmt.Sprintf("so da pra desestruturar lista ou dicionario, veio %s", cont.Type()), Kind: "runtime"}})
			}
			ip++
		case code.OpIterSeq:
			it := vm.pop()
			switch c := it.(type) {
			case *object.Lista:
				vm.push(c)
			case *object.Dicionario:
				chaves := make([]object.Object, 0, len(c.Pares))
				for _, par := range c.Pares {
					chaves = append(chaves, par.Chave)
				}
				vm.push(&object.Lista{Elements: chaves})
			default:
				panic(VMError{err: &object.Erro{Message: fmt.Sprintf("pra_cada ... em ... so funciona com lista ou dicionario, e isso ai e %s", it.Type()), Kind: "runtime"}})
			}
			ip++
		case code.OpGetLocal:
			idx := int(fn.Bytecode[ip+1])
			ip += 2
			vm.push(vm.stack[frame.basePointer+idx])
		case code.OpSetLocal:
			idx := int(fn.Bytecode[ip+1])
			ip += 2
			vm.stack[frame.basePointer+idx] = vm.pop()
		case code.OpGetFree:
			idx := int(fn.Bytecode[ip+1])
			ip += 2
			if idx >= len(fn.Free) {
				panic(VMError{err: &object.Erro{Message: "freevar fora do range", Kind: "runtime"}})
			}
			vm.push(fn.Free[idx])
		case code.OpClosure:
			constIdx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			numFree := int(fn.Bytecode[ip+3])
			ip += 4
			cf, ok := vm.constants[constIdx].(*object.CompiledFunction)
			if !ok {
				panic(VMError{err: &object.Erro{Message: "OpClosure nao aponta pra CompiledFunction", Kind: "runtime"}})
			}
			// popula freevars com os ultimos `numFree` valores da pilha
			// (empilhados pelo compiler antes de emitir OpClosure).
			var free []object.Object
			if numFree > 0 {
				free = make([]object.Object, numFree)
				copy(free, vm.stack[vm.sp-numFree:vm.sp])
				vm.sp -= numFree
			}
			vm.push(&object.CompiledFunction{
				Name: cf.Name, NumArgs: cf.NumArgs, NumLocals: cf.NumLocals,
				Bytecode: cf.Bytecode, Free: free, Linhas: cf.Linhas,
			})
		case code.OpCall:
			opPos := ip // offset do OpCall (call site) pro traço de pilha
			argc := int(fn.Bytecode[ip+1])
			ip += 2
			callee := vm.stack[vm.sp-1]
			if cf, ok := callee.(*object.CompiledFunction); ok {
				if argc != cf.NumArgs {
					panic(VMError{err: &object.Erro{Message: fmt.Sprintf("essa gambiarra quer %d parametro(s), voce mandou %d", cf.NumArgs, argc), Kind: "runtime"}})
				}
				// bp = endereco do arg0 (local 0). args em stack[sp-1-argc .. sp-2].
				bp := vm.sp - 1 - argc
				newFrame := &Frame{fn: cf, ip: 0, basePointer: bp, callPos: opPos}
				// reserva os slots de locals (params + `bota`/vars sinteticos como
				// os do `pra_cada em`): a pilha de trabalho comeca ACIMA deles,
				// senao um push do corpo sobrescreve um local. OpReturn volta sp
				// pra bp, entao aqui subimos ate bp+NumLocals.
				vm.sp = bp + cf.NumLocals
				frame.ip = ip // ponto de retomada quando a funcao retornar
				vm.pushFrame(newFrame)
				// troca ITERATIVA de frame (sem recursao Go): o unwinding de
				// erro pode pular varios frames de uma vez sem deixar
				// invocacoes pendentes retomando codigo ja abandonado.
				frame = newFrame
				fn = cf
				ip = 0
				continue
			}
			if b, ok := callee.(*object.Builtin); ok {
				args := make([]object.Object, argc)
				// args em stack[sp-1-argc .. sp-2]
				copy(args, vm.stack[vm.sp-1-argc:vm.sp-1])
				vm.sp -= argc + 1 // popa args + callee
				res := b.Fn(args)
				if e, ok := res.(*object.Erro); ok && e != nil && !e.Handled {
					panic(VMError{err: e})
				}
				vm.push(res)
				continue
			}
			panic(VMError{err: &object.Erro{Message: fmt.Sprintf("isso ai (%s) nao e gambiarra pra voce sair chamando", callee.Type()), Kind: "runtime"}})
		case code.OpCallBuiltin:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			argc := int(fn.Bytecode[ip+3])
			ip += 4
			args := make([]object.Object, argc)
			copy(args, vm.stack[vm.sp-argc:vm.sp])
			vm.sp -= argc
			nome := compiler.BuiltinNomes()[idx]
			b := vm.builtins[nome]
			if b == nil {
				panic(VMError{err: &object.Erro{Message: "builtin " + nome + " nao registrada na VM", Kind: "runtime"}})
			}
			res := b.Fn(args)
			if e, ok := res.(*object.Erro); ok && e != nil && !e.Handled {
				panic(VMError{err: e})
			}
			vm.push(res)
		case code.OpReturn:
			val := vm.pop()
			returnedFn := vm.popFrame()
			vm.sp = returnedFn.basePointer
			vm.push(val)
			vm.limpaTriesOrfaos()
			if vm.framesIdx < baseIdx {
				return nil // o frame que esta invocacao comecou retornou
			}
			frame = vm.currentFrame()
			fn = frame.fn
			ip = frame.ip
		case code.OpReturnNada:
			returnedFn := vm.popFrame()
			vm.sp = returnedFn.basePointer
			vm.push(NADA)
			vm.limpaTriesOrfaos()
			if vm.framesIdx < baseIdx {
				return nil
			}
			frame = vm.currentFrame()
			fn = frame.fn
			ip = frame.ip
		case code.OpBoraCall:
			argc := int(fn.Bytecode[ip+1])
			ip += 2
			vm.execBoraCall(argc)
		case code.OpGetBuiltin:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			nome := compiler.BuiltinNomes()[idx]
			b := vm.builtins[nome]
			if b == nil {
				panic(VMError{err: &object.Erro{Message: "builtin " + nome + " nao registrada", Kind: "runtime"}})
			}
			vm.push(b)
		case code.OpThrow:
			val := vm.pop()
			e, ok := val.(*object.Erro)
			if !ok {
				panic(VMError{err: &object.Erro{Message: "so da pra jogar Erro, veio " + string(val.Type()), Kind: "runtime"}})
			}
			panic(VMError{err: e})
		case code.OpTry:
			catchAddr := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			vm.errStack = append(vm.errStack, tryHandler{catchAddr: catchAddr, frameIdx: vm.framesIdx})
		case code.OpTryEnd:
			if len(vm.errStack) > 0 {
				vm.errStack = vm.errStack[:len(vm.errStack)-1]
			}
			ip++
		default:
			return fmt.Errorf("opcode desconhecido: %d", op)
		}
	}
	// fallthrough: frame esgotou sem return
	return nil
}

// handleVMError recebe um erro runtime: monta o traço de pilha (call sites
// dos frames entre o try e o ponto do erro), acha o handler mais interno e
// desempilha frames ate o dono do try (unwinding real — o erro pode ter
// estourado em funcao chamada dentro do bloco arruma). Sem handler:
// desempilha tudo e marca framesIdx=0 (erro nao capturado).
func (vm *VM) handleVMError(e *object.Erro) {
	// profundidade do try mais interno (1 = frame raiz, quando nao ha try:
	// traço cobre todos os frames alem do raiz)
	inicio := 1
	if len(vm.errStack) > 0 {
		inicio = vm.errStack[len(vm.errStack)-1].frameIdx
	}
	// traço externo->interno (igual o tree-walker): frames[j] foi chamado de
	// frames[j-1] no offset callPos — a linha vem da tabela do PAI. So
	// preenche uma vez (o erro pode re-propagar depois de capturado).
	if len(e.Stack) == 0 {
		for j := inicio; j < vm.framesIdx; j++ {
			e.Stack = append(e.Stack, object.StackFrame{
				Funcao: vm.frames[j].fn.Name,
				Line:   vm.frames[j-1].fn.LinhaDoPC(vm.frames[j].callPos),
			})
		}
	}

	if len(vm.errStack) == 0 {
		// sem handler: destroi frames e marca erro nao capturado
		vm.framesIdx = 0
		return
	}
	h := vm.errStack[len(vm.errStack)-1]
	vm.errStack = vm.errStack[:len(vm.errStack)-1]
	// desempilha frames ate o que registrou o try
	for vm.framesIdx > h.frameIdx {
		vm.popFrame()
	}
	alvo := vm.currentFrame()
	// descarta operandos pendentes e restabelece o espaco de locals
	vm.sp = alvo.basePointer + alvo.fn.NumLocals
	vm.push(e)
	alvo.ip = h.catchAddr
}

// limpaTriesOrfaos descarta handlers de try registrados por frames que ja
// retornaram (um `funciona` dentro de `arruma` sai da funcao sem passar pelo
// OpTryEnd). Sem isso, um erro futuro saltaria pra um catchAddr de bytecode
// de outro frame.
func (vm *VM) limpaTriesOrfaos() {
	for len(vm.errStack) > 0 && vm.errStack[len(vm.errStack)-1].frameIdx > vm.framesIdx {
		vm.errStack = vm.errStack[:len(vm.errStack)-1]
	}
}

type VMError struct{ err *object.Erro }

func (v VMError) Error() string { return v.err.Message }

func (vm *VM) execBinario(op code.Opcode) {
	right := vm.pop()
	left := vm.pop()
	ln, lok := left.(*object.Numero)
	rn, rok := right.(*object.Numero)
	if lok && rok {
		// bitwise so faz sentido com inteiros; tratamos antes do fast-path
		// float pra nao contaminar caminho aritmetico. Igual ao tree-walker,
		// operando nao-inteiro num op bitwise e erro (nao cai no caminho float).
		if ehBitwise(op) {
			if !ln.EhInt || !rn.EhInt {
				msg := simboloBinario(op) + " bitwise so faz sentido com inteiros"
				if ehShift(op) {
					msg = "shift so faz sentido com inteiros"
				}
				panic(VMError{err: &object.Erro{Message: msg, Kind: "runtime"}})
			}
			r, _ := vmExecBinarioBitwise(op, ln.Int, rn.Int)
			vm.push(r)
			return
		}
		// fast path inteiros exatos
		if r, ok := vmExecBinarioIntShort(op, ln, rn); ok {
			vm.push(r)
			return
		}
		vm.push(vmExecBinarioNumero(op, ln.Value, rn.Value))
		return
	}
	if op == code.OpAdd && (left.Type() == object.TEXTO_OBJ || right.Type() == object.TEXTO_OBJ) {
		vm.push(&object.Texto{Value: left.Inspect() + right.Inspect()})
		return
	}
	// mesma mensagem do tree-walker: "nao da pra fazer TEXTO - NUMERO"
	panic(VMError{err: &object.Erro{Message: fmt.Sprintf("nao da pra fazer %s %s %s", left.Type(), simboloBinario(op), right.Type()), Kind: "runtime"}})
}

// simboloBinario devolve o simbolo textual do operador aritmetico/bitwise, pra
// que as mensagens de erro da VM fiquem identicas as do interpretador (que usa
// node.Operator).
func simboloBinario(op code.Opcode) string {
	switch op {
	case code.OpAdd:
		return "+"
	case code.OpSub:
		return "-"
	case code.OpMul:
		return "*"
	case code.OpDiv:
		return "/"
	case code.OpMod:
		return "%"
	case code.OpBAnd:
		return "&"
	case code.OpBOr:
		return "|"
	case code.OpBXor:
		return "^"
	case code.OpLShift:
		return "<<"
	case code.OpRShift:
		return ">>"
	case code.OpGreaterThan:
		return ">"
	case code.OpGreaterEqual:
		return ">="
	case code.OpMenor:
		return "<"
	case code.OpMenorEqual:
		return "<="
	}
	return "?"
}

// ehBitwise diz se o opcode e uma operacao bitwise (&, |, ^, <<, >>).
func ehBitwise(op code.Opcode) bool {
	switch op {
	case code.OpBAnd, code.OpBOr, code.OpBXor, code.OpLShift, code.OpRShift:
		return true
	}
	return false
}

// ehShift diz se o opcode e um deslocamento (<< ou >>).
func ehShift(op code.Opcode) bool {
	return op == code.OpLShift || op == code.OpRShift
}

func vmExecBinarioNumero(op code.Opcode, l, r float64) object.Object {
	if op == code.OpDiv && r == 0 {
		panic(VMError{err: &object.Erro{Message: "nao da pra dividir por zero, parca — nem na gambiarra", Kind: "runtime"}})
	}
	if op == code.OpMod && r == 0 {
		panic(VMError{err: &object.Erro{Message: "resto de divisao por zero? ai voce quer demais", Kind: "runtime"}})
	}
	var res float64
	switch op {
	case code.OpAdd:
		res = l + r
	case code.OpSub:
		res = l - r
	case code.OpMul:
		res = l * r
	case code.OpDiv:
		res = l / r
	case code.OpMod:
		res = math.Mod(l, r)
	}
	return &object.Numero{Value: res}
}

// vmExecBinarioIntShort usa aritmetica int64 exata quando AMBOS operandos
// sao inteiros exatos (EhInt=true) e o operador e inteiro-aware (+,-,*,%).
// Divisao continua float (pois pode dar nao-inteiro). Cresce pra int128
// apenas no limite via overflow detection: se passa int64, cai pra float64.
func vmExecBinarioIntShort(op code.Opcode, lo, ro *object.Numero) (object.Object, bool) {
	if !lo.EhInt || !ro.EhInt {
		return nil, false
	}
	switch op {
	case code.OpAdd:
		// deteccao de overflow simples: se sinais iguais e resultado estoura
		r := lo.Int + ro.Int
		if (lo.Int > 0 && ro.Int > 0 && r < 0) || (lo.Int < 0 && ro.Int < 0 && r > 0) {
			return nil, false
		}
		return object.NumInt(r), true
	case code.OpSub:
		r := lo.Int - ro.Int
		if (lo.Int > 0 && ro.Int < 0 && r < 0) || (lo.Int < 0 && ro.Int > 0 && r > 0) {
			return nil, false
		}
		return object.NumInt(r), true
	case code.OpMul:
		// deteccao simples: se |lo|*|ro| > max int64
		if lo.Int == 0 || ro.Int == 0 {
			return object.NumInt(0), true
		}
		r := lo.Int * ro.Int
		if r/ro.Int != lo.Int {
			return nil, false
		}
		return object.NumInt(r), true
	}
	return nil, false
}

func (vm *VM) execComparacao(op code.Opcode) {
	right := vm.pop()
	left := vm.pop()
	ln, lok := left.(*object.Numero)
	rn, rok := right.(*object.Numero)
	if lok && rok {
		switch op {
		case code.OpGreaterThan:
			vm.push(boolNativo(ln.Value > rn.Value))
		case code.OpGreaterEqual:
			vm.push(boolNativo(ln.Value >= rn.Value))
		case code.OpMenor:
			vm.push(boolNativo(ln.Value < rn.Value))
		case code.OpMenorEqual:
			vm.push(boolNativo(ln.Value <= rn.Value))
		case code.OpEqual:
			vm.push(boolNativo(ln.Value == rn.Value))
		case code.OpNotEqual:
			vm.push(boolNativo(ln.Value != rn.Value))
		}
		return
	}
	switch op {
	case code.OpEqual:
		vm.push(boolNativo(iguais(left, right)))
	case code.OpNotEqual:
		vm.push(boolNativo(!iguais(left, right)))
	default:
		// mesma mensagem do tree-walker: "nao da pra fazer TEXTO > NUMERO"
		panic(VMError{err: &object.Erro{Message: fmt.Sprintf("nao da pra fazer %s %s %s", left.Type(), simboloBinario(op), right.Type()), Kind: "runtime"}})
	}
}

func (vm *VM) execMinus() {
	o := vm.pop()
	if n, ok := o.(*object.Numero); ok {
		// preserva a inteireza exata: -1 tem que continuar EhInt, senao vira
		// float e escapa de checagens como o shift por valor negativo.
		if n.EhInt {
			vm.push(object.NumInt(-n.Int))
		} else {
			vm.push(&object.Numero{Value: -n.Value})
		}
		return
	}
	panic(VMError{err: &object.Erro{Message: fmt.Sprintf("nao da pra colocar - na frente de %s", o.Type()), Kind: "runtime"}})
}

// vmExecBinarioBitwise trata operacoes bitwise. Devolve (r, true) se o op
// for bitwise; (nil, false) caso contrario — caller segue o fluxo normal.
func vmExecBinarioBitwise(op code.Opcode, l, r int64) (object.Object, bool) {
	switch op {
	case code.OpBAnd:
		return object.NumInt(l & r), true
	case code.OpBOr:
		return object.NumInt(l | r), true
	case code.OpBXor:
		return object.NumInt(l ^ r), true
	case code.OpLShift:
		if r < 0 {
			panic(VMError{err: &object.Erro{Message: "<< por valor negativo? naoiedade", Kind: "runtime"}})
		}
		return object.NumInt(l << uint(r)), true
	case code.OpRShift:
		if r < 0 {
			panic(VMError{err: &object.Erro{Message: ">> por valor negativo? naoiedade", Kind: "runtime"}})
		}
		return object.NumInt(l >> uint(r)), true
	}
	return nil, false
}

func vmIndex(cont, idx object.Object) (object.Object, error) {
	switch c := cont.(type) {
	case *object.Lista:
		n, ok := idx.(*object.Numero)
		if !ok {
			return nil, fmt.Errorf("indice de lista tem que ser numero")
		}
		pos := int(n.Value)
		if pos < 0 || pos >= len(c.Elements) {
			return nil, fmt.Errorf("esse indice (%d) ta fora da lista, o", pos)
		}
		return c.Elements[pos], nil
	case *object.Dicionario:
		chave, ok := idx.(object.Chaveavel)
		if !ok {
			return nil, fmt.Errorf("chave de dicionario invalida")
		}
		par, existe := c.Pares[chave.ChaveHash()]
		if !existe {
			return NADA, nil
		}
		return par.Valor, nil
	}
	// igual ao tree-walker: so lista e dicionario sao indexaveis (texto nao).
	return nil, fmt.Errorf("so da pra indexar lista ou dicionario, e isso ai e %s", cont.Type())
}

func vmIndexSet(cont, idx, val object.Object) error {
	switch c := cont.(type) {
	case *object.Lista:
		n, ok := idx.(*object.Numero)
		if !ok {
			return fmt.Errorf("indice de lista tem que ser numero")
		}
		pos := int(n.Value)
		if pos < 0 || pos >= len(c.Elements) {
			return fmt.Errorf("esse indice (%d) ta fora da lista, o", pos)
		}
		c.Elements[pos] = val
	case *object.Dicionario:
		chave, ok := idx.(object.Chaveavel)
		if !ok {
			return fmt.Errorf("chave de dicionario invalida")
		}
		c.Pares[chave.ChaveHash()] = object.ParDic{Chave: idx, Valor: val}
	default:
		return fmt.Errorf("nao da pra atribuir indice em %s", cont.Type())
	}
	return nil
}

func boolNativo(b bool) *object.Booleano {
	if b {
		return DEU_BOM
	}
	return DEU_RUIM
}

func ehVerdade(o object.Object) bool {
	switch o := o.(type) {
	case *object.Nada:
		return false
	case *object.Booleano:
		return o.Value
	default:
		return true
	}
}

func iguais(a, b object.Object) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch av := a.(type) {
	case *object.Texto:
		return av.Value == b.(*object.Texto).Value
	case *object.Booleano:
		return av.Value == b.(*object.Booleano).Value
	case *object.Numero:
		return av.Value == b.(*object.Numero).Value
	case *object.Nada:
		return true
	case *object.Lista:
		bl := b.(*object.Lista)
		if len(av.Elements) != len(bl.Elements) {
			return false
		}
		for i, e := range av.Elements {
			if !iguais(e, bl.Elements[i]) {
				return false
			}
		}
		return true
	case *object.Dicionario:
		bd := b.(*object.Dicionario)
		if len(av.Pares) != len(bd.Pares) {
			return false
		}
		for k, pa := range av.Pares {
			pb, ok := bd.Pares[k]
			if !ok || !iguais(pa.Valor, pb.Valor) {
				return false
			}
		}
		return true
	}
	return a == b
}
