package ast

import (
	"strings"

	"gambiarrascript/token"
)

// ---- pattern matching no escolhe/caso e cardapio (enum) ----
//
// Um `caso` aceita, alem do valor de sempre (comparado com ==), um PADRAO
// estrutural: lista `[a, ...resto]`, dicionario `{"tipo": t}` ou treta
// `Ponto{x: 0, y}`. So DENTRO desses padroes um nome solto amarra (vira
// variavel); fora deles `caso x` continua comparando com a variavel x.
// `_` e o curinga (casa qualquer coisa e nao amarra nada).
//
// Dentro de um padrao, cada pedaco e um sub-padrao: outro padrao estrutural,
// um PadraoNome (amarra/curinga) ou um VALOR — literal (numero, texto,
// deu_bom/deu_ruim, nada, numero negativo) ou caminho com ponto
// (`Cor.vermelho`, `mod.LIMITE`), comparado com ==.

// PadraoNome e um nome solto dentro de um padrao: amarra o valor (ou, se for
// `_`, so aceita qualquer coisa).
type PadraoNome struct {
	Token token.Token
	Nome  *Identifier
}

func (p *PadraoNome) expressionNode()      {}
func (p *PadraoNome) TokenLiteral() string { return p.Token.Literal }
func (p *PadraoNome) String() string       { return p.Nome.Value }

// Curinga diz se e o `_` (nao amarra nada).
func (p *PadraoNome) Curinga() bool { return p.Nome.Value == "_" }

// PadraoLista e `[p1, p2]` (tamanho exato) ou `[p1, ...resto]` (pelo menos
// os da frente; o resto vira lista nova). Resto `..._` so ignora.
type PadraoLista struct {
	Token     token.Token // o '['
	Elementos []Expression
	Resto     *Identifier // nil = sem `...`
}

func (p *PadraoLista) expressionNode()      {}
func (p *PadraoLista) TokenLiteral() string { return p.Token.Literal }
func (p *PadraoLista) String() string {
	partes := make([]string, 0, len(p.Elementos)+1)
	for _, e := range p.Elementos {
		partes = append(partes, e.String())
	}
	if p.Resto != nil {
		partes = append(partes, "..."+p.Resto.Value)
	}
	return "[" + strings.Join(partes, ", ") + "]"
}

// PadraoDict e `{"chave": padrao, nome}`: as chaves listadas tem que existir
// e casar; chave a mais no dicionario pode. `nome` sozinho = `"nome": nome`.
type PadraoDict struct {
	Token   token.Token  // o '{'
	Chaves  []Expression // TextoLiteral, NumeroLiteral ou BooleanoLiteral
	Valores []Expression
	Curto   []bool // entrada escrita so com o nome (`{nome}`)
}

func (p *PadraoDict) expressionNode()      {}
func (p *PadraoDict) TokenLiteral() string { return p.Token.Literal }
func (p *PadraoDict) String() string {
	partes := make([]string, len(p.Chaves))
	for i := range p.Chaves {
		if p.Curto[i] {
			partes[i] = p.Valores[i].String()
			continue
		}
		partes[i] = textoChave(p.Chaves[i]) + ": " + p.Valores[i].String()
	}
	return "{" + strings.Join(partes, ", ") + "}"
}

func textoChave(e Expression) string {
	if t, ok := e.(*TextoLiteral); ok {
		return `"` + t.Value + `"`
	}
	return e.String()
}

// PadraoTreta e `Ponto{x: 0, y}` (por nome: so os campos listados; `y`
// sozinho = `y: y`) ou `Ponto{0, y}` (Posicional: na ordem dos campos, todos,
// igual o literal). Casa instancia DAQUELA treta cujos campos casam.
type PadraoTreta struct {
	Token      token.Token // o '{'
	Tipo       Expression  // Identifier ou modulo.Nome
	Nomes      []*Identifier
	Valores    []Expression
	Curto      []bool
	Posicional bool // Nomes/Curto vazios
}

