package interpreter

import (
	"fmt"
	"strings"

	"gambiarrascript/object"
)

// builtinFormata: o printf da gambiarra. Usa os verbos do Go (%v %s %d %f,
// com padding/casas: %05d, %.2f, %-10s...). Numero inteiro vira int64, float
// vira float64, texto vira string, booleano vira bool; o resto vai de Inspect.
func builtinFormata(args []object.Object) object.Object {
	if len(args) < 1 {
		return erroBuiltin("formata() quer o modelo (+valores), veio nada")
	}
	modelo, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("formata: o modelo tem que ser texto, veio %s", args[0].Type())
	}
	vals := make([]interface{}, len(args)-1)
	for i, a := range args[1:] {
		switch v := a.(type) {
		case *object.Numero:
			if v.EhInt {
				vals[i] = v.Int
			} else {
				vals[i] = v.Value
			}
		case *object.Texto:
			vals[i] = v.Value
		default:
			// booleano/nada/lista/dict: usa a cara da linguagem (deu_bom,
			// nada, [1, 2]...) em vez da representacao do Go.
			vals[i] = a.Inspect()
		}
	}
	return &object.Texto{Value: fmt.Sprintf(modelo.Value, vals...)}
}

func builtinSepara(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("separa() quer 2 argumentos (texto, separador), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("separa() espera texto no 1o argumento, veio %s", args[0].Type())
	}
	sep, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("separa() espera texto no separador, veio %s", args[1].Type())
	}
	partes := strings.Split(t.Value, sep.Value)
	elems := make([]object.Object, len(partes))
	for i, p := range partes {
		elems[i] = &object.Texto{Value: p}
	}
	return &object.Lista{Elements: elems}
}

func builtinJunta(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("junta() quer 2 argumentos (lista, separador), veio %d", len(args))
	}
	l, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("junta() espera uma lista no 1o argumento, veio %s", args[0].Type())
	}
	sep, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("junta() espera texto no separador, veio %s", args[1].Type())
	}
	parts := make([]string, len(l.Elements))
	for i, e := range l.Elements {
		parts[i] = e.Inspect()
	}
	return &object.Texto{Value: strings.Join(parts, sep.Value)}
}

func builtinMaiusculo(args []object.Object) object.Object {
	t, err := textoArg(args, "maiusculo")
	if err != nil {
		return err
	}
	return &object.Texto{Value: strings.ToUpper(t)}
}

func builtinMinusculo(args []object.Object) object.Object {
	t, err := textoArg(args, "minusculo")
	if err != nil {
		return err
	}
	return &object.Texto{Value: strings.ToLower(t)}
}

func builtinSubstitui(args []object.Object) object.Object {
	if len(args) != 3 {
		return erroBuiltin("substitui() quer 3 argumentos (texto, antigo, novo), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("substitui() espera texto, veio %s", args[0].Type())
	}
	antigo, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("substitui() espera texto em antigo, veio %s", args[1].Type())
	}
	novo, ok := args[2].(*object.Texto)
	if !ok {
		return erroBuiltin("substitui() espera texto em novo, veio %s", args[2].Type())
	}
	return &object.Texto{Value: strings.ReplaceAll(t.Value, antigo.Value, novo.Value)}
}

func builtinFatia(args []object.Object) object.Object {
	if len(args) < 2 || len(args) > 3 {
		return erroBuiltin("fatia() quer 2 ou 3 argumentos (texto, inicio, [fim]), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("fatia() espera texto, veio %s", args[0].Type())
	}
	inicio, ok := args[1].(*object.Numero)
	if !ok {
		return erroBuiltin("fatia() espera numero no inicio, veio %s", args[1].Type())
	}
	runes := []rune(t.Value)
	i := int(inicio.Value)
	j := len(runes)
	if len(args) == 3 {
		fim, ok := args[2].(*object.Numero)
		if !ok {
			return erroBuiltin("fatia() espera numero no fim, veio %s", args[2].Type())
		}
		j = int(fim.Value)
	}
	if i < 0 {
		i = 0
	}
	if j > len(runes) {
		j = len(runes)
	}
	if j < i {
		j = i
	}
	return &object.Texto{Value: string(runes[i:j])}
}

func builtinContem(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("contem() quer 2 argumentos (texto, pedaco), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("contem() espera texto, veio %s", args[0].Type())
	}
	sub, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("contem() espera texto no pedaco, veio %s", args[1].Type())
	}
	return boolDoNativo(strings.Contains(t.Value, sub.Value))
}

func builtinComecaCom(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("comeca_com() quer 2 argumentos (texto, prefixo), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("comeca_com() espera texto, veio %s", args[0].Type())
	}
	pre, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("comeca_com() espera texto no prefixo, veio %s", args[1].Type())
	}
	return boolDoNativo(strings.HasPrefix(t.Value, pre.Value))
}

func builtinTerminaCom(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("termina_com() quer 2 argumentos (texto, sufixo), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("termina_com() espera texto, veio %s", args[0].Type())
	}
	suf, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("termina_com() espera texto no sufixo, veio %s", args[1].Type())
	}
	return boolDoNativo(strings.HasSuffix(t.Value, suf.Value))
}

func builtinTiraEspaco(args []object.Object) object.Object {
	t, err := textoArg(args, "tira_espaco")
	if err != nil {
		return err
	}
	return &object.Texto{Value: strings.TrimSpace(t)}
}

// textoArg valida e devolve o conteúdo texto do 1o argumento de uma builtin
// de 1 argumento. Em caso de erro devolve o *object.Erro (não-nil) e string vazia.
func textoArg(args []object.Object, nome string) (string, *object.Erro) {
	if len(args) != 1 {
		return "", erroBuiltin("%s() quer 1 argumento, veio %d", nome, len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return "", erroBuiltin("%s() espera texto, veio %s", nome, args[0].Type())
	}
	return t.Value, nil
}
