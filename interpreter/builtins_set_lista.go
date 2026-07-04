package interpreter

import (
	"sort"

	"gambiarrascript/object"
)

// Lib padrão — Set (conjunto) e mais builtins de lista.
//
//	conjunto(listaOuTexto)       → novo Conjunto a partir da colecao (dedup)
//	contem_conjunto(conj, v)    → deu_bom/deu_ruim
//	adiciona_conjunto(conj, v)  → adiciona, devolve o proprio conjunto
//	remove_conjunto(conj, v)   → remove, devolve o proprio conjunto
//	uniao(a, b)                → conjunto resultante
//	intersecao(a, b)           → conjunto resultante
//	diferenca(a, b)            → conjunto resultante
//
//	reduz(lista, fn, [inicial])   → fold left (fn(acc, elem) ou fn(elem, acc))
//	acha(lista, fn)               → 1o elem onde fn deu_bom, ou nada
//	acha_indice(lista, fn)        → indice do 1o elem onde fn deu_bom, ou -1
//	unicos(lista)                 → lista com duplicatas removidas (preserva ordem)
//	achatada(listaDeListas)       → 1 nivel de flattening

func builtinConjunto(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("conjunto() quer 1 arg, veio %d", len(args))
	}
	c := object.NovoConjunto()
	switch v := args[0].(type) {
	case *object.Lista:
		for _, e := range v.Elements {
			c.Adiciona(e)
		}
	case *object.Texto:
		for _, r := range v.Value {
			c.Adiciona(&object.Texto{Value: string(r)})
		}
	case *object.Dicionario:
		for _, p := range v.Pares {
			c.Adiciona(p.Chave)
		}
	case *object.Conjunto:
		for _, e := range v.Items {
			c.Adiciona(e)
		}
	default:
		return erroBuiltin("conjunto() nao aceita %s", args[0].Type())
	}
	return c
}

func builtinContemConjunto(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("contem_conjunto() quer 2 args (conj, v), veio %d", len(args))
	}
	c, ok := args[0].(*object.Conjunto)
	if !ok {
		return erroBuiltin("contem_conjunto: 1o arg precisa ser conjunto, veio %s", args[0].Type())
	}
	return boolDoNativo(c.Contem(args[1]))
}

func builtinAdicionaConjunto(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("adiciona_conjunto() quer 2 args (conj, v), veio %d", len(args))
	}
	c, ok := args[0].(*object.Conjunto)
	if !ok {
		return erroBuiltin("adiciona_conjunto: 1o arg precisa ser conjunto, veio %s", args[0].Type())
	}
	c.Adiciona(args[1])
	return c
}

func builtinRemoveConjunto(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("remove_conjunto() quer 2 args (conj, v), veio %d", len(args))
	}
	c, ok := args[0].(*object.Conjunto)
	if !ok {
		return erroBuiltin("remove_conjunto: 1o arg precisa ser conjunto, veio %s", args[0].Type())
	}
	c.Remove(args[1])
	return c
}

func doisConjuntos(args []object.Object, nome string) (*object.Conjunto, *object.Conjunto, *object.Erro) {
	if len(args) != 2 {
		return nil, nil, erroBuiltin("%s() quer 2 args (conj, conj), veio %d", nome, len(args))
	}
	a, ok := args[0].(*object.Conjunto)
	if !ok {
		return nil, nil, erroBuiltin("%s: 1o precisa ser conjunto, veio %s", nome, args[0].Type())
	}
	b, ok := args[1].(*object.Conjunto)
	if !ok {
		return nil, nil, erroBuiltin("%s: 2o precisa ser conjunto, veio %s", nome, args[1].Type())
	}
	return a, b, nil
}

func builtinUniao(args []object.Object) object.Object {
	a, b, e := doisConjuntos(args, "uniao")
	if e != nil {
		return e
	}
	out := object.NovoConjunto()
	for _, v := range a.Items {
		out.Adiciona(v)
	}
	for _, v := range b.Items {
		out.Adiciona(v)
	}
	return out
}

