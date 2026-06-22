package interpreter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gambiarrascript/object"
)

// leChave extrai o valor de uma chave-texto de um dicionario de resposta.
func leChave(t *testing.T, d *object.Dicionario, chave string) object.Object {
	t.Helper()
	par, ok := d.Pares[(&object.Texto{Value: chave}).ChaveHash()]
	if !ok {
		t.Fatalf("resposta nao tem a chave %q", chave)
	}
	return par.Valor
}

func dicOuFalha(t *testing.T, o object.Object) *object.Dicionario {
	t.Helper()
	d, ok := o.(*object.Dicionario)
	if !ok {
		t.Fatalf("esperava dicionario, got %s: %s", o.Type(), o.Inspect())
	}
	return d
}

func TestBuscaGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("esperava GET, veio %s", r.Method)
		}
		w.Header().Set("X-Teste", "valor")
		io.WriteString(w, "ola mundo")
	}))
	defer srv.Close()

	res := builtinBusca([]object.Object{&object.Texto{Value: srv.URL}})
	d := dicOuFalha(t, res)
	if s := leChave(t, d, "status"); s.Inspect() != "200" {
		t.Fatalf("status: got %s", s.Inspect())
	}
	if ok := leChave(t, d, "ok"); ok.Inspect() != "deu_bom" {
		t.Fatalf("ok: got %s", ok.Inspect())
	}
	if c := leChave(t, d, "corpo"); c.Inspect() != "ola mundo" {
		t.Fatalf("corpo: got %q", c.Inspect())
	}
	cab := dicOuFalha(t, leChave(t, d, "cabecalhos"))
	if h := leChave(t, cab, "X-Teste"); h.Inspect() != "valor" {
		t.Fatalf("cabecalho X-Teste: got %q", h.Inspect())
	}
}

func TestBuscaPostComCorpoEHeader(t *testing.T) {
	var corpoRecebido, autorizacao string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("esperava POST, veio %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		corpoRecebido = string(b)
		autorizacao = r.Header.Get("Authorization")
		io.WriteString(w, "criado")
	}))
	defer srv.Close()

	opcoes := &object.Dicionario{Pares: map[object.HashKey]object.ParDic{}}
	porChave := func(k string, v object.Object) {
		key := &object.Texto{Value: k}
		opcoes.Pares[key.ChaveHash()] = object.ParDic{Chave: key, Valor: v}
	}
	porChave("metodo", &object.Texto{Value: "POST"})
	porChave("corpo", &object.Texto{Value: "oi servidor"})
	cab := &object.Dicionario{Pares: map[object.HashKey]object.ParDic{}}
	hk := &object.Texto{Value: "Authorization"}
	cab.Pares[hk.ChaveHash()] = object.ParDic{Chave: hk, Valor: &object.Texto{Value: "Bearer 123"}}
	porChave("cabecalhos", cab)

	res := builtinBusca([]object.Object{&object.Texto{Value: srv.URL}, opcoes})
	d := dicOuFalha(t, res)
	if corpoRecebido != "oi servidor" {
		t.Fatalf("o servidor recebeu corpo %q", corpoRecebido)
	}
	if autorizacao != "Bearer 123" {
		t.Fatalf("o servidor recebeu Authorization %q", autorizacao)
	}
	if ok := leChave(t, d, "ok"); ok.Inspect() != "deu_bom" {
		t.Fatalf("ok: got %s", ok.Inspect())
	}
	if c := leChave(t, d, "corpo"); c.Inspect() != "criado" {
		t.Fatalf("corpo: got %q", c.Inspect())
	}
}

func TestBuscaTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		io.WriteString(w, "nunca chega")
	}))
	defer srv.Close()

	opcoes := &object.Dicionario{Pares: map[object.HashKey]object.ParDic{}}
	porChave := func(k string, v object.Object) {
		key := &object.Texto{Value: k}
		opcoes.Pares[key.ChaveHash()] = object.ParDic{Chave: key, Valor: v}
	}
	porChave("timeout", &object.Numero{Value: 0.05})

	res := builtinBusca([]object.Object{&object.Texto{Value: srv.URL}, opcoes})
	if res.Type() != object.ERRO_OBJ {
		t.Fatalf("esperava ERRO por timeout, got %s: %s", res.Type(), res.Inspect())
	}
}

func TestBuscaConexaoRecusada(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // porta morta

	res := builtinBusca([]object.Object{&object.Texto{Value: url}})
	if res.Type() != object.ERRO_OBJ {
		t.Fatalf("esperava ERRO por conexao recusada, got %s: %s", res.Type(), res.Inspect())
	}
}

func TestBuscaStatusNaoOk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		io.WriteString(w, "nao achei")
	}))
	defer srv.Close()

	res := builtinBusca([]object.Object{&object.Texto{Value: srv.URL}})
	d := dicOuFalha(t, res) // 404 NAO e erro: e dicionario normal
	if s := leChave(t, d, "status"); s.Inspect() != "404" {
		t.Fatalf("status: got %s", s.Inspect())
	}
	if ok := leChave(t, d, "ok"); ok.Inspect() != "deu_ruim" {
		t.Fatalf("ok deveria ser deu_ruim, got %s", ok.Inspect())
	}
}

func TestBuscaErroDeTransporte(t *testing.T) {
	// URL sem esquema valido -> erro de transporte (capturavel por arruma)
	res := builtinBusca([]object.Object{&object.Texto{Value: "://url-quebrada"}})
	if res.Type() != object.ERRO_OBJ {
		t.Fatalf("esperava ERRO, got %s: %s", res.Type(), res.Inspect())
	}
}

func TestBuscaValidacao(t *testing.T) {
	// url nao-texto
	if res := builtinBusca([]object.Object{&object.Numero{Value: 1}}); res.Type() != object.ERRO_OBJ {
		t.Fatalf("url numerica deveria dar erro, got %s", res.Type())
	}
	// metodo invalido
	opcoes := &object.Dicionario{Pares: map[object.HashKey]object.ParDic{}}
	k := &object.Texto{Value: "metodo"}
	opcoes.Pares[k.ChaveHash()] = object.ParDic{Chave: k, Valor: &object.Texto{Value: "VOA"}}
	if res := builtinBusca([]object.Object{&object.Texto{Value: "http://x"}, opcoes}); res.Type() != object.ERRO_OBJ {
		t.Fatalf("metodo invalido deveria dar erro, got %s", res.Type())
	}
}
