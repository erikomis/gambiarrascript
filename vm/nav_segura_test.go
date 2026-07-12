package vm

import "testing"

// Navegacao segura (obj?.campo) deve casar nos 2 engines, inclusive no caminho
// NAO-nada (regressao: a VM fazia underflow de pilha e panicava).
func TestVMNavegacaoSegura(t *testing.T) {
	comparaEngines(t, `bota m = {"a": 7}
mostra m?.a`)
	comparaEngines(t, `bota d = nada
mostra d?.x`)
	comparaEngines(t, `bota m = {"a": {"b": 9}}
mostra m?.a?.b`)
	comparaEngines(t, `bota m = {"a": nada}
mostra m?.a?.b`)
}
