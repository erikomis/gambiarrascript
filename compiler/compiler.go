package compiler

import (
	"fmt"

	"gambiarrascript/ast"
	"gambiarrascript/code"
	"gambiarrascript/object"
)

type Compiler struct {
	instructions code.Instructions
	constants    []object.Object
}

func New() *Compiler {
	return &Compiler{instructions: code.Instructions{}, constants: []object.Object{}}
}

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{Instructions: c.instructions, Constants: c.constants}
}

func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, s := range node.Statements {
			if err := c.Compile(s); err != nil {
				return err
			}
		}
	case *ast.ExpressionStatement:
		if err := c.Compile(node.Expression); err != nil {
			return err
		}
		c.emit(code.OpPop)
	case *ast.MostraStatement:
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		c.emit(code.OpMostra)
	case *ast.NumeroLiteral:
		c.emit(code.OpConstant, c.addConstant(&object.Numero{Value: node.Value}))
	case *ast.TextoLiteral:
		c.emit(code.OpConstant, c.addConstant(&object.Texto{Value: node.Value}))
	case *ast.BooleanoLiteral:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}
	case *ast.NadaLiteral:
		c.emit(code.OpNada)
	case *ast.PrefixExpression:
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		switch node.Operator {
		case "-":
			c.emit(code.OpMinus)
		case "nao":
			c.emit(code.OpNao)
		default:
			return fmt.Errorf("operador prefixo desconhecido na VM: %s", node.Operator)
		}
	case *ast.InfixExpression:
		if node.Operator == "e" || node.Operator == "ou" {
			return fmt.Errorf("a VM ainda nao sabe fazer '%s' (vem na proxima parte)", node.Operator)
		}
		if node.Operator == "<" || node.Operator == "<=" {
			if err := c.Compile(node.Right); err != nil {
				return err
			}
			if err := c.Compile(node.Left); err != nil {
				return err
			}
			if node.Operator == "<" {
				c.emit(code.OpGreaterThan)
			} else {
				c.emit(code.OpGreaterEqual)
			}
			return nil
		}
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		switch node.Operator {
		case "+":
			c.emit(code.OpAdd)
		case "-":
			c.emit(code.OpSub)
		case "*":
			c.emit(code.OpMul)
		case "/":
			c.emit(code.OpDiv)
		case "%":
			c.emit(code.OpMod)
		case ">":
			c.emit(code.OpGreaterThan)
		case ">=":
			c.emit(code.OpGreaterEqual)
		case "==":
			c.emit(code.OpEqual)
		case "!=":
			c.emit(code.OpNotEqual)
		default:
			return fmt.Errorf("operador infixo desconhecido na VM: %s", node.Operator)
		}
	default:
		return fmt.Errorf("a VM ainda nao sabe compilar %T (vem numa proxima parte)", node)
	}
	return nil
}

func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := len(c.instructions)
	c.instructions = append(c.instructions, ins...)
	return pos
}
