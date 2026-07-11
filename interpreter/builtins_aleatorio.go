package interpreter

import (
	"fmt"

	"gambiarrascript/object"
)

// builtinSemente fixa a semente do gerador compartilhado, deixando
// aleatorio/embaralha/escolhe_um/uuid reprodutiveis: semente(42) gera a mesma
// sequencia toda vez. Devolve nada.
func builtinSemente(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("semente() quer 1 arg (numero), veio %d", len(args))
	}
	n, ok := args[0].(*object.Numero)
	if !ok {
		return erroBuiltin("semente() espera numero, veio %s", args[0].Type())
	}
	rngSemente(int64(n.Value))
	return NADA
}

// builtinEmbaralha devolve uma NOVA lista com os elementos embaralhados
// (Fisher-Yates). A original nao e mexida.
func builtinEmbaralha(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("embaralha() quer 1 arg (lista), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("embaralha: lista esperada, veio %s", args[0].Type())
	}
	out := make([]object.Object, len(lst.Elements))
	copy(out, lst.Elements)
	for i := len(out) - 1; i > 0; i-- {
		j := rngIntn(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return &object.Lista{Elements: out}
}

// builtinEscolheUm devolve um elemento aleatorio da lista. Lista vazia da erro.
func builtinEscolheUm(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("escolhe_um() quer 1 arg (lista), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("escolhe_um: lista esperada, veio %s", args[0].Type())
	}
	if len(lst.Elements) == 0 {
		return erroBuiltin("escolhe_um: lista vazia, nao tem de onde escolher")
	}
	return lst.Elements[rngIntn(len(lst.Elements))]
}

// builtinUuid devolve um UUID versao 4 em texto. Usa o gerador compartilhado
// (entao `semente` o torna reprodutivel) — nao use pra fins de seguranca.
func builtinUuid(args []object.Object) object.Object {
	if len(args) != 0 {
		return erroBuiltin("uuid() nao quer argumento, veio %d", len(args))
	}
	var b [16]byte
	rngRead(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // versao 4
	b[8] = (b[8] & 0x3f) | 0x80 // variante RFC 4122
	s := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	return &object.Texto{Value: s}
}
