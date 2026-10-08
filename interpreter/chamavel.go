package interpreter

import "gambiarrascript/object"

// ehChamavel diz se o valor da pra chamar como gambiarra: `*object.Funcao` no
// tree-walker, `*object.CompiledFunction` na VM, `*object.Builtin` pros nativos.
func ehChamavel(o object.Object) bool {
	switch o.(type) {
	case *object.Funcao, *object.CompiledFunction, *object.Builtin, *object.MetodoLigado:
		return true
	}
	return false
}
