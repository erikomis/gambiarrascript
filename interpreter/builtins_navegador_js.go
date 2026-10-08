//go:build js

// Versoes do playground (WebAssembly) dos builtins que precisam de rede, de
// processo ou de TLS. No navegador nada disso funciona: o net/http do Go
// espera uma promessa do JS que nunca resolve enquanto o evaluate roda
// sincrono no worker (o `busca` travava ate o timeout de 10s), servidor nao
// tem onde escutar e processo nao existe. Tirar esses pacotes do build corta
// mais ou menos metade do gs.wasm. Os nomes continuam registrados (a VM chama
// builtin por indice) e respondem com erro claro na hora.

package interpreter

import "gambiarrascript/object"

const soNoNativo = "isso nao roda no navegador (playground) — so no gs instalado"

func erroNavegador(nome string) object.Object {
	return erroBuiltinKind(KindRede, "%s(): %s", nome, soNoNativo)
}

// servidorEstado do navegador: sem estado nenhum, todo builtin de servidor
// so avisa.
type servidorEstado struct{}

func novoServidorEstado(_ *Interpreter) *servidorEstado { return &servidorEstado{} }

func (s *servidorEstado) builtinRota(args []object.Object) object.Object {
	return erroNavegador("rota")
}
func (s *servidorEstado) builtinEscuta(args []object.Object) object.Object {
	return erroNavegador("escuta")
}
func (s *servidorEstado) builtinRotaWs(args []object.Object) object.Object {
	return erroNavegador("rota_ws")
}
func (s *servidorEstado) builtinAntes(args []object.Object) object.Object {
	return erroNavegador("antes")
}
func (s *servidorEstado) builtinDepois(args []object.Object) object.Object {
	return erroNavegador("depois")
}
func (s *servidorEstado) builtinCors(args []object.Object) object.Object {
	return erroNavegador("cors")
}
func (s *servidorEstado) builtinServePasta(args []object.Object) object.Object {
	return erroNavegador("serve_pasta")
}

func builtinBusca(args []object.Object) object.Object        { return erroNavegador("busca") }
func builtinRespondeJson(args []object.Object) object.Object { return erroNavegador("responde_json") }
func builtinGeraCertificado(args []object.Object) object.Object {
	return erroNavegador("gera_certificado")
}
func builtinRodaComando(args []object.Object) object.Object {
	return erroBuiltin("roda_comando(): %s", soNoNativo)
}
