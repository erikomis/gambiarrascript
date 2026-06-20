package ast

import (
	"strings"

	"gambiarrascript/token"
)

type Node interface {
	TokenLiteral() string
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

// ---- Program ----

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out strings.Builder
	for i, s := range p.Statements {
		if i > 0 {
			out.WriteString("\n")
		}
		out.WriteString(s.String())
	}
	return out.String()
}

// ---- Statements ----

type BotaStatement struct {
	Token token.Token
	Name  *Identifier
	Value Expression
}

func (s *BotaStatement) statementNode()       {}
func (s *BotaStatement) TokenLiteral() string { return s.Token.Literal }
func (s *BotaStatement) String() string {
	return "bota " + s.Name.String() + " = " + s.Value.String()
}

type MostraStatement struct {
	Token token.Token
	Value Expression
}

func (s *MostraStatement) statementNode()       {}
func (s *MostraStatement) TokenLiteral() string { return s.Token.Literal }
func (s *MostraStatement) String() string       { return "mostra " + s.Value.String() }

type FuncionaStatement struct {
	Token token.Token
	Value Expression
}

func (s *FuncionaStatement) statementNode()       {}
func (s *FuncionaStatement) TokenLiteral() string { return s.Token.Literal }
func (s *FuncionaStatement) String() string       { return "funciona " + s.Value.String() }

type VazaStatement struct{ Token token.Token }

func (s *VazaStatement) statementNode()       {}
func (s *VazaStatement) TokenLiteral() string { return s.Token.Literal }
func (s *VazaStatement) String() string       { return "vaza" }

type ContinuaStatement struct{ Token token.Token }

func (s *ContinuaStatement) statementNode()       {}
func (s *ContinuaStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ContinuaStatement) String() string       { return "continua" }

type ExpressionStatement struct {
	Token      token.Token
	Expression Expression
}

func (s *ExpressionStatement) statementNode()       {}
func (s *ExpressionStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ExpressionStatement) String() string {
	if s.Expression != nil {
		return s.Expression.String()
	}
	return ""
}

type BlockStatement struct {
	Token      token.Token
	Statements []Statement
}

func (s *BlockStatement) statementNode()       {}
func (s *BlockStatement) TokenLiteral() string { return s.Token.Literal }
func (s *BlockStatement) String() string {
	var out strings.Builder
	for _, st := range s.Statements {
		out.WriteString(st.String())
		out.WriteString("\n")
	}
	return out.String()
}

type SeColarStatement struct {
	Token        token.Token
	Conditions   []Expression
	Consequences []*BlockStatement
	Alternative  *BlockStatement
}

func (s *SeColarStatement) statementNode()       {}
func (s *SeColarStatement) TokenLiteral() string { return s.Token.Literal }
func (s *SeColarStatement) String() string {
	var out strings.Builder
	for i, c := range s.Conditions {
		if i == 0 {
			out.WriteString("se_colar ")
		} else {
			out.WriteString("se_nao_colar se_colar ")
		}
		out.WriteString(c.String())
		out.WriteString(" ")
		if i < len(s.Consequences) {
			out.WriteString(s.Consequences[i].String())
		}
	}
	if s.Alternative != nil {
		out.WriteString("se_nao_colar ")
		out.WriteString(s.Alternative.String())
	}
	out.WriteString("acabou_finalmente")
	return out.String()
}

type EnquantoStatement struct {
	Token     token.Token
	Condition Expression
	Body      *BlockStatement
}

func (s *EnquantoStatement) statementNode()       {}
func (s *EnquantoStatement) TokenLiteral() string { return s.Token.Literal }
func (s *EnquantoStatement) String() string {
	return "enquanto " + s.Condition.String() + " " + s.Body.String() + "acabou_finalmente"
}

type PraCadaNumStatement struct {
	Token token.Token
	Var   *Identifier
	Start Expression
	End   Expression
	Body  *BlockStatement
}

func (s *PraCadaNumStatement) statementNode()       {}
func (s *PraCadaNumStatement) TokenLiteral() string { return s.Token.Literal }
func (s *PraCadaNumStatement) String() string {
	return "pra_cada " + s.Var.String() + " de " + s.Start.String() + " ate " + s.End.String() + " " + s.Body.String() + "acabou_finalmente"
}

type PraCadaListStatement struct {
	Token    token.Token
	Var      *Identifier
	Iterable Expression
	Body     *BlockStatement
}

