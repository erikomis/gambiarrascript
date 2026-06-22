package interpreter

import (
	"fmt"
	"strconv"
	"strings"

	"gambiarrascript/object"
)

var builtins = map[string]*object.Builtin{
	"tamanho": {Nome: "tamanho", Fn: builtinTamanho},
	"chaves":  {Nome: "chaves", Fn: builtinChaves},
	"tem":     {Nome: "tem", Fn: builtinTem},
	"texto":   {Nome: "texto", Fn: builtinTexto},
	"numero":  {Nome: "numero", Fn: builtinNumero},
}

func erroBuiltin(formato string, args ...interface{}) *object.Erro {
	return &object.Erro{Message: "deu ruim: " + fmt.Sprintf(formato, args...)}
}

func builtinTamanho(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("tamanho() quer 1 argumento, veio %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Lista:
		return &object.Numero{Value: float64(len(arg.Elements))}
	case *object.Dicionario:
		return &object.Numero{Value: float64(len(arg.Pares))}
	case *object.Texto:
		return &object.Numero{Value: float64(len([]rune(arg.Value)))}
	default:
		return erroBuiltin("tamanho() nao funciona com %s", args[0].Type())
	}
}

func builtinChaves(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("chaves() quer 1 argumento, veio %d", len(args))
	}
	d, ok := args[0].(*object.Dicionario)
	if !ok {
		return erroBuiltin("chaves() so funciona com dicionario, veio %s", args[0].Type())
	}
	elems := make([]object.Object, 0, len(d.Pares))
	for _, par := range d.Pares {
		elems = append(elems, par.Chave)
	}
	return &object.Lista{Elements: elems}
}

func builtinTem(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("tem() quer 2 argumentos (dicionario, chave), veio %d", len(args))
	}
	d, ok := args[0].(*object.Dicionario)
	if !ok {
		return erroBuiltin("tem() espera um dicionario no primeiro argumento, veio %s", args[0].Type())
	}
	chave, ok := args[1].(object.Chaveavel)
	if !ok {
		return erroBuiltin("tem() nao consegue usar %s como chave", args[1].Type())
	}
	_, existe := d.Pares[chave.ChaveHash()]
	return boolDoNativo(existe)
}

func builtinTexto(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("texto() quer 1 argumento, veio %d", len(args))
	}
	return &object.Texto{Value: args[0].Inspect()}
}

func builtinNumero(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("numero() quer 1 argumento, veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("numero() so converte texto, veio %s", args[0].Type())
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(t.Value), 64)
	if err != nil {
		return erroBuiltin("isso ai nao e numero, parca: %q", t.Value)
	}
	return &object.Numero{Value: v}
}
