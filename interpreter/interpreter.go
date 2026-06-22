package interpreter

import (
	"fmt"
	"io"
	"math"

	"gambiarrascript/ast"
	"gambiarrascript/object"
)

var (
	DEU_BOM  = &object.Booleano{Value: true}
	DEU_RUIM = &object.Booleano{Value: false}
	NADA     = &object.Nada{}
)

type Interpreter struct {
	out io.Writer
}

func New(out io.Writer) *Interpreter {
	return &Interpreter{out: out}
}

func (i *Interpreter) Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	case *ast.Program:
		return i.evalProgram(node, env)
	case *ast.ExpressionStatement:
		return i.Eval(node.Expression, env)
	case *ast.MostraStatement:
		val := i.Eval(node.Value, env)
		if isError(val) {
			return val
		}
		fmt.Fprintln(i.out, val.Inspect())
		return val

	// --- literais ---
	case *ast.NumeroLiteral:
		return &object.Numero{Value: node.Value}
	case *ast.TextoLiteral:
		return &object.Texto{Value: node.Value}
	case *ast.BooleanoLiteral:
		return boolDoNativo(node.Value)
	case *ast.NadaLiteral:
		return NADA
	case *ast.Identifier:
		return i.evalIdentifier(node, env)
	case *ast.ListaLiteral:
		elems := i.evalExpressions(node.Elements, env)
		if len(elems) == 1 && isError(elems[0]) {
			return elems[0]
		}
		return &object.Lista{Elements: elems}
	case *ast.DicionarioLiteral:
		return i.evalDicionario(node, env)

	// --- operadores ---
	case *ast.PrefixExpression:
		right := i.Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return i.evalPrefix(node.Operator, right, node.Token.Line)
	case *ast.InfixExpression:
		return i.evalInfix(node, env)
	case *ast.IndexExpression:
		left := i.Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := i.Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return i.evalIndex(left, index, node.Token.Line)

	case *ast.BotaStatement:
		val := i.Eval(node.Value, env)
		if isError(val) {
			return val
		}
		if node.Name != nil {
			env.Set(node.Name.Value, val)
			return NADA
		}
		return i.evalAtribuiIndice(node.Indice, val, env)
	case *ast.FuncionaStatement:
		val := i.Eval(node.Value, env)
		if isError(val) {
			return val
		}
		return &object.Retorno{Value: val}
	case *ast.VazaStatement:
		return &object.Vaza{Line: node.Token.Line}
	case *ast.ContinuaStatement:
		return &object.Continua{Line: node.Token.Line}
	case *ast.BlockStatement:
		return i.evalBlock(node, env)
	case *ast.SeColarStatement:
		return i.evalSeColar(node, env)
	case *ast.EnquantoStatement:
		return i.evalEnquanto(node, env)
	case *ast.PraCadaNumStatement:
		return i.evalPraCadaNum(node, env)
	case *ast.PraCadaListStatement:
		return i.evalPraCadaList(node, env)
	case *ast.GambiarraStatement:
		fn := &object.Funcao{Parameters: node.Parameters, Body: node.Body, Env: env}
		env.Set(node.Name.Value, fn)
		return NADA
	case *ast.ArrumaStatement:
		return i.evalArruma(node, env)
	case *ast.CallExpression:
		fn := i.Eval(node.Function, env)
		if isError(fn) {
			return fn
		}
		args := i.evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return i.applyFunction(fn, args, node.Token.Line)
	}
	return NADA
}

func (i *Interpreter) evalProgram(prog *ast.Program, env *object.Environment) object.Object {
	var result object.Object = NADA
	for _, stmt := range prog.Statements {
		result = i.Eval(stmt, env)
		switch r := result.(type) {
		case *object.Retorno:
			return r.Value
		case *object.Erro:
			return r
		case *object.Vaza:
			return newError(r.Line, "deu `vaza` fora de um loop, parca — vaza pra onde?")
		case *object.Continua:
			return newError(r.Line, "deu `continua` fora de um loop, parca")
		}
	}
	return result
}

