package interpreter

import (
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
	l.Adiciona(args[1])
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
	// acha por valor num retrato (iguais pode travar colecao aninhada, entao
	// nao roda com a trava desta lista) e tira pela identidade do elemento
	// achado. Se outra goroutine tirou esse mesmo elemento no meio do caminho,
	// procura de novo: o remove nunca "some" sem tirar nada.
	for {
		var alvo object.Object
		for _, e := range l.Visao() {
			if iguais(e, args[1]) {
				alvo = e
				break
			}
		}
		if alvo == nil || l.RemoveObjeto(alvo) {
			return NADA
		}
	}
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
	l.Ordena(func(a, b object.Object) bool {
		if primeiroErro != nil {
			return false
		}
		menor, ok := comparaLista(a, b)
		if !ok {
			primeiroErro = erroBuiltin("ordena() nao soube comparar %s com %s", a.Type(), b.Type())
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
	l.Inverte()
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
