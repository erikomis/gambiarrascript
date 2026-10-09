package compiler

import (
	"gambiarrascript/code"
	"gambiarrascript/object"
)

// MaxPilha calcula a profundidade maxima da pilha de operandos de um trecho de
// bytecode (corpo de gambiarra, programa principal ou corpo de modulo), contada
// a partir do topo dos locals do frame. E o max_stack da JVM: com ele a VM
// reserva a pilha UMA vez por frame (no OpCall e cia.) e o push nao precisa
// checar capacidade a cada empilhada.
//
// A conta simula o efeito de cada opcode na pilha seguindo TODOS os caminhos
// (jumps, cadeias OpGet*Ou, handler do arruma). Onde dois caminhos se juntam
// com profundidades diferentes vale a maior — o resultado e um teto, nunca
// menos do que a execucao real usa.
//
// Devolve 0 quando nao da pra garantir um teto (opcode sem efeito conhecido,
// jump pro meio de instrucao, pilha negativa ou laco que cresce a pilha a cada
// volta). 0 e o "desconhecido": a VM cai no caminho checado pra essa funcao.
// Bytecode valido sempre devolve >= 1.
func MaxPilha(ins code.Instructions, consts []object.Object) int {
	n := len(ins)
	if n == 0 {
		return 1
	}
	// inicio[pc] = true se pc e comeco de instrucao (jump so pode cair ai)
	inicio := make([]bool, n)
	for pc := 0; pc < n; {
		def, err := code.Lookup(ins[pc])
		if err != nil {
			return 0
		}
		inicio[pc] = true
		tam := 1
		for _, w := range def.OperandWidths {
			tam += w
		}
		if pc+tam > n {
			return 0
		}
		pc += tam
	}

	// teto de sanidade: cada instrucao empilha no maximo 1 a mais do que
	// tira, entao num caminho sem laco a pilha nao passa do numero de
	// instrucoes (que e <= n). Passou disso, e laco crescendo a pilha.
	teto := n + 2
	entrada := make([]int, n) // profundidade na entrada de cada pc (-1 = nao visitado)
	for i := range entrada {
		entrada[i] = -1
	}
	max := 1
	fila := []int{0}
	entrada[0] = 0
	// visita propaga a profundidade d pro pc alvo; false = estourou o teto
	visita := func(alvo, d int) bool {
		if alvo == n {
			return true // saiu pelo fim do bytecode (fim do programa)
		}
		if alvo < 0 || alvo > n || !inicio[alvo] || d > teto {
			return false
		}
		if d > entrada[alvo] {
			entrada[alvo] = d
			fila = append(fila, alvo)
		}
		return true
	}
	for len(fila) > 0 {
		pc := fila[len(fila)-1]
		fila = fila[:len(fila)-1]
		d := entrada[pc]
		op := code.Opcode(ins[pc])
		ef, ok := efeitoPilha(op, ins[pc+1:], consts)
		if !ok || d < ef.tira {
			return 0
		}
		depois := d - ef.tira + ef.poe
		if depois > max {
			max = depois
		}
		def, _ := code.Lookup(ins[pc])
		prox := pc + 1
		for _, w := range def.OperandWidths {
			prox += w
		}
		alvo := -1
		if ef.salta {
			alvo = int(code.ReadUint16(ins[pc+1:]))
		}
		switch {
		case ef.fim:
			// OpReturn/OpThrow/OpHalt: nada segue no mesmo frame
		case op == code.OpJump:
			if !visita(alvo, depois) {
				return 0
			}
		case op == code.OpTry:
			// o catch entra com a pilha zerada (a VM descarta os operandos
			// pendentes e volta pro topo dos locals) mais o erro empilhado
			if !visita(alvo, 1) || !visita(prox, depois) {
				return 0
			}
		case ef.salta && ef.poeSeSalta:
			// OpGet*Ou: achou valor -> empilha e pula; senao segue sem empilhar
			if depois+1 > max {
				max = depois + 1
			}
			if !visita(alvo, depois+1) || !visita(prox, depois) {
				return 0
			}
		case ef.salta && ef.poeSoSegue:
			// OpIterProx: acabou -> pula sem empilhar; senao segue com os valores
			if !visita(alvo, d-ef.tira) || !visita(prox, depois) {
				return 0
			}
		case ef.salta:
			if !visita(alvo, depois) || !visita(prox, depois) {
				return 0
			}
		default:
			if !visita(prox, depois) {
				return 0
			}
		}
	}
	return max
}

