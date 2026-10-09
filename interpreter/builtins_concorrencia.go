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
	if con, ok := object.ConexaoDe(args[0]); ok {
		if err := con.Envia(args[1]); err != nil {
			return erroBuiltinKind(KindRede, "envia(): %s", err)
		}
		return NADA
	}
	cano, ok := args[0].(*object.Cano)
	if !ok {
		return erroBuiltin("envia() espera um cano ou conexao no 1o arg, veio %s", args[0].Type())
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
	if con, ok := object.ConexaoDe(args[0]); ok {
		v, err := con.Recebe()
		if err != nil {
			return erroBuiltinKind(KindRede, "recebe(): %s", err)
		}
		if v == nil {
			return NADA
		}
		return v
	}
	cano, ok := args[0].(*object.Cano)
	if !ok {
		return erroBuiltin("recebe() espera um cano ou conexao, veio %s", args[0].Type())
	}
	v, aberto := <-cano.Ch
	if !aberto {
		return NADA
	}
	return v
}

// builtinFecha fecha um recurso: Cano (channel), conexao de banco (*Nativo
// embrulhando *conexaoBD) ou gerador (roda os finalmente pendentes). Idempotente. O `fecha` do banco ja existia
// em builtins.go antes do `fecha` de cano ser adicionado em builtinsInstancia;
// pra manter um nome so, este builtin aceita os dois.
func (i *Interpreter) builtinFecha(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("fecha() quer 1 argumento (cano, conexao ou gerador), veio %d", len(args))
	}
	switch v := args[0].(type) {
	case *object.Gerador:
		// roda os finalmente pendentes do corpo pausado (gerador.go)
		if falha := v.Fecha(); falha != nil {
			return falha
		}
		return NADA
	case *object.Cano:
		v.Fechar()
		return NADA
	case *object.Nativo:
		if con, ok := v.Valor.(object.Conexao); ok {
			if err := con.Fecha(); err != nil {
				return erroBuiltinKind(KindRede, "fecha(): %s", err)
			}
			return NADA
		}
		// delega pro builtin global de banco (mesmo nome) pra fechar a conexao.
		return builtinFecha([]object.Object{v})
	}
	return erroBuiltin("fecha() espera um cano, conexao ou gerador, veio %s", args[0].Type())
}

// builtinTrava cria uma trava (lock) pra usar com com_trava.
//
//	bota t = trava()
//	com_trava(t, gambiarra() { bota d["n"] = d["n"] + 1 })
//
// Cada operacao numa lista/dicionario/conjunto ja e atomica sozinha; a trava
// e pra quando voce precisa que VARIAS operacoes (ler, calcular, escrever)
// rodem juntas sem outra goroutine se meter no meio.
func builtinTrava(args []object.Object) object.Object {
	if len(args) != 0 {
		return erroBuiltin("trava() nao quer argumento nenhum, veio %d", len(args))
	}
	return object.NovaTrava()
}

// builtinComTrava: com_trava(trava, gambiarra) roda a gambiarra (sem
// argumentos) segurando a trava e devolve o que ela devolver. Solta a trava
// SEMPRE — inclusive quando a gambiarra da erro, que sobe igualzinho pra quem
// chamou (da pra pegar com arruma/quebrou). A trava NAO e reentrante: chamar
// com_trava na mesma trava de dentro da propria gambiarra e erro (senao
// travava pra sempre). Outra goroutine (bora, handler) que pedir a mesma trava
// so espera a vez.
func (i *Interpreter) builtinComTrava(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("com_trava() quer 2 argumentos (trava, gambiarra), veio %d", len(args))
	}
	t, ok := args[0].(*object.Trava)
	if !ok {
		return erroBuiltin("com_trava() espera uma trava no 1o arg (cria com trava()), veio %s", object.NomeTipo(args[0]))
	}
	fn := args[1]
	if !ehChamavel(fn) {
		return erroBuiltin("com_trava() espera uma gambiarra no 2o arg, veio %s", object.NomeTipo(fn))
	}
	// corpo de gerador e o fluxo de quem consome (gerador.go)
	reentrou := travaComQuemConsome(t)
	var res object.Object
	if !reentrou {
		res, reentrou = t.Segura(func() object.Object {
			return i.applyFunction(fn, nil, 0, "<com_trava>")
		})
	}
	if reentrou {
		return erroBuiltin("com_trava(): essa trava ja ta com voce — trava nao e reentrante, pedir de novo de dentro do com_trava ia travar pra sempre")
	}
	return res
}
