package vm

import (
	"fmt"
	"sync"

	"gambiarrascript/code"
	"gambiarrascript/object"
)

// celula guarda um local capturado por closure: a funcao dona e as closures
// criadas nela leem e escrevem a MESMA celula, entao uma atribuicao feita
// depois de criar a closure aparece nela (igual o tree-walker, onde a closure
// guarda o Environment da funcao). v == nil: o nome ainda nao foi botado.
//
// Nunca vaza pro programa: so mora em slot de local e em CompiledFunction.Free,
// e toda leitura pela linguagem passa pelo valor de dentro.
type celula struct{ v object.Object }

func (c *celula) Type() object.ObjectType { return "CELULA" }
func (c *celula) Inspect() string         { return fmt.Sprintf("celula(%v)", c.v) }

// celulasMu: com goroutine de usuario no ar (bora, handler de servidor) a
// celula pode ser lida por uma closure enquanto a dona escreve. Igual as
// globais, a trava so e paga depois que o programa liga a concorrencia.
var celulasMu sync.RWMutex

func (c *celula) pega() object.Object {
	if object.ConcorrenciaAtiva() {
		celulasMu.RLock()
		v := c.v
		celulasMu.RUnlock()
		return v
	}
	return c.v
}

func (c *celula) poe(v object.Object) {
	if object.ConcorrenciaAtiva() {
		celulasMu.Lock()
		c.v = v
		celulasMu.Unlock()
		return
	}
	c.v = v
}

// valorLivre devolve o valor de uma freevar: celula (o normal) ou valor cru
// (local que nao e celula — so o que o importa sem alias cria dentro de uma
// gambiarra).
func valorLivre(o object.Object) object.Object {
	if c, ok := o.(*celula); ok {
		return c.pega()
	}
	return o
}

// limpaLocais zera os slots de local (fora os params) de um frame novo: slot
// nil = nome ainda nao botado. Sem isso o slot herdava lixo da pilha (um
// `se_colar` que nao rodou deixava a leitura pegar o valor de outra chamada).
func (vm *VM) limpaLocais(bp int, cf *object.CompiledFunction) {
	if cf.NumLocals > cf.NumArgs {
		clear(vm.stack[bp+cf.NumArgs : bp+cf.NumLocals])
	}
}

// erroNaoBotou e o erro de ler um nome sem valor — mesma mensagem do
// tree-walker. `op` aponta pro operando com o indice da constante do nome.
func (vm *VM) erroNaoBotou(op []byte) VMError {
	nome := vm.constants[int(code.ReadUint16(op))].(*object.Texto).Value
	return VMError{err: &object.Erro{Message: fmt.Sprintf("cade o `%s`? voce nao botou isso ainda", nome), Kind: "runtime"}}
}