func (i *Interpreter) evalExpressions(exps []ast.Expression, env *object.Environment) []object.Object {
	var result []object.Object
	for _, e := range exps {
		ev := i.Eval(e, env)
		if isError(ev) {
			return []object.Object{ev}
		}
		result = append(result, ev)
	}
	return result
}

func (i *Interpreter) evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}
	return newError(node.Token.Line, "cade o `%s`? voce nao botou isso ainda", node.Value)
}

func (i *Interpreter) evalPrefix(op string, right object.Object, linha int) object.Object {
	switch op {
	case "nao":
		return boolDoNativo(!isTruthy(right))
	case "-":
		num, ok := right.(*object.Numero)
		if !ok {
			return newError(linha, "nao da pra colocar - na frente de %s", right.Type())
		}
		return &object.Numero{Value: -num.Value}
	}
	return newError(linha, "operador prefixo desconhecido: %s", op)
}

func (i *Interpreter) evalInfix(node *ast.InfixExpression, env *object.Environment) object.Object {
	// operadores logicos com curto-circuito
	if node.Operator == "e" || node.Operator == "ou" {
		left := i.Eval(node.Left, env)
		if isError(left) {
			return left
		}
		if node.Operator == "e" && !isTruthy(left) {
			return DEU_RUIM
		}
		if node.Operator == "ou" && isTruthy(left) {
			return DEU_BOM
		}
		right := i.Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return boolDoNativo(isTruthy(right))
	}

	left := i.Eval(node.Left, env)
	if isError(left) {
		return left
	}
	right := i.Eval(node.Right, env)
	if isError(right) {
		return right
	}

	ln, lok := left.(*object.Numero)
	rn, rok := right.(*object.Numero)
	if lok && rok {
		return i.evalInfixNumero(node.Operator, ln.Value, rn.Value, node.Token.Line)
	}

	if node.Operator == "+" && (left.Type() == object.TEXTO_OBJ || right.Type() == object.TEXTO_OBJ) {
		return &object.Texto{Value: left.Inspect() + right.Inspect()}
	}

	switch node.Operator {
	case "==":
		return boolDoNativo(iguais(left, right))
	case "!=":
		return boolDoNativo(!iguais(left, right))
	}

	return newError(node.Token.Line, "nao da pra fazer %s %s %s", left.Type(), node.Operator, right.Type())
}

func (i *Interpreter) evalInfixNumero(op string, l, r float64, linha int) object.Object {
	switch op {
	case "+":
		return &object.Numero{Value: l + r}
	case "-":
		return &object.Numero{Value: l - r}
	case "*":
		return &object.Numero{Value: l * r}
	case "/":
		if r == 0 {
			return newError(linha, "nao da pra dividir por zero, parca — nem na gambiarra")
		}
		return &object.Numero{Value: l / r}
	case "%":
		if r == 0 {
			return newError(linha, "resto de divisao por zero? ai voce quer demais")
		}
		return &object.Numero{Value: math.Mod(l, r)}
	case "<":
		return boolDoNativo(l < r)
	case ">":
		return boolDoNativo(l > r)
	case "<=":
		return boolDoNativo(l <= r)
	case ">=":
		return boolDoNativo(l >= r)
	case "==":
		return boolDoNativo(l == r)
	case "!=":
		return boolDoNativo(l != r)
	}
	return newError(linha, "operador desconhecido pra numeros: %s", op)
}

func (i *Interpreter) evalAtribuiIndice(alvo *ast.IndexExpression, val object.Object, env *object.Environment) object.Object {
	cont := i.Eval(alvo.Left, env)
	if isError(cont) {
		return cont
	}
	idx := i.Eval(alvo.Index, env)
	if isError(idx) {
		return idx
	}
	linha := alvo.Token.Line
	switch c := cont.(type) {
	case *object.Lista:
		n, ok := idx.(*object.Numero)
		if !ok {
			return newError(linha, "indice de lista tem que ser numero, veio %s", idx.Type())
		}
		pos := int(n.Value)
		if pos < 0 || pos >= len(c.Elements) {
			return newError(linha, "esse indice (%d) ta fora da lista, o", pos)
		}
		c.Elements[pos] = val
		return NADA
	case *object.Dicionario:
		chave, ok := idx.(object.Chaveavel)
		if !ok {
			return newError(linha, "essa chave (%s) nao da pra usar num dicionario", idx.Type())
		}
		c.Pares[chave.ChaveHash()] = object.ParDic{Chave: idx, Valor: val}
		return NADA
	default:
		return newError(linha, "so da pra atribuir indice em lista ou dicionario, e isso ai e %s", cont.Type())
	}
}

