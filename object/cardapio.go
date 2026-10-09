package object

import "fmt"

// ---- cardapio (enum) ----
//
// `cardapio Cor` / `vermelho` / `verde` / `acabou_finalmente` cria um valor
// Cardapio com uma Opcao por linha. Opcao compara por identidade (so e igual a
// ela mesma), imprime como `Cor.vermelho`, `tipo()` da "Cor" e serve de
// chave de dicionario/conjunto. API minima: `.nome` e `.indice`.

const (
	CARDAPIO_OBJ = "CARDAPIO"
	OPCAO_OBJ    = "OPCAO"
)

// Cardapio e o enum declarado: o nome e as opcoes, na ordem da declaracao.
type Cardapio struct {
	Nome    string
	Opcoes  []*Opcao
	porNome map[string]*Opcao
}

func (c *Cardapio) Type() ObjectType { return CARDAPIO_OBJ }
func (c *Cardapio) Inspect() string  { return "<cardapio " + c.Nome + ">" }

// NovoCardapio monta o cardapio com as opcoes nessa ordem.
func NovoCardapio(nome string, membros []string) *Cardapio {
	c := &Cardapio{Nome: nome, Opcoes: make([]*Opcao, len(membros)), porNome: make(map[string]*Opcao, len(membros))}
	for i, m := range membros {
		o := &Opcao{Cardapio: c, Nome: m, Indice: i}
		c.Opcoes[i] = o
		c.porNome[m] = o
	}
	return c
}

// Membro le `Cor.vermelho`. Devolve a mensagem de erro (ou "").
func (c *Cardapio) Membro(nome string) (Object, string) {
	if o, ok := c.porNome[nome]; ok {
		return o, ""
	}
	return nil, fmt.Sprintf("o cardapio %s nao tem %s", c.Nome, nome)
}

// ListaOpcoes devolve as opcoes como lista nova (pra iterar).
func (c *Cardapio) ListaOpcoes() []Object {
	out := make([]Object, len(c.Opcoes))
	for i, o := range c.Opcoes {
		out[i] = o
	}
	return out
}

// Opcao e um membro do cardapio (`Cor.vermelho`).
type Opcao struct {
	Cardapio *Cardapio
	Nome     string
	Indice   int
}

func (o *Opcao) Type() ObjectType { return OPCAO_OBJ }
func (o *Opcao) Inspect() string  { return o.Cardapio.Nome + "." + o.Nome }

// ChaveHash: identidade (o ponteiro entra na chave — dois cardapios com o
// mesmo nome, de modulos diferentes, nao se misturam).
func (o *Opcao) ChaveHash() HashKey {
	return HashKey{Tipo: OPCAO_OBJ, Valor: fmt.Sprintf("%s.%s@%p", o.Cardapio.Nome, o.Nome, o)}
}

// Membro le `Cor.vermelho.nome` / `.indice`.
func (o *Opcao) Membro(nome string) (Object, string) {
	switch nome {
	case "nome":
		return &Texto{Value: o.Nome}, ""
	case "indice":
		return NumInt(int64(o.Indice)), ""
	}
	return nil, fmt.Sprintf("%s so tem .nome e .indice, nao .%s", o.Inspect(), nome)
}

// MembroDePonto resolve `x.nome` pra cardapio e opcao. ok = false: x nao e
// nenhum dos dois (o chamador segue o caminho dele).
func MembroDePonto(x Object, idx Object) (v Object, msg string, ok bool) {
	var nome string
	switch c := x.(type) {
	case *Cardapio:
		t, ehTexto := idx.(*Texto)
		if !ehTexto {
			return nil, fmt.Sprintf("opcao de cardapio e por nome (%s.opcao), veio %s", c.Nome, NomeTipo(idx)), true
		}
		nome = t.Value
		v, msg = c.Membro(nome)
		return v, msg, true
	case *Opcao:
		t, ehTexto := idx.(*Texto)
		if !ehTexto {
			return nil, fmt.Sprintf("%s so tem .nome e .indice", c.Inspect()), true
		}
		v, msg = c.Membro(t.Value)
		return v, msg, true
	}
	return nil, "", false
}

// DescCardapio e o descritor que o compilador poe no pool pra OpCardapio.
type DescCardapio struct {
	Nome    string
	Membros []string
}

func (d *DescCardapio) Type() ObjectType { return DESCRITOR_OBJ }
func (d *DescCardapio) Inspect() string  { return "<descritor cardapio " + d.Nome + ">" }
