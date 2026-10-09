package ast

import "fmt"

// ErroCrava aponta um lugar que tenta mudar (ou cravar de novo) um nome
// cravado. Linha/coluna sao do nome no codigo-fonte.
type ErroCrava struct {
	Linha  int
	Coluna int
	Nome   string
	Msg    string
}

// MsgCravada e a mensagem de quem tenta reatribuir um nome cravado. Fica aqui
// pros dois engines e o linter falarem a mesma coisa.
func MsgCravada(nome string) string {
	return fmt.Sprintf("`%s` foi cravada, nao da pra mudar", nome)
}

// MsgCravadaDeNovo e a mensagem de quem crava duas vezes o mesmo nome.
func MsgCravadaDeNovo(nome string) string {
	return fmt.Sprintf("`%s` ja foi cravada, nao da pra cravar de novo", nome)
}

// ChecaCravadas procura, sem rodar nada, quem tenta mudar um nome cravado.
//
// O escopo e o da funcao (estilo Python, igual o `bota`): blocos (se_colar,
// enquanto, pra_cada, arruma, escolhe) dividem o escopo de quem os contem, e
// cada gambiarra (nomeada ou lambda) abre um escopo novo — la dentro um
// `bota NOME` cria um local que sombreia o de fora, entao nao e erro. A ordem
// vale pelo texto, nao pela execucao: um `crava` dentro de laco nao reclama na
// segunda volta, e um `bota` depois de um `crava` reclama mesmo num ramo que
// nunca roda. Assim tree-walker e VM decidem igualzinho.
//
// `globais` traz o que ja foi cravado no topo (entradas anteriores do REPL,
// modulos importados) e sai com o que este programa cravou no topo. Pode ser nil.
func ChecaCravadas(prog *Program, globais map[string]bool) []ErroCrava {
	if globais == nil {
		globais = map[string]bool{}
	}
	ch := &checaCrava{escopos: []map[string]bool{globais}}
	for _, s := range prog.Statements {
		ch.stmt(s)
	}
	return ch.erros
}

type checaCrava struct {
	escopos []map[string]bool // um por funcao; o [0] e o topo
	erros   []ErroCrava
}

func (ch *checaCrava) atual() map[string]bool { return ch.escopos[len(ch.escopos)-1] }

// atribui registra erro se `id` ja foi cravado no escopo atual.
func (ch *checaCrava) atribui(id *Identifier) {
	if id != nil && ch.atual()[id.Value] {
		ch.erros = append(ch.erros, ErroCrava{Linha: id.Token.Line, Coluna: id.Token.Coluna, Nome: id.Value, Msg: MsgCravada(id.Value)})
	}
}

func (ch *checaCrava) funcao(params []*Parametro, corpo *BlockStatement) {
	ch.escopos = append(ch.escopos, map[string]bool{})
	for _, p := range params {
		if p != nil && p.Padrao != nil {
			ch.expr(p.Padrao)
		}
	}
	ch.bloco(corpo)
	ch.escopos = ch.escopos[:len(ch.escopos)-1]
}

func (ch *checaCrava) bloco(b *BlockStatement) {
	if b == nil {
		return
	}
	for _, s := range b.Statements {
		ch.stmt(s)
	}
}

