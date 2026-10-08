//go:build js

// Stub de WebSocket pro build WebAssembly (playground): o servidor nao sobe
// no navegador e o cliente do coder/websocket la nao manda cabecalho. A
// versao de verdade fica em builtins_ws.go.

package interpreter

import "gambiarrascript/object"

func builtinConectaWs(args []object.Object) object.Object {
	return erroBuiltin("conecta_ws() nao funciona no navegador (wasm) — websocket so roda no gs nativo")
}
