package interpreter

import (
	"bufio"
	"io"
	"os"
	"sync"

	"gambiarrascript/object"
)

// builtinLeTudo devolve todo o stdin (ate EOF), sem parar na primeira linha.
// Permite `cat arq | gs roda prog.gs` lendo o documento inteiro.
func (i *Interpreter) builtinLeTudo(args []object.Object) object.Object {
	if len(args) != 0 {
		return erroBuiltin("le_tudo() nao quer argumento, veio %d", len(args))
	}
	bs, err := io.ReadAll(i.bufferStdin())
	if err != nil && err != io.EOF {
		return erroBuiltinKind(KindIO, "le_tudo(): nao consegui ler stdin: %v", err)
	}
	return &object.Texto{Value: string(bs)}
}

// builtinLeLinhas le todo o stdin e devolve uma Lista de Texto, uma por linha,
// sem o '\n' final. Pra iterar com `pra_cada linha em le_linhas()`.
func (i *Interpreter) builtinLeLinhas(args []object.Object) object.Object {
	if len(args) != 0 {
		return erroBuiltin("le_linhas() nao quer argumento, veio %d", len(args))
	}
	scanner := bufio.NewScanner(i.bufferStdin())
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024) // suporta linhas ate 16MB
	var elems []object.Object
	for scanner.Scan() {
		elems = append(elems, &object.Texto{Value: scanner.Text()})
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return erroBuiltinKind(KindIO, "le_linhas(): nao consegui ler stdin: %v", err)
	}
	if elems == nil {
		elems = []object.Object{}
	}
	return &object.Lista{Elements: elems}
}

// builtinEscreve escreve texto no stdout (sem quebra de linha automática).
// Diferente de mostra — que sempre poe \n — escreve e crú, pra composicao de
// pipes filtros. Eh uma runtime: precisa do i.out.
func (i *Interpreter) builtinEscreve(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("escreve() quer 1 argumento (texto), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		t = &object.Texto{Value: args[0].Inspect()}
	}
	if _, err := io.WriteString(i.out, t.Value); err != nil {
		return erroBuiltinKind(KindIO, "escreve(): %v", err)
	}
	return NADA
}

// builtinEscreveErro escreve no stderr (saida de diagnostico). Pra scripts de
// terminal que querem separar resultado de mensagens.
func (i *Interpreter) builtinEscreveErro(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("escreve_erro() quer 1 argumento (texto), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		t = &object.Texto{Value: args[0].Inspect()}
	}
	w := i.erroOut
	if w == nil {
		w = os.Stderr
	}
	if _, err := io.WriteString(w, t.Value); err != nil {
		return erroBuiltinKind(KindIO, "escreve_erro(): %v", err)
	}
	return NADA
}

// builtinEnv devolve o valor de uma variavel de ambiente. Sem arg → nada.
func (i *Interpreter) builtinEnv(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("env() quer 1 argumento (nome), veio %d", len(args))
	}
	nome, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("env() espera texto (nome), veio %s", args[0].Type())
	}
	v, existe := os.LookupEnv(nome.Value)
	if !existe {
		return NADA
	}
	return &object.Texto{Value: v}
}

// builtinParalelo aplica uma gambiarra a cada elemento da lista em paralelo
// (uma goroutine por elemento) e devolve uma nova lista na ordem original.
// Isolamento: cada chamada ganha seu proprio Environment filho — nada de
// compartilhar estado mutavel entre goroutines (se o handler mexer em algo
// compartilhado, e responsabilidade dele; aqui so garantimos o Application).
//
// Limite: 256 goroutines simultaneas pra nao estourar转子 em listas grandes.
func (i *Interpreter) builtinParalelo(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("paralelo() quer 2 argumentos (lista, gambiarra), veio %d", len(args))
	}
	l, ok := args[0].(*object.Lista)
	if !ok {
		return erroBuiltin("paralelo() espera uma lista, veio %s", args[0].Type())
	}
	fn := args[1]

	n := len(l.Elements)
	if n == 0 {
		return &object.Lista{Elements: []object.Object{}}
	}

	const limiteGoroutines = 256
	janelas := (n + limiteGoroutines - 1) / limiteGoroutines
	out := make([]object.Object, n)

	for ini := 0; ini < n; ini += limiteGoroutines {
		fim := ini + limiteGoroutines
		if fim > n {
			fim = n
		}
		var wg sync.WaitGroup
		for idx := ini; idx < fim; idx++ {
			wg.Add(1)
			go func(pos int, elem object.Object) {
				defer wg.Done()
				// applyFunction e seguro de chamar concorrentemente? Hoje SIM:
				// o Environment raiz agora e thread-safe (RWMutex) e cada
				// chamada cria seu proprio escopo filho. Rotas HTTP e paralelo
				// dispõem do mesmo contrato.
				res := i.applyFunction(fn, []object.Object{elem}, 0, "<paralelo>")
				out[pos] = res
			}(idx, l.Elements[idx])
		}
		wg.Wait()
		_ = janelas
	}

	// Se qualquer goroutine devolveu Erro, propaga o primeiro encontrado.
	for _, r := range out {
		if isError(r) {
			return r
		}
	}
	return &object.Lista{Elements: out}
}