func (ch *checaCrava) stmt(s Statement) {
	switch n := s.(type) {
	case *CravaStatement:
		ch.expr(n.Value)
		if ch.atual()[n.Name.Value] {
			ch.erros = append(ch.erros, ErroCrava{Linha: n.Name.Token.Line, Coluna: n.Name.Token.Coluna, Nome: n.Name.Value, Msg: MsgCravadaDeNovo(n.Name.Value)})
		}
		ch.atual()[n.Name.Value] = true
	case *BotaStatement:
		ch.expr(n.Value)
		if n.Indice != nil {
			// XS[0] = 2 muda o conteudo, nao o nome: liberado.
			ch.expr(n.Indice)
		}
		ch.atribui(n.Name)
	case *DesestruturaStatement:
		ch.expr(n.Value)
		for _, nome := range n.Names {
			ch.atribui(nome)
		}
	case *MostraStatement:
		ch.expr(n.Value)
	case *FuncionaStatement:
		ch.expr(n.Value)
	case *RendeStatement:
		ch.expr(n.Value)
	case *ExpressionStatement:
		ch.expr(n.Expression)
	case *GambiarraStatement:
		ch.atribui(n.Name)
		ch.funcao(n.Parameters, n.Body)
	case *SeColarStatement:
		for i, c := range n.Conditions {
			ch.expr(c)
			if i < len(n.Consequences) {
				ch.bloco(n.Consequences[i])
			}
		}
		ch.bloco(n.Alternative)
	case *EnquantoStatement:
		ch.expr(n.Condition)
		ch.bloco(n.Body)
	case *PraCadaNumStatement:
		ch.expr(n.Start)
		ch.expr(n.End)
		ch.atribui(n.Var)
		ch.bloco(n.Body)
	case *PraCadaListStatement:
		ch.expr(n.Iterable)
		for _, v := range n.Vars {
			ch.atribui(v)
		}
		ch.bloco(n.Body)
	case *EscolheStatement:
		ch.expr(n.Subject)
		for _, braco := range n.Casos {
			// nome solto dentro de padrao amarra: conta como atribuicao
			for _, v := range braco.Values {
				PercorrePadrao(v, ch.atribui, ch.expr)
			}
			ch.expr(braco.Guarda)
			ch.bloco(braco.Body)
		}
		ch.bloco(n.Default)
	case *ArrumaStatement:
		ch.bloco(n.Try)
		for _, q := range n.Quebrous {
			ch.atribui(q.Nome)
			ch.expr(q.Filtro)
			ch.bloco(q.Corpo)
		}
		ch.bloco(n.Finally)
	case *ImportaStatement:
		ch.atribui(n.Alias)
	case *BlockStatement:
		ch.bloco(n)
	// POO: treta/combinado ligam o nome (igual gambiarra); metodo abre escopo
	// de funcao (receiver + params) e nao liga nome nenhum
	case *TretaDecl:
		for _, c := range n.Campos {
			ch.expr(c.Padrao)
			ch.expr(c.Embutida)
		}
		ch.atribui(n.Nome)
	case *CombinadoDecl:
		ch.atribui(n.Nome)
	case *CardapioDecl:
		ch.atribui(n.Nome)
	case *MetodoDecl:
		ch.funcao(n.ParametrosComReceptor(), n.Body)
	}
}

// expr so desce atras de lambdas: e la que mora statement dentro de expressao.
func (ch *checaCrava) expr(e Expression) {
	switch n := e.(type) {
	case *FuncaoLiteral:
		ch.funcao(n.Parameters, n.Body)
	case *PrefixExpression:
		ch.expr(n.Right)
	case *InfixExpression:
		ch.expr(n.Left)
		ch.expr(n.Right)
	case *CallExpression:
		ch.expr(n.Function)
		for _, a := range n.Arguments {
			ch.expr(a)
		}
	case *BoraExpression:
		if n.Call != nil {
			ch.expr(n.Call)
		}
	case *IndexExpression:
		ch.expr(n.Left)
		ch.expr(n.Index)
	case *FatiaExpression:
		ch.expr(n.Left)
		ch.expr(n.Inicio)
		ch.expr(n.Fim)
	case *ListaLiteral:
		for _, el := range n.Elements {
			ch.expr(el)
		}
	case *DicionarioLiteral:
		for _, p := range n.Pares {
			ch.expr(p.Chave)
			ch.expr(p.Valor)
		}
	case *RangeExpression:
		ch.expr(n.Start)
		ch.expr(n.End)
	case *TernarioExpression:
		ch.expr(n.Cond)
		ch.expr(n.SeVerdadeiro)
		ch.expr(n.SeFalso)
	case *CoalesceExpression:
		ch.expr(n.Left)
		ch.expr(n.Right)
	case *TextoInterpolado:
		for _, p := range n.Parts {
			ch.expr(p)
		}
	case *TretaLiteral:
		ch.expr(n.Tipo)
		for _, v := range n.Valores {
			ch.expr(v)
		}
	}
}
