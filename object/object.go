package object

import (
	"math"
	"strconv"
	"strings"

	"gambiarrascript/ast"
)

type ObjectType string

const (
	NUMERO_OBJ   = "NUMERO"
	TEXTO_OBJ    = "TEXTO"
	BOOLEANO_OBJ = "BOOLEANO"
	NADA_OBJ     = "NADA"
	LISTA_OBJ    = "LISTA"
	FUNCAO_OBJ   = "FUNCAO"
	RETORNO_OBJ  = "RETORNO"
	ERRO_OBJ     = "ERRO"
	VAZA_OBJ     = "VAZA"
	CONTINUA_OBJ = "CONTINUA"
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

// FormatNumero imprime inteiros sem casa decimal e o resto com precisao minima.
func FormatNumero(f float64) string {
	if !math.IsInf(f, 0) && !math.IsNaN(f) && f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

type Numero struct{ Value float64 }

func (n *Numero) Type() ObjectType { return NUMERO_OBJ }
func (n *Numero) Inspect() string  { return FormatNumero(n.Value) }

type Texto struct{ Value string }

func (t *Texto) Type() ObjectType { return TEXTO_OBJ }
func (t *Texto) Inspect() string  { return t.Value }

type Booleano struct{ Value bool }

func (b *Booleano) Type() ObjectType { return BOOLEANO_OBJ }
func (b *Booleano) Inspect() string {
	if b.Value {
		return "deu_bom"
	}
	return "deu_ruim"
}

type Nada struct{}

func (n *Nada) Type() ObjectType { return NADA_OBJ }
func (n *Nada) Inspect() string  { return "nada" }

type Lista struct{ Elements []Object }

func (l *Lista) Type() ObjectType { return LISTA_OBJ }
func (l *Lista) Inspect() string {
	partes := make([]string, len(l.Elements))
	for i, e := range l.Elements {
		partes[i] = e.Inspect()
	}
	return "[" + strings.Join(partes, ", ") + "]"
}

type Funcao struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Funcao) Type() ObjectType { return FUNCAO_OBJ }
func (f *Funcao) Inspect() string {
	nomes := make([]string, len(f.Parameters))
	for i, p := range f.Parameters {
		nomes[i] = p.Value
	}
	return "gambiarra(" + strings.Join(nomes, ", ") + ")"
}

type Retorno struct{ Value Object }

func (r *Retorno) Type() ObjectType { return RETORNO_OBJ }
func (r *Retorno) Inspect() string  { return r.Value.Inspect() }

type Erro struct{ Message string }

func (e *Erro) Type() ObjectType { return ERRO_OBJ }
func (e *Erro) Inspect() string  { return e.Message }

type Vaza struct{}

func (v *Vaza) Type() ObjectType { return VAZA_OBJ }
func (v *Vaza) Inspect() string  { return "vaza" }

type Continua struct{}

func (c *Continua) Type() ObjectType { return CONTINUA_OBJ }
func (c *Continua) Inspect() string  { return "continua" }
