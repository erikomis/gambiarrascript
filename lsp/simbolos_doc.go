package lsp

import (
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/token"
)

// ---- textDocument/documentSymbol ----
//
// Outline do arquivo: gambiarras (com as aninhadas como filhas), variaveis de
// topo (`bota`, desestruturacao), constantes (`crava`) e alias de importa.
// Blocos de topo (se_colar, enquanto...) nao abrem escopo, entao um `bota`
// la dentro tambem e variavel de topo. Cada nome aparece uma vez (a primeira
// ligacao); reatribuicao nao vira simbolo novo.

const (
	kindModulo    = 2
	kindFuncao    = 12
	kindVariavel  = 13
	kindConstante = 14
)

type SimboloDoc struct {
	Name           string       `json:"name"`
	Detail         string       `json:"detail,omitempty"`
	Kind           int          `json:"kind"`
	Range          Faixa        `json:"range"`
	SelectionRange Faixa        `json:"selectionRange"`
	Children       []SimboloDoc `json:"children,omitempty"`
}

type montaSimbolos struct {
	a      *analise
	tokens []token.Token
	idx    map[[2]int]int // posicao -> indice no slice de tokens
}

func (s *Servidor) simbolosDoDocumento(uri string) []SimboloDoc {
	if _, ok := s.docs[uri]; !ok {
		return []SimboloDoc{}
	}
	_, a := s.analisarDoc(uri)
	return simbolosDe(a)
}

func simbolosDe(a *analise) []SimboloDoc {
	m := &montaSimbolos{a: a, idx: map[[2]int]int{}}
	l := lexer.New(a.texto)
	for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
		m.idx[[2]int{tok.Line, tok.Coluna}] = len(m.tokens)
		m.tokens = append(m.tokens, tok)
	}
	out := []SimboloDoc{}
	vistos := map[string]bool{}
	for _, st := range a.prog.Statements {
		m.doEscopo(st, &out, vistos, true)
	}
	return out
}

// doEscopo junta os simbolos do statement no nivel atual. topo=true no
// escopo do arquivo (variaveis entram); dentro de gambiarra so entram as
// gambiarras aninhadas.
func (m *montaSimbolos) doEscopo(st ast.Statement, out *[]SimboloDoc, vistos map[string]bool, topo bool) {
	if ehNil(st) {
		return
	}
	novaVar := func(id *ast.Identifier, tokStmt token.Token, kind int, detalhe string) {
		if !topo || id == nil || vistos[id.Value] {
			return
		}
		vistos[id.Value] = true
		*out = append(*out, SimboloDoc{
			Name:           id.Value,
			Detail:         detalhe,
			Kind:           kind,
			Range:          Faixa{Start: m.a.linhas.posLSP(tokStmt.Line, tokStmt.Coluna), End: m.a.linhas.fimDaLinha(id.Token.Line)},
			SelectionRange: m.a.linhas.faixa(id.Token.Line, id.Token.Coluna, len([]rune(id.Value))),
		})
	}
	switch n := st.(type) {
	case *ast.GambiarraStatement:
		if n.Name == nil {
			return
		}
		if topo {
			if vistos[n.Name.Value] {
				return
			}
			vistos[n.Name.Value] = true
		}
		*out = append(*out, m.funcao(n.Name, n.Token, n.Parameters, n.Body))
	case *ast.BotaStatement:
		if n.Name == nil || n.OpComposto != "" {
			return
		}
		if fl, ok := n.Value.(*ast.FuncaoLiteral); ok && fl != nil {
			// bota f = gambiarra(x) ... : mostra como funcao
			if topo && !vistos[n.Name.Value] {
				vistos[n.Name.Value] = true
				s := m.funcao(n.Name, fl.Token, fl.Parameters, fl.Body)
				s.Range.Start = m.a.linhas.posLSP(n.Token.Line, n.Token.Coluna)
				*out = append(*out, s)
			} else if !topo {
				s := m.funcao(n.Name, fl.Token, fl.Parameters, fl.Body)
				s.Range.Start = m.a.linhas.posLSP(n.Token.Line, n.Token.Coluna)
				*out = append(*out, s)
			}
			return
		}
		novaVar(n.Name, n.Token, kindVariavel, "")
	case *ast.CravaStatement:
		novaVar(n.Name, n.Token, kindConstante, "crava")
	case *ast.DesestruturaStatement:
		for _, nome := range n.Names {
			novaVar(nome, n.Token, kindVariavel, "")
		}
	case *ast.ImportaStatement:
		if n.Alias != nil {
			detalhe := ""
			if t, ok := n.Path.(*ast.TextoLiteral); ok && t != nil {
				detalhe = `"` + t.Value + `"`
			}
			novaVar(n.Alias, n.Token, kindModulo, detalhe)
		}
	case *ast.SeColarStatement:
		for _, b := range n.Consequences {
			m.doBloco(b, out, vistos, topo)
		}
		m.doBloco(n.Alternative, out, vistos, topo)
	case *ast.EnquantoStatement:
		m.doBloco(n.Body, out, vistos, topo)
	case *ast.PraCadaNumStatement:
		m.doBloco(n.Body, out, vistos, topo)
	case *ast.PraCadaListStatement:
		m.doBloco(n.Body, out, vistos, topo)
	case *ast.ArrumaStatement:
		m.doBloco(n.Try, out, vistos, topo)
		m.doBloco(n.Catch, out, vistos, topo)
		m.doBloco(n.Finally, out, vistos, topo)
	case *ast.EscolheStatement:
		for _, c := range n.Casos {
			m.doBloco(c.Body, out, vistos, topo)
		}
		m.doBloco(n.Default, out, vistos, topo)
	}
}

