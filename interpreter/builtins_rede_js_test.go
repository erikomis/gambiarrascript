//go:build js

package interpreter

import (
	"bytes"
	"strings"
	"testing"

	"gambiarrascript/object"
)

// No wasm os builtins de rede existem (o indice da VM casa) mas so avisam.
// Roda com: GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./interpreter/
func TestRedeNoNavegadorAvisa(t *testing.T) {
	i := New(&bytes.Buffer{})
	for _, nome := range []string{"conecta_tcp", "escuta_tcp", "endereco", "escuta_udp", "envia_udp", "conecta_udp"} {
		b, ok := i.BuiltinsVisiveis()[nome]
		if !ok {
			t.Fatalf("%s nao registrado no wasm", nome)
		}
		r := b.Fn([]object.Object{&object.Texto{Value: "127.0.0.1:9000"}})
		e, ok := r.(*object.Erro)
		if !ok || e.Kind != KindRede || !strings.Contains(e.Message, "rede nao funciona no navegador") {
			t.Fatalf("%s devia avisar que nao tem rede no navegador, veio %s", nome, r.Inspect())
		}
	}
}

// No navegador o busca travava ate o timeout (o net/http do Go espera uma
// promessa do JS que nao resolve com o evaluate sincrono). Agora rede,
// servidor, processo e TLS ficam fora do wasm e avisam na hora.
func TestServidorProcessoETLSNoNavegadorAvisam(t *testing.T) {
	i := New(&bytes.Buffer{})
	for _, nome := range []string{"busca", "rota", "escuta", "rota_ws", "antes", "depois", "cors", "serve_pasta", "responde_json", "gera_certificado", "roda_comando"} {
		b, ok := i.BuiltinsVisiveis()[nome]
		if !ok {
			t.Fatalf("%s nao registrado no wasm (a VM chama por indice)", nome)
		}
		r := b.Fn([]object.Object{&object.Texto{Value: "x"}})
		e, ok := r.(*object.Erro)
		if !ok || !strings.Contains(e.Message, "nao roda no navegador") {
			t.Fatalf("%s devia avisar que nao roda no navegador, veio %s", nome, r.Inspect())
		}
	}
	// sai continua funcionando no navegador
	if _, ok := builtinSai([]object.Object{object.NumInt(3)}).(*object.Sair); !ok {
		t.Fatal("sai() devia devolver o controle de saida no wasm")
	}
}
