package object

import (
	"math"
	"testing"
)

func TestPotencia(t *testing.T) {
	casos := []struct {
		base, exp *Numero
		esp       string
	}{
		{NumInt(2), NumInt(10), "1024"},
		{NumInt(2), NumInt(0), "1"},
		{NumInt(-3), NumInt(3), "-27"},
		{NumInt(2), NumInt(-1), "0.5"},
		{NumInt(4), NumFloat(0.5), "2"},
		{NumFloat(1.5), NumInt(2), "2.25"},
		// estourou o int64: vira float (sem dar a volta no negativo)
		{NumInt(2), NumInt(64), "18446744073709552000"},
		{NumInt(10), NumInt(30), "1e+30"},
		// limite exato do int64 ainda e inteiro
		{NumInt(-2), NumInt(63), "-9223372036854775808"},
	}
	for _, c := range casos {
		iv, fv, ehInt, err := Potencia(c.base, c.exp)
		if err != nil {
			t.Fatalf("%s ** %s: erro %v", c.base.Inspect(), c.exp.Inspect(), err)
		}
		got := FormatNumero(fv)
		if ehInt {
			got = NumInt(iv).Inspect()
		}
		if got != c.esp {
			t.Errorf("%s ** %s = %s, esperado %s", c.base.Inspect(), c.exp.Inspect(), got, c.esp)
		}
	}
}

func TestPotenciaInteiroFicaInteiro(t *testing.T) {
	if _, _, ehInt, _ := Potencia(NumInt(3), NumInt(4)); !ehInt {
		t.Fatalf("3 ** 4 devia ser inteiro")
	}
	if _, _, ehInt, _ := Potencia(NumInt(2), NumInt(-2)); ehInt {
		t.Fatalf("2 ** -2 devia ser float")
	}
}

func TestPotenciaZeroANegativo(t *testing.T) {
	if _, _, _, err := Potencia(NumInt(0), NumInt(-1)); err != ErrZeroANegativo {
		t.Fatalf("0 ** -1 devia dar ErrZeroANegativo, deu %v", err)
	}
}

func TestFormatNumeroFloatGigante(t *testing.T) {
	casos := map[float64]string{
		3:                "3",
		-7:               "-7",
		math.Pow(2, 66):  "73786976294838210000",
		1e300:            "1e+300",
		0.5:              "0.5",
		9007199254740993: "9007199254740992",
	}
	for f, esp := range casos {
		if got := FormatNumero(f); got != esp {
			t.Errorf("FormatNumero(%v) = %q, esperado %q", f, got, esp)
		}
	}
}
