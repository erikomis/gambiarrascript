//go:build gsdebugpilha

package vm

import "fmt"

// checaPilha liga a conferencia do teto da pilha a cada push. So com a tag:
//
//	go test -tags gsdebugpilha ./...
//
// Garante que o MaxStack que o compilador calculou nunca e ultrapassado de
// verdade (nao so que a pilha coube porque um frame mais fundo ja tinha
// reservado espaco de sobra).
const checaPilha = true

// confereTeto estoura (panic do Go, nao erro do programa) se o push vai passar
// do teto do frame atual. Frame sem teto (MaxStack 0, caminho checado) nao tem
// o que conferir.
func (vm *VM) confereTeto() {
	if vm.framesIdx == 0 {
		return // retorno do frame raiz: o valor cai no slot dele
	}
	fr := vm.frames[vm.framesIdx-1]
	if fr.fn.MaxStack == 0 {
		return
	}
	teto := fr.basePointer + fr.fn.NumLocals + fr.fn.MaxStack
	if vm.sp >= teto {
		panic(fmt.Sprintf("gsdebugpilha: push passou do teto em %s (ip %d): sp=%d teto=%d (bp=%d locals=%d MaxStack=%d)",
			fr.fn.Name, fr.ip, vm.sp, teto, fr.basePointer, fr.fn.NumLocals, fr.fn.MaxStack))
	}
}
