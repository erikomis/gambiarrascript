package vm

import "testing"

// Default params e varargs devem funcionar identico nos 2 engines. Regressao:
// OpClosure perdia MinArgs/Variadic ao criar a closure, quebrando a VM.
func TestVMDefaultParam(t *testing.T) {
	comparaEngines(t, `gambiarra f(x, y = 10)
    funciona x + y
acabou_finalmente
mostra f(5)`)
	comparaEngines(t, `gambiarra f(x, y = 10)
    funciona x + y
acabou_finalmente
mostra f(5, 100)`)
}

func TestVMVarargs(t *testing.T) {
	comparaEngines(t, `gambiarra f(a, ...resto)
    funciona resto
acabou_finalmente
mostra f(1, 2, 3)`)
	comparaEngines(t, `gambiarra f(a, ...resto)
    funciona resto
acabou_finalmente
mostra f(9)`)
}
