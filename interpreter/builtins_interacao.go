package interpreter

import (
	"bufio"
	"io"
	"os"
	"strings"

	"gambiarrascript/object"
)

// builtinMapeia aplica func em cada elemento da lista e devolve uma nova lista.
func (i *Interpreter) builtinMapeia(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("mapeia() quer 2 argumentos (lista, gambiarra), veio %d", len(args))
	}
	l, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("mapeia() espera uma lista, veio %s", args[0].Type())
	}
	fn := args[1]
	out := make([]object.Object, 0, len(l.Elements))
	for _, e := range l.Elements {
		res := i.applyFunction(fn, []object.Object{e}, 0, "<mapeia>")
		if isError(res) {
			return res
		}
		out = append(out, res)
	}
	return &object.Lista{Elements: out}
}

// builtinFiltra devolve uma nova lista so com os elementos em que func devolveu
// verdadeiro.
func (i *Interpreter) builtinFiltra(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("filtra() quer 2 argumentos (lista, gambiarra), veio %d", len(args))
	}
	l, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("filtra() espera uma lista, veio %s", args[0].Type())
	}
	fn := args[1]
	out := make([]object.Object, 0)
	for _, e := range l.Elements {
		res := i.applyFunction(fn, []object.Object{e}, 0, "<filtra>")
		if isError(res) {
			return res
		}
		if isTruthy(res) {
			out = append(out, e)
		}
	}
	return &object.Lista{Elements: out}
}

// builtinPergunta mostra um prompt e le uma linha do stdin.
func (i *Interpreter) builtinPergunta(args []object.Object) object.Object {
	if len(args) > 1 {
		return erroBuiltin("pergunta() quer 0 ou 1 argumento (prompt), veio %d", len(args))
	}
	if len(args) == 1 {
		prompt, ok := args[0].(*object.Texto)
		if !ok {
			return erroBuiltin("pergunta() espera texto no prompt, veio %s", args[0].Type())
		}
		io.WriteString(i.out, prompt.Value)
	}
	linha, err := i.bufferStdin().ReadString('\n')
	if err != nil && err != io.EOF {
		return erroBuiltin("pergunta(): nao consegui ler entrada: %v", err)
	}
	return &object.Texto{Value: strings.TrimRight(linha, "\r\n")}
}

// builtinArgumentos devolve os argumentos de linha de comando passados apos o
// arquivo do script, como uma lista de textos.
func (i *Interpreter) builtinArgumentos(args []object.Object) object.Object {
	if len(args) != 0 {
		return erroBuiltin("argumentos() nao quer argumento, veio %d", len(args))
	}
	elems := make([]object.Object, len(i.argumentos))
	for idx, a := range i.argumentos {
		elems[idx] = &object.Texto{Value: a}
	}
	return &object.Lista{Elements: elems}
}

// bufferStdin cria um bufio.Reader preguiçoso sobre o stdin do interpretador,
// mantendo o mesmo buffer entre chamadas pra nao perder dados.
func (i *Interpreter) bufferStdin() *bufio.Reader {
	if i.inBuf == nil {
		r := i.in
		if r == nil {
			r = os.Stdin
		}
		i.inBuf = bufio.NewReader(r)
	}
	return i.inBuf
}