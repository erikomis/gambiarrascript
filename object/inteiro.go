package object

import (
	"math"
	"math/bits"
)

// Conta entre inteiros exatos, a mesma pros dois motores e pro constant
// folding do compilador. Cada funcao devolve (resultado, true) quando o
// resultado e um inteiro que cabe no int64; ok=false manda quem chamou fazer
// a conta em real (float64) — inteiro que estoura vira real, nunca da a volta
// nem satura. Os testes de estouro sao os classicos de sinal (sem divisao),
// baratos o bastante pro caminho quente da VM.

// SomaInt faz a + b.
func SomaInt(a, b int64) (int64, bool) {
	r := a + b
	// estourou se a e b tem o mesmo sinal e r tem o outro
	return r, (a^r)&(b^r) >= 0
}

// SubInt faz a - b.
func SubInt(a, b int64) (int64, bool) {
	r := a - b
	// estourou se a e b tem sinais diferentes e r nao tem o sinal de a
	return r, (a^b)&(a^r) >= 0
}

// MulInt faz a * b.
func MulInt(a, b int64) (int64, bool) {
	// os dois cabem em 32 bits: o produto cabe em 64, sem teste nenhum
	if uint64(a+1<<31) < 1<<32 && uint64(b+1<<31) < 1<<32 {
		return a * b, true
	}
	ua, ub := uint64(a), uint64(b)
	if a < 0 {
		ua = -ua
	}
	if b < 0 {
		ub = -ub
	}
	hi, lo := bits.Mul64(ua, ub)
	if hi != 0 {
		return 0, false
	}
	if (a < 0) != (b < 0) {
		if lo > 1<<63 {
			return 0, false
		}
		return -int64(lo), true // lo == 1<<63 da o minimo certinho
	}
	if lo > math.MaxInt64 {
		return 0, false
	}
	return int64(lo), true
}

// NegInt faz -a (so o minimo do int64 nao tem oposto).
func NegInt(a int64) (int64, bool) {
	return -a, a != math.MinInt64
}

// DivInt faz a / b quando a divisao e exata (sem resto) e cabe: 6 / 3 e
// inteiro, 7 / 2 nao (vira 3.5 em real). b == 0 tambem devolve ok=false (quem
// chamou reporta a divisao por zero).
func DivInt(a, b int64) (int64, bool) {
	if b == 0 || a%b != 0 || (a == math.MinInt64 && b == -1) {
		return 0, false
	}
	return a / b, true
}

// RestoInt faz a % b (b == 0 devolve ok=false). Nunca estoura: no Go,
// minimo % -1 e 0.
func RestoInt(a, b int64) (int64, bool) {
	if b == 0 {
		return 0, false
	}
	return a % b, true
}
