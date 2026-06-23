package code

import "testing"

func TestMake(t *testing.T) {
	casos := []struct {
		op       Opcode
		operands []int
		esperado []byte
	}{
		{OpConstant, []int{65534}, []byte{byte(OpConstant), 255, 254}},
		{OpAdd, []int{}, []byte{byte(OpAdd)}},
	}
	for _, c := range casos {
		ins := Make(c.op, c.operands...)
		if len(ins) != len(c.esperado) {
			t.Fatalf("tamanho: got %d, esperado %d", len(ins), len(c.esperado))
		}
		for i, b := range c.esperado {
			if ins[i] != b {
				t.Fatalf("byte %d: got %d, esperado %d", i, ins[i], b)
			}
		}
	}
}

func TestInstructionsString(t *testing.T) {
	instructions := []Instructions{Make(OpAdd), Make(OpConstant, 2), Make(OpConstant, 65535)}
	esperado := "0000 OpAdd\n0001 OpConstant 2\n0004 OpConstant 65535\n"
	concat := Instructions{}
	for _, ins := range instructions {
		concat = append(concat, ins...)
	}
	if concat.String() != esperado {
		t.Fatalf("disassembly:\ngot %q\nesperado %q", concat.String(), esperado)
	}
}

func TestReadOperands(t *testing.T) {
	ins := Make(OpConstant, 65535)
	def, err := Lookup(byte(OpConstant))
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	operands, n := ReadOperands(def, ins[1:])
	if n != 2 {
		t.Fatalf("bytes lidos: got %d, esperado 2", n)
	}
	if operands[0] != 65535 {
		t.Fatalf("operando: got %d, esperado 65535", operands[0])
	}
}
