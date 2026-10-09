package object

import (
	"sync/atomic"
	"unsafe"
)

// Concatenacao amortizada de texto.
//
// `s += pedaco` num laco era O(n²): cada volta copiava o texto inteiro pra um
// string novo do Go, e com o texto crescendo o GC rodava a cada punhado de
// voltas (100 mil voltas de 1 caractere: ~0,7s contra 0,04s do Python). O
// truque e o mesmo do append de slice do Go: o texto grande guarda um buffer
// com folga, e quem concatena NO FIM desse buffer escreve no espaco livre em
// vez de copiar tudo.
//
// Por que e seguro: os bytes de [0, usado) de um buffer nunca mudam depois de
// escritos — um *Texto aponta pra um prefixo do buffer e so le ate o tamanho
// dele. Escrever depois de `usado` nao muda nenhum texto que ja existe. O
// `usado` e reservado com CompareAndSwap: so quem tem o texto que termina
// exatamente no fim do buffer consegue esticar, e so um por vez (dois fluxos
// concatenando no mesmo texto — `bora`, handler do `escuta` — um ganha a
// reserva, o outro copia pra um buffer novo).

// textoMinBuffer: abaixo disso concatena do jeito simples (texto pequeno nao
// ganha nada com folga e pagaria ate 2x de memoria).
const textoMinBuffer = 256

type bufTexto struct {
	dados []byte // len == cap: o espaco inteiro
	usado atomic.Int64
}

// Concatena devolve o texto de esq + dir (o `+` com algum operando texto).
// Usado pelos dois engines, entao o resultado e byte a byte o mesmo de
// esq.Inspect() + dir.Inspect().
func Concatena(esq, dir Object) *Texto {
	d := dir.Inspect()
	t, ok := esq.(*Texto)
	if !ok {
		return &Texto{Value: esq.Inspect() + d}
	}
	n := len(t.Value)
	total := n + len(d)
	if total < textoMinBuffer {
		return &Texto{Value: t.Value + d}
	}
	// estica no lugar: o texto termina no fim do que foi escrito e cabe
	if b := t.buf; b != nil && total <= len(b.dados) && b.usado.CompareAndSwap(int64(n), int64(total)) {
		copy(b.dados[n:total], d)
		return &Texto{Value: unsafe.String(&b.dados[0], total), buf: b}
	}
	// buffer novo com o dobro, pra proxima volta caber
	b := &bufTexto{dados: make([]byte, total*2)}
	copy(b.dados, t.Value)
	copy(b.dados[n:], d)
	b.usado.Store(int64(total))
	return &Texto{Value: unsafe.String(&b.dados[0], total), buf: b}
}
