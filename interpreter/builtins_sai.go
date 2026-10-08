package interpreter

import "gambiarrascript/object"

// builtinSai encerra o script com um codigo de saida (default 0). Devolve o
// objeto de controle *Sair, que desenrola blocos/loops/funcoes ate o runner.
func builtinSai(args []object.Object) object.Object {
	if len(args) > 1 {
		return erroBuiltin("sai() quer 0 ou 1 arg (codigo), veio %d", len(args))
	}
	codigo := 0
	if len(args) == 1 {
		n, ok := args[0].(*object.Numero)
		if !ok {
			return erroBuiltin("sai() espera numero (codigo), veio %s", args[0].Type())
		}
		codigo = int(n.Value)
	}
	return &object.Sair{Codigo: codigo}
}
