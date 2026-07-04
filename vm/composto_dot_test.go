package vm

import "testing"

func TestVMAtribuicaoComposta(t *testing.T) {
	casos := []struct{ input, esp string }{
		{"bota x = 10\nx += 5\nx", "15"},
		{"bota x = 8\nx >>= 2\nx", "2"},
		{"bota x = 3\nx *= 7\nx", "21"},
		{"bota xs = [1, 2]\nxs[0] += 41\nxs[0]", "42"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("%q => %s, esperado %s", c.input, got.Inspect(), c.esp)
		}
	}
}

func TestVMDotAccess(t *testing.T) {
	casos := []struct{ input, esp string }{
		{`bota p = {"nome": "Erik"}` + "\np.nome", "Erik"},
		{`bota p = {"n": 1}` + "\nbota p.n = 42\np.n", "42"},
		{`bota c = {"t": 0}` + "\nc.t += 10\nc.t", "10"},
		{`bota cfg = {"db": {"porta": 5432}}` + "\ncfg.db.porta", "5432"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.input)
		if got.Inspect() != c.esp {
			t.Errorf("%q => %s, esperado %s", c.input, got.Inspect(), c.esp)
		}
	}
}
