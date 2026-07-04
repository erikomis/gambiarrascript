package interpreter

import "testing"

func TestBitwiseAndOrXor(t *testing.T) {
	cases := []struct{ src, esp string }{
		{`mostra 12 & 10`, "8\n"},
		{`mostra 12 | 2`, "14\n"},
		{`mostra 12 ^ 15`, "3\n"},
	}
	for _, c := range cases {
		if out := rodar(t, c.src); out != c.esp {
			t.Fatalf("%q => %q, esperado %q", c.src, out, c.esp)
		}
	}
}

func TestBitwiseShifts(t *testing.T) {
	if out := rodar(t, `mostra 1 << 4`); out != "16\n" {
		t.Fatalf("<<: %q", out)
	}
	if out := rodar(t, `mostra 256 >> 4`); out != "16\n" {
		t.Fatalf(">>: %q", out)
	}
	if out := rodar(t, `mostra 1 >> 1`); out != "0\n" {
		t.Fatalf(">> zero: %q", out)
	}
}

func TestBitwiseNot(t *testing.T) {
	if out := rodar(t, `mostra ~0`); out != "-1\n" {
		t.Fatalf("~0: %q", out)
	}
	if out := rodar(t, `mostra ~5`); out != "-6\n" {
		t.Fatalf("~5: %q", out)
	}
}

func TestNumerosHexOctBin(t *testing.T) {
	if out := rodar(t, `mostra 0xFF`); out != "255\n" {
		t.Fatalf("hex: %q", out)
	}
	if out := rodar(t, `mostra 0b1010`); out != "10\n" {
		t.Fatalf("bin: %q", out)
	}
	if out := rodar(t, `mostra 0o17`); out != "15\n" {
		t.Fatalf("oct: %q", out)
	}
	if out := rodar(t, `mostra 0xCAFE & 0x0F`); out != "14\n" {
		t.Fatalf("hex&: %q", out)
	}
}

func TestBitwiseComVariaveis(t *testing.T) {
	out := rodar(t, `bota a = 0b1010
bota b = 0b1100
mostra a | b
mostra a & b`)
	if out != "14\n8\n" {
		t.Fatalf("saida %q", out)
	}
}
