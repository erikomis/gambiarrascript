package vm

import (
	"gambiarrascript/code"
	"gambiarrascript/object"
)

// Pattern matching (escolhe/caso) e cardapio na VM. A regra de casar e a de
// object/padrao.go, a mesma do tree-walker.

// execPadrao roda OpCasa/OpAmarrado/OpCardapio e devolve quantos bytes a
// instrucao ocupa (0 = nao e nenhum deles).
func (vm *VM) execPadrao(op code.Opcode, ops []byte) int {
	switch op {
	case code.OpCasa:
		d := vm.constants[code.ReadUint16(ops)].(*object.DescPadrao)
		valores := make([]object.Object, d.NValores)
		copy(valores, vm.stack[vm.sp-d.NValores:vm.sp])
		vm.sp -= d.NValores
		subject := vm.pop()
		amarras := make([]object.Object, len(d.Nomes))
		ok, msg := object.Casa(d, subject, valores, amarras, iguais)
		if msg != "" {
			erroPOO(msg)
		}
		if ok {
			vm.amarras = amarras
		}
		vm.push(boolNativo(ok))
		return 3
	case code.OpAmarrado:
		vm.push(vm.amarras[ops[0]])
		return 2
	case code.OpCardapio:
		d := vm.constants[code.ReadUint16(ops)].(*object.DescCardapio)
		vm.push(object.NovoCardapio(d.Nome, d.Membros))
		return 3
	}
	return 0
}
