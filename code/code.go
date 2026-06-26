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
	// --- fase 6b ---
	OpGetGlobal
	OpSetGlobal
	OpJump
	OpJumpIfFalse
	OpJumpIfTrue
	OpVaza
	OpContinua
	OpMenor
	OpMenorEqual
	// --- fase 6c: colecoes ---
	OpArray     // numElems (2 bytes): cria lista dos ultimos N do stack
	OpHash      // numPares (2 bytes): 2*N valores na pilha -> Dicionario
	OpIndex     // pop idx, pop container, push container[idx]
	OpIndexSet  // pop val, pop idx, pop container, atribui
	// --- fase 6d: funcoes ---
	OpClosure      // constIdx (2): cria closure apontando pra CompiledFunction + freevars
	OpCall          // argc (1): chama funcao na pilha
	OpReturn        // retorna valor (pop frame)
	OpReturnNada    // retorna nada
	OpGetLocal      // idx (1): push locals[bp+idx]
	OpSetLocal      // idx (1): pop -> locals[bp+idx]
	OpGetBuiltin    // idx (2): push builtin registrado
	OpCallBuiltin   // idx (2) + argc (1)
	OpGetFree       // idx (1): variavel capturada
	// --- fase 6e: erros ---
	OpThrow     // pop erro, unwinding
	OpTry       // catchAddr (2): registra handler em catchStack
	OpTryEnd    // desempilha handler atual
	// misc
	OpHalt // para execucao
)

type Definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*Definition{
	OpConstant:      {"OpConstant", []int{2}},
	OpPop:           {"OpPop", []int{}},
	OpAdd:           {"OpAdd", []int{}},
	OpSub:           {"OpSub", []int{}},
	OpMul:           {"OpMul", []int{}},
	OpDiv:           {"OpDiv", []int{}},
	OpMod:           {"OpMod", []int{}},
	OpTrue:          {"OpTrue", []int{}},
	OpFalse:         {"OpFalse", []int{}},
	OpNada:          {"OpNada", []int{}},
	OpEqual:         {"OpEqual", []int{}},
	OpNotEqual:      {"OpNotEqual", []int{}},
	OpGreaterThan:   {"OpGreaterThan", []int{}},
	OpGreaterEqual: {"OpGreaterEqual", []int{}},
	OpMinus:         {"OpMinus", []int{}},
	OpNao:           {"OpNao", []int{}},
	OpMostra:        {"OpMostra", []int{}},
	// fase 6b
	OpGetGlobal:     {"OpGetGlobal", []int{2}},
	OpSetGlobal:     {"OpSetGlobal", []int{2}},
	OpJump:          {"OpJump", []int{2}},
	OpJumpIfFalse:   {"OpJumpIfFalse", []int{2}},
	OpJumpIfTrue:    {"OpJumpIfTrue", []int{2}},
	OpVaza:          {"OpVaza", []int{}},
	OpContinua:      {"OpContinua", []int{}},
	OpMenor:         {"OpMenor", []int{}},
	OpMenorEqual:    {"OpMenorEqual", []int{}},
	// fase 6c
	OpArray:         {"OpArray", []int{2}},
	OpHash:          {"OpHash", []int{2}},
	OpIndex:         {"OpIndex", []int{}},
	OpIndexSet:      {"OpIndexSet", []int{}},
	// fase 6d
	OpClosure:       {"OpClosure", []int{2}},
	OpCall:          {"OpCall", []int{1}},
	OpReturn:        {"OpReturn", []int{}},
	OpReturnNada:    {"OpReturnNada", []int{}},
	OpGetLocal:      {"OpGetLocal", []int{1}},
	OpSetLocal:      {"OpSetLocal", []int{1}},
	OpGetBuiltin:    {"OpGetBuiltin", []int{2}},
	OpCallBuiltin:   {"OpCallBuiltin", []int{3}},
	OpGetFree:       {"OpGetFree", []int{1}},
	// fase 6e
	OpThrow:         {"OpThrow", []int{}},
	OpTry:           {"OpTry", []int{2}},
	OpTryEnd:        {"OpTryEnd", []int{}},
	// misc
	OpHalt:          {"OpHalt", []int{}},
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
		case 1:
			instrucao[offset] = byte(o)
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
		case 1:
			operands[i] = int(ReadUint8(ins[offset:]))
		case 2:
			operands[i] = int(ReadUint16(ins[offset:]))
		}
		offset += w
	}
	return operands, offset
}

func ReadUint8(ins Instructions) uint8  { return ins[0] }
func ReadUint16(ins Instructions) uint16 { return binary.BigEndian.Uint16(ins) }

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
	case 2:
		return fmt.Sprintf("%s %d %d", def.Name, operands[0], operands[1])
	}
	return fmt.Sprintf("ERRO: fmtInstrucao nao trata %d operandos", n)
}
