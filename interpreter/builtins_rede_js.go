//go:build js

// Stub de rede baixo nivel pro build WebAssembly (playground). Navegador nao
// deixa abrir socket TCP/UDP cru, entao os builtins existem (o indice da VM
// tem que casar) mas so avisam. A versao de verdade fica em builtins_rede.go.

package interpreter

import "gambiarrascript/object"

func semRedeNoNavegador(nome string) object.Object {
	return erroBuiltinKind(KindRede, "%s(): rede nao funciona no navegador (wasm) — socket so roda no gs nativo", nome)
}

func (i *Interpreter) builtinConectaTcp(args []object.Object) object.Object {
	return semRedeNoNavegador("conecta_tcp")
}

func (i *Interpreter) builtinEscutaTcp(args []object.Object) object.Object {
	return semRedeNoNavegador("escuta_tcp")
}

func (i *Interpreter) builtinEscutaUdp(args []object.Object) object.Object {
	return semRedeNoNavegador("escuta_udp")
}

func builtinEndereco(args []object.Object) object.Object {
	return semRedeNoNavegador("endereco")
}

func builtinConectaUdp(args []object.Object) object.Object {
	return semRedeNoNavegador("conecta_udp")
}

func builtinEnviaUdp(args []object.Object) object.Object {
	return semRedeNoNavegador("envia_udp")
}
