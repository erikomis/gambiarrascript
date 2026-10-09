package object

import (
	"math"
	"slices"
	"strings"
)

// OrdenaNatural e o caminho rapido do `ordena`: lista so de numeros ou so de
// textos. O caminho geral (sort.SliceStable com comparador generico) e
// insertion sort + merge sem buffer, O(n log² n) com troca via reflection —
// 300 mil inteiros levavam ~0,37s (o Python leva ~0,04s).
//
// Aqui a chave e extraida uma vez e o slices.SortFunc (pdqsort, O(n log n))
// ordena pares (chave, posicao original). Desempatar pela posicao deixa o
// resultado ESTAVEL e identico ao do caminho geral: mesma comparacao
// (Numero.Value com <, texto byte a byte), mesma ordem entre iguais (1 e 1.0
// ficam como vieram).
//
// Devolve false sem mexer na lista quando nao se aplica (mistura de tipos,
// outro tipo, NaN — que nao tem ordem total); quem chama cai no caminho geral,
// que da o erro de sempre.
func (l *Lista) OrdenaNatural() bool {
	if concorrencia.Load() {
		l.mu.Lock()
		defer l.mu.Unlock()
	}
	e := l.elems
	if len(e) < 2 {
		return true // nada pra comparar (o caminho geral tambem nao compara)
	}
	if _, ok := e[0].(*Numero); ok {
		pares := make([]parNum, len(e))
		for i, o := range e {
			n, ok := o.(*Numero)
			if !ok || math.IsNaN(n.Value) {
				return false
			}
			pares[i] = parNum{n.Value, i}
		}
		slices.SortFunc(pares, func(a, b parNum) int {
			if a.v < b.v {
				return -1
			}
			if a.v > b.v {
				return 1
			}
			return a.i - b.i
		})
		aplicaOrdem(e, pares, func(p parNum) int { return p.i })
		return true
	}
	if _, ok := e[0].(*Texto); ok {
		pares := make([]parTxt, len(e))
		for i, o := range e {
			t, ok := o.(*Texto)
			if !ok {
				return false
			}
			pares[i] = parTxt{t.Value, i}
		}
		slices.SortFunc(pares, func(a, b parTxt) int {
			if c := strings.Compare(a.v, b.v); c != 0 {
				return c
			}
			return a.i - b.i
		})
		aplicaOrdem(e, pares, func(p parTxt) int { return p.i })
		return true
	}
	return false
}

type parNum struct {
	v float64
	i int
}

type parTxt struct {
	v string
	i int
}

func aplicaOrdem[P any](e []Object, pares []P, pos func(P) int) {
	velho := slices.Clone(e)
	for k, p := range pares {
		e[k] = velho[pos(p)]
	}
}
