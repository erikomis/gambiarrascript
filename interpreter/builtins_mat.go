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

// trigonometria: angulo em radianos (graus * pi / 180).
func builtinSeno(args []object.Object) object.Object {
	v, err := numeroArg(args, "seno")
	if err != nil {
		return err
	}
	return &object.Numero{Value: math.Sin(v)}
}

func builtinCosseno(args []object.Object) object.Object {
	v, err := numeroArg(args, "cosseno")
	if err != nil {
		return err
	}
	return &object.Numero{Value: math.Cos(v)}
}

func builtinTangente(args []object.Object) object.Object {
	v, err := numeroArg(args, "tangente")
	if err != nil {
		return err
	}
	return &object.Numero{Value: math.Tan(v)}
}

// builtinLog e o logaritmo natural (base e). Zero ou negativo nao tem log.
func builtinLog(args []object.Object) object.Object {
	v, err := numeroArg(args, "log")
	if err != nil {
		return err
	}
	if v <= 0 {
		return erroBuiltin("log() so aceita numero maior que zero, veio %s", object.FormatNumero(v))
	}
	return &object.Numero{Value: math.Log(v)}
}

func builtinLog10(args []object.Object) object.Object {
	v, err := numeroArg(args, "log10")
	if err != nil {
		return err
	}
	if v <= 0 {
		return erroBuiltin("log10() so aceita numero maior que zero, veio %s", object.FormatNumero(v))
	}
	return &object.Numero{Value: math.Log10(v)}
}

func builtinExp(args []object.Object) object.Object {
	v, err := numeroArg(args, "exp")
	if err != nil {
		return err
	}
	return &object.Numero{Value: math.Exp(v)}
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
