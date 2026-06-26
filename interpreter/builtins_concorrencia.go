package interpreter

import (
	"gambiarrascript/object"
)

// builtinCano cria um novo Cano (channel).
//
//	can()              → cano sincrono (capacidade 0)
//	cano(capacidade)   → cano bufferizado
//
// Cap < 0 vira 0. Use envia/recebe pra trocar mensagens entre goroutines
// disparadas por `bora`.
func (i *Interpreter) builtinCano(args []object.Object) object.Object {
	cap := 0
	if len(args) > 1 {
		return erroBuiltin("cano() quer 0 ou 1 argumento (capacidade), veio %d", len(args))
	}
	if len(args) == 1 {
		n, ok := args[0].(*object.Numero)
		if !ok {
			return erroBuiltin("cano() espera numero (capacidade), veio %s", args[0].Type())
		}
		cap = int(n.Value)
	}
	return object.NovoCano(cap)
}

// builtinEnvia manda um valor pra dentro do cano.
// Bloqueia se o cano estiver cheio (capacidade limitada) ou se nao houver
// receptor (cano sincrono). Devolve nada quando o envio completa. Se o cano
// foi fechado, devolve *Erro (send on closed channel).
func (i *Interpreter) builtinEnvia(args []object.Object) (resultado object.Object) {
	if len(args) != 2 {
		return erroBuiltin("envia() quer 2 argumentos (cano, valor), veio %d", len(args))
	}
	cano, ok := args[0].(*object.Cano)
	if !ok {
		return erroBuiltin("envia() espera um cano no 1o arg, veio %s", args[0].Type())
	}
	resultado = NADA
	defer func() {
		if r := recover(); r != nil {
			resultado = erroBuiltinKind(KindRuntime, "envia(): cano fechado, nao da pra mandar mais nada")
		}
	}()
	cano.Ch <- args[1]
	return resultado
}

// builtinRecebe pega o proximo valor do cano. Bloqueia ate ter algo (ou ate o
// cano ser fechado). Se fechado e vazio, devolve nada.
func (i *Interpreter) builtinRecebe(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("recebe() quer 1 argumento (cano), veio %d", len(args))
	}
	cano, ok := args[0].(*object.Cano)
	if !ok {
		return erroBuiltin("recebe() espera um cano, veio %s", args[0].Type())
	}
	v, aberto := <-cano.Ch
	if !aberto {
		return NADA
	}
	return v
}

// builtinFecha fecha um recurso pode ser Cano (channel)ou conexao de banco
// (*Nativo embrulhando *conexaoBD). Idempotente. O `fecha` do banco ja existia
// em builtins.go antes do `fecha` de cano ser adicionado em builtinsInstancia;
// pra manter um nome so, este builtin aceita os dois.
func (i *Interpreter) builtinFecha(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("fecha() quer 1 argumento (cano ou conexao), veio %d", len(args))
	}
	switch v := args[0].(type) {
	case *object.Cano:
		v.Fechar()
		return NADA
	case *object.Nativo:
		// delega pro builtin global de banco (mesmo nome) pra fechar a conexao.
		return builtinFecha([]object.Object{v})
	}
	return erroBuiltin("fecha() espera um cano ou conexao, veio %s", args[0].Type())
}