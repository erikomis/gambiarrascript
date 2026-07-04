//go:build js

// Stub de banco de dados para o build WebAssembly (navegador). Os drivers SQL
// (modernc.org/sqlite, go-sql-driver/mysql, pgx) dependem de syscalls e de libc
// que nao existem em GOOS=js, entao no navegador conecta()/fecha() apenas
// avisam que o recurso nao esta disponivel. A versao real fica em builtins_bd.go.

package interpreter

import "gambiarrascript/object"

func builtinConecta(args []object.Object) object.Object {
	return erroBuiltin("conecta() nao funciona no navegador (wasm) — banco de dados so roda no gs nativo")
}

func builtinFecha(args []object.Object) object.Object {
	return erroBuiltin("fecha() de banco nao funciona no navegador (wasm) — banco de dados so roda no gs nativo")
}

func builtinConsulta(args []object.Object) object.Object {
	return erroBuiltin("consulta() nao funciona no navegador (wasm) — banco de dados so roda no gs nativo")
}

func builtinExecuta(args []object.Object) object.Object {
	return erroBuiltin("executa() nao funciona no navegador (wasm) — banco de dados so roda no gs nativo")
}