func (i *Interpreter) evalDicionario(node *ast.DicionarioLiteral, env *object.Environment) object.Object {
	pares := map[object.HashKey]object.ParDic{}
	for _, par := range node.Pares {
		chave := i.Eval(par.Chave, env)
		if isError(chave) {
			return chave
		}
		chaveavel, ok := chave.(object.Chaveavel)
		if !ok {
			return newError(node.Token.Line, "nao da pra usar %s como chave de dicionario", chave.Type())
		}
		valor := i.Eval(par.Valor, env)
		if isError(valor) {
			return valor
		}
		pares[chaveavel.ChaveHash()] = object.ParDic{Chave: chave, Valor: valor}
	}
	return &object.Dicionario{Pares: pares}
}

func (i *Interpreter) evalIndex(left, index object.Object, linha int) object.Object {
	switch c := left.(type) {
	case *object.Lista:
		idx, ok := index.(*object.Numero)
		if !ok {
			return newError(linha, "indice de lista tem que ser numero, veio %s", index.Type())
		}
		pos := int(idx.Value)
		if pos < 0 || pos >= len(c.Elements) {
			return newError(linha, "esse indice (%d) ta fora da lista, o", pos)
		}
		return c.Elements[pos]
	case *object.Dicionario:
		chave, ok := index.(object.Chaveavel)
		if !ok {
			return newError(linha, "essa chave (%s) nao da pra usar num dicionario", index.Type())
		}
		par, existe := c.Pares[chave.ChaveHash()]
		if !existe {
			return NADA
		}
		return par.Valor
	default:
		return newError(linha, "so da pra indexar lista ou dicionario, e isso ai e %s", left.Type())
	}
}

func boolDoNativo(b bool) *object.Booleano {
	if b {
		return DEU_BOM
	}
	return DEU_RUIM
}

func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Nada:
		return false
	case *object.Booleano:
		return obj.Value
	default:
		return true
	}
}

func iguais(a, b object.Object) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch av := a.(type) {
	case *object.Texto:
		return av.Value == b.(*object.Texto).Value
	case *object.Booleano:
		return av.Value == b.(*object.Booleano).Value
	case *object.Numero:
		return av.Value == b.(*object.Numero).Value
	case *object.Nada:
		return true
	case *object.Dicionario:
		bd := b.(*object.Dicionario)
		if len(av.Pares) != len(bd.Pares) {
			return false
		}
		for k, pa := range av.Pares {
			pb, ok := bd.Pares[k]
			if !ok || !iguais(pa.Valor, pb.Valor) {
				return false
			}
		}
		return true
	}
	return a == b
}

// evalBlock avalia statements em sequencia e propaga sinais de controle
// (Retorno, Erro, Vaza, Continua) sem desembrulhar — quem propaga decide.
func (i *Interpreter) evalBlock(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object = NADA
	for _, stmt := range block.Statements {
		result = i.Eval(stmt, env)
		if result != nil {
			switch result.Type() {
			case object.RETORNO_OBJ, object.ERRO_OBJ, object.VAZA_OBJ, object.CONTINUA_OBJ:
				return result
			}
		}
	}
	return result
}

func (i *Interpreter) evalSeColar(node *ast.SeColarStatement, env *object.Environment) object.Object {
	for idx, cond := range node.Conditions {
		c := i.Eval(cond, env)
		if isError(c) {
			return c
		}
		if isTruthy(c) {
			return i.evalBlock(node.Consequences[idx], env)
		}
	}
	if node.Alternative != nil {
		return i.evalBlock(node.Alternative, env)
	}
	return NADA
}

