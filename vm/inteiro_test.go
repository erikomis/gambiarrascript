package vm

import "testing"

func TestVMInteiroGrandePreservaPrecisao(t *testing.T) {
	casos := []struct{ in, esp string }{
		{`9007199254740993`, "9007199254740993"},
		{`9007199254740992 + 1`, "9007199254740993"},
		{`99999999 * 99999999`, "9999999800000001"},
		{`10 / 2`, "5"},
		{`7 / 2`, "3.5"},
	}
	for _, c := range casos {
		got, _ := rodaVM(t, c.in)
		if got.Inspect() != c.esp {
			t.Errorf("%q => got %s, esperado %s", c.in, got.Inspect(), c.esp)
		}
	}
}
