package object

import (
	"math"
	"math/rand"
	"testing"
)

// O caminho rapido tem que dar EXATAMENTE a ordem do caminho geral (estavel,
// mesmos objetos nas mesmas posicoes), inclusive com 1 e 1.0 empatados.
func TestOrdenaNaturalIgualAoGeral(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	menor := func(a, b Object) bool {
		if an, ok := a.(*Numero); ok {
			return an.Value < b.(*Numero).Value
		}
		return a.(*Texto).Value < b.(*Texto).Value
	}
	for rodada := 0; rodada < 200; rodada++ {
		n := r.Intn(300)
		elems := make([]Object, n)
		for i := range elems {
			switch {
			case rodada%2 == 1:
				elems[i] = &Texto{Value: string(rune('a' + r.Intn(5)))}
			case r.Intn(2) == 0:
				elems[i] = NumInt(int64(r.Intn(20) - 10))
			default:
				elems[i] = NumFloat(float64(r.Intn(20)-10) + []float64{0, 0.5}[r.Intn(2)])
			}
		}
		rapida := NovaLista(append([]Object(nil), elems...))
		geral := NovaLista(append([]Object(nil), elems...))
		if !rapida.OrdenaNatural() {
			t.Fatalf("rodada %d: caminho rapido recusou lista homogenea", rodada)
		}
		geral.Ordena(menor)
		for i := range elems {
			if rapida.elems[i] != geral.elems[i] {
				t.Fatalf("rodada %d pos %d: %s vs %s", rodada, i, rapida.elems[i].Inspect(), geral.elems[i].Inspect())
			}
		}
	}
}

func TestOrdenaNaturalRecusa(t *testing.T) {
	casos := [][]Object{
		{NumInt(1), &Texto{Value: "a"}},
		{&Texto{Value: "a"}, NumInt(1)},
		{NumInt(1), NumFloat(math.NaN())},
		{&Booleano{Value: true}, &Booleano{Value: true}},
	}
	for i, c := range casos {
		l := NovaLista(append([]Object(nil), c...))
		if l.OrdenaNatural() {
			t.Fatalf("caso %d: deveria recusar", i)
		}
		for k := range c {
			if l.elems[k] != c[k] {
				t.Fatalf("caso %d: mexeu na lista ao recusar", i)
			}
		}
	}
}
