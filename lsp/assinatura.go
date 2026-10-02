package lsp

import (
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/token"
)

// ---- textDocument/signatureHelp ----
//
// Acha a chamada aberta no cursor pelos TOKENS (funciona com o codigo pela
// metade enquanto a pessoa digita, quando o parse ainda falha) e monta a
// assinatura: gambiarra do usuario (params com padrao e ...resto, comentarios
// `#` de cima viram a doc) ou builtin (mesma doc do hover).

type InfoParametro struct {
	// Label e [inicio, fim) em unidades UTF-16 dentro do label da assinatura
	Label [2]int `json:"label"`
}

type InfoAssinatura struct {
	Label           string          `json:"label"`
	Documentation   string          `json:"documentation,omitempty"`
	Parameters      []InfoParametro `json:"parameters"`
	ActiveParameter int             `json:"activeParameter"`
}

type AjudaAssinatura struct {
	Signatures      []InfoAssinatura `json:"signatures"`
	ActiveSignature int              `json:"activeSignature"`
	ActiveParameter int              `json:"activeParameter"`
}

// chamadaAberta descreve a chamada mais interna que contem o cursor.
type chamadaAberta struct {
	nome  string
	alias string // `m.f(`: m
	linha int    // posicao do token do nome (1-based, runes)
	col   int
	arg   int // indice do argumento onde o cursor esta
}

// chamadaNoCursor varre os tokens ate o cursor mantendo uma pilha de
// parenteses/colchetes/chaves; virgula avanca o argumento do topo.
func chamadaNoCursor(texto string, pos Posicao) *chamadaAberta {
	ls := novasLinhas(texto)
	linhaC := pos.Line + 1
	colC := ls.utf16ParaRune(pos.Line, pos.Character) + 1

	type moldura struct {
		ch  *chamadaAberta // nil = agrupamento, lista ou dicionario
		tok token.TokenType
	}
	var pilha []moldura
	var ant [3]token.Token // ant[0] = token anterior, ant[1] o de antes...
	l := lexer.New(texto)
	for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
		if tok.Line > linhaC || (tok.Line == linhaC && tok.Coluna >= colC) {
			break
		}
		switch tok.Type {
		case token.LPAREN:
			m := moldura{tok: tok.Type}
			if ant[0].Type == token.IDENT && ant[1].Type != token.GAMBIARRA {
				m.ch = &chamadaAberta{nome: ant[0].Literal, linha: ant[0].Line, col: ant[0].Coluna}
				if (ant[1].Type == token.DOT || ant[1].Type == token.QDOT) && ant[2].Type == token.IDENT {
					m.ch.alias = ant[2].Literal
				}
			}
			pilha = append(pilha, m)
		case token.LBRACKET, token.LBRACE:
			pilha = append(pilha, moldura{tok: tok.Type})
		case token.RPAREN, token.RBRACKET, token.RBRACE:
			if len(pilha) > 0 {
				pilha = pilha[:len(pilha)-1]
			}
		case token.COMMA:
			if len(pilha) > 0 && pilha[len(pilha)-1].ch != nil {
				pilha[len(pilha)-1].ch.arg++
			}
		}
		ant[2], ant[1], ant[0] = ant[1], ant[0], tok
	}
	for i := len(pilha) - 1; i >= 0; i-- {
		if pilha[i].ch != nil {
			return pilha[i].ch
		}
	}
	return nil
}

func (s *Servidor) ajudaAssinatura(uri string, pos Posicao) *AjudaAssinatura {
	texto, ok := s.docs[uri]
	if !ok {
		return nil
	}
	ch := chamadaNoCursor(texto, pos)
	if ch == nil {
		return nil
	}
	_, a := s.analisarDoc(uri)
	var sig *InfoAssinatura
	variadico := false
	if decl := declDaChamada(a, ch); decl != nil {
		sig, variadico = assinaturaUsuario(decl)
	} else if ch.alias == "" {
		sig, variadico = assinaturaBuiltin(ch.nome)
	}
	if sig == nil {
		return nil
	}
	ativo := ch.arg
	if variadico && len(sig.Parameters) > 0 && ativo >= len(sig.Parameters) {
		ativo = len(sig.Parameters) - 1
	}
	sig.ActiveParameter = ativo
	return &AjudaAssinatura{Signatures: []InfoAssinatura{*sig}, ActiveParameter: ativo}
}

