package object

import (
	"math"
	"math/big"
	"testing"
)

// confere SomaInt/SubInt/MulInt/NegInt/DivInt contra a conta exata do
// math/big nos cantos do int64.
func TestContaIntContraBig(t *testing.T) {
	vals := []int64{0, 1, -1, 2, -2, 3, 1 << 31, -(1 << 31), 1<<31 - 1, 1 << 32, 3037000499, 3037000500, -3037000500,
		math.MaxInt64, math.MinInt64, math.MaxInt64 - 1, math.MinInt64 + 1, math.MaxInt64 / 2, math.MinInt64 / 2, 1 << 62, -(1 << 62)}
	cabe := func(b *big.Int) bool { return b.IsInt64() }
	for _, a := range vals {
		for _, b := range vals {
			A, B := big.NewInt(a), big.NewInt(b)
			casos := []struct {
				nome string
				f    func(int64, int64) (int64, bool)
				exat *big.Int
			}{
				{"soma", SomaInt, new(big.Int).Add(A, B)},
				{"sub", SubInt, new(big.Int).Sub(A, B)},
				{"mul", MulInt, new(big.Int).Mul(A, B)},
			}
			for _, c := range casos {
				r, ok := c.f(a, b)
				if ok != cabe(c.exat) || (ok && r != c.exat.Int64()) {
					t.Errorf("%s(%d, %d) = (%d, %v), exato %s", c.nome, a, b, r, ok, c.exat)
				}
			}
			if b != 0 {
				q, m := new(big.Int).QuoRem(A, B, new(big.Int))
				r, ok := DivInt(a, b)
				esp := m.Sign() == 0 && cabe(q)
				if ok != esp || (ok && r != q.Int64()) {
					t.Errorf("div(%d, %d) = (%d, %v), exato %s resto %s", a, b, r, ok, q, m)
				}
			}
		}
		r, ok := NegInt(a)
		exat := new(big.Int).Neg(big.NewInt(a))
		if ok != cabe(exat) || (ok && r != exat.Int64()) {
			t.Errorf("neg(%d) = (%d, %v)", a, r, ok)
		}
	}
}