func (i *Interpreter) evalEnquanto(node *ast.EnquantoStatement, env *object.Environment) object.Object {
	for {
		c := i.Eval(node.Condition, env)
		if isError(c) {
			return c
		}
		if !isTruthy(c) {
			break
		}
		res := i.evalBlock(node.Body, env)
		if res != nil {
			switch res.Type() {
			case object.ERRO_OBJ, object.RETORNO_OBJ:
				return res
			case object.VAZA_OBJ:
				return NADA
			case object.CONTINUA_OBJ:
				continue
			}
		}
	}
	return NADA
}

func (i *Interpreter) evalPraCadaNum(node *ast.PraCadaNumStatement, env *object.Environment) object.Object {
	inicio := i.Eval(node.Start, env)
	if isError(inicio) {
		return inicio
	}
	fim := i.Eval(node.End, env)
	if isError(fim) {
		return fim
	}
	ni, ok1 := inicio.(*object.Numero)
	nf, ok2 := fim.(*object.Numero)
	if !ok1 || !ok2 {
		return newError(node.Token.Line, "no pra_cada de..ate eu preciso de numeros, parca")
	}
	for v := ni.Value; v <= nf.Value; v++ {
		env.Set(node.Var.Value, &object.Numero{Value: v})
		res := i.evalBlock(node.Body, env)
		if res != nil {
			switch res.Type() {
			case object.ERRO_OBJ, object.RETORNO_OBJ:
				return res
			case object.VAZA_OBJ:
				return NADA
			case object.CONTINUA_OBJ:
				continue
			}
		}
	}
	return NADA
}

func (i *Interpreter) evalPraCadaList(node *ast.PraCadaListStatement, env *object.Environment) object.Object {
	it := i.Eval(node.Iterable, env)
	if isError(it) {
		return it
	}

	var itens []object.Object
	switch c := it.(type) {
	case *object.Lista:
		itens = c.Elements
	case *object.Dicionario:
		for _, par := range c.Pares {
			itens = append(itens, par.Chave)
		}
	default:
		return newError(node.Token.Line, "pra_cada ... em ... so funciona com lista ou dicionario, e isso ai e %s", it.Type())
	}

	for _, item := range itens {
		env.Set(node.Var.Value, item)
		res := i.evalBlock(node.Body, env)
		if res != nil {
			switch res.Type() {
			case object.ERRO_OBJ, object.RETORNO_OBJ:
				return res
			case object.VAZA_OBJ:
				return NADA
			case object.CONTINUA_OBJ:
				continue
			}
		}
	}
	return NADA
}

func (i *Interpreter) evalArruma(node *ast.ArrumaStatement, env *object.Environment) object.Object {
	res := i.evalBlock(node.Try, env)
	if res != nil && res.Type() == object.ERRO_OBJ {
		erro := res.(*object.Erro)
		env.Set(node.ErrName.Value, &object.Texto{Value: erro.Message})
		return i.evalBlock(node.Catch, env)
	}
	if res != nil {
		switch res.Type() {
		case object.RETORNO_OBJ, object.VAZA_OBJ, object.CONTINUA_OBJ:
			return res
		}
	}
	return NADA
}

func (i *Interpreter) applyFunction(fn object.Object, args []object.Object, linha int) object.Object {
	funcao, ok := fn.(*object.Funcao)
	if !ok {
		return newError(linha, "isso ai (%s) nao e gambiarra pra voce sair chamando", fn.Type())
	}
	if len(args) != len(funcao.Parameters) {
		return newError(linha, "essa gambiarra quer %d parametro(s), voce mandou %d", len(funcao.Parameters), len(args))
	}
	escopo := object.NewEnclosedEnvironment(funcao.Env)
	for idx, p := range funcao.Parameters {
		escopo.Set(p.Value, args[idx])
	}
	avaliado := i.evalBlock(funcao.Body, escopo)
	if ret, ok := avaliado.(*object.Retorno); ok {
		return ret.Value
	}
	if isError(avaliado) {
		return avaliado
	}
	if v, ok := avaliado.(*object.Vaza); ok {
		return newError(v.Line, "deu `vaza` fora de um loop, parca — vaza pra onde?")
	}
	if c, ok := avaliado.(*object.Continua); ok {
		return newError(c.Line, "deu `continua` fora de um loop, parca")
	}
	return NADA
}
