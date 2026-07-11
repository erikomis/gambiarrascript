package interpreter

import (
	"sort"

	"gambiarrascript/object"
)

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

// builtinOrdenaPor ordena uma lista de dicionarios por um campo (crescente) e
// devolve uma lista NOVA (a original nao e mexida). Compara numeros e textos.
func builtinOrdenaPor(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("ordena_por() quer 2 args (lista, campo), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("ordena_por: 1o arg tem que ser lista, veio %s", args[0].Type())
	}
	campo, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("ordena_por: 2o arg (campo) tem que ser texto, veio %s", args[1].Type())
	}
	chave := campo.ChaveHash()

	// Extrai o campo de todo elemento ANTES de ordenar: erra cedo se algum nao
	// for dicionario ou nao tiver o campo (mesmo com 0/1 elemento, onde o
	// comparator do sort nem roda), e evita relookup a cada comparacao.
	type parOrd struct {
		elem     object.Object
		valChave object.Object
	}
	pares := make([]parOrd, len(lst.Elements))
	for idx, e := range lst.Elements {
		d, ok := e.(*object.Dicionario)
		if !ok {
			return erroBuiltin("ordena_por: elemento %d nao e dicionario, veio %s", idx, e.Type())
		}
		par, existe := d.Pares[chave]
		if !existe {
			return erroBuiltin("ordena_por: dicionario nao tem o campo %q", campo.Value)
		}
		pares[idx] = parOrd{elem: e, valChave: par.Valor}
	}

	var primeiroErro object.Object
	sort.SliceStable(pares, func(a, b int) bool {
		if primeiroErro != nil {
			return false
		}
		menor, ok := comparaLista(pares[a].valChave, pares[b].valChave)
		if !ok {
			primeiroErro = erroBuiltin("ordena_por: nao soube comparar %s com %s no campo %q", pares[a].valChave.Type(), pares[b].valChave.Type(), campo.Value)
			return false
		}
		return menor
	})
	if primeiroErro != nil {
		return primeiroErro
	}
	out := make([]object.Object, len(pares))
	for i, p := range pares {
		out[i] = p.elem
	}
	return &object.Lista{Elements: out}
}

// builtinAgrupaPor agrupa os elementos num dicionario {chave: [elementos]},
// onde a chave vem de aplicar a gambiarra em cada elemento. A ordem dentro de
// cada grupo segue a lista original. Higher-order: usa applyFunction (funciona
// nos dois engines via a ponte ChamaCompilada).
func (i *Interpreter) builtinAgrupaPor(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("agrupa_por() quer 2 args (lista, gambiarra), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("agrupa_por: 1o arg tem que ser lista, veio %s", args[0].Type())
	}
	fn := args[1]
	dic := &object.Dicionario{Pares: map[object.HashKey]object.ParDic{}}
	for _, e := range lst.Elements {
		chaveObj := i.applyFunction(fn, []object.Object{e}, 0, "<agrupa_por>")
		if isError(chaveObj) {
			return chaveObj
		}
		chaveavel, ok := chaveObj.(object.Chaveavel)
		if !ok {
			return erroBuiltin("agrupa_por: a gambiarra devolveu %s, que nao serve de chave (use texto, numero ou booleano)", chaveObj.Type())
		}
		hk := chaveavel.ChaveHash()
		par, existe := dic.Pares[hk]
		if !existe {
			par = object.ParDic{Chave: chaveObj, Valor: &object.Lista{Elements: []object.Object{}}}
		}
		grupo := par.Valor.(*object.Lista)
		grupo.Elements = append(grupo.Elements, e)
		par.Valor = grupo
		dic.Pares[hk] = par
	}
	return dic
}
