package ast

import "sort"

// Statements executaveis: a fonte unica de "o que conta como linha" pra
// cobertura (gs testa --cobertura) e pro gancho de linha dos dois engines.
//
// Um statement executavel e todo statement do usuario que mora numa lista de
// statements (topo do programa ou corpo de bloco) — em qualquer profundidade:
// corpo de se_colar/enquanto/pra_cada/escolhe/arruma, de gambiarra, de metodo
// e de lambda (inclusive lambda dentro de expressao). Declaracoes (gambiarra,
// treta, combinado, metodo, importa) contam: elas rodam (ligam o nome) quando
// a execucao passa por elas. Ficam de fora o BlockStatement em si (so os
// filhos contam), as linhas de campo da treta e de assinatura do combinado
// (nunca rodam soltas) e statement sintetico dos engines (desugar), que nao
// esta na arvore do parser.
//
// Os dois engines disparam o gancho exatamente pros nodes desta lista (o
// tree-walker no laco do evalBlock/evalProgram, a VM num OpLinha que o
// compilador so emite com instrumentacao ligada), entao o conjunto de linhas
// executaveis e as contagens batem entre eles por construcao.

// StatementsExecutaveis devolve, na ordem do texto, os statements executaveis
// do programa (os que tem linha > 0).
func StatementsExecutaveis(prog *Program) []Statement {
	if prog == nil {
		return nil
	}
	w := &coletaExec{}
	w.lista(prog.Statements)
	return w.out
}

// LinhasExecutaveis devolve as linhas (ordenadas, sem repeticao) que comecam
// algum statement executavel.
func LinhasExecutaveis(prog *Program) []int {
	visto := map[int]bool{}
	var linhas []int
	for _, s := range StatementsExecutaveis(prog) {
		l := LinhaDoStatement(s)
		if !visto[l] {
			visto[l] = true
			linhas = append(linhas, l)
		}
	}
	sort.Ints(linhas)
	return linhas
}

// LinhaDoStatement devolve a linha onde o statement comeca (0 se nao sabe).
func LinhaDoStatement(s Statement) int {
	switch n := s.(type) {
	case *BotaStatement:
		return n.Token.Line
	case *CravaStatement:
		return n.Token.Line
	case *EscolheStatement:
		return n.Token.Line
	case *DesestruturaStatement:
		return n.Token.Line
	case *MostraStatement:
		return n.Token.Line
	case *FuncionaStatement:
		return n.Token.Line
	case *VazaStatement:
		return n.Token.Line
	case *ContinuaStatement:
		return n.Token.Line
	case *ExpressionStatement:
		return n.Token.Line
	case *SeColarStatement:
		return n.Token.Line
	case *EnquantoStatement:
		return n.Token.Line
	case *PraCadaNumStatement:
		return n.Token.Line
	case *PraCadaListStatement:
		return n.Token.Line
	case *GambiarraStatement:
		return n.Token.Line
	case *ArrumaStatement:
		return n.Token.Line
	case *ImportaStatement:
		return n.Token.Line
	case *TretaDecl:
		return n.Token.Line
	case *CombinadoDecl:
		return n.Token.Line
	case *MetodoDecl:
		return n.Token.Line
	}
	return 0
}

type coletaExec struct {
	out []Statement
}

func (w *coletaExec) lista(stmts []Statement) {
	for _, s := range stmts {
		if b, ok := s.(*BlockStatement); ok {
			w.bloco(b)
			continue
		}
		if LinhaDoStatement(s) > 0 {
			w.out = append(w.out, s)
		}
		w.filhos(s)
	}
}

func (w *coletaExec) bloco(b *BlockStatement) {
	if b != nil {
		w.lista(b.Statements)
	}
}

func (w *coletaExec) params(ps []*Parametro) {
	for _, p := range ps {
		if p != nil {
			w.expr(p.Padrao)
		}
	}
}

// filhos desce nos blocos e nas expressoes do statement (lambda mora em
// expressao).
func (w *coletaExec) filhos(s Statement) {
	switch n := s.(type) {
	case *BotaStatement:
		if n.Indice != nil {
			w.expr(n.Indice)
		}
		w.expr(n.Value)
	case *CravaStatement:
		w.expr(n.Value)
	case *DesestruturaStatement:
		w.expr(n.Value)
	case *MostraStatement:
		w.expr(n.Value)
	case *FuncionaStatement:
		w.expr(n.Value)
	case *ExpressionStatement:
		w.expr(n.Expression)
	case *GambiarraStatement:
		w.params(n.Parameters)
		w.bloco(n.Body)
	case *MetodoDecl:
		w.params(n.Parameters)
		w.bloco(n.Body)
	case *SeColarStatement:
		for i, c := range n.Conditions {
			w.expr(c)
			if i < len(n.Consequences) {
				w.bloco(n.Consequences[i])
			}
		}
		w.bloco(n.Alternative)
	case *EnquantoStatement:
		w.expr(n.Condition)
		w.bloco(n.Body)
	case *PraCadaNumStatement:
		w.expr(n.Start)
		w.expr(n.End)
		w.bloco(n.Body)
	case *PraCadaListStatement:
		w.expr(n.Iterable)
		w.bloco(n.Body)
	case *EscolheStatement:
		w.expr(n.Subject)
		for _, braco := range n.Casos {
			for _, v := range braco.Values {
				w.expr(v)
			}
			w.bloco(braco.Body)
		}
		w.bloco(n.Default)
	case *ArrumaStatement:
		w.bloco(n.Try)
		for _, q := range n.Quebrous {
			if q != nil {
				w.expr(q.Filtro)
				w.bloco(q.Corpo)
			}
		}
		w.bloco(n.Finally)
	case *ImportaStatement:
		w.expr(n.Path)
	case *TretaDecl:
		for _, c := range n.Campos {
			if c != nil {
				w.expr(c.Padrao)
				w.expr(c.Embutida)
			}
		}
	case *CombinadoDecl:
		for _, m := range n.Metodos {
			if m != nil {
				w.params(m.Parametros)
				w.expr(m.Embutido)
			}
		}
	}
}

// expr so procura lambda (FuncaoLiteral): e o unico jeito de ter statement
// dentro de expressao.
func (w *coletaExec) expr(e Expression) {
	switch n := e.(type) {
	case *FuncaoLiteral:
		w.params(n.Parameters)
		w.bloco(n.Body)
	case *PrefixExpression:
		w.expr(n.Right)
	case *InfixExpression:
		w.expr(n.Left)
		w.expr(n.Right)
	case *CallExpression:
		w.expr(n.Function)
		for _, a := range n.Arguments {
			w.expr(a)
		}
	case *BoraExpression:
		if n.Call != nil {
			w.expr(n.Call)
		}
	case *IndexExpression:
		w.expr(n.Left)
		w.expr(n.Index)
	case *FatiaExpression:
		w.expr(n.Left)
		w.expr(n.Inicio)
		w.expr(n.Fim)
	case *ListaLiteral:
		for _, el := range n.Elements {
			w.expr(el)
		}
	case *DicionarioLiteral:
		for _, p := range n.Pares {
			w.expr(p.Chave)
			w.expr(p.Valor)
		}
	case *RangeExpression:
		w.expr(n.Start)
		w.expr(n.End)
	case *TernarioExpression:
		w.expr(n.Cond)
		w.expr(n.SeVerdadeiro)
		w.expr(n.SeFalso)
	case *CoalesceExpression:
		w.expr(n.Left)
		w.expr(n.Right)
	case *TextoInterpolado:
		for _, p := range n.Parts {
			w.expr(p)
		}
	case *TretaLiteral:
		w.expr(n.Tipo)
		for _, v := range n.Valores {
			w.expr(v)
		}
	}
}
