package interpreter

import (
	"gambiarrascript/ast"
	"gambiarrascript/object"
	"gambiarrascript/token"
)

// POO no tree-walker: treta, combinado, metodo e literal Tipo{...}. A regra
// (promotion, zero-value, satisfacao) mora em object/poo.go, igual na VM.

// thunkDoPadrao embrulha o valor padrao de um campo: literal simples vira o
// valor (nao muda nunca); o resto vira uma gambiarra sem parametro que roda
// a cada instancia (`xs = []` da uma lista nova pra cada um). A VM faz igual.
func (i *Interpreter) thunkDoPadrao(expr ast.Expression, env *object.Environment, nome string) object.Object {
	switch n := expr.(type) {
	case *ast.NumeroLiteral, *ast.TextoLiteral, *ast.BooleanoLiteral, *ast.NadaLiteral:
		return i.Eval(n, env)
	}
	corpo := &ast.BlockStatement{Statements: []ast.Statement{&ast.FuncionaStatement{Token: token.Token{Literal: "funciona"}, Value: expr}}}
	return &object.Funcao{Body: corpo, Env: env, Nome: nome}
}

func (i *Interpreter) evalTretaDecl(node *ast.TretaDecl, env *object.Environment) object.Object {
	linha := node.Token.Line
	campos := make([]object.CampoTreta, len(node.Campos))
	for k, c := range node.Campos {
		campos[k].Nome = c.Nome.Value
		if c.Embutida != nil {
			v := i.Eval(c.Embutida, env)
			if isError(v) {
				return v
			}
			t, ok := v.(*object.Treta)
			if !ok {
				return newError(linha, "puxadinho na treta %s: %s", node.Nome.Value, object.MsgNaoETreta(c.Embutida.String(), v))
			}
			campos[k].Embutida = t
			continue
		}
		if c.Padrao != nil {
			v := i.thunkDoPadrao(c.Padrao, env, node.Nome.Value+"."+c.Nome.Value+"(padrao)")
			if isError(v) {
				return v
			}
			campos[k].Padrao = v
		}
	}
	t, msg := object.NovaTreta(node.Nome.Value, campos)
	if msg != "" {
		return newError(linha, "%s", msg)
	}
	env.Set(node.Nome.Value, t)
	return NADA
}

func (i *Interpreter) evalCombinadoDecl(node *ast.CombinadoDecl, env *object.Environment) object.Object {
	linha := node.Token.Line
	var proprios []object.AssinaturaMetodo
	var embutidos []*object.Combinado
	for _, m := range node.Metodos {
		if m.Embutido != nil {
			v := i.Eval(m.Embutido, env)
			if isError(v) {
				return v
			}
			c, ok := v.(*object.Combinado)
			if !ok {
				return newError(linha, "no combinado %s: `%s` nao e combinado (e %s)", node.Nome.Value, m.Embutido.String(), object.NomeTipo(v))
			}
			embutidos = append(embutidos, c)
			continue
		}
		proprios = append(proprios, object.AssinaturaMetodo{Nome: m.Nome.Value, NumParams: len(m.Parametros)})
	}
	c, msg := object.NovoCombinado(node.Nome.Value, proprios, embutidos)
	if msg != "" {
		return newError(linha, "%s", msg)
	}
	env.Set(node.Nome.Value, c)
	return NADA
}

func (i *Interpreter) evalMetodoDecl(node *ast.MetodoDecl, env *object.Environment) object.Object {
	v := i.evalIdentifier(node.Tipo, env)
	if isError(v) {
		return v
	}
	t, ok := v.(*object.Treta)
	if !ok {
		return newError(node.Token.Line, "metodo %s: %s", node.Nome.Value, object.MsgNaoETreta(node.Tipo.Value, v))
	}
	fn := &object.Funcao{Parametros: node.ParametrosComReceptor(), Body: node.Body, Env: env, Nome: node.Tipo.Value + "." + node.Nome.Value}
	if msg := t.DefineMetodo(node.Nome.Value, fn); msg != "" {
		return newError(node.Token.Line, "%s", msg)
	}
	return NADA
}