// efeito e o que uma instrucao faz com a pilha: tira `tira`, poe `poe`.
// salta = o primeiro operando e um endereco de jump; poeSeSalta = so empilha
// (1) no caminho do salto (cadeia OpGet*Ou); fim = nao tem sucessor no frame.
type efeito struct {
	tira, poe  int
	salta      bool
	poeSeSalta bool
	poeSoSegue bool // so empilha (poe) no caminho que segue; o salto so tira
	fim        bool
}

// efeitoPilha devolve o efeito do opcode. Os de aridade variavel leem o
// operando (ou a constante que ele aponta). Opcode que a VM nao executa (ou
// novo, ainda sem entrada aqui) devolve false — o teste
// TestEfeitoPilhaCobreTodoOpcode cobra que todo opcode definido tenha efeito.
func efeitoPilha(op code.Opcode, operandos []byte, consts []object.Object) (efeito, bool) {
	u8 := func(i int) int { return int(operandos[i]) }
	u16 := func(i int) int { return int(code.ReadUint16(operandos[i:])) }
	switch op {
	case code.OpConstant, code.OpTrue, code.OpFalse, code.OpNada,
		code.OpGetGlobal, code.OpGetLocal, code.OpGetFree, code.OpGetBuiltin,
		code.OpGetCelula, code.OpGetFreeCelula, code.OpImporta,
		code.OpGetLocalChk, code.OpGetGlobalChk, code.OpGetFreeChk, code.OpGetCelulaChk:
		return efeito{poe: 1}, true
	case code.OpPop, code.OpMostra, code.OpSetGlobal, code.OpSetLocal, code.OpSetCelula:
		return efeito{tira: 1}, true
	case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod, code.OpPow,
		code.OpBAnd, code.OpBOr, code.OpBXor, code.OpLShift, code.OpRShift,
		code.OpEqual, code.OpNotEqual, code.OpGreaterThan, code.OpGreaterEqual,
		code.OpMenor, code.OpMenorEqual,
		code.OpIndex, code.OpRange, code.OpIndexOuNada:
		return efeito{tira: 2, poe: 1}, true
	case code.OpMinus, code.OpNao, code.OpBNot, code.OpIsNada, code.OpBinConst:
		return efeito{tira: 1, poe: 1}, true
	case code.OpDup:
		return efeito{tira: 1, poe: 2}, true
	case code.OpCelula, code.OpTryEnd, code.OpLinha:
		return efeito{}, true
	case code.OpIndexSet:
		return efeito{tira: 3}, true
	case code.OpFatia:
		return efeito{tira: 3, poe: 1}, true
	case code.OpIterPar:
		return efeito{tira: 3, poe: 2}, true
	case code.OpIterSeq:
		// marca, iteravel -> orig, seq, tamanho
		return efeito{tira: 2, poe: 3}, true
	case code.OpIterMarca:
		return efeito{poe: 1}, true
	case code.OpIterFim:
		return efeito{tira: 1}, true
	case code.OpRelancaFecha:
		// so olha o topo (relancar e erro, sem sucessor nesse caminho)
		return efeito{}, true
	case code.OpIterProx:
		// [orig] seq it tam -> valor(es); no salto (acabou) nao empilha nada
		nomes := u8(2)
		return efeito{tira: nomes + 2, poe: nomes, salta: true, poeSoSegue: true}, true
	case code.OpRende:
		return efeito{tira: 1}, true
	case code.OpGerador:
		// na chamada vira retorno (o gerador cai no slot do chamador, igual o
		// OpReturn); o corpo segue daqui na sub-VM do gerador com a mesma
		// pilha — por isso conta como instrucao que nao mexe e segue
		return efeito{}, true
	case code.OpJump:
		return efeito{salta: true}, true
	case code.OpJumpIfFalse, code.OpJumpIfTrue:
		return efeito{tira: 1, salta: true}, true
	case code.OpTry:
		return efeito{salta: true}, true
	case code.OpGetLocalOu, code.OpGetGlobalOu, code.OpGetFreeOu, code.OpGetCelulaOu:
		return efeito{salta: true, poeSeSalta: true}, true
	case code.OpReturn, code.OpThrow:
		return efeito{tira: 1, fim: true}, true
	case code.OpReturnNada, code.OpHalt:
		return efeito{fim: true}, true
	case code.OpArray:
		return efeito{tira: u16(0), poe: 1}, true
	case code.OpHash:
		return efeito{tira: 2 * u16(0), poe: 1}, true
	case code.OpClosure:
		return efeito{tira: u8(2), poe: 1}, true
	case code.OpCall, code.OpBoraCall:
		// callee + argc args -> resultado. A abertura de metodo ligado (o
		// receiver entra na frente) reserva o proprio espaco; o frame
		// chamado reserva o dele.
		return efeito{tira: u8(0) + 1, poe: 1}, true
	case code.OpTailCall:
		// gambiarra: troca o frame (a reserva e a do chamado); builtin/metodo:
		// devolve o resultado e sai. Seguir pra proxima instrucao com o
		// resultado na pilha e um teto seguro pros dois casos.
		return efeito{tira: u8(0) + 1, poe: 1}, true
	case code.OpCallBuiltin:
		return efeito{tira: u8(2), poe: 1}, true
	case code.OpCallEspalha, code.OpBoraEspalha:
		// a mascara tem um char por argumento ESCRITO; o que as listas abrem
		// a mais o espalhaArgs reserva sozinho e some na chamada
		m, ok := constTexto(consts, u16(0))
		if !ok {
			return efeito{}, false
		}
		return efeito{tira: len(m) + 1, poe: 1}, true
	case code.OpTreta:
		d, ok := constAt(consts, u16(0)).(*object.DescTreta)
		if !ok {
			return efeito{}, false
		}
		k := 0
		for _, c := range d.Campos {
			if c.Embutida || c.TemPadrao {
				k++
			}
		}
		return efeito{tira: k, poe: 1}, true
	case code.OpCombinado:
		d, ok := constAt(consts, u16(0)).(*object.DescCombinado)
		if !ok {
			return efeito{}, false
		}
		return efeito{tira: len(d.Embutidos), poe: 1}, true
	case code.OpMetodo:
		return efeito{tira: 2}, true
	case code.OpInstancia:
		d, ok := constAt(consts, u16(0)).(*object.DescLiteral)
		if !ok {
			return efeito{}, false
		}
		return efeito{tira: d.N + 1, poe: 1}, true
	case code.OpCasa:
		d, ok := constAt(consts, u16(0)).(*object.DescPadrao)
		if !ok {
			return efeito{}, false
		}
		return efeito{tira: d.NValores + 1, poe: 1}, true
	case code.OpAmarrado, code.OpCardapio:
		return efeito{poe: 1}, true
	}
	// OpVaza/OpContinua: definidos mas nunca emitidos (a VM nao executa)
	return efeito{}, false
}

func constAt(consts []object.Object, i int) object.Object {
	if i < 0 || i >= len(consts) {
		return nil
	}
	return consts[i]
}

func constTexto(consts []object.Object, i int) (string, bool) {
	t, ok := constAt(consts, i).(*object.Texto)
	if !ok {
		return "", false
	}
	return t.Value, true
}
