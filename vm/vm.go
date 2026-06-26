package vm

import (
	"fmt"
	"io"
	"math"

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
}

type VM struct {
	constants []object.Object
	inst      code.Instructions
	stack     []object.Object
	sp        int
	globals   []object.Object
	frames    []*Frame
	framesIdx int

	// erros: pilha de handlers [catchAddr]. Throw percorre ate achar um.
	errStack []int

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
	return &VM{
		constants:  bytecode.Constants,
		inst:       bytecode.Instructions,
		stack:      make([]object.Object, StackSize),
		globals:    make([]object.Object, MaxGlobals),
		frames:     make([]*Frame, MaxFrames),
		builtinIdx: bidx,
		builtins:   interp.BuiltinsVisiveis(),
		out:        out,
	}
}

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

func (vm *VM) push(o object.Object) { vm.stack[vm.sp] = o; vm.sp++ }
func (vm *VM) pop() object.Object   { vm.sp--; return vm.stack[vm.sp] }

func (vm *VM) currentFrame() *Frame { return vm.frames[vm.framesIdx-1] }
func (vm *VM) pushFrame(f *Frame)   { vm.frames[vm.framesIdx] = f; vm.framesIdx++ }
func (vm *VM) popFrame() *Frame      { vm.framesIdx--; return vm.frames[vm.framesIdx] }

// Run executa o bytecode. frame e ip reciclados entre chamadas via execFrame.
func (vm *VM) Run() error {
	main := &object.CompiledFunction{Name: "<main>", Bytecode: vm.inst, NumLocals: 0}
	vm.frames[0] = &Frame{fn: main, ip: 0, basePointer: 0}
	vm.framesIdx = 1

	return vm.execFrame(vm.frames[0])
}

// execFrame executa um frame ate return/throw. Erros propagam como VMError
// ate achar um handler (OpTry) ou ate o top (Run devolve como Go error).
func (vm *VM) execFrame(frame *Frame) (errRet error) {
	// Recupera panics transformando em VMError — toda construcao de erro
	// runtime usa panic(VMError{...}) por simplicidade.
	defer func() {
		if r := recover(); r != nil {
			if vme, ok := r.(VMError); ok {
				vm.handleVMError(vme.err, frame)
				// apos handle: ou temos handler (continua) ou propaga
				errRet = nil
				// se ainda ha erro pendente (sem handler), sinalizamos
				if vm.framesIdx == 0 {
					// top-level sem handler: erro fatal
					errRet = fmt.Errorf("erro nao capturado: %s", vme.err.Message)
					return
				}
				// handler achado — resume no novo frame
				errRet = vm.execFrame(vm.currentFrame())
				return
			}
			panic(r)
		}
	}()

	fn := frame.fn
	ip := frame.ip
	for ip < len(fn.Bytecode) {
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
		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod:
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
			vm.push(NADA)
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
			ip += 3
			cf, ok := vm.constants[constIdx].(*object.CompiledFunction)
			if !ok {
				panic(VMError{err: &object.Erro{Message: "OpClosure nao aponta pra CompiledFunction", Kind: "runtime"}})
			}
			// freevars: ainda nao suportadas (fase futura). cria copia sem free.
			vm.push(&object.CompiledFunction{
				Name: cf.Name, NumArgs: cf.NumArgs, NumLocals: cf.NumLocals,
				Bytecode: cf.Bytecode, Free: nil,
			})
case code.OpCall:
			argc := int(fn.Bytecode[ip+1])
			ip += 2
			callee := vm.stack[vm.sp-1]
			if cf, ok := callee.(*object.CompiledFunction); ok {
				// bp = endereco do arg0 (local 0). args em stack[sp-1-argc .. sp-2].
				bp := vm.sp - 1 - argc
				newFrame := &Frame{fn: cf, ip: 0, basePointer: bp}
				vm.pushFrame(newFrame)
				frame.ip = ip
				if err := vm.execFrame(newFrame); err != nil {
					return err
				}
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
			panic(VMError{err: &object.Erro{Message: fmt.Sprintf("nao da pra chamar %s", callee.Type()), Kind: "runtime"}})
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
			return nil
		case code.OpReturnNada:
			returnedFn := vm.popFrame()
			vm.sp = returnedFn.basePointer
			vm.push(NADA)
			return nil
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
			vm.errStack = append(vm.errStack, catchAddr)
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

// handleVMError recebe um erro runtime e tenta achar um handler de try.
// Se achar: posiciona o frame atual no catchAddr e empurra o erro na pilha.
// Se nao achar: desempilha todos os frames e marca framesIdx=0 (top-level).
func (vm *VM) handleVMError(e *object.Erro, frame *Frame) {
	for len(vm.errStack) > 0 {
		catchAddr := vm.errStack[len(vm.errStack)-1]
		vm.errStack = vm.errStack[:len(vm.errStack)-1]
		// assumimos que o catch esta neste frame (simplificacao valida:
		// OpTry do mesmo frame que ativou o handler).
		// Descarta operandos pendentes e restabelece o espaco de locals.
		vm.sp = frame.basePointer + frame.fn.NumLocals
		vm.push(e)
		frame.ip = catchAddr
		return
	}
	// sem handler: destroi frames e marca erro nao capturado
	vm.framesIdx = 0
}

type VMError struct{ err *object.Erro }

func (v VMError) Error() string { return v.err.Message }

func (vm *VM) execBinario(op code.Opcode) {
	right := vm.pop()
	left := vm.pop()
	ln, lok := left.(*object.Numero)
	rn, rok := right.(*object.Numero)
	if lok && rok {
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
	panic(VMError{err: &object.Erro{Message: fmt.Sprintf("nao da pra operar %s com %s", left.Type(), right.Type()), Kind: "runtime"}})
}

func vmExecBinarioNumero(op code.Opcode, l, r float64) object.Object {
	if op == code.OpDiv && r == 0 {
		panic(VMError{err: &object.Erro{Message: "nao da pra dividir por zero, parca", Kind: "runtime"}})
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
		panic(VMError{err: &object.Erro{Message: fmt.Sprintf("nao da pra comparar %s com %s", left.Type(), right.Type()), Kind: "runtime"}})
	}
}

func (vm *VM) execMinus() {
	o := vm.pop()
	if n, ok := o.(*object.Numero); ok {
		vm.push(&object.Numero{Value: -n.Value})
		return
	}
	panic(VMError{err: &object.Erro{Message: fmt.Sprintf("nao da pra colocar - na frente de %s", o.Type()), Kind: "runtime"}})
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
			return nil, fmt.Errorf("indice %d fora da lista", pos)
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
	case *object.Texto:
		n, ok := idx.(*object.Numero)
		if !ok {
			return nil, fmt.Errorf("indice de texto tem que ser numero")
		}
		pos := int(n.Value)
		runas := []rune(c.Value)
		if pos < 0 || pos >= len(runas) {
			return nil, fmt.Errorf("indice %d fora do texto", pos)
		}
		return &object.Texto{Value: string(runas[pos])}, nil
	}
	return nil, fmt.Errorf("nao da pra indexar %s", cont.Type())
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
			return fmt.Errorf("indice %d fora da lista", pos)
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