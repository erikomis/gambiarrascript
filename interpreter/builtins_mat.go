package interpreter

import (
	"math"

	"gambiarrascript/object"
)

func builtinRaiz(args []object.Object) object.Object {
	v, err := numeroArg(args, "raiz")
	if err != nil {
		return err
	}
	return &object.Numero{Value: math.Sqrt(v)}
}

func builtinAleatorio(args []object.Object) object.Object {
	if len(args) > 1 {
		return erroBuiltin("aleatorio() quer 0 ou 1 argumento, veio %d", len(args))
	}
	if len(args) == 0 {
		return &object.Numero{Value: rngFloat()}
	}
	max, ok := args[0].(*object.Numero)
	if !ok {
		return erroBuiltin("aleatorio() espera numero, veio %s", args[0].Type())
	}
	return &object.Numero{Value: rngFloat() * max.Value}
}

func builtinArredonda(args []object.Object) object.Object {
	v, err := numeroArg(args, "arredonda")
	if err != nil {
		return err
	}
	return &object.Numero{Value: float64(math.Round(v))}
}

func builtinTeto(args []object.Object) object.Object {
	v, err := numeroArg(args, "teto")
	if err != nil {
		return err
	}
	return &object.Numero{Value: math.Ceil(v)}
}

func builtinChao(args []object.Object) object.Object {
	v, err := numeroArg(args, "chao")
	if err != nil {
		return err
	}
	return &object.Numero{Value: math.Floor(v)}
}

func builtinAbs(args []object.Object) object.Object {
	v, err := numeroArg(args, "abs")
	if err != nil {
		return err
	}
	return &object.Numero{Value: math.Abs(v)}
}

func builtinMin(args []object.Object) object.Object {
	if len(args) < 1 {
		return erroBuiltin("min() quer pelo menos 1 numero, veio 0")
	}
	var menor *object.Numero
	for _, a := range args {
		n, ok := a.(*object.Numero)
		if !ok {
			return erroBuiltin("min() so funciona com numeros, veio %s", a.Type())
		}
		if menor == nil || n.Value < menor.Value {
			menor = n
		}
	}
	return menor
}

func builtinMax(args []object.Object) object.Object {
	if len(args) < 1 {
		return erroBuiltin("max() quer pelo menos 1 numero, veio 0")
	}
	var maior *object.Numero
	for _, a := range args {
		n, ok := a.(*object.Numero)
		if !ok {
			return erroBuiltin("max() so funciona com numeros, veio %s", a.Type())
		}
		if maior == nil || n.Value > maior.Value {
			maior = n
		}
	}
	return maior
}

// numeroArg valida e devolve o valor float64 do 1o argumento de uma builtin
// numerica de 1 argumento. Em caso de erro devolve o *object.Erro (não-nil).
func numeroArg(args []object.Object, nome string) (float64, *object.Erro) {
	if len(args) != 1 {
		return 0, erroBuiltin("%s() quer 1 argumento, veio %d", nome, len(args))
	}
	n, ok := args[0].(*object.Numero)
	if !ok {
		return 0, erroBuiltin("%s() espera numero, veio %s", nome, args[0].Type())
	}
	return n.Value, nil
}
