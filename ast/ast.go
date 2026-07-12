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
	Token  token.Token
	Name   *Identifier      // setado quando o alvo e uma variavel simples
	Indice *IndexExpression // setado quando o alvo e uma atribuicao por indice
	Value  Expression
	// OpComposto marca atribuicao composta (`x += 1`, sem `bota`): o parser
	// desugara Value pra `x + 1`, e o formatter usa isso pra reimprimir a
	// forma original. Engines tratam como BotaStatement normal.
	OpComposto string
}

func (s *BotaStatement) statementNode()       {}
func (s *BotaStatement) TokenLiteral() string { return s.Token.Literal }
func (s *BotaStatement) String() string {
	alvo := ""
	if s.Name != nil {
		alvo = s.Name.String()
	} else if s.Indice != nil {
		alvo = s.Indice.String()
	}
	return "bota " + alvo + " = " + s.Value.String()
}

// EscolheStatement e o switch/match:
//
//	escolhe x
//	caso 1, 2
//	    ...
//	caso 3
//	    ...
//	se_nao_colar
//	    ...
//	acabou_finalmente
//
// Sem fallthrough: casa o primeiro caso igual (semantica do ==) e sai.
type EscolheStatement struct {
	Token   token.Token
	Subject Expression
	Casos   []CasoBraco
	Default *BlockStatement // bloco do se_nao_colar (opcional)
}

// CasoBraco e um braco `caso v1, v2, ...` com o corpo.
type CasoBraco struct {
	Values []Expression
	Body   *BlockStatement
}

func (s *EscolheStatement) statementNode()       {}
func (s *EscolheStatement) TokenLiteral() string { return s.Token.Literal }
func (s *EscolheStatement) String() string {
	var sb strings.Builder
	sb.WriteString("escolhe " + s.Subject.String() + " ")
	for _, c := range s.Casos {
		vals := make([]string, len(c.Values))
		for i, v := range c.Values {
			vals[i] = v.String()
		}
		sb.WriteString("caso " + strings.Join(vals, ", ") + " " + c.Body.String())
	}
	if s.Default != nil {
		sb.WriteString("se_nao_colar " + s.Default.String())
	}
	sb.WriteString("acabou_finalmente")
	return sb.String()
}

// DesestruturaStatement e `bota [a, b] = lista` (por posicao) ou
// `bota {x, y} = dict` (por chave). Nome sem valor correspondente vira nada
// (lenient — e gambiarra, nao pattern matching de Haskell).
type DesestruturaStatement struct {
	Token  token.Token // o 'bota'
	Names  []*Identifier
	DeDict bool // true: {x, y} por chave; false: [a, b] por posicao
	Value  Expression
}

func (s *DesestruturaStatement) statementNode()       {}
func (s *DesestruturaStatement) TokenLiteral() string { return s.Token.Literal }
func (s *DesestruturaStatement) String() string {
	nomes := make([]string, len(s.Names))
	for i, n := range s.Names {
		nomes[i] = n.Value
	}
	abre, fecha := "[", "]"
	if s.DeDict {
		abre, fecha = "{", "}"
	}
	return "bota " + abre + strings.Join(nomes, ", ") + fecha + " = " + s.Value.String()
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
	Vars     []*Identifier // 1 nome (valor) ou 2 nomes (indice/chave, valor)
	Iterable Expression
	Body     *BlockStatement
}

func (s *PraCadaListStatement) statementNode()       {}
func (s *PraCadaListStatement) TokenLiteral() string { return s.Token.Literal }
func (s *PraCadaListStatement) String() string {
	nomes := make([]string, len(s.Vars))
	for i, v := range s.Vars {
		nomes[i] = v.String()
	}
	return "pra_cada " + strings.Join(nomes, ", ") + " em " + s.Iterable.String() + " " + s.Body.String() + "acabou_finalmente"
}

type GambiarraStatement struct {
	Token       token.Token
	Name        *Identifier
	Parameters  []*Parametro
	Body        *BlockStatement
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
	Finally *BlockStatement // opcional; bloco roda sempre (try+catch), com/sem erro
}

func (s *ArrumaStatement) statementNode()       {}
func (s *ArrumaStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ArrumaStatement) String() string {
	out := "arruma " + s.Try.String() + "quebrou " + s.ErrName.String() + " " + s.Catch.String()
	if s.Finally != nil {
		out += "finalmente " + s.Finally.String()
	}
	out += "acabou_finalmente"
	return out
}

// ImportaStatement carrega e executa outro arquivo .gs, trazendo suas
// definicoes (bota/gambiarra) para o escopo atual.
type ImportaStatement struct {
	Token token.Token
	Path  Expression
	Alias *Identifier // nil = importa sem alias (despeja no escopo)
}

func (s *ImportaStatement) statementNode()       {}
func (s *ImportaStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ImportaStatement) String() string {
	if s.Alias != nil {
		return "importa " + s.Path.String() + " como " + s.Alias.Value
	}
	return "importa " + s.Path.String()
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
	Int   int64 // valor exato quando EhInt
	EhInt bool  // true se o literal nao tem ponto/expoente (inteiro)
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

// TextoInterpolado representa uma string com interpolação `${expr}`. Parts
// alterna *TextoLiteral (pedaco literal fixo) e Expression (expressao a
// avaliar e converter pra texto).
type TextoInterpolado struct {
	Token token.Token
	Parts []Expression // *TextoLiteral (literal) ou Expression (interp)
}

func (e *TextoInterpolado) expressionNode()      {}
func (e *TextoInterpolado) TokenLiteral() string { return e.Token.Literal }
func (e *TextoInterpolado) String() string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, p := range e.Parts {
		sb.WriteString(p.String())
	}
	sb.WriteByte('"')
	return sb.String()
}

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
	// Dot marca acesso por ponto (`obj.campo`) — acucar sintatico pra
	// `obj["campo"]`. Engines ignoram; o formatter reimprime com ponto.
	Dot bool
	// Safe marca navegacao segura (`obj?.campo`): se Left for nada, devolve
	// nada em vez de erro. So faz sentido com Dot=true.
	Safe bool
}

