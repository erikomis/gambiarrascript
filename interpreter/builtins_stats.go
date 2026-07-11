package interpreter

import "gambiarrascript/object"

// builtinSoma soma os numeros de uma lista. Se todos forem inteiros exatos, o
// resultado sai inteiro; se tiver algum float, sai float. Lista vazia = 0.
func builtinSoma(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("soma() quer 1 arg (lista), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("soma: lista esperada, veio %s", args[0].Type())
	}
	var somaInt int64
	var somaFloat float64
	todosInt := true
	for idx, e := range lst.Elements {
		n, ok := e.(*object.Numero)
		if !ok {
			return erroBuiltin("soma: elemento %d nao e numero, veio %s", idx, e.Type())
		}
		if n.EhInt {
			somaInt += n.Int
		} else {
			todosInt = false
		}
		somaFloat += n.Value
	}
	if todosInt {
		return object.NumInt(somaInt)
	}
	return object.NumFloat(somaFloat)
}

// builtinMedia devolve a media aritmetica dos numeros da lista (sempre float).
// Lista vazia da erro (nao da pra tirar media de nada).
func builtinMedia(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("media() quer 1 arg (lista), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("media: lista esperada, veio %s", args[0].Type())
	}
	if len(lst.Elements) == 0 {
		return erroBuiltin("media: lista vazia, nao da pra tirar media de nada")
	}
	var total float64
	for idx, e := range lst.Elements {
		n, ok := e.(*object.Numero)
		if !ok {
			return erroBuiltin("media: elemento %d nao e numero, veio %s", idx, e.Type())
		}
		total += n.Value
	}
	return object.NumFloat(total / float64(len(lst.Elements)))
}

// builtinZip casa duas listas em pares [a[i], b[i]], parando no tamanho da
// menor. zip([1,2,3], ["a","b"]) -> [[1, a], [2, b]].
func builtinZip(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("zip() quer 2 args (lista, lista), veio %d", len(args))
	}
	a, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("zip: 1o arg tem que ser lista, veio %s", args[0].Type())
	}
	b, ok := args[1].(*object.Lista)
	if !ok {
		return erroBuiltin("zip: 2o arg tem que ser lista, veio %s", args[1].Type())
	}
	n := len(a.Elements)
	if len(b.Elements) < n {
		n = len(b.Elements)
	}
	out := make([]object.Object, 0, n)
	for i := 0; i < n; i++ {
		par := &object.Lista{Elements: []object.Object{a.Elements[i], b.Elements[i]}}
		out = append(out, par)
	}
	return &object.Lista{Elements: out}
}

// builtinEnumera devolve pares [indice, valor] pra cada elemento da lista.
// enumera(["a","b"]) -> [[0, a], [1, b]]. Util pra iterar com o indice.
func builtinEnumera(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("enumera() quer 1 arg (lista), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("enumera: lista esperada, veio %s", args[0].Type())
	}
	out := make([]object.Object, 0, len(lst.Elements))
	for i, e := range lst.Elements {
		par := &object.Lista{Elements: []object.Object{object.NumInt(int64(i)), e}}
		out = append(out, par)
	}
	return &object.Lista{Elements: out}
}
