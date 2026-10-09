package object

import (
	"fmt"

	"gambiarrascript/ast"
)

// ---- pattern matching (escolhe/caso) ----
//
// Os dois engines casam padrao com o MESMO codigo: o padrao do AST vira um
// DescPadrao (MontaPadrao), os valores dele (literais, `Cor.vermelho`, o tipo
// de cada treta) sao avaliados antes, em pre-ordem, por cada engine, e o
// Casa decide e devolve o que amarrar. A VM guarda o DescPadrao no pool de
// constantes (por isso e uma struct so, sem interface: gob simples).

const (
	PadValor  = iota // compara com valores[Idx] (==)
	PadAmarra        // amarra no slot Idx; -1 = curinga `_`
	PadLista         // Filhos na ordem; Resto: -2 sem `...`, -1 `..._`, >= 0 slot
	PadDict          // Chaves (literais) -> Filhos
	PadTreta         // valores[Idx] e a treta; Campos -> Filhos
)

const (
	semResto     = -2
	restoCuringa = -1
)

// DescPadrao e um no do padrao. Na raiz, Nomes diz o nome de cada slot e
// NValores quantos valores o engine avalia antes de casar.
type DescPadrao struct {
	Tipo   int
	Idx    int
	Resto  int
	Filhos []*DescPadrao
	Chaves []Object
	Campos []string
	// PadTreta na ordem dos campos (`Ponto{0, y}`): Campos vazio, Filhos
	// tem que ter um por campo da treta
	Posicional bool

	Nomes    []string
	NValores int
}

func (d *DescPadrao) Type() ObjectType { return DESCRITOR_OBJ }
func (d *DescPadrao) Inspect() string  { return "<descritor padrao>" }

// MontaPadrao traduz o padrao do AST. `valores` sao as expressoes que o
// engine avalia (em ordem) antes de chamar Casa — a mesma ordem do
// ast.PercorrePadrao.
func MontaPadrao(e ast.Expression) (*DescPadrao, []ast.Expression) {
	m := &montador{slots: map[string]int{}}
	raiz := m.monta(e)
	raiz.Nomes = m.nomes
	raiz.NValores = len(m.valores)
	return raiz, m.valores
}

type montador struct {
	valores []ast.Expression
	nomes   []string
	slots   map[string]int
}

func (m *montador) slot(nome string) int {
	if nome == "_" {
		return -1
	}
	if s, ok := m.slots[nome]; ok {
		return s
	}
	m.slots[nome] = len(m.nomes)
	m.nomes = append(m.nomes, nome)
	return len(m.nomes) - 1
}

func (m *montador) valor(e ast.Expression) int {
	m.valores = append(m.valores, e)
	return len(m.valores) - 1
}

func (m *montador) monta(e ast.Expression) *DescPadrao {
	switch p := e.(type) {
	case *ast.PadraoNome:
		return &DescPadrao{Tipo: PadAmarra, Idx: m.slot(p.Nome.Value)}
	case *ast.PadraoLista:
		d := &DescPadrao{Tipo: PadLista, Resto: semResto}
		for _, el := range p.Elementos {
			d.Filhos = append(d.Filhos, m.monta(el))
		}
		if p.Resto != nil {
			d.Resto = m.slot(p.Resto.Value)
		}
		return d
	case *ast.PadraoDict:
		d := &DescPadrao{Tipo: PadDict}
		for i, ch := range p.Chaves {
			d.Chaves = append(d.Chaves, literalDeChave(ch))
			d.Filhos = append(d.Filhos, m.monta(p.Valores[i]))
		}
		return d
	case *ast.PadraoTreta:
		d := &DescPadrao{Tipo: PadTreta, Idx: m.valor(p.Tipo), Posicional: p.Posicional}
		for i, v := range p.Valores {
			if !p.Posicional {
				d.Campos = append(d.Campos, p.Nomes[i].Value)
			}
			d.Filhos = append(d.Filhos, m.monta(v))
		}
		return d
	}
	return &DescPadrao{Tipo: PadValor, Idx: m.valor(e)}
}

func literalDeChave(e ast.Expression) Object {
	switch n := e.(type) {
	case *ast.TextoLiteral:
		return &Texto{Value: n.Value}
	case *ast.NumeroLiteral:
		return &Numero{Value: n.Value, Int: n.Int, EhInt: n.EhInt}
	case *ast.BooleanoLiteral:
		return &Booleano{Value: n.Value}
	}
	return &Texto{Value: e.String()}
}

// Casa tenta casar `v` com o padrao. `valores` sao os NValores ja avaliados,
// `amarras` tem len(Nomes) (e so vale se casou), `igual` e o == do engine.
// msg != "" e erro de verdade (ex.: o tipo do padrao nao e treta).
func Casa(d *DescPadrao, v Object, valores, amarras []Object, igual func(a, b Object) bool) (bool, string) {
	switch d.Tipo {
	case PadValor:
		return igual(v, valores[d.Idx]), ""
	case PadAmarra:
		if d.Idx >= 0 {
			amarras[d.Idx] = v
		}
		return true, ""
	case PadLista:
		l, ok := v.(*Lista)
		if !ok {
			return false, ""
		}
		elems := l.Visao()
		n := len(d.Filhos)
		if len(elems) < n || (d.Resto == semResto && len(elems) != n) {
			return false, ""
		}
		for i, f := range d.Filhos {
			if ok, msg := Casa(f, elems[i], valores, amarras, igual); !ok || msg != "" {
				return false, msg
			}
		}
		if d.Resto >= 0 {
			resto := make([]Object, len(elems)-n)
			copy(resto, elems[n:])
			amarras[d.Resto] = NovaLista(resto)
		}
		return true, ""
	case PadDict:
		dic, ok := v.(*Dicionario)
		if !ok {
			return false, ""
		}
		for i, ch := range d.Chaves {
			par, existe := dic.Pega(ch.(Chaveavel).ChaveHash())
			if !existe {
				return false, ""
			}
			if ok, msg := Casa(d.Filhos[i], par.Valor, valores, amarras, igual); !ok || msg != "" {
				return false, msg
			}
		}
		return true, ""
	case PadTreta:
		t, ehTreta := valores[d.Idx].(*Treta)
		if !ehTreta {
			return false, fmt.Sprintf("no padrao Tipo{...} o Tipo tem que ser treta, e isso ai e %s", NomeTipo(valores[d.Idx]))
		}
		if d.Posicional && len(d.Filhos) != len(t.Campos) {
			return false, fmt.Sprintf("o padrao %s{...} na ordem tem que passar os %d campos (passou %d) — pra so alguns, vai por nome: %s{campo: padrao}",
				t.Nome, len(t.Campos), len(d.Filhos), t.Nome)
		}
		inst, ok := v.(*Instancia)
		if !ok || inst.Tipo != t {
			return false, ""
		}
		if d.Posicional {
			for i, f := range d.Filhos {
				if ok, msg := Casa(f, inst.PegaCampo(i), valores, amarras, igual); !ok || msg != "" {
					return false, msg
				}
			}
			return true, ""
		}
		for i, campo := range d.Campos {
			cv, msg := inst.Membro(campo)
			if msg != "" {
				return false, msg
			}
			if ok, msg := Casa(d.Filhos[i], cv, valores, amarras, igual); !ok || msg != "" {
				return false, msg
			}
		}
		return true, ""
	}
	return false, ""
}
