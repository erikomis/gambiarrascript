package interpreter

import (
	"sort"

	"gambiarrascript/object"
)

func builtinAdiciona(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("adiciona() quer 2 argumentos (lista, item), veio %d", len(args))
	}
	l, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("adiciona() espera uma lista, veio %s", args[0].Type())
	}
	l.Elements = append(l.Elements, args[1])
	return NADA
}

func builtinRemove(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("remove() quer 2 argumentos (lista, item), veio %d", len(args))
	}
	l, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("remove() espera uma lista, veio %s", args[0].Type())
	}
	for idx, e := range l.Elements {
		if iguais(e, args[1]) {
			l.Elements = append(l.Elements[:idx], l.Elements[idx+1:]...)
			break
		}
	}
	return NADA
}

func builtinOrdena(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("ordena() quer 1 argumento (lista), veio %d", len(args))
	}
	l, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("ordena() espera uma lista, veio %s", args[0].Type())
	}
	var primeiroErro *object.Erro
	sort.SliceStable(l.Elements, func(i, j int) bool {
		if primeiroErro != nil {
			return false
		}
		menor, ok := comparaLista(l.Elements[i], l.Elements[j])
		if !ok {
			primeiroErro = erroBuiltin("ordena() nao soube comparar %s com %s", l.Elements[i].Type(), l.Elements[j].Type())
			return false
		}
		return menor
	})
	if primeiroErro != nil {
		return primeiroErro
	}
	return NADA
}

func builtinInverte(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("inverte() quer 1 argumento (lista), veio %d", len(args))
	}
	l, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("inverte() espera uma lista, veio %s", args[0].Type())
	}
	for i, j := 0, len(l.Elements)-1; i < j; i, j = i+1, j-1 {
		l.Elements[i], l.Elements[j] = l.Elements[j], l.Elements[i]
	}
	return NADA
}

// comparaLista devolve (menor, ok): menor=true se a < b. So compara numeros e
// textos entre si; tipos diferentes ou nao-ordenaveis devolvem ok=false.
func comparaLista(a, b object.Object) (bool, bool) {
	an, aok := a.(*object.Numero)
	bn, bok := b.(*object.Numero)
	if aok && bok {
		return an.Value < bn.Value, true
	}
	at, aok := a.(*object.Texto)
	bt, bok := b.(*object.Texto)
	if aok && bok {
		return at.Value < bt.Value, true
	}
	return false, false
}