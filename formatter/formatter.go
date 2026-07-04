package formatter

import (
	"strconv"
	"strings"

	"gambiarrascript/ast"
)

// Formata devolve a fonte formatada (indentada, 4 espacos por nivel) de um
// programa GambiarraScript. Comentarios sao descartados (o formatter reemite
// so o AST).
func Formata(prog *ast.Program) string {
	f := &formatter{indent: "    "}
	for _, s := range prog.Statements {
		f.emitStmt(s, 0)
	}
	return f.out.String()
}

type formatter struct {
	out    strings.Builder
	indent string
}

func (f *formatter) escreve(nivel int, s string) {
	f.out.WriteString(strings.Repeat(f.indent, nivel))
	f.out.WriteString(s)
	f.out.WriteString("\n")
}

func (f *formatter) emitStmt(s ast.Statement, nivel int) {
	switch n := s.(type) {
	case *ast.BotaStatement:
		alvo := ""
		if n.Name != nil {
			alvo = n.Name.Value
		} else if n.Indice != nil {
			alvo = f.emitExpr(n.Indice)
		}
		if n.OpComposto != "" {
			// atribuicao composta (`x += 1`): Value e o desugar `x + 1`;
			// reimprime a forma original usando so o lado direito do infix.
			if inf, ok := n.Value.(*ast.InfixExpression); ok {
				f.escreve(nivel, alvo+" "+n.OpComposto+" "+f.emitExpr(inf.Right))
				return
			}
		}
		f.escreve(nivel, "bota "+alvo+" = "+f.emitExpr(n.Value))
	case *ast.DesestruturaStatement:
		nomes := make([]string, len(n.Names))
		for i, nm := range n.Names {
			nomes[i] = nm.Value
		}
		abre, fecha := "[", "]"
		if n.DeDict {
			abre, fecha = "{", "}"
		}
		f.escreve(nivel, "bota "+abre+strings.Join(nomes, ", ")+fecha+" = "+f.emitExpr(n.Value))
	case *ast.MostraStatement:
		f.escreve(nivel, "mostra "+f.emitExpr(n.Value))
	case *ast.FuncionaStatement:
		f.escreve(nivel, "funciona "+f.emitExpr(n.Value))
	case *ast.VazaStatement:
		f.escreve(nivel, "vaza")
	case *ast.ContinuaStatement:
		f.escreve(nivel, "continua")
	case *ast.ExpressionStatement:
		if n.Expression != nil {
			f.escreve(nivel, f.emitExpr(n.Expression))
		}
	case *ast.ImportaStatement:
		f.escreve(nivel, "importa "+f.emitExpr(n.Path))
	case *ast.GambiarraStatement:
		params := make([]string, len(n.Parameters))
		for i, p := range n.Parameters {
			params[i] = p.Value
		}
		f.escreve(nivel, "gambiarra "+n.Name.Value+"("+strings.Join(params, ", ")+")")
		f.emitBlock(n.Body, nivel+1)
		f.escreve(nivel, "acabou_finalmente")
	case *ast.SeColarStatement:
		for i, c := range n.Conditions {
			if i == 0 {
				f.escreve(nivel, "se_colar "+f.emitExpr(c))
			} else {
				f.escreve(nivel, "se_nao_colar se_colar "+f.emitExpr(c))
			}
			f.emitBlock(n.Consequences[i], nivel+1)
		}
		if n.Alternative != nil {
			f.escreve(nivel, "se_nao_colar")
			f.emitBlock(n.Alternative, nivel+1)
		}
		f.escreve(nivel, "acabou_finalmente")
	case *ast.EnquantoStatement:
		f.escreve(nivel, "enquanto "+f.emitExpr(n.Condition))
		f.emitBlock(n.Body, nivel+1)
		f.escreve(nivel, "acabou_finalmente")
	case *ast.PraCadaNumStatement:
		f.escreve(nivel, "pra_cada "+n.Var.Value+" de "+f.emitExpr(n.Start)+" ate "+f.emitExpr(n.End))
		f.emitBlock(n.Body, nivel+1)
		f.escreve(nivel, "acabou_finalmente")
	case *ast.PraCadaListStatement:
		f.escreve(nivel, "pra_cada "+n.Var.Value+" em "+f.emitExpr(n.Iterable))
		f.emitBlock(n.Body, nivel+1)
		f.escreve(nivel, "acabou_finalmente")
	case *ast.EscolheStatement:
		f.escreve(nivel, "escolhe "+f.emitExpr(n.Subject))
		for _, braco := range n.Casos {
			vals := make([]string, len(braco.Values))
			for i, v := range braco.Values {
				vals[i] = f.emitExpr(v)
			}
			f.escreve(nivel, "caso "+strings.Join(vals, ", "))
			f.emitBlock(braco.Body, nivel+1)
		}
		if n.Default != nil {
			f.escreve(nivel, "se_nao_colar")
			f.emitBlock(n.Default, nivel+1)
		}
		f.escreve(nivel, "acabou_finalmente")
	case *ast.ArrumaStatement:
		f.escreve(nivel, "arruma")
		f.emitBlock(n.Try, nivel+1)
		if n.Catch != nil {
			if n.ErrName != nil {
				f.escreve(nivel, "quebrou "+n.ErrName.Value)
			} else {
				f.escreve(nivel, "quebrou")
			}
			f.emitBlock(n.Catch, nivel+1)
		}
		if n.Finally != nil {
			f.escreve(nivel, "finalmente")
			f.emitBlock(n.Finally, nivel+1)
		}
		f.escreve(nivel, "acabou_finalmente")
	default:
		if s != nil {
			f.escreve(nivel, s.String())
		}
	}
}

func (f *formatter) emitBlock(b *ast.BlockStatement, nivel int) {
	if b == nil {
		return
	}
	for _, s := range b.Statements {
		f.emitStmt(s, nivel)
	}
}

// inlineBlock formata um bloco em uma linha so (statements separados por
// espaco), pra usar dentro de expressoes (lambdas). Termina com espaco.
func (f *formatter) inlineBlock(b *ast.BlockStatement) string {
	if b == nil || len(b.Statements) == 0 {
		return ""
	}
	sub := &formatter{indent: ""}
	sub.emitBlock(b, 0)
	// indent="" => cada linha e so o statement; troca \n por espaco simples.
	// (Nao usar Fields/split generico: colapsaria espacos DENTRO de strings.)
	plano := strings.TrimRight(sub.out.String(), "\n")
	return strings.ReplaceAll(plano, "\n", " ") + " "
}

func (f *formatter) emitExpr(e ast.Expression) string {
	return f.emitExprPrec(e, precLowest)
}

// precedencias (espelhadas do parser) pra decidir quando envolver em ().
const (
	precLowest      = 1
	precOr          = 2
	precAnd         = 3
	precRange       = 4
	precBOr         = 5
	precBXor        = 6
	precBAnd        = 7
	precEquals      = 8
	precLessGreater = 9
	precShift       = 10
	precSum         = 11
	precProduct     = 12
	precPrefix      = 13
	precCall        = 14
	precIndex       = 15
)

func precOf(op string) int {
	switch op {
	case "ou":
		return precOr
	case "e":
		return precAnd
	case "|":
		return precBOr
	case "^":
		return precBXor
	case "&":
		return precBAnd
	case "==", "!=":
		return precEquals
	case "<", ">", "<=", ">=":
		return precLessGreater
	case "<<", ">>":
		return precShift
	case "+", "-":
		return precSum
	case "*", "/", "%":
		return precProduct
	}
	return precLowest
}

func (f *formatter) emitExprPrec(e ast.Expression, parent int) string {
	switch n := e.(type) {
	case *ast.Identifier:
		return n.Value
	case *ast.NumeroLiteral:
		return n.TokenLiteral()
	case *ast.TextoLiteral:
		return strconv.Quote(n.Value)
	case *ast.TextoInterpolado:
		// reconstruir a string com `${...}` — usa o Literal do token (o texto cru
		// que o lexer leu, com markers `${...}` preservados). Sicaro parse sempre
		// passa pelo lexer, que mantem `${...}` no Literal.
		return strconv.Quote(n.Token.Literal)
	case *ast.BooleanoLiteral:
		if n.Value {
			return "deu_bom"
		}
		return "deu_ruim"
	case *ast.NadaLiteral:
		return "nada"
	case *ast.ListaLiteral:
		parts := make([]string, len(n.Elements))
		for i, el := range n.Elements {
			parts[i] = f.emitExpr(el)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *ast.DicionarioLiteral:
		parts := make([]string, len(n.Pares))
		for i, p := range n.Pares {
			parts[i] = f.emitExpr(p.Chave) + ": " + f.emitExpr(p.Valor)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *ast.PrefixExpression:
		return n.Operator + f.emitExprPrec(n.Right, precPrefix)
	case *ast.InfixExpression:
		my := precOf(n.Operator)
		s := f.emitExprPrec(n.Left, my) + " " + n.Operator + " " + f.emitExprPrec(n.Right, my+1)
		if my < parent {
			return "(" + s + ")"
		}
		return s
	case *ast.CallExpression:
		args := make([]string, len(n.Arguments))
		for i, a := range n.Arguments {
			args[i] = f.emitExpr(a)
		}
		return f.emitExprPrec(n.Function, precCall) + "(" + strings.Join(args, ", ") + ")"
	case *ast.RangeExpression:
		s := f.emitExprPrec(n.Start, precRange) + ".." + f.emitExprPrec(n.End, precRange+1)
		if precRange < parent {
			return "(" + s + ")"
		}
		return s
	case *ast.IndexExpression:
		if n.Dot {
			if t, ok := n.Index.(*ast.TextoLiteral); ok {
				return f.emitExprPrec(n.Left, precIndex) + "." + t.Value
			}
		}
		return f.emitExprPrec(n.Left, precIndex) + "[" + f.emitExpr(n.Index) + "]"
	case *ast.FuncaoLiteral:
		// lambda anonima em posicao de expressao: forma inline (newline nao
		// e significativo na linguagem, entao reparseia igual).
		params := make([]string, len(n.Parameters))
		for i, p := range n.Parameters {
			params[i] = p.Value
		}
		return "gambiarra(" + strings.Join(params, ", ") + ") " +
			f.inlineBlock(n.Body) + "acabou_finalmente"
	case *ast.BoraExpression:
		// `bora` precede uma chamada; formata como prefix.
		if n.Call != nil {
			return "bora " + f.emitExpr(n.Call)
		}
		return "bora"
	}
	return ""
}
