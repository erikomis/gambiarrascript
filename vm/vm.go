package vm

import (
	"fmt"
	"io"
	"math"

	"gambiarrascript/code"
	"gambiarrascript/compiler"
	"gambiarrascript/object"
)

const StackSize = 2048

var (
	DEU_BOM  = &object.Booleano{Value: true}
	DEU_RUIM = &object.Booleano{Value: false}
	NADA     = &object.Nada{}
)

type VM struct {
	constants    []object.Object
	instructions code.Instructions
	stack        []object.Object
	sp           int
	out          io.Writer
}

func New(bytecode *compiler.Bytecode, out io.Writer) *VM {
	return &VM{
		instructions: bytecode.Instructions,
		constants:    bytecode.Constants,
		stack:        make([]object.Object, StackSize),
		sp:           0,
		out:          out,
	}
}

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

func (vm *VM) push(o object.Object) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("estourou a pilha (stack overflow)")
	}
	vm.stack[vm.sp] = o
	vm.sp++
	return nil
}

func (vm *VM) pop() object.Object {
	o := vm.stack[vm.sp-1]
	vm.sp--
	return o
}

func (vm *VM) Run() error {
	for ip := 0; ip < len(vm.instructions); ip++ {
		op := code.Opcode(vm.instructions[ip])
		switch op {
		case code.OpConstant:
			idx := code.ReadUint16(vm.instructions[ip+1:])
			ip += 2
			if err := vm.push(vm.constants[idx]); err != nil {
				return err
			}
		case code.OpPop:
			vm.pop()
		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod:
			if err := vm.execBinario(op); err != nil {
				return err
			}
		case code.OpTrue:
			if err := vm.push(DEU_BOM); err != nil {
				return err
			}
		case code.OpFalse:
			if err := vm.push(DEU_RUIM); err != nil {
				return err
			}
		case code.OpNada:
			if err := vm.push(NADA); err != nil {
				return err
			}
		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan, code.OpGreaterEqual:
			if err := vm.execComparacao(op); err != nil {
				return err
			}
		case code.OpMinus:
			if err := vm.execMinus(); err != nil {
				return err
			}
		case code.OpNao:
			if err := vm.execNao(); err != nil {
				return err
			}
		case code.OpMostra:
			fmt.Fprintln(vm.out, vm.pop().Inspect())
		default:
			return fmt.Errorf("opcode desconhecido: %d", op)
		}
	}
	return nil
}

func (vm *VM) execBinario(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()
	ln, lok := left.(*object.Numero)
	rn, rok := right.(*object.Numero)
	if lok && rok {
		return vm.execBinarioNumero(op, ln.Value, rn.Value)
	}
	if op == code.OpAdd && (left.Type() == object.TEXTO_OBJ || right.Type() == object.TEXTO_OBJ) {
		return vm.push(&object.Texto{Value: left.Inspect() + right.Inspect()})
	}
	return fmt.Errorf("nao da pra operar %s com %s", left.Type(), right.Type())
}

func (vm *VM) execBinarioNumero(op code.Opcode, l, r float64) error {
	var res float64
	switch op {
	case code.OpAdd:
		res = l + r
	case code.OpSub:
		res = l - r
	case code.OpMul:
		res = l * r
	case code.OpDiv:
		if r == 0 {
			return fmt.Errorf("nao da pra dividir por zero, parca")
		}
		res = l / r
	case code.OpMod:
		if r == 0 {
			return fmt.Errorf("resto de divisao por zero? ai voce quer demais")
		}
		res = math.Mod(l, r)
	default:
		return fmt.Errorf("operador numerico desconhecido: %d", op)
	}
	return vm.push(&object.Numero{Value: res})
}

func (vm *VM) execComparacao(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()
	ln, lok := left.(*object.Numero)
	rn, rok := right.(*object.Numero)
	if lok && rok {
		switch op {
		case code.OpGreaterThan:
			return vm.push(boolNativo(ln.Value > rn.Value))
		case code.OpGreaterEqual:
			return vm.push(boolNativo(ln.Value >= rn.Value))
		case code.OpEqual:
			return vm.push(boolNativo(ln.Value == rn.Value))
		case code.OpNotEqual:
			return vm.push(boolNativo(ln.Value != rn.Value))
		}
	}
	switch op {
	case code.OpEqual:
		return vm.push(boolNativo(iguais(left, right)))
	case code.OpNotEqual:
		return vm.push(boolNativo(!iguais(left, right)))
	}
	return fmt.Errorf("nao da pra comparar %s com %s", left.Type(), right.Type())
}

func (vm *VM) execMinus() error {
	o := vm.pop()
	n, ok := o.(*object.Numero)
	if !ok {
		return fmt.Errorf("nao da pra colocar - na frente de %s", o.Type())
	}
	return vm.push(&object.Numero{Value: -n.Value})
}

func (vm *VM) execNao() error {
	return vm.push(boolNativo(!ehVerdade(vm.pop())))
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
	}
	return a == b
}
