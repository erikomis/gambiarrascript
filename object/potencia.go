package object

import (
	"errors"
	"math"
)

// ErrZeroANegativo e o erro de `0 ** -n`: na real e uma divisao por zero.
var ErrZeroANegativo = errors.New("zero elevado a negativo e divisao por zero disfarcada, parca")

// Potencia calcula base ** exp — a mesma conta pros dois engines. Inteiro
// elevado a inteiro nao-negativo fica inteiro exato (2 ** 10 == 1024); se
// estourar o int64, ou se tiver float/expoente negativo, vai de math.Pow.
// Devolve (inteiro, float, ehInt): so um dos dois vale, conforme ehInt.
func Potencia(base, exp *Numero) (int64, float64, bool, error) {
	if base.Value == 0 && exp.Value < 0 {
		return 0, 0, false, ErrZeroANegativo
	}
	if base.EhInt && exp.EhInt && exp.Int >= 0 {
		if r, ok := potenciaInt(base.Int, exp.Int); ok {
			return r, 0, true, nil
		}
	}
	return 0, math.Pow(base.Value, exp.Value), false, nil
}

// potenciaInt faz exponenciacao por quadrados em int64; ok=false se estourar.
func potenciaInt(b, e int64) (int64, bool) {
	r := int64(1)
	for e > 0 {
		var ok bool
		if e&1 == 1 {
			if r, ok = mulInt(r, b); !ok {
				return 0, false
			}
		}
		e >>= 1
		if e > 0 {
			if b, ok = mulInt(b, b); !ok {
				return 0, false
			}
		}
	}
	return r, true
}

// mulInt multiplica detectando estouro do int64.
func mulInt(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64) {
		return 0, false
	}
	r := a * b
	if r/b != a {
		return 0, false
	}
	return r, true
}
