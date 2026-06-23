package code

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type Instructions []byte

type Opcode byte

const (
	OpConstant Opcode = iota
	OpPop
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpTrue
	OpFalse
	OpNada
	OpEqual
	OpNotEqual
	OpGreaterThan
	OpGreaterEqual
	OpMinus
	OpNao
	OpMostra
)

type Definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*Definition{
	OpConstant:     {"OpConstant", []int{2}},
	OpPop:          {"OpPop", []int{}},
	OpAdd:          {"OpAdd", []int{}},
	OpSub:          {"OpSub", []int{}},
	OpMul:          {"OpMul", []int{}},
	OpDiv:          {"OpDiv", []int{}},
	OpMod:          {"OpMod", []int{}},
	OpTrue:         {"OpTrue", []int{}},
	OpFalse:        {"OpFalse", []int{}},
	OpNada:         {"OpNada", []int{}},
	OpEqual:        {"OpEqual", []int{}},
	OpNotEqual:     {"OpNotEqual", []int{}},
	OpGreaterThan:  {"OpGreaterThan", []int{}},
	OpGreaterEqual: {"OpGreaterEqual", []int{}},
	OpMinus:        {"OpMinus", []int{}},
	OpNao:          {"OpNao", []int{}},
	OpMostra:       {"OpMostra", []int{}},
}

func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d sem definicao", op)
	}
	return def, nil
}

func Make(op Opcode, operands ...int) []byte {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}
	tamanho := 1
	for _, w := range def.OperandWidths {
		tamanho += w
	}
	instrucao := make([]byte, tamanho)
	instrucao[0] = byte(op)
	offset := 1
	for i, o := range operands {
		w := def.OperandWidths[i]
		switch w {
		case 2:
			binary.BigEndian.PutUint16(instrucao[offset:], uint16(o))
		}
		offset += w
	}
	return instrucao
}

func ReadOperands(def *Definition, ins Instructions) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0
	for i, w := range def.OperandWidths {
		switch w {
		case 2:
			operands[i] = int(ReadUint16(ins[offset:]))
		}
		offset += w
	}
	return operands, offset
}

func ReadUint16(ins Instructions) uint16 {
	return binary.BigEndian.Uint16(ins)
}

func (ins Instructions) String() string {
	var out bytes.Buffer
	i := 0
	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "ERRO: %s\n", err)
			i++
			continue
		}
		operands, read := ReadOperands(def, ins[i+1:])
		fmt.Fprintf(&out, "%04d %s\n", i, ins.fmtInstrucao(def, operands))
		i += 1 + read
	}
	return out.String()
}

func (ins Instructions) fmtInstrucao(def *Definition, operands []int) string {
	n := len(def.OperandWidths)
	if len(operands) != n {
		return fmt.Sprintf("ERRO: operandos %d != definidos %d", len(operands), n)
	}
	switch n {
	case 0:
		return def.Name
	case 1:
		return fmt.Sprintf("%s %d", def.Name, operands[0])
	}
	return fmt.Sprintf("ERRO: fmtInstrucao nao trata %d operandos", n)
}