// declDaChamada acha a declaracao (gambiarra ou bota f = lambda) chamada.
func declDaChamada(a *analise, ch *chamadaAberta) *ocorrencia {
	var achada *ocorrencia
	for _, o := range a.ocorrencias {
		if o.linha == ch.linha && o.col == ch.col {
			achada = o
			break
		}
	}
	if achada != nil {
		if achada.simb != nil && achada.simb.decl != nil && achada.simb.decl.ehFunc {
			return achada.simb.decl
		}
		return nil // builtin, ou resolveu pra algo que nao e gambiarra (param, var...)
	}
	// o parse falhou bem na chamada (digitando)
	if ch.alias != "" {
		// `m.f(`: resolve o m pelo `importa ... como m` e acha f no modulo
		for _, imp := range a.importacoes {
			if imp.alias == nil || imp.alias.nome != ch.alias || imp.resolvido == "" {
				continue
			}
			if mod := a.ctx.modulo(imp.resolvido); mod != nil {
				if sb := mod.simboloDeTopo(ch.nome, map[string]bool{}); sb != nil && sb.decl != nil && sb.decl.ehFunc {
					return sb.decl
				}
			}
		}
		return nil
	}
	// procura pelo nome
	var melhor *ocorrencia
	for _, o := range a.ocorrencias {
		if o.nome == ch.nome && o.ehFunc && (melhor == nil || o.linha <= ch.linha) {
			melhor = o
		}
	}
	if melhor != nil {
		return melhor
	}
	if sb := a.externo(ch.nome, map[string]bool{}); sb != nil && sb.decl != nil && sb.decl.ehFunc {
		return sb.decl
	}
	return nil
}

// tamUTF16 e o tamanho do texto em unidades UTF-16.
func tamUTF16(s string) int {
	n := 0
	for _, r := range s {
		n += larguraUTF16(r)
	}
	return n
}

func assinaturaUsuario(decl *ocorrencia) (*InfoAssinatura, bool) {
	var b strings.Builder
	b.WriteString(decl.nome + "(")
	sig := &InfoAssinatura{Parameters: []InfoParametro{}}
	variadico := false
	for i, p := range decl.params {
		if p == nil || p.Nome == nil {
			continue
		}
		if i > 0 {
			b.WriteString(", ")
		}
		ini := tamUTF16(b.String())
		b.WriteString(textoParametro(p))
		sig.Parameters = append(sig.Parameters, InfoParametro{Label: [2]int{ini, tamUTF16(b.String())}})
		variadico = p.Variadico
	}
	b.WriteString(")")
	sig.Label = b.String()
	if decl.escopo != nil && decl.escopo.a != nil {
		sig.Documentation = comentariosAntes(decl.escopo.a.linhas, decl.linha)
	}
	return sig, variadico
}

func textoParametro(p *ast.Parametro) string {
	if p.Variadico {
		return "..." + p.Nome.Value
	}
	if p.Padrao != nil {
		return p.Nome.Value + " = " + p.Padrao.String()
	}
	return p.Nome.Value
}

// comentariosAntes junta os comentarios `#` colados acima da linha 1-based
// (mesma regra do `gs doc`).
func comentariosAntes(ls linhasDoc, linha int) string {
	var col []string
	for i := linha - 2; i >= 0 && i < len(ls); i-- {
		t := strings.TrimSpace(string(ls[i]))
		if !strings.HasPrefix(t, "#") {
			break
		}
		col = append([]string{strings.TrimSpace(strings.TrimPrefix(t, "#"))}, col...)
	}
	return strings.Join(col, "\n")
}

// assinaturaBuiltin monta a assinatura a partir da doc do hover, no formato
// `nome(a, [b], c...) -> retorno: descricao`.
func assinaturaBuiltin(nome string) (*InfoAssinatura, bool) {
	doc, ok := docsBuiltin[nome]
	if !ok {
		return nil, false
	}
	abre := strings.Index(doc, "(")
	if abre < 0 || !strings.HasPrefix(doc, nome+"(") {
		return nil, false // ex: `pi` e valor, nao chamada
	}
	fecha, prof := -1, 0
	for i := abre; i < len(doc); i++ {
		if doc[i] == '(' {
			prof++
		} else if doc[i] == ')' {
			prof--
			if prof == 0 {
				fecha = i
				break
			}
		}
	}
	if fecha < 0 {
		return nil, false
	}
	label, descricao := doc[:fecha+1], ""
	resto := doc[fecha+1:]
	if k := strings.Index(resto, ": "); k >= 0 {
		label += resto[:k]
		descricao = strings.TrimSpace(resto[k+2:])
	} else {
		label += strings.TrimSuffix(resto, ".")
	}
	sig := &InfoAssinatura{Label: label, Documentation: descricao, Parameters: []InfoParametro{}}
	variadico := false
	// separa os params na virgula de nivel zero (colchete de opcional conta)
	ini, prof := abre+1, 0
	for i := abre + 1; i <= fecha; i++ {
		c := doc[i]
		switch {
		case c == '[' || c == '(':
			prof++
		case c == ']' || (c == ')' && i != fecha):
			prof--
		case (c == ',' && prof == 0) || i == fecha:
			pedaco := doc[ini:i]
			esq := len(pedaco) - len(strings.TrimLeft(pedaco, " "))
			p := strings.TrimSpace(pedaco)
			if p != "" {
				a := ini + esq
				sig.Parameters = append(sig.Parameters, InfoParametro{Label: [2]int{tamUTF16(doc[:a]), tamUTF16(doc[:a+len(p)])}})
				variadico = strings.Contains(p, "...")
			}
			ini = i + 1
		}
	}
	return sig, variadico
}
