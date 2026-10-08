//go:build !gsdebugpilha

package vm

// checaPilha desligado: o `if checaPilha` do push some na compilacao e o push
// continua inlinavel (ver pilha_debug.go).
const checaPilha = false

func (vm *VM) confereTeto() {}
