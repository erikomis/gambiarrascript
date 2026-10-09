//go:build js

// Stub das tarefas agendadas pro playground (WebAssembly): o script roda
// sincrono e acaba, nao tem processo pra ficar de pe esperando o proximo
// disparo. Os nomes continuam registrados (a VM chama builtin por indice) e
// respondem com erro claro. A versao de verdade fica em builtins_agenda.go.

package interpreter

import "gambiarrascript/object"

type agendador struct{}

func erroAgendaNavegador(nome string) object.Object {
	return erroBuiltin("%s(): %s", nome, soNoNativo)
}

func (i *Interpreter) builtinACada(args []object.Object) object.Object {
	return erroAgendaNavegador("a_cada")
}
func (i *Interpreter) builtinDepoisDe(args []object.Object) object.Object {
	return erroAgendaNavegador("depois_de")
}
func (i *Interpreter) builtinAgenda(args []object.Object) object.Object {
	return erroAgendaNavegador("agenda")
}
func (i *Interpreter) builtinCancela(args []object.Object) object.Object {
	return erroAgendaNavegador("cancela")
}

// AgendamentosAtivos: no navegador nunca tem.
func (i *Interpreter) AgendamentosAtivos() int { return 0 }

// CancelaAgendamentos: nada pra cancelar no navegador.
func (i *Interpreter) CancelaAgendamentos() {}

// EsperaAgendamentos: nada pra esperar no navegador.
func (i *Interpreter) EsperaAgendamentos() *object.Sair { return nil }