func (s *PraCadaListStatement) statementNode()       {}
func (s *PraCadaListStatement) TokenLiteral() string { return s.Token.Literal }
func (s *PraCadaListStatement) String() string {
	return "pra_cada " + s.Var.String() + " em " + s.Iterable.String() + " " + s.Body.String() + "acabou_finalmente"
}

type GambiarraStatement struct {
	Token      token.Token
	Name       *Identifier
	Parameters []*Identifier
	Body       *BlockStatement
}

func (s *GambiarraStatement) statementNode()       {}
func (s *GambiarraStatement) TokenLiteral() string { return s.Token.Literal }
func (s *GambiarraStatement) String() string {
	params := make([]string, len(s.Parameters))
	for i, p := range s.Parameters {
		params[i] = p.String()
	}
	return "gambiarra " + s.Name.String() + "(" + strings.Join(params, ", ") + ") " + s.Body.String() + "acabou_finalmente"
}

type ArrumaStatement struct {
	Token   token.Token
	Try     *BlockStatement
	ErrName *Identifier
	Catch   *BlockStatement
}

func (s *ArrumaStatement) statementNode()       {}
func (s *ArrumaStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ArrumaStatement) String() string {
	return "arruma " + s.Try.String() + "quebrou " + s.ErrName.String() + " " + s.Catch.String() + "acabou_finalmente"
}

// ---- Expressions ----

type Identifier struct {
	Token token.Token
	Value string
}

func (e *Identifier) expressionNode()      {}
func (e *Identifier) TokenLiteral() string { return e.Token.Literal }
func (e *Identifier) String() string       { return e.Value }

type NumeroLiteral struct {
	Token token.Token
	Value float64
}

func (e *NumeroLiteral) expressionNode()      {}
func (e *NumeroLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *NumeroLiteral) String() string       { return e.Token.Literal }

type TextoLiteral struct {
	Token token.Token
	Value string
}

func (e *TextoLiteral) expressionNode()      {}
func (e *TextoLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *TextoLiteral) String() string       { return `"` + e.Value + `"` }

type BooleanoLiteral struct {
	Token token.Token
	Value bool
}

func (e *BooleanoLiteral) expressionNode()      {}
func (e *BooleanoLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *BooleanoLiteral) String() string       { return e.Token.Literal }

type NadaLiteral struct{ Token token.Token }

func (e *NadaLiteral) expressionNode()      {}
func (e *NadaLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *NadaLiteral) String() string       { return "nada" }

type ListaLiteral struct {
	Token    token.Token
	Elements []Expression
}

func (e *ListaLiteral) expressionNode()      {}
func (e *ListaLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *ListaLiteral) String() string {
	elems := make([]string, len(e.Elements))
	for i, el := range e.Elements {
		elems[i] = el.String()
	}
	return "[" + strings.Join(elems, ", ") + "]"
}

type PrefixExpression struct {
	Token    token.Token
	Operator string
	Right    Expression
}

func (e *PrefixExpression) expressionNode()      {}
func (e *PrefixExpression) TokenLiteral() string { return e.Token.Literal }
func (e *PrefixExpression) String() string {
	return "(" + e.Operator + e.Right.String() + ")"
}

type InfixExpression struct {
	Token    token.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (e *InfixExpression) expressionNode()      {}
func (e *InfixExpression) TokenLiteral() string { return e.Token.Literal }
func (e *InfixExpression) String() string {
	return "(" + e.Left.String() + " " + e.Operator + " " + e.Right.String() + ")"
}

type CallExpression struct {
	Token     token.Token
	Function  Expression
	Arguments []Expression
}

func (e *CallExpression) expressionNode()      {}
func (e *CallExpression) TokenLiteral() string { return e.Token.Literal }
func (e *CallExpression) String() string {
	args := make([]string, len(e.Arguments))
	for i, a := range e.Arguments {
		args[i] = a.String()
	}
	return e.Function.String() + "(" + strings.Join(args, ", ") + ")"
}

type IndexExpression struct {
	Token token.Token
	Left  Expression
	Index Expression
}

func (e *IndexExpression) expressionNode()      {}
func (e *IndexExpression) TokenLiteral() string { return e.Token.Literal }
func (e *IndexExpression) String() string {
	return "(" + e.Left.String() + "[" + e.Index.String() + "])"
}
