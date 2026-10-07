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
		for _, e := range v.Visao() {
			c.Adiciona(e)
		}
	case *object.Texto:
		for _, r := range v.Value {
			c.Adiciona(&object.Texto{Value: string(r)})
		}
	case *object.Dicionario:
		v.Itera(func(p object.ParDic) { c.Adiciona(p.Chave) })
	case *object.Conjunto:
		for _, e := range v.Valores() {
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
	for _, v := range a.Valores() {
		out.Adiciona(v)
	}
	for _, v := range b.Valores() {
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
	for _, v := range a.Valores() {
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
	for _, v := range a.Valores() {
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
	seq := lst.ParaIterar()
	n := seq.Tamanho()
	if n == 0 && len(args) < 3 {
		return NADA
	}
	fn := args[1]
	var acc object.Object
	idx := 0
	if len(args) == 3 {
		acc = args[2]
	} else {
		acc, _ = seq.Pega(0)
		idx = 1
	}
	for ; idx < n; idx++ {
		e, ok := seq.Pega(idx)
		if !ok {
			break // a propria gambiarra encolheu a lista
		}
		acc = i.applyFunction(fn, []object.Object{acc, e}, 0, "<reduz>")
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
	var achado object.Object = NADA
	percorre(lst, func(_ int, e object.Object) bool {
		r := i.applyFunction(args[1], []object.Object{e}, 0, "<acha>")
		if isError(r) {
			achado = r
			return false
		}
		if ehVerdadeiro(r) {
			achado = e
			return false
		}
		return true
	})
	return achado
}

func (i *Interpreter) builtinAchaIndice(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("acha_indice() quer 2 args (lista, fn), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("acha_indice: lista esperada, veio %s", args[0].Type())
	}
	var achado object.Object = object.NumInt(-1)
	percorre(lst, func(idx int, e object.Object) bool {
		r := i.applyFunction(args[1], []object.Object{e}, 0, "<acha_indice>")
		if isError(r) {
			achado = r
			return false
		}
		if ehVerdadeiro(r) {
			achado = object.NumInt(int64(idx))
			return false
		}
		return true
	})
	return achado
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
	elems := lst.Visao()
	out := make([]object.Object, 0, len(elems))
	for _, e := range elems {
		if seen.Adiciona(e) {
			out = append(out, e)
		}
	}
	return object.NovaLista(out)
}

func builtinAchatada(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("achatada() quer 1 arg (lista), veio %d", len(args))
	}
	lst, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("achatada: lista esperada, veio %s", args[0].Type())
	}
	elems := lst.Visao()
	out := make([]object.Object, 0, len(elems))
	for _, e := range elems {
		if sub, ok := e.(*object.Lista); ok {
			out = append(out, sub.Visao()...)
		} else {
			out = append(out, e)
		}
	}
	return object.NovaLista(out)
}

// percorre chama f(indice, elemento) pra cada elemento da lista ate f devolver
// false. Feito pra builtin que roda gambiarra do usuario a cada elemento: com
// o modo concorrente ligado percorre um retrato (ParaIterar); sem, le elemento
// a elemento pelo Pega (que passa a travar sozinho se a gambiarra ligar a
// concorrencia no meio do caminho). O tamanho e o do inicio, igual o
// pra_cada; se a gambiarra encolher a lista o laco para em vez de estourar.
func percorre(l *object.Lista, f func(int, object.Object) bool) {
	seq := l.ParaIterar()
	n := seq.Tamanho()
	for idx := 0; idx < n; idx++ {
		e, ok := seq.Pega(idx)
		if !ok || !f(idx, e) {
			return
		}
	}
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
