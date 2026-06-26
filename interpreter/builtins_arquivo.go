package interpreter

import (
	"os"

	"gambiarrascript/object"
)

func builtinLeArquivo(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("le_arquivo() quer 1 argumento (caminho), veio %d", len(args))
	}
	caminho, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("le_arquivo() espera texto (caminho), veio %s", args[0].Type())
	}
	bs, err := os.ReadFile(caminho.Value)
	if err != nil {
		return erroBuiltinKind(KindIO, "nao consegui ler %q: %v", caminho.Value, err)
	}
	return &object.Texto{Value: string(bs)}
}

func builtinEscreveArquivo(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("escreve_arquivo() quer 2 argumentos (caminho, texto), veio %d", len(args))
	}
	caminho, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("escreve_arquivo() espera texto (caminho), veio %s", args[0].Type())
	}
	conteudo, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("escreve_arquivo() espera texto (conteudo), veio %s", args[1].Type())
	}
	if err := os.WriteFile(caminho.Value, []byte(conteudo.Value), 0644); err != nil {
		return erroBuiltinKind(KindIO, "nao consegui escrever em %q: %v", caminho.Value, err)
	}
	return NADA
}

// builtinAnexaArquivo acrescenta texto ao final do arquivo (cria se nao existir).
// Distinto de escreve_arquivo: nao sobrescreve. Pré: fluxo pra logs, appending
// incremental em scripts de CLI processando pipes grandes.
func builtinAnexaArquivo(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("anexa_arquivo() quer 2 argumentos (caminho, texto), veio %d", len(args))
	}
	caminho, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("anexa_arquivo() espera texto (caminho), veio %s", args[0].Type())
	}
	conteudo, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("anexa_arquivo() espera texto (conteudo), veio %s", args[1].Type())
	}
	f, err := os.OpenFile(caminho.Value, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return erroBuiltinKind(KindIO, "nao consegui abrir %q pra anexar: %v", caminho.Value, err)
	}
	defer f.Close()
	if _, err := f.WriteString(conteudo.Value); err != nil {
		return erroBuiltinKind(KindIO, "nao consegui anexar em %q: %v", caminho.Value, err)
	}
	return NADA
}