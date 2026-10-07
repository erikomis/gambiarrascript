package formatter

import (
	"strconv"
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/parser"
)

// Formata devolve a fonte formatada (indentada, 4 espacos por nivel) so a
// partir do AST: sem a fonte nao tem comentario nem linha em branco. Pra
// formatar arquivo de verdade usa o FormataFonte (comentarios.go).
func Formata(prog *ast.Program) string {
	f := novoFormatter()
	f.emitStmts(prog.Statements, 0, ponto{})
	return f.out.String()
}

type formatter struct {
	out    *strings.Builder
	indent string
	nivel  int // nivel da linha sendo montada (lambda/lista em bloco usam)

	// so no FormataFonte; zerados, o formatter so reimprime o AST
	pos    *parser.Posicoes
	coms   []comentario
	prox   int             // primeiro comentario talvez nao usado
	branco map[[2]int]bool // tokens com linha em branco antes
	piso   ponto           // dentro de lambda/lista: so solta comentario depois disso
}

func novoFormatter() *formatter {
	return &formatter{out: &strings.Builder{}, indent: "    "}
}

func (f *formatter) escreve(nivel int, s string) {
	f.out.WriteString(strings.Repeat(f.indent, nivel))
	f.out.WriteString(s)
	f.out.WriteString("\n")
}

// emitStmts escreve os statements de um bloco e, no fim, os comentarios que
// sobraram antes do terminador (`limite`), no nivel do bloco.
func (f *formatter) emitStmts(stmts []ast.Statement, nivel int, limite ponto) {
	velho := f.nivel
	primeiro := true
	for _, s := range stmts {
		f.soltaComentarios(f.inicio(s), nivel, &primeiro)
		f.emitStmt(s, nivel, &primeiro)
	}
	f.soltaComentarios(limite, nivel, &primeiro)
	f.nivel = velho
}

func (f *formatter) emitBlock(b *ast.BlockStatement, nivel int) {
	if b == nil {
		return
	}
	f.emitStmts(b.Statements, nivel, f.fimBloco(b))
}

func (f *formatter) emitStmt(s ast.Statement, nivel int, primeiro *bool) {
	f.nivel = nivel
	// simples: statement de uma linha (o rabo e o comentario apos o ultimo token)
	simples := func(txt string) { f.abre(s, nivel, primeiro, txt, f.fim(s)) }
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
				simples(alvo + " " + n.OpComposto + " " + f.emitExpr(inf.Right))
				return
			}
		}
		simples("bota " + alvo + " = " + f.emitExpr(n.Value))
	case *ast.CravaStatement:
		simples("crava " + n.Name.Value + " = " + f.emitExpr(n.Value))
	case *ast.DesestruturaStatement:
		nomes := make([]string, len(n.Names))
		for i, nm := range n.Names {
			nomes[i] = nm.Value
		}
		abre, fecha := "[", "]"
		if n.DeDict {
			abre, fecha = "{", "}"
		}
		simples("bota " + abre + strings.Join(nomes, ", ") + fecha + " = " + f.emitExpr(n.Value))
	case *ast.MostraStatement:
		simples("mostra " + f.emitExpr(n.Value))
	case *ast.FuncionaStatement:
		simples("funciona " + f.emitExpr(n.Value))
	case *ast.VazaStatement:
		simples("vaza")
	case *ast.ContinuaStatement:
		simples("continua")
	case *ast.ExpressionStatement:
		if n.Expression != nil {
			simples(f.emitExpr(n.Expression))
		}
	case *ast.ImportaStatement:
		if n.Alias != nil {
			simples("importa " + f.emitExpr(n.Path) + " como " + n.Alias.Value)
		} else {
			simples("importa " + f.emitExpr(n.Path))
		}
	case *ast.GambiarraStatement:
		params := make([]string, len(n.Parameters))
		for i, p := range n.Parameters {
			params[i] = p.String()
		}
		f.abre(s, nivel, primeiro, "gambiarra "+n.Name.Value+"("+strings.Join(params, ", ")+")", f.cabeca(n.Body))
		f.emitBlock(n.Body, nivel+1)
		f.linha(nivel, "acabou_finalmente", f.fim(s))
	case *ast.SeColarStatement:
		for i, c := range n.Conditions {
			if i == 0 {
				f.abre(s, nivel, primeiro, "se_colar "+f.emitExpr(c), f.cabeca(n.Consequences[i]))
			} else {
				f.linha(nivel, "se_nao_colar se_colar "+f.emitExpr(c), f.cabeca(n.Consequences[i]))
			}
			f.emitBlock(n.Consequences[i], nivel+1)
		}
		if n.Alternative != nil {
			f.linha(nivel, "se_nao_colar", f.cabeca(n.Alternative))
			f.emitBlock(n.Alternative, nivel+1)
		}
		f.linha(nivel, "acabou_finalmente", f.fim(s))
	case *ast.EnquantoStatement:
		f.abre(s, nivel, primeiro, "enquanto "+f.emitExpr(n.Condition), f.cabeca(n.Body))
		f.emitBlock(n.Body, nivel+1)
		f.linha(nivel, "acabou_finalmente", f.fim(s))
	case *ast.PraCadaNumStatement:
		f.abre(s, nivel, primeiro, "pra_cada "+n.Var.Value+" de "+f.emitExpr(n.Start)+" ate "+f.emitExpr(n.End), f.cabeca(n.Body))
		f.emitBlock(n.Body, nivel+1)
		f.linha(nivel, "acabou_finalmente", f.fim(s))
	case *ast.PraCadaListStatement:
		nomes := make([]string, len(n.Vars))
		for i, v := range n.Vars {
			nomes[i] = v.Value
		}
		f.abre(s, nivel, primeiro, "pra_cada "+strings.Join(nomes, ", ")+" em "+f.emitExpr(n.Iterable), f.cabeca(n.Body))
		f.emitBlock(n.Body, nivel+1)
		f.linha(nivel, "acabou_finalmente", f.fim(s))
	case *ast.EscolheStatement:
		f.abre(s, nivel, primeiro, "escolhe "+f.emitExpr(n.Subject), f.cabeca(n))
		for i, braco := range n.Casos {
			if i == 0 {
				// comentario entre o `escolhe x` e o 1o caso fica no nivel do caso
				p := true
				f.soltaComentarios(f.caso(braco.Body), nivel, &p)
			}
			vals := make([]string, len(braco.Values))
			for i, v := range braco.Values {
				vals[i] = f.emitExpr(v)
			}
			f.linha(nivel, "caso "+strings.Join(vals, ", "), f.cabeca(braco.Body))
			f.emitBlock(braco.Body, nivel+1)
		}
		if n.Default != nil {
			f.linha(nivel, "se_nao_colar", f.cabeca(n.Default))
			f.emitBlock(n.Default, nivel+1)
		}
		f.linha(nivel, "acabou_finalmente", f.fim(s))
	case *ast.ArrumaStatement:
		f.abre(s, nivel, primeiro, "arruma", f.cabeca(n.Try))
		f.emitBlock(n.Try, nivel+1)
		if n.Catch != nil {
			if n.ErrName != nil {
				f.linha(nivel, "quebrou "+n.ErrName.Value, f.cabeca(n.Catch))
			} else {
				f.linha(nivel, "quebrou", f.cabeca(n.Catch))
			}
			f.emitBlock(n.Catch, nivel+1)
		}
		if n.Finally != nil {
			f.linha(nivel, "finalmente", f.cabeca(n.Finally))
			f.emitBlock(n.Finally, nivel+1)
		}
		f.linha(nivel, "acabou_finalmente", f.fim(s))
	// POO (Tier 8): treta/combinado sao um campo/assinatura por linha
	case *ast.TretaDecl:
		f.abre(s, nivel, primeiro, "treta "+n.Nome.Value, f.cabeca(n))
		linhas := make([]ast.Statement, len(n.Campos))
		for i, c := range n.Campos {
			linhas[i] = c
		}
		f.emitStmts(linhas, nivel+1, f.fim(s))
		f.linha(nivel, "acabou_finalmente", f.fim(s))
	case *ast.CampoTreta:
		switch {
		case n.Embutida != nil:
			simples(f.emitExpr(n.Embutida))
		case n.Padrao != nil:
			simples(n.Nome.Value + " = " + f.emitExpr(n.Padrao))
		default:
			simples(n.Nome.Value)
		}
	case *ast.CombinadoDecl:
		f.abre(s, nivel, primeiro, "combinado "+n.Nome.Value, f.cabeca(n))
		linhas := make([]ast.Statement, len(n.Metodos))
		for i, m := range n.Metodos {
			linhas[i] = m
		}
		f.emitStmts(linhas, nivel+1, f.fim(s))
		f.linha(nivel, "acabou_finalmente", f.fim(s))
	case *ast.AssinaturaMetodo:
		if n.Embutido != nil {
			simples(f.emitExpr(n.Embutido))
			return
		}
		simples(n.String())
	case *ast.MetodoDecl:
		params := make([]string, len(n.Parameters))
		for i, p := range n.Parameters {
			params[i] = p.String()
		}
		cab := "gambiarra (" + n.Receptor.Value + " " + n.Tipo.Value + ") " + n.Nome.Value + "(" + strings.Join(params, ", ") + ")"
		f.abre(s, nivel, primeiro, cab, f.cabeca(n.Body))
		f.emitBlock(n.Body, nivel+1)
		f.linha(nivel, "acabou_finalmente", f.fim(s))
	default:
		if s != nil {
			simples(s.String())
		}
	}
}

// inlineBlock formata um bloco em uma linha so (statements separados por
// espaco), pra usar dentro de expressoes (lambdas). Termina com espaco.
func (f *formatter) inlineBlock(b *ast.BlockStatement) string {
	if b == nil || len(b.Statements) == 0 {
		return ""
	}
	sub := &formatter{out: &strings.Builder{}} // sem posicoes: tudo inline
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
	precPower       = 14
	precCall        = 15
	precIndex       = 16
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
	case "**":
		return precPower
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
		// `${` literal veio de um `\${` escapado: sem a barra voltava a interpolar
		return strings.ReplaceAll(strconv.Quote(n.Value), "${", `\${`)
	case *ast.TextoInterpolado:
		// reconstruir a string com `${...}` — usa o Literal do token (o texto cru
		// que o lexer leu, com markers `${...}` preservados). Sicaro parse sempre
		// passa pelo lexer, que mantem `${...}` no Literal.
		return citaInterpolado(n.Token.Literal)
	case *ast.BooleanoLiteral:
		if n.Value {
			return "deu_bom"
		}
		return "deu_ruim"
	case *ast.NadaLiteral:
		return "nada"
	case *ast.ListaLiteral:
		if txt, ok := f.listaEmLinhas(n); ok {
			return txt
		}
		parts := make([]string, len(n.Elements))
		for i, el := range n.Elements {
			parts[i] = f.emitExpr(el)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *ast.DicionarioLiteral:
		if txt, ok := f.dictEmLinhas(n); ok {
			return txt
		}
		parts := make([]string, len(n.Pares))
		for i, p := range n.Pares {
			parts[i] = f.emitExpr(p.Chave) + ": " + f.emitExpr(p.Valor)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *ast.PrefixExpression:
		op := n.Operator
		if op == "nao" {
			op += " "
		}
		s := op + f.emitExprPrec(n.Right, precPrefix)
		// (-2) ** 2 e (-x)[0]: sem o parentese o menos engoliria o resto
		if precPrefix < parent {
			return "(" + s + ")"
		}
		return s
	case *ast.InfixExpression:
		my := precOf(n.Operator)
		esq, dir := my, my+1
		if n.Operator == "**" {
			// associa pela direita, e o lado direito aceita menos solto (2 ** -1)
			esq, dir = my+1, precPrefix
		}
		s := f.emitExprPrec(n.Left, esq) + " " + n.Operator + " " + f.emitExprPrec(n.Right, dir)
		if my < parent {
			return "(" + s + ")"
		}
		return s
	case *ast.CallExpression:
		args := make([]string, len(n.Arguments))
		for i, a := range n.Arguments {
			args[i] = f.emitExpr(a)
			if n.Espalha(i) {
				args[i] = "..." + args[i]
			}
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
				op := "."
				if n.Safe {
					op = "?."
				}
				return f.emitExprPrec(n.Left, precIndex) + op + t.Value
			}
		}
		return f.emitExprPrec(n.Left, precIndex) + "[" + f.emitExpr(n.Index) + "]"
	case *ast.FuncaoLiteral:
		// lambda anonima em posicao de expressao: escrita numa linha so fica
		// inline (newline nao e significativo, reparseia igual); escrita em
		// varias linhas vira bloco indentado (e guarda os comentarios).
		params := make([]string, len(n.Parameters))
		for i, p := range n.Parameters {
			params[i] = p.String()
		}
		cab := "gambiarra(" + strings.Join(params, ", ") + ")"
		if txt, ok := f.lambdaEmBloco(n, cab); ok {
			return txt
		}
		return cab + " " + f.inlineBlock(n.Body) + "acabou_finalmente"
	case *ast.FatiaExpression:
		inicio := ""
		if n.Inicio != nil {
			inicio = f.emitExpr(n.Inicio)
		}
		fim := ""
		if n.Fim != nil {
			fim = f.emitExpr(n.Fim)
		}
		return f.emitExprPrec(n.Left, precIndex) + "[" + inicio + ":" + fim + "]"
	case *ast.TernarioExpression:
		return "se_colar " + f.emitExpr(n.Cond) + " entao " + f.emitExpr(n.SeVerdadeiro) +
			" se_nao_colar " + f.emitExpr(n.SeFalso)
	case *ast.CoalesceExpression:
		my := precOr // ?? tem mesma prec de ou
		s := f.emitExprPrec(n.Left, my) + " ?? " + f.emitExprPrec(n.Right, my+1)
		if my < parent {
			return "(" + s + ")"
		}
		return s
	case *ast.BoraExpression:
		// `bora` precede uma chamada; formata como prefix.
		if n.Call != nil {
			return "bora " + f.emitExpr(n.Call)
		}
		return "bora"
	case *ast.TretaLiteral:
		// o `{` cola no tipo (sem espaco): e assim que o parser reconhece
		tipo := f.emitExprPrec(n.Tipo, precIndex)
		if txt, ok := f.tretaEmLinhas(n, tipo); ok {
			return txt
		}
		parts := make([]string, len(n.Valores))
		for i, v := range n.Valores {
			parts[i] = f.emitExpr(v)
			if n.Nomes != nil {
				parts[i] = n.Nomes[i].Value + ": " + parts[i]
			}
		}
		return tipo + "{" + strings.Join(parts, ", ") + "}"
	}
	return ""
}

// citaInterpolado poe aspas no texto cru com `${...}` escapando SO o que esta
// fora das marcas: o lexer copia o miolo do ${} cru, entao escapar o `"` la
// dentro quebrava `${junta(xs, " ")}`. `\${...}` (escapado) sai igual veio.
func citaInterpolado(lit string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	fora := 0 // inicio do trecho fora de ${}
	for i := 0; i < len(lit); i++ {
		ini := i
		if lit[i] == '\\' && strings.HasPrefix(lit[i+1:], "${") {
			i++
		}
		if !strings.HasPrefix(lit[i:], "${") {
			i = ini
			continue
		}
		q := strconv.Quote(lit[fora:ini])
		sb.WriteString(q[1 : len(q)-1])
		// copia cru ate a chave que fecha (ou o fim, se nao fechou)
		depth, j := 0, i+1
		for ; j < len(lit); j++ {
			if lit[j] == '{' {
				depth++
			} else if lit[j] == '}' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if j >= len(lit) {
			j = len(lit) - 1
		}
		sb.WriteString(lit[ini : j+1])
		i = j
		fora = j + 1
	}
	q := strconv.Quote(lit[fora:])
	sb.WriteString(q[1 : len(q)-1])
	sb.WriteByte('"')
	return sb.String()
}
