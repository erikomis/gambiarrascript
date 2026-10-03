//go:build js

// Stub de WebSocket pro build WebAssembly (playground): o servidor nao sobe
// no navegador e o cliente do coder/websocket la nao manda cabecalho. A
// versao de verdade fica em builtins_ws.go.

package interpreter

import (
	"net/http"

	"gambiarrascript/object"
)

func (s *servidorEstado) fechaWebsockets() {}

func (s *servidorEstado) atendeWS(w http.ResponseWriter, r *http.Request, rota *rotaHTTP, pedido *object.Dicionario, cors *configCors) {
	escreveTexto(w, http.StatusNotImplemented, "websocket nao funciona no navegador (wasm), parca")
}

func builtinConectaWs(args []object.Object) object.Object {
	return erroBuiltin("conecta_ws() nao funciona no navegador (wasm) — websocket so roda no gs nativo")
}
