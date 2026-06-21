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

func (i *Interpreter) evalIndex(left, index object.Object, linha int) object.Object {
	lista, ok := left.(*object.Lista)
	if !ok {
		return newError(linha, "so da pra indexar lista, e isso ai e %s", left.Type())
	}
	idx, ok := index.(*object.Numero)
	if !ok {
		return newError(linha, "indice de lista tem que ser numero, veio %s", index.Type())
	}
	pos := int(idx.Value)
	if pos < 0 || pos >= len(lista.Elements) {
		return newError(linha, "esse indice (%d) ta fora da lista, o", pos)
	}
	return lista.Elements[pos]
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
	case *object.Nada:
		return true
	}
	return a == b
}
