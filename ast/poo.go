package ast

import (
	"strings"

	"gambiarrascript/token"
)

// ---- POO no modelo do Go (Tier 8) ----
//
// treta = struct (campos + metodos com receiver), combinado = interface
// (satisfeita de forma implicita), puxadinho = embedding (um campo que e so o
// nome de outra treta). Sem class/this/new/heranca.

// TretaDecl e `treta Nome <campos> acabou_finalmente`. Cada campo e um
// *CampoTreta (que tambem e Statement so pra o formatter guardar posicao e
// comentario de cada linha — os engines nunca avaliam campo solto).
type TretaDecl struct {
	Token  token.Token // o 'treta'
	Nome   *Identifier
	Campos []*CampoTreta
}

func (s *TretaDecl) statementNode()       {}
func (s *TretaDecl) TokenLiteral() string { return s.Token.Literal }
func (s *TretaDecl) String() string {
	var sb strings.Builder
	sb.WriteString("treta " + s.Nome.Value + " ")
	for _, c := range s.Campos {
		sb.WriteString(c.String() + "\n")
	}
	sb.WriteString("acabou_finalmente")
	return sb.String()
}

// CampoTreta e uma linha da treta: `nome`, `nome = padrao` ou o puxadinho
// (embedding) `Outra` / `modulo.Outra`, que vem em Embutida (e Nome fica com o
// nome da treta embutida, que e como se acessa: `c.Animal`).
type CampoTreta struct {
	Token    token.Token // primeiro token da linha
	Nome     *Identifier
	Padrao   Expression // nil = sem valor padrao (zero-value = nada)
	Embutida Expression // != nil: puxadinho (Identifier ou modulo.Nome)
}

func (s *CampoTreta) statementNode()       {}
func (s *CampoTreta) TokenLiteral() string { return s.Token.Literal }
func (s *CampoTreta) String() string {
	if s.Embutida != nil {
		return s.Embutida.String()
	}
	if s.Padrao != nil {
		return s.Nome.Value + " = " + s.Padrao.String()
	}
	return s.Nome.Value
}

// CombinadoDecl e `combinado Nome <assinaturas> acabou_finalmente`: o
// contrato (interface). Uma treta satisfaz o combinado se tem todos os
// metodos — sem declarar nada (igual Go).
type CombinadoDecl struct {
	Token   token.Token // o 'combinado'
	Nome    *Identifier
	Metodos []*AssinaturaMetodo
}

func (s *CombinadoDecl) statementNode()       {}
func (s *CombinadoDecl) TokenLiteral() string { return s.Token.Literal }
func (s *CombinadoDecl) String() string {
	var sb strings.Builder
	sb.WriteString("combinado " + s.Nome.Value + " ")
	for _, m := range s.Metodos {
		sb.WriteString(m.String() + "\n")
	}
	sb.WriteString("acabou_finalmente")
	return sb.String()
}

// AssinaturaMetodo e uma linha do combinado: `escreve(texto)` ou o nome de
// outro combinado embutido (Embutido != nil), cujos metodos entram junto.
type AssinaturaMetodo struct {
	Token      token.Token
	Nome       *Identifier
	Parametros []*Parametro
	Embutido   Expression // != nil: combinado embutido (Identifier ou modulo.Nome)
}

func (s *AssinaturaMetodo) statementNode()       {}
func (s *AssinaturaMetodo) TokenLiteral() string { return s.Token.Literal }
func (s *AssinaturaMetodo) String() string {
	if s.Embutido != nil {
		return s.Embutido.String()
	}
	ps := make([]string, len(s.Parametros))
	for i, p := range s.Parametros {
		ps[i] = p.String()
	}
	return s.Nome.Value + "(" + strings.Join(ps, ", ") + ")"
}

// MetodoDecl e a gambiarra com receiver: `gambiarra (p Ponto) nome(params)
// <corpo> acabou_finalmente`. Vira uma funcao cujo primeiro parametro e o
// receiver, registrada na tabela de metodos da treta.
type MetodoDecl struct {
	Token      token.Token // o 'gambiarra'
	Receptor   *Identifier // `p`
	Tipo       *Identifier // `Ponto`
	Nome       *Identifier
	Parameters []*Parametro
	Body       *BlockStatement
}

func (s *MetodoDecl) statementNode()       {}
func (s *MetodoDecl) TokenLiteral() string { return s.Token.Literal }
func (s *MetodoDecl) String() string {
	ps := make([]string, len(s.Parameters))
	for i, p := range s.Parameters {
		ps[i] = p.String()
	}
	return "gambiarra (" + s.Receptor.Value + " " + s.Tipo.Value + ") " + s.Nome.Value +
		"(" + strings.Join(ps, ", ") + ") " + s.Body.String() + "acabou_finalmente"
}

// ParametrosComReceptor devolve [receptor, params...]: o metodo roda como uma
// gambiarra comum que recebe o receiver no primeiro parametro.
func (s *MetodoDecl) ParametrosComReceptor() []*Parametro {
	out := make([]*Parametro, 0, len(s.Parameters)+1)
	out = append(out, &Parametro{Nome: s.Receptor})
	return append(out, s.Parameters...)
}

// TretaLiteral e `Ponto{x: 1, y: 2}` (nomeado) ou `Ponto{1, 2}`
// (posicional). Nomes == nil = posicional. Token e o `{`.
type TretaLiteral struct {
	Token   token.Token
	Tipo    Expression // Identifier ou modulo.Nome
	Nomes   []*Identifier
	Valores []Expression
}

func (e *TretaLiteral) expressionNode()      {}
func (e *TretaLiteral) TokenLiteral() string { return e.Token.Literal }
func (e *TretaLiteral) String() string {
	partes := make([]string, len(e.Valores))
	for i, v := range e.Valores {
		partes[i] = v.String()
		if e.Nomes != nil {
			partes[i] = e.Nomes[i].Value + ": " + partes[i]
		}
	}
	return e.Tipo.String() + "{" + strings.Join(partes, ", ") + "}"
}

// Posicional diz se o literal passa os campos na ordem (`Ponto{1, 2}`).
func (e *TretaLiteral) Posicional() bool { return e.Nomes == nil }
