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

// builtinRemove tira um item da colecao, mutando ela. Um nome so pras tres:
//
//	remove(lista, valor)       → tira a 1a ocorrencia igual
//	remove(dicionario, chave)  → apaga a chave (e o valor junto)
//	remove(conjunto, item)     → tira o item
//
// Devolve sempre nada, e tirar o que nao esta la e no-op calado (o contrato
// que o remove de lista sempre teve). Chave que nao pode ser chave (lista,
// dicionario...) no dicionario e erro, igual o tem(). O remove_conjunto
// continua existindo e devolvendo o proprio conjunto.
func builtinRemove(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("remove() quer 2 argumentos (colecao, item), veio %d", len(args))
	}
	switch c := args[0].(type) {
	case *object.Lista:
		return removeDaLista(c, args[1])
	case *object.Dicionario:
		chave, ok := args[1].(object.Chaveavel)
		if !ok {
			return erroBuiltin("remove() nao consegue usar %s como chave", args[1].Type())
		}
		c.Tira(chave.ChaveHash())
		return NADA
	case *object.Conjunto:
		c.Remove(args[1])
		return NADA
	}
	return erroBuiltin("remove() espera lista, dicionario ou conjunto, veio %s", args[0].Type())
}

func removeDaLista(l *object.Lista, item object.Object) object.Object {
	// acha por valor num retrato (iguais pode travar colecao aninhada, entao
	// nao roda com a trava desta lista) e tira pela identidade do elemento
	// achado. Se outra goroutine tirou esse mesmo elemento no meio do caminho,
	// procura de novo: o remove nunca "some" sem tirar nada.
	for {
		var alvo object.Object
		for _, e := range l.Visao() {
			if iguais(e, item) {
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
	if l.OrdenaNatural() { // so numeros ou so textos: caminho rapido
		return NADA
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