func builtinIntersecao(args []object.Object) object.Object {
	a, b, e := doisConjuntos(args, "intersecao")
	if e != nil {
		return e
	}
	out := object.NovoConjunto()
	for _, v := range a.Items {
		if b.Contem(v) {
			out.Adiciona(v)
		}
	}
	return out
}

func builtinDiferenca(args []object.Object) object.Object {
	a, b, e := doisConjuntos(args, "diferenca")
	if e != nil {
		return e
	}
	out := object.NovoConjunto()
	for _, v := range a.Items {
		if !b.Contem(v) {
			out.Adiciona(v)
		}
	}
	return out
}

// builtinReduz: reduce/fold. Metodo do Interpreter pra chamar QUALQUER funcao
// (gambiarra do usuario, lambda, builtin) via applyFunction — que tambem
// delega pra VM quando a fn e CompiledFunction.
func (i *Interpreter) builtinReduz(args []object.Object) object.Object {
	if len(args) < 2 || len(args) > 3 {
		return erroBuiltin("reduz() quer lista + fn (+inicial), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("reduz: lista esperada, veio %s", args[0].Type())
	}
	if len(lst.Elements) == 0 && len(args) < 3 {
		return NADA
	}
	fn := args[1]
	var acc object.Object
	idx := 0
	if len(args) == 3 {
		acc = args[2]
	} else {
		acc = lst.Elements[0]
		idx = 1
	}
	for ; idx < len(lst.Elements); idx++ {
		acc = i.applyFunction(fn, []object.Object{acc, lst.Elements[idx]}, 0, "<reduz>")
		if isError(acc) {
			return acc
		}
	}
	return acc
}

func (i *Interpreter) builtinAcha(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("acha() quer 2 args (lista, fn), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("acha: lista esperada, veio %s", args[0].Type())
	}
	for _, e := range lst.Elements {
		r := i.applyFunction(args[1], []object.Object{e}, 0, "<acha>")
		if isError(r) {
			return r
		}
		if ehVerdadeiro(r) {
			return e
		}
	}
	return NADA
}

func (i *Interpreter) builtinAchaIndice(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("acha_indice() quer 2 args (lista, fn), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("acha_indice: lista esperada, veio %s", args[0].Type())
	}
	for idx, e := range lst.Elements {
		r := i.applyFunction(args[1], []object.Object{e}, 0, "<acha_indice>")
		if isError(r) {
			return r
		}
		if ehVerdadeiro(r) {
			return object.NumInt(int64(idx))
		}
	}
	return object.NumInt(-1)
}

func builtinUnicos(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("unicos() quer 1 arg (lista), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("unicos: lista esperada, veio %s", args[0].Type())
	}
	seen := object.NovoConjunto()
	out := make([]object.Object, 0, len(lst.Elements))
	for _, e := range lst.Elements {
		if seen.Adiciona(e) {
			out = append(out, e)
		}
	}
	return &object.Lista{Elements: out}
}

func builtinAchatada(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("achatada() quer 1 arg (lista), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("achatada: lista esperada, veio %s", args[0].Type())
	}
	out := make([]object.Object, 0, len(lst.Elements))
	for _, e := range lst.Elements {
		if sub, ok := e.(*object.Lista); ok {
			out = append(out, sub.Elements...)
		} else {
			out = append(out, e)
		}
	}
	return &object.Lista{Elements: out}
}

// ehVerdadeiro — fallback simples: nil/nada/deu_ruim → falso; resto → verdade.
// Usado por builtins que precisam interpretar booleanidade sem depender de
// interpreter.go (que tem versao igual chamada isTruthy).
func ehVerdadeiro(o object.Object) bool {
	switch v := o.(type) {
	case *object.Nada:
		return false
	case *object.Booleano:
		return v.Value
	case nil:
		return false
	}
	return true
}

// sort.Strings importado só pra evitar dependencia sem uso em alguns builds.
var _ = sort.Strings
