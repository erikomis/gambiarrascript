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
	// bitwise
	OpBAnd
	OpBOr
	OpBXor
	OpBNot
	OpLShift
	OpRShift
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
	OpArray       // numElems (2 bytes): cria lista dos ultimos N do stack
	OpHash        // numPares (2 bytes): 2*N valores na pilha -> Dicionario
	OpIndex       // pop idx, pop container, push container[idx]
	OpIndexSet    // pop val, pop idx, pop container, atribui
	OpRange       // pop fim, pop inicio, push lista [inicio..fim] inclusive
	OpIterSeq     // pop iteravel; push orig, seq (lista dos elementos/chaves, ou o gerador) e tamanho
	OpIndexOuNada // igual OpIndex, mas indice/chave ausente vira nada (desestruturacao)
	// --- fase 6d: funcoes ---
	OpClosure     // constIdx (2): cria closure apontando pra CompiledFunction + freevars
	OpCall        // argc (1): chama funcao na pilha
	OpReturn      // retorna valor (pop frame)
	OpReturnNada  // retorna nada
	OpGetLocal    // idx (1): push locals[bp+idx]
	OpSetLocal    // idx (1): pop -> locals[bp+idx]
	OpGetBuiltin  // idx (2): push builtin registrado
	OpCallBuiltin // idx (2) + argc (1)
	OpGetFree     // idx (1): variavel capturada
	// --- fase 6e: erros ---
	OpThrow  // pop erro, unwinding
	OpTry    // catchAddr (2): registra handler em catchStack
	OpTryEnd // desempilha handler atual
	// --- fase 6f: concorrencia ---
	OpBoraCall // argc (1): dispara chamada em goroutine, push Futuro
	// misc
	OpFatia    // pop fim, pop inicio, pop left; push fatia left[inicio:fim]
	OpIterPar  // pop __it, pop __seq, pop __orig; push (key, value) pra pra_cada 2-nomes
	OpDup      // duplica topo da pilha
	OpIsNada   // pop x; push (x == nada)
	OpTailCall // argc (1): tail call — reusa o frame atual (recursao em cauda)
	// OpBinConst funde `OpConstant K` + operacao binaria numa instrucao so:
	// aplica a operacao entre o topo da pilha e a constante, no lugar. Cobre o
	// caso mais comum de aritmetica (`i + 1`, `i * 2`, `i < 200000`) sem o
	// push/pop da constante nem o segundo dispatch.
	OpBinConst // constIdx (2) + opcode da operacao (1)
	OpPow      // pop exp, pop base, push base ** exp
	// chamada com `...lista`: o operando e o indice de uma constante texto
	// tipo "010" (1 = arg espalhado; o tamanho e o argc). Ficam fora do
	// OpCall pra nao pesar no caminho quente das chamadas normais.
	OpCallEspalha // maskIdx (2): igual OpCall, abrindo as listas marcadas
	OpBoraEspalha // maskIdx (2): igual OpBoraCall, abrindo as listas marcadas
	// importa: roda o modulo (descritor object.Modulo na constante) uma vez so
	// por processo e empilha o namespace dele (dicionario). O segundo operando
	// e a constante texto com o arquivo onde o importa esta escrito (cadeia
	// pra detectar import circular).
	OpImporta // moduloIdx (2) + atualIdx (2)
	// --- escopo de funcao (estilo Python, igual o tree-walker) ---
	// Leitura de nome que pode ainda nao ter sido botado: se o slot tem
	// valor, empilha e pula pro alvo (fim da cadeia); se nao, segue pra
	// proxima instrucao, que le o escopo de fora (ou um OpGet*Chk). O alvo vem
	// PRIMEIRO pra reusar o backpatch dos jumps.
	OpGetLocalOu  // alvo (2) + idx (1)
	OpGetGlobalOu // alvo (2) + idx (2)
	OpGetFreeOu   // alvo (2) + idx (1): freevar (celula)
	OpGetCelulaOu // alvo (2) + idx (1): local capturado por closure (celula)
	// local capturado por closure mora numa celula (a closure enxerga a
	// variavel, nao uma copia do valor).
	OpCelula        // idx (1): embrulha o slot numa celula nova (prologo)
	OpGetCelula     // idx (1): le o valor da celula do slot
	OpSetCelula     // idx (1): pop -> valor da celula do slot
	OpGetFreeCelula // idx (1): empilha a CELULA da freevar (repasse pra closure de dentro)
	// ultimo lugar da cadeia: sem valor -> erro "cade o `nome`? voce nao
	// botou isso ainda" (nome na constante). Caminho quente de toda leitura
	// de variavel: um teste de nil a mais que o OpGet* cru.
	OpGetLocalChk  // idx (1) + nomeIdx (2)
	OpGetGlobalChk // idx (2) + nomeIdx (2)
	OpGetFreeChk   // idx (1) + nomeIdx (2)
	OpGetCelulaChk // idx (1) + nomeIdx (2)
	OpHalt         // para execucao
	// --- POO (Tier 8) ---
	// OpTreta: descritor object.DescTreta na constante; tira da pilha um valor
	// por campo com puxadinho (a treta embutida) ou com padrao (constante ou
	// thunk), na ordem dos campos, e empilha a *object.Treta.
	OpTreta // descIdx (2)
	// OpCombinado: descritor object.DescCombinado; tira os combinados
	// embutidos e empilha o *object.Combinado.
	OpCombinado // descIdx (2)
	// OpMetodo: pop closure, pop treta; pendura o metodo (nome na constante).
	OpMetodo // nomeIdx (2)
	// OpInstancia: descritor object.DescLiteral; pop N valores, pop treta;
	// empilha a instancia (`Ponto{x: 1}`).
	OpInstancia // descIdx (2)
	// --- instrumentacao (gancho de linha; veja object/gancho.go) ---
	// OpLinha: chama o gancho de linha da VM com o sitio (arquivo:linha) do
	// statement que comeca aqui. So existe em bytecode compilado com
	// compiler.Instrumentar: o bytecode normal nunca tem OpLinha.
	OpLinha // sitioIdx (2)
	// --- pattern matching e cardapio ---
	// OpCasa: descritor object.DescPadrao; pop NValores valores (os do
	// padrao, em pre-ordem), pop o subject; empilha deu_bom/deu_ruim. Se casou,
	// guarda o que amarrar na VM (lido logo depois pelo OpAmarrado).
	OpCasa // descIdx (2)
	// OpAmarrado: empilha o valor amarrado no slot pelo ultimo OpCasa.
	OpAmarrado // slot (1)
	// OpCardapio: descritor object.DescCardapio; empilha o *object.Cardapio.
	OpCardapio // descIdx (2)
	// --- geradores (`rende`) e protocolo de iteracao ---
	// OpGerador: primeira instrucao do corpo de um gerador (depois do
	// prologo de celulas e padroes): guarda o frame recem-aberto num gerador
	// (sub-VM propria) e retorna o gerador pra quem chamou, sem rodar o corpo.
	OpGerador
	// OpRende: pop valor; entrega pra quem pediu e pausa o gerador (o frame
	// fica parado na instrucao seguinte ate o proximo pedido).
	OpRende
	// OpIterProx: uma volta do pra_cada. Pilha: [orig] seq it tam (orig so
	// com 2 nomes). Acabou: pula pro alvo sem empilhar nada; senao empilha o
	// valor (1 nome) ou indice/chave + valor (2 nomes). seq e lista (tam e o
	// tamanho no comeco do laco) ou gerador (pede o proximo valor).
	OpIterProx // alvo (2) + nomes (1)
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
	OpBAnd:         {"OpBAnd", []int{}},
	OpBOr:          {"OpBOr", []int{}},
	OpBXor:         {"OpBXor", []int{}},
	OpBNot:         {"OpBNot", []int{}},
	OpLShift:       {"OpLShift", []int{}},
	OpRShift:       {"OpRShift", []int{}},
	// fase 6b
	OpGetGlobal:   {"OpGetGlobal", []int{2}},
	OpSetGlobal:   {"OpSetGlobal", []int{2}},
	OpJump:        {"OpJump", []int{2}},
	OpJumpIfFalse: {"OpJumpIfFalse", []int{2}},
	OpJumpIfTrue:  {"OpJumpIfTrue", []int{2}},
	OpVaza:        {"OpVaza", []int{}},
	OpContinua:    {"OpContinua", []int{}},
	OpMenor:       {"OpMenor", []int{}},
	OpMenorEqual:  {"OpMenorEqual", []int{}},
	// fase 6c
	OpArray:       {"OpArray", []int{2}},
	OpHash:        {"OpHash", []int{2}},
	OpIndex:       {"OpIndex", []int{}},
	OpIndexSet:    {"OpIndexSet", []int{}},
	OpRange:       {"OpRange", []int{}},
	OpIterSeq:     {"OpIterSeq", []int{}},
	OpIndexOuNada: {"OpIndexOuNada", []int{}},
	// fase 6d
	OpClosure:     {"OpClosure", []int{2, 1}}, // constIdx (2), numFree (1)
	OpCall:        {"OpCall", []int{1}},
	OpTailCall:    {"OpTailCall", []int{1}},
	OpBinConst:    {"OpBinConst", []int{2, 1}},
	OpReturn:      {"OpReturn", []int{}},
	OpReturnNada:  {"OpReturnNada", []int{}},
	OpGetLocal:    {"OpGetLocal", []int{1}},
	OpSetLocal:    {"OpSetLocal", []int{1}},
	OpGetBuiltin:  {"OpGetBuiltin", []int{2}},
	OpCallBuiltin: {"OpCallBuiltin", []int{2, 1}}, // idx (uint16) + argc (uint8)
	OpGetFree:     {"OpGetFree", []int{1}},
	// fase 6e
	OpThrow:  {"OpThrow", []int{}},
	OpTry:    {"OpTry", []int{2}},
	OpTryEnd: {"OpTryEnd", []int{}},
	// fase 6f
	OpBoraCall: {"OpBoraCall", []int{1}},
	// misc
	OpFatia:       {"OpFatia", []int{}},
	OpIterPar:     {"OpIterPar", []int{}},
	OpDup:         {"OpDup", []int{}},
	OpIsNada:      {"OpIsNada", []int{}},
	OpPow:         {"OpPow", []int{}},
	OpCallEspalha: {"OpCallEspalha", []int{2}},
	OpBoraEspalha: {"OpBoraEspalha", []int{2}},
	OpImporta:     {"OpImporta", []int{2, 2}},
	// escopo de funcao
	OpGetLocalOu:    {"OpGetLocalOu", []int{2, 1}},
	OpGetGlobalOu:   {"OpGetGlobalOu", []int{2, 2}},
	OpGetFreeOu:     {"OpGetFreeOu", []int{2, 1}},
	OpGetCelulaOu:   {"OpGetCelulaOu", []int{2, 1}},
	OpCelula:        {"OpCelula", []int{1}},
	OpGetCelula:     {"OpGetCelula", []int{1}},
	OpSetCelula:     {"OpSetCelula", []int{1}},
	OpGetFreeCelula: {"OpGetFreeCelula", []int{1}},
	OpGetLocalChk:   {"OpGetLocalChk", []int{1, 2}},
	OpGetGlobalChk:  {"OpGetGlobalChk", []int{2, 2}},
	OpGetFreeChk:    {"OpGetFreeChk", []int{1, 2}},
	OpGetCelulaChk:  {"OpGetCelulaChk", []int{1, 2}},
	OpHalt:          {"OpHalt", []int{}},
	// POO
	OpTreta:     {"OpTreta", []int{2}},
	OpCombinado: {"OpCombinado", []int{2}},
	OpMetodo:    {"OpMetodo", []int{2}},
	OpInstancia: {"OpInstancia", []int{2}},
	// instrumentacao
	OpLinha: {"OpLinha", []int{2}},
	// pattern matching e cardapio
	OpCasa:     {"OpCasa", []int{2}},
	OpAmarrado: {"OpAmarrado", []int{1}},
	OpCardapio: {"OpCardapio", []int{2}},
	// geradores e iteracao
	OpGerador:  {"OpGerador", []int{}},
	OpRende:    {"OpRende", []int{}},
	OpIterProx: {"OpIterProx", []int{2, 1}},
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

func ReadUint8(ins Instructions) uint8   { return ins[0] }
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
		// OpBinConst carrega um OPCODE no segundo operando: mostra o nome dele
		// em vez do numero cru, senao o disasm vira adivinhacao.
		if def.Name == "OpBinConst" {
			if sub, err := Lookup(byte(operands[1])); err == nil {
				return fmt.Sprintf("%s %d %s", def.Name, operands[0], sub.Name)
			}
		}
		return fmt.Sprintf("%s %d %d", def.Name, operands[0], operands[1])
	}
	return fmt.Sprintf("ERRO: fmtInstrucao nao trata %d operandos", n)
}
