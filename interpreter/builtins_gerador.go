package interpreter

import (
	"gambiarrascript/object"
)

// Builtins de gerador (os dois engines usam estas): proximo, acabou, pega e
// lista. O mapeia/filtra/reduz aceitam gerador consumindo ele inteiro antes
// (consomeSeGerador): gerador infinito ali nunca volta — pra esse, pega(g, n).

func init() {
	builtins["proximo"] = &object.Builtin{Nome: "proximo", Fn: builtinProximo}
	builtins["acabou"] = &object.Builtin{Nome: "acabou", Fn: builtinAcabou}
	builtins["pega"] = &object.Builtin{Nome: "pega", Fn: builtinPega}
}

func geradorDoArg(nome string, args []object.Object, min, max int) (*object.Gerador, *object.Erro) {
	if len(args) < min || len(args) > max {
		if min == max {
			return nil, erroBuiltin("%s() quer %d argumento(s), veio %d", nome, min, len(args))
		}
		return nil, erroBuiltin("%s() quer de %d a %d argumentos, veio %d", nome, min, max, len(args))
	}
	g, ok := args[0].(*object.Gerador)
	if !ok {
		return nil, erroBuiltin("%s() espera um gerador, veio %s", nome, object.NomeTipo(args[0]))
	}
	return g, nil
}

// proximo(g) devolve o proximo valor do gerador, ou nada quando ele acabou.
// Como o gerador pode render nada de proposito, proximo(g, padrao) devolve o
// padrao no fim (e acabou(g) diz com certeza).
func builtinProximo(args []object.Object) object.Object {
	g, erro := geradorDoArg("proximo", args, 1, 2)
	if erro != nil {
		return erro
	}
	v, ok, falha := g.Proximo()
	if falha != nil {
		return falha
	}
	if !ok {
		if len(args) == 2 {
			return args[1]
		}
		return NADA
	}
	return v
}

// acabou(g) diz se o gerador nao tem mais valor (pode rodar o corpo ate o
// proximo rende pra descobrir; o valor fica guardado pro proximo pedido).
func builtinAcabou(args []object.Object) object.Object {
	g, erro := geradorDoArg("acabou", args, 1, 1)
	if erro != nil {
		return erro
	}
	fim, falha := g.Acabou()
	if falha != nil {
		return falha
	}
	return boolDoNativo(fim)
}

// pega(g, n) puxa ate n valores do gerador numa lista (menos se ele acabar
// antes). E o jeito de usar gerador infinito com o resto da biblioteca.
func builtinPega(args []object.Object) object.Object {
	g, erro := geradorDoArg("pega", args, 2, 2)
	if erro != nil {
		return erro
	}
	n, ok := args[1].(*object.Numero)
	if !ok || !n.EhInt || n.Int < 0 {
		return erroBuiltin("pega() quer quantos (inteiro >= 0) no segundo argumento, veio %s", args[1].Inspect())
	}
	out := []object.Object{}
	for k := int64(0); k < n.Int; k++ {
		v, ok, falha := g.Proximo()
		if falha != nil {
			return falha
		}
		if !ok {
			break
		}
		out = append(out, v)
	}
	return object.NovaLista(out)
}

// consomeGerador puxa todos os valores do gerador numa lista.
func consomeGerador(g *object.Gerador) object.Object {
	out := []object.Object{}
	for {
		v, ok, falha := g.Proximo()
		if falha != nil {
			return falha
		}
		if !ok {
			return object.NovaLista(out)
		}
		out = append(out, v)
	}
}

// consomeSeGerador troca o primeiro argumento pela lista dos valores quando
// ele e gerador (mapeia/filtra/reduz). Devolve os args novos ou a falha.
func consomeSeGerador(args []object.Object) ([]object.Object, object.Object) {
	if len(args) == 0 {
		return args, nil
	}
	g, ok := args[0].(*object.Gerador)
	if !ok {
		return args, nil
	}
	l := consomeGerador(g)
	if _, ok := l.(*object.Lista); !ok {
		return nil, l
	}
	novos := make([]object.Object, len(args))
	copy(novos, args)
	novos[0] = l
	return novos, nil
}

// builtinLista monta uma lista nova: gerador (consumido inteiro), lista
// (copia rasa), conjunto (itens na ordem), dicionario (chaves) ou treta com
// itera() (o que ele devolve, convertido igual).
func (i *Interpreter) builtinLista(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("lista() quer 1 argumento, veio %d", len(args))
	}
	v := args[0]
	if inst, ok := v.(*object.Instancia); ok {
		v = i.iteravel(inst, 0)
		if isError(v) || v.Type() == object.SAIR_OBJ {
			return v
		}
	}
	switch c := v.(type) {
	case *object.Gerador:
		return consomeGerador(c)
	case *object.Lista:
		return object.NovaLista(c.Copia())
	case *object.Conjunto:
		return object.NovaLista(c.Valores())
	case *object.Cardapio:
		return object.NovaLista(c.ListaOpcoes())
	case *object.Dicionario:
		chaves := make([]object.Object, 0, c.Tamanho())
		c.Itera(func(par object.ParDic) { chaves = append(chaves, par.Chave) })
		return object.NovaLista(chaves)
	}
	return erroBuiltin("lista() quer gerador, lista, conjunto, dicionario, cardapio ou treta com itera(), veio %s", object.NomeTipo(v))
}