func (i *Interpreter) evalTretaLiteral(node *ast.TretaLiteral, env *object.Environment) object.Object {
	linha := node.Token.Line
	v := i.Eval(node.Tipo, env)
	if isError(v) {
		return v
	}
	t, ok := v.(*object.Treta)
	if !ok {
		return newError(linha, "nao da pra fazer %s{...}: %s", node.Tipo.String(), object.MsgNaoETreta(node.Tipo.String(), v))
	}
	valores := make([]object.Object, len(node.Valores))
	for k, e := range node.Valores {
		val := i.Eval(e, env)
		if isError(val) {
			return val
		}
		valores[k] = val
	}
	var nomes []string
	if !node.Posicional() {
		nomes = make([]string, len(node.Nomes))
		for k, n := range node.Nomes {
			nomes[k] = n.Value
		}
	}
	res, msg := object.MontaInstancia(t, nomes, valores, func(fn object.Object) object.Object {
		return i.applyFunction(fn, nil, linha, "<padrao de "+t.Nome+">")
	})
	if msg != "" {
		return newError(linha, "%s", msg)
	}
	return res
}

// membroDaInstancia e o `obj.nome` / `obj["nome"]` numa instancia.
func membroDaInstancia(inst *object.Instancia, idx object.Object, linha int) object.Object {
	nome, ok := idx.(*object.Texto)
	if !ok {
		return newError(linha, "%s", object.MsgCampoNaoTexto(idx))
	}
	v, msg := inst.Membro(nome.Value)
	if msg != "" {
		return newError(linha, "%s", msg)
	}
	return v
}

// poeMembroDaInstancia e o `bota obj.nome = v`.
func poeMembroDaInstancia(inst *object.Instancia, idx, val object.Object, linha int) object.Object {
	nome, ok := idx.(*object.Texto)
	if !ok {
		return newError(linha, "%s", object.MsgCampoNaoTexto(idx))
	}
	if msg := inst.PoeMembro(nome.Value, val); msg != "" {
		return newError(linha, "%s", msg)
	}
	return NADA
}

// desembrulhaMetodo confere a aridade (sem o receiver) e devolve a funcao com
// o receiver na frente dos args.
func desembrulhaMetodo(m *object.MetodoLigado, args []object.Object, linha int) (object.Object, []object.Object, object.Object) {
	if msg := object.ChecaAridadeMetodo(m, len(args)); msg != "" {
		return nil, nil, newError(linha, "%s", msg)
	}
	comRecv := make([]object.Object, 0, len(args)+1)
	comRecv = append(comRecv, m.Receptor)
	return m.Fn, append(comRecv, args...), nil
}

// builtins de POO: registradas aqui (e nao no mapa grande) pra ficar tudo
// junto. Puras: valem igual nos dois engines.
func init() {
	builtins["satisfaz"] = &object.Builtin{Nome: "satisfaz", Fn: builtinSatisfaz}
	builtins["como_tipo"] = &object.Builtin{Nome: "como_tipo", Fn: builtinComoTipo}
}

// satisfaz(valor, Tipo) -> booleano: Tipo treta = e instancia dela; Tipo
// combinado = tem os metodos (satisfacao implicita, igual Go).
func builtinSatisfaz(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("satisfaz() quer 2 argumentos (valor, Tipo), veio %d", len(args))
	}
	ok, _, valido := object.Satisfaz(args[0], args[1])
	if !valido {
		return erroBuiltin("satisfaz(valor, Tipo): o Tipo tem que ser treta ou combinado, veio %s", object.NomeTipo(args[1]))
	}
	return boolDoNativo(ok)
}

// como_tipo(valor, Tipo) -> valor: o type assertion do Go (`v.(Ponto)`).
// Devolve o proprio valor se ele satisfaz o Tipo; senao quebra com o motivo.
func builtinComoTipo(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("como_tipo() quer 2 argumentos (valor, Tipo), veio %d", len(args))
	}
	ok, motivo, valido := object.Satisfaz(args[0], args[1])
	if !valido {
		return erroBuiltin("como_tipo(valor, Tipo): o Tipo tem que ser treta ou combinado, veio %s", object.NomeTipo(args[1]))
	}
	if !ok {
		return erroBuiltin("como_tipo: %s", motivo)
	}
	return args[0]
}
