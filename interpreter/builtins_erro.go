package interpreter

import (
	"strings"

	"gambiarrascript/object"
)

// builtinQuebra devolve um erro lancavel pelo usuario. Uso:
//
//	quebra("mensagem")               -> Erro kind=usuario
//	quebra("loop infinito", erro_origem) -> Erro kind=usuario com Cause
//
// Como Erro e propagado normalmente pelas avaliacoes (isError), basta usar
// `quebra(...)` em qualquer lugar — o `arruma` mais externo pega.
func builtinQuebra(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("quebra() quer 1 ou 2 argumentos (msg, [causa]), veio %d", len(args))
	}
	msg, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("quebra() espera texto como mensagem, veio %s", args[0].Type())
	}
	err := &object.Erro{
		Message: "quebra: " + msg.Value,
		Kind:    KindUsuario,
	}
	if len(args) == 2 {
		causa, ok := args[1].(*object.Erro)
		if !ok {
			return erroBuiltin("quebra(): a causa tem que ser um erro, veio %s", args[1].Type())
		}
		err.Cause = causa
	}
	return err
}

// builtinErroMsg devolve a mensagem textual de um Erro (compativel com o que
// antes era so `erro` — texto direto).
func builtinErroMsg(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("erro_msg() quer 1 argumento (erro), veio %d", len(args))
	}
	e, ok := args[0].(*object.Erro)
	if !ok {
		return erroBuiltin("erro_msg() espera um erro, veio %s", args[0].Type())
	}
	return &object.Texto{Value: e.Message}
}

func builtinErroLinha(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("erro_linha() quer 1 argumento (erro), veio %d", len(args))
	}
	e, ok := args[0].(*object.Erro)
	if !ok {
		return erroBuiltin("erro_linha() espera um erro, veio %s", args[0].Type())
	}
	return &object.Numero{Value: float64(e.Line)}
}

func builtinErroTipo(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("erro_tipo() quer 1 argumento (erro), veio %d", len(args))
	}
	e, ok := args[0].(*object.Erro)
	if !ok {
		return erroBuiltin("erro_tipo() espera um erro, veio %s", args[0].Type())
	}
	kind := e.Kind
	if kind == "" {
		kind = KindBuiltin
	}
	return &object.Texto{Value: kind}
}

// builtinErroPilha devolve a lista de frames do traço de pilha do erro, cada
// frame como um dicionario {"funcao": ..., "linha": ...}.
func builtinErroPilha(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("erro_pilha() quer 1 argumento (erro), veio %d", len(args))
	}
	e, ok := args[0].(*object.Erro)
	if !ok {
		return erroBuiltin("erro_pilha() espera um erro, veio %s", args[0].Type())
	}
	elems := make([]object.Object, 0, len(e.Stack))
	for _, f := range e.Stack {
		pares := map[object.HashKey]object.ParDic{}
		k1 := &object.Texto{Value: "funcao"}
		pares[k1.ChaveHash()] = object.ParDic{Chave: k1, Valor: &object.Texto{Value: f.Funcao}}
		k2 := &object.Texto{Value: "linha"}
		pares[k2.ChaveHash()] = object.ParDic{Chave: k2, Valor: &object.Numero{Value: float64(f.Line)}}
		elems = append(elems, &object.Dicionario{Pares: pares})
	}
	return &object.Lista{Elements: elems}
}

func builtinErroCausa(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("erro_causa() quer 1 argumento (erro), veio %d", len(args))
	}
	e, ok := args[0].(*object.Erro)
	if !ok {
		return erroBuiltin("erro_causa() espera um erro, veio %s", args[0].Type())
	}
	if e.Cause == nil {
		return NADA
	}
	return e.Cause
}

// builtinEnvolveErro cria um novo erro que "envolve" uma causa: o tipo fica
// definido pelo caller (texto) e a mensagem e livre. Uso:
//
//	envolve_erro("io", "deu ruim no disco", erro_original)
//
// O erro resultante tem Cause apontando pro original e Kind = tipo informado.
func builtinEnvolveErro(args []object.Object) object.Object {
	if len(args) != 3 {
		return erroBuiltin("envolve_erro() quer 3 argumentos (tipo, msg, causa), veio %d", len(args))
	}
	tipo, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("envolve_erro(): o tipo tem que ser texto, veio %s", args[0].Type())
	}
	msg, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("envolve_erro(): a mensagem tem que ser texto, veio %s", args[1].Type())
	}
	causa, ok := args[2].(*object.Erro)
	if !ok {
		return erroBuiltin("envolve_erro(): a causa tem que ser um erro, veio %s", args[2].Type())
	}
	return &object.Erro{
		Message: "envolve: " + msg.Value + " :: " + causa.Message,
		Kind:    strings.TrimSpace(tipo.Value),
		Cause:   causa,
	}
}