func (e *IndexExpression) expressionNode()      {}
func (e *IndexExpression) TokenLiteral() string { return e.Token.Literal }
func (e *IndexExpression) String() string {
	if e.Dot {
		if t, ok := e.Index.(*TextoLiteral); ok {
			op := "."
			if e.Safe {
				op = "?."
			}
			return "(" + e.Left.String() + op + t.Value + ")"
		}
	}
	return "(" + e.Left.String() + "[" + e.Index.String() + "])"
}

type ParAST struct {
	Chave Expression
	Valor Expression
}

type DicionarioLiteral struct {
	Token token.Token
	Pares []ParAST
}

func (e *DicionarioLiteral) expressionNode()      {}
func (e *DicionarioLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *DicionarioLiteral) String() string {
	partes := make([]string, len(e.Pares))
	for i, par := range e.Pares {
		partes[i] = par.Chave.String() + ": " + par.Valor.String()
	}
	return "{" + strings.Join(partes, ", ") + "}"
}

// FuncaoLiteral e uma gambiarra anonima usada como EXPRESSAO:
// `gambiarra(x) ... acabou_finalmente`. Mesmo shape da GambiarraStatement,
// so que sem nome — avalia pra um valor de funcao (closure).
type FuncaoLiteral struct {
	Token      token.Token
	Parameters []*Parametro
	Body       *BlockStatement
}

func (e *FuncaoLiteral) expressionNode()      {}
func (e *FuncaoLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *FuncaoLiteral) String() string {
	params := make([]string, len(e.Parameters))
	for i, p := range e.Parameters {
		params[i] = p.String()
	}
	return "gambiarra(" + strings.Join(params, ", ") + ") " + e.Body.String() + "acabou_finalmente"
}

// RangeExpression representa `inicio..fim` (inclusive). Devolve uma Lista
// [inicio, inicio+1, ..., fim] quando avaliada. So com inteiros.
type RangeExpression struct {
	Token token.Token
	Start Expression
	End   Expression
}

func (e *RangeExpression) expressionNode()      {}
func (e *RangeExpression) TokenLiteral() string { return e.Token.Literal }
func (e *RangeExpression) String() string       { return e.Start.String() + ".." + e.End.String() }

// BoraExpression e a prefix-expression `bora fn(args)`: dispara a chamada
// de `fn` numa goroutine e devolve imediatamente um Futuro. O Call interno
// e a expressao que seria avaliada de forma sincrona; o `bora` so envelopa
// pra rodar em paralelo.
type BoraExpression struct {
	Token token.Token
	Call  *CallExpression // chamada que sera despachada em paralelo
}

func (e *BoraExpression) expressionNode()      {}
func (e *BoraExpression) TokenLiteral() string { return e.Token.Literal }
func (e *BoraExpression) String() string {
	if e.Call != nil {
		return "bora " + e.Call.String()
	}
	return "bora"
}

// ---- FatiaExpression: xs[1:3], xs[:2], xs[2:] ----

// FatiaExpression representa uma fatia sintatica [inicio:fim] de uma lista ou
// texto. Inicio/Fim nil = omitido (xs[:2], xs[2:], xs[:]).
type FatiaExpression struct {
	Token   token.Token
	Left    Expression
	Inicio  Expression // nil = do comeco
	Fim     Expression // nil = ate o fim
}

func (e *FatiaExpression) expressionNode()      {}
func (e *FatiaExpression) TokenLiteral() string { return e.Token.Literal }
func (e *FatiaExpression) String() string {
	inicio := ""
	if e.Inicio != nil {
		inicio = e.Inicio.String()
	}
	fim := ""
	if e.Fim != nil {
		fim = e.Fim.String()
	}
	return e.Left.String() + "[" + inicio + ":" + fim + "]"
}

// ---- TernarioExpression: se_colar cond entao a se_nao_colar b ----

type TernarioExpression struct {
	Token       token.Token
	Cond        Expression
	SeVerdadeiro Expression
	SeFalso      Expression
}

func (e *TernarioExpression) expressionNode()      {}
func (e *TernarioExpression) TokenLiteral() string { return e.Token.Literal }
func (e *TernarioExpression) String() string {
	return "se_colar " + e.Cond.String() + " entao " + e.SeVerdadeiro.String() + " se_nao_colar " + e.SeFalso.String()
}

// ---- CoalesceExpression: x ?? padrao ----

type CoalesceExpression struct {
	Token    token.Token
	Left     Expression
	Right    Expression
}

func (e *CoalesceExpression) expressionNode()      {}
func (e *CoalesceExpression) TokenLiteral() string { return e.Token.Literal }
func (e *CoalesceExpression) String() string {
	return "(" + e.Left.String() + " ?? " + e.Right.String() + ")"
}

// ---- Parametro: nome + valor padrao + flag varargs ----

// Parametro representa um parametro de gambiarra com valor padrao opcional
// e/ou flag varargs (...resto).
type Parametro struct {
	Nome    *Identifier
	Padrao  Expression // nil = sem valor padrao
	Variadico bool     // true: ...resto (coleta args extras numa lista)
}

func (p *Parametro) String() string {
	if p.Variadico {
		return "..." + p.Nome.Value
	}
	if p.Padrao != nil {
		return p.Nome.Value + " = " + p.Padrao.String()
	}
	return p.Nome.Value
}