func (p *PadraoTreta) expressionNode()      {}
func (p *PadraoTreta) TokenLiteral() string { return p.Token.Literal }
func (p *PadraoTreta) String() string {
	partes := make([]string, len(p.Valores))
	for i, v := range p.Valores {
		switch {
		case p.Posicional, p.Curto[i]:
			partes[i] = v.String()
		default:
			partes[i] = p.Nomes[i].Value + ": " + v.String()
		}
	}
	return p.Tipo.String() + "{" + strings.Join(partes, ", ") + "}"
}

// EhPadrao diz se a expressao de um caso e um padrao (e nao um valor comum
// comparado com ==).
func EhPadrao(e Expression) bool {
	switch e.(type) {
	case *PadraoNome, *PadraoLista, *PadraoDict, *PadraoTreta:
		return true
	}
	return false
}

// PercorrePadrao anda num padrao em pre-ordem: `amarra` recebe cada nome que
// o padrao amarra (o curinga `_` nao conta) e `valor` cada expressao avaliada
// antes de casar (valores comparados com == e o tipo das tretas), na MESMA
// ordem em que os dois engines avaliam. Valor comum (fora de padrao) vai
// inteiro pro `valor`.
func PercorrePadrao(e Expression, amarra func(*Identifier), valor func(Expression)) {
	switch p := e.(type) {
	case *PadraoNome:
		if !p.Curinga() && amarra != nil {
			amarra(p.Nome)
		}
	case *PadraoLista:
		for _, el := range p.Elementos {
			PercorrePadrao(el, amarra, valor)
		}
		if p.Resto != nil && p.Resto.Value != "_" && amarra != nil {
			amarra(p.Resto)
		}
	case *PadraoDict:
		for _, v := range p.Valores {
			PercorrePadrao(v, amarra, valor)
		}
	case *PadraoTreta:
		if valor != nil {
			valor(p.Tipo)
		}
		for _, v := range p.Valores {
			PercorrePadrao(v, amarra, valor)
		}
	default:
		if valor != nil && e != nil {
			valor(e)
		}
	}
}

// NomesDoPadrao devolve os nomes que o padrao amarra, em ordem (sem repetir).
func NomesDoPadrao(e Expression) []*Identifier {
	var out []*Identifier
	visto := map[string]bool{}
	PercorrePadrao(e, func(id *Identifier) {
		if !visto[id.Value] {
			visto[id.Value] = true
			out = append(out, id)
		}
	}, nil)
	return out
}

// ---- cardapio (enum) ----

// CardapioDecl e `cardapio Cor <um membro por linha> acabou_finalmente`.
// Cada membro e um *MembroCardapio (Statement so pra o formatter guardar
// posicao e comentario de cada linha, igual o CampoTreta).
type CardapioDecl struct {
	Token   token.Token // o 'cardapio'
	Nome    *Identifier
	Membros []*MembroCardapio
}

func (s *CardapioDecl) statementNode()       {}
func (s *CardapioDecl) TokenLiteral() string { return s.Token.Literal }
func (s *CardapioDecl) String() string {
	var sb strings.Builder
	sb.WriteString("cardapio " + s.Nome.Value + " ")
	for _, m := range s.Membros {
		sb.WriteString(m.String() + "\n")
	}
	sb.WriteString("acabou_finalmente")
	return sb.String()
}

// NomesMembros devolve os nomes dos membros, na ordem.
func (s *CardapioDecl) NomesMembros() []string {
	out := make([]string, len(s.Membros))
	for i, m := range s.Membros {
		out[i] = m.Nome.Value
	}
	return out
}

// MembroCardapio e uma linha do cardapio (o nome do membro).
type MembroCardapio struct {
	Token token.Token
	Nome  *Identifier
}

func (s *MembroCardapio) statementNode()       {}
func (s *MembroCardapio) TokenLiteral() string { return s.Token.Literal }
func (s *MembroCardapio) String() string       { return s.Nome.Value }