// builtinEspera tem dois papeis, desambiguados por arity:
//   - espera(futuro)            → 1 arg: bloqueia ate o Futuro resolver e devolve o valor.
//     Se for uma lista de Futuros, devolve lista de valores.
//   - espera(recebido, esperado) → 2 args: assert de teste (gs testa).
func (i *Interpreter) builtinEspera(args []object.Object) object.Object {
	if len(args) == 1 {
		return i.esperaFuturo(args[0])
	}
	if len(args) != 2 {
		return erroBuiltin("espera() quer 1 arg (futuro) ou 2 (assert), veio %d", len(args))
	}
	return i.esperaAssert(args[0], args[1])
}

// esperaFuturo bloqueia ate o Futuro (ou lista de Futuros) resolver e devolve
// o valor (ou lista de valores). Erros devolvidos pelo Future propagam como
// *Erro normal.
func (i *Interpreter) esperaFuturo(arg object.Object) object.Object {
	// caso unico: 1 futuro
	if f, ok := arg.(*object.Futuro); ok {
		return f.Aguarda()
	}
	// caso lista: espera todos em paralelo
	if l, ok := arg.(*object.Lista); ok {
		out := make([]object.Object, len(l.Elements))
		var wg sync.WaitGroup
		for idx, el := range l.Elements {
			f, ok := el.(*object.Futuro)
			if !ok {
				return erroBuiltin("espera(lista): elemento %d nao e futuro, e %s", idx, el.Type())
			}
			wg.Add(1)
			go func(pos int, fu *object.Futuro) {
				defer wg.Done()
				out[pos] = fu.Aguarda()
			}(idx, f)
		}
		wg.Wait()
		return &object.Lista{Elements: out}
	}
	return erroBuiltin("espera(futuro) espera um Futuro ou lista de Futuros, veio %s", arg.Type())
}

// esperaAssert e o assert de teste (loop antigo). Mantido intacto.
func (i *Interpreter) esperaAssert(recebido, esperado object.Object) object.Object {
	i.totalEspera++
	if iguais(recebido, esperado) {
		i.totalEsperaOk++
		return NADA
	}
	w := i.erroOut
	if w == nil {
		w = os.Stderr
	}
	io.WriteString(w, "FALHA: espera("+recebido.Inspect()+", "+esperado.Inspect()+") — valores differentes\n")
	return NADA
}

// builtinAfirma asserta verdade. Escreve no stderr se falhar. Equivalente a
// `espera(cond, deu_bom)` mas com mensagem customizada.
func (i *Interpreter) builtinAfirma(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("afirma() quer 1 ou 2 argumentos (cond, [msg]), veio %d", len(args))
	}
	i.totalEspera++
	if isTruthy(args[0]) {
		i.totalEsperaOk++
		return NADA
	}
	w := i.erroOut
	if w == nil {
		w = os.Stderr
	}
	msg := "afirma falhou: " + args[0].Inspect()
	if len(args) == 2 {
		if t, ok := args[1].(*object.Texto); ok {
			msg = "afirma falhou: " + t.Value
		}
	}
	io.WriteString(w, "FALHA: "+msg+"\n")
	return NADA
}