func (m *montaSimbolos) doBloco(b *ast.BlockStatement, out *[]SimboloDoc, vistos map[string]bool, topo bool) {
	if b == nil {
		return
	}
	for _, st := range b.Statements {
		m.doEscopo(st, out, vistos, topo)
	}
}

// funcao monta o simbolo de uma gambiarra, com as gambiarras de dentro como
// filhas. O range vai do token `gambiarra` ate o acabou_finalmente dela.
func (m *montaSimbolos) funcao(nome *ast.Identifier, tok token.Token, params []*ast.Parametro, corpo *ast.BlockStatement) SimboloDoc {
	ps := make([]string, 0, len(params))
	for _, p := range params {
		if p != nil && p.Nome != nil {
			ps = append(ps, textoParametro(p))
		}
	}
	sel := m.a.linhas.faixa(nome.Token.Line, nome.Token.Coluna, len([]rune(nome.Value)))
	inicio := m.a.linhas.posLSP(tok.Line, tok.Coluna)
	fim := m.fimDoBloco(tok)
	if fim.Line < sel.End.Line || (fim.Line == sel.End.Line && fim.Character < sel.End.Character) {
		fim = sel.End
	}
	s := SimboloDoc{
		Name:           nome.Value,
		Detail:         "(" + strings.Join(ps, ", ") + ")",
		Kind:           kindFuncao,
		Range:          Faixa{Start: inicio, End: fim},
		SelectionRange: sel,
	}
	filhos := []SimboloDoc{}
	m.doBloco(corpo, &filhos, map[string]bool{}, false)
	if len(filhos) > 0 {
		s.Children = filhos
	}
	return s
}

// fimDoBloco conta abre/fecha de bloco nos tokens a partir do `gambiarra`
// ate o acabou_finalmente que fecha ele. Abrem bloco: gambiarra, enquanto,
// pra_cada, arruma, escolhe e se_colar (menos o ternario, que o AST marca,
// e o `se_nao_colar se_colar` do senao-se, que divide o acabou do se_colar).
func (m *montaSimbolos) fimDoBloco(tok token.Token) Posicao {
	i, ok := m.idx[[2]int{tok.Line, tok.Coluna}]
	if !ok {
		return m.a.linhas.fimDaLinha(tok.Line)
	}
	var pilha []token.TokenType
	for j := i; j < len(m.tokens); j++ {
		t := m.tokens[j]
		switch t.Type {
		case token.GAMBIARRA, token.ENQUANTO, token.PRA_CADA, token.ARRUMA, token.ESCOLHE:
			pilha = append(pilha, t.Type)
		case token.SE_COLAR:
			if m.a.ternarios[[2]int{t.Line, t.Coluna}] {
				continue
			}
			if j > 0 && m.tokens[j-1].Type == token.SE_NAO_COLAR && len(pilha) > 0 && pilha[len(pilha)-1] == token.SE_COLAR {
				continue // senao-se
			}
			pilha = append(pilha, t.Type)
		case token.ACABOU:
			if len(pilha) > 0 {
				pilha = pilha[:len(pilha)-1]
			}
			if len(pilha) == 0 {
				return m.a.linhas.posLSP(t.Line, t.Coluna+len([]rune(t.Literal)))
			}
		}
	}
	ultimo := m.tokens[len(m.tokens)-1]
	return m.a.linhas.fimDaLinha(ultimo.Line)
}
