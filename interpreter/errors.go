package interpreter

import (
	"fmt"

	"gambiarrascript/object"
)

// Kinds de erro. Devem ser usados em object.Erro.Kind pra inspecao programatica.
const (
	KindRuntime  = "runtime"  // erro durante avaliacao (div por zero, tipo, indice)
	KindBuiltin  = "builtin"  // erro dentro de uma builtin (sem linha do chamador)
	KindIO       = "io"       // leitura/escrita de arquivo, stdin
	KindRede     = "rede"     // HTTP/rede
	KindParse    = "parse"    // erro importando modulo
	KindUsuario  = "usuario"  // lancado via quebra()
)

// newError cria um erro de runtime carregando a linha de origem do AST.
func newError(linha int, formato string, args ...interface{}) *object.Erro {
	msg := fmt.Sprintf(formato, args...)
	return &object.Erro{
		Message: fmt.Sprintf("deu ruim na linha %d: %s", linha, msg),
		Line:    linha,
		Kind:    KindRuntime,
	}
}

// newErrorKind cria um erro marcando o kind explicito (io/rede/parse/usuario).
// A linha ainda e conhecida. O texto mantem o prefixo "deu ruim na linha N:".
func newErrorKind(kind string, linha int, formato string, args ...interface{}) *object.Erro {
	msg := fmt.Sprintf(formato, args...)
	prefixo := "deu ruim"
	if linha > 0 {
		prefixo = fmt.Sprintf("deu ruim na linha %d", linha)
	}
	return &object.Erro{
		Message: prefixo + ": " + msg,
		Line:    linha,
		Kind:    kind,
	}
}

// erroBuiltin cria um erro dentro de uma builtin — sem linha do chamador
// (builtins nao enxergam o AST). Mantem o prefixo historico "deu ruim:".
func erroBuiltin(formato string, args ...interface{}) *object.Erro {
	return &object.Erro{
		Message: "deu ruim: " + fmt.Sprintf(formato, args...),
		Kind:    KindBuiltin,
	}
}

// erroBuiltinKind e como erroBuiltin mas marca um kind diferente de "builtin".
// Usado por IO/rede quando a builtin sabe classificar melhor o erro.
func erroBuiltinKind(kind, formato string, args ...interface{}) *object.Erro {
	return &object.Erro{
		Message: "deu ruim: " + fmt.Sprintf(formato, args...),
		Kind:    kind,
	}
}

// isError verifica se obj e um *object.Erro NAO-Handled. Um erro capturado
// por `arruma` e marcado Handled pra poder ser usado em expressoes (string
// concat, log) sem voltar a propagar — entao isError devolve false nele.
func isError(obj object.Object) bool {
	if obj == nil || obj.Type() != object.ERRO_OBJ {
		return false
	}
	if e, ok := obj.(*object.Erro); ok {
		return !e.Handled
	}
	return true
}

// empilhaFrame prepende o frame na pilha de erros, mantendo ordem externo->
// interno conforme o erro bubble-up pelas chamadas de applyFunction.
func empilhaFrame(err *object.Erro, nome string, linha int) {
	if err == nil {
		return
	}
	frame := object.StackFrame{Funcao: nome, Line: linha}
	err.Stack = append([]object.StackFrame{frame}, err.Stack...)
}