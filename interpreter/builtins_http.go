package interpreter

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gambiarrascript/object"
)

const timeoutPadraoHTTP = 30 * time.Second

func builtinBusca(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("busca() quer 1 ou 2 argumentos (url, [opcoes]), veio %d", len(args))
	}
	urlObj, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("busca() espera a url como texto, veio %s", args[0].Type())
	}

	metodo := "GET"
	var corpoReq io.Reader
	cabecalhos := map[string]string{}
	timeout := timeoutPadraoHTTP
	var cfgTLS *tls.Config

	if len(args) == 2 {
		opcoes, ok := args[1].(*object.Dicionario)
		if !ok {
			return erroBuiltin("busca() espera um dicionario de opcoes, veio %s", args[1].Type())
		}
		if v, erro := opcaoTexto(opcoes, "metodo"); erro != nil {
			return erro
		} else if v != "" {
			metodo = strings.ToUpper(v)
		}
		if !metodoValido(metodo) {
			return erroBuiltin("metodo HTTP desconhecido: %q", metodo)
		}
		corpo, tipoCorpo, erro := opcaoCorpo(opcoes)
		if erro != nil {
			return erro
		}
		if corpo != nil {
			corpoReq = bytes.NewReader(corpo)
		}
		cab, erro := opcaoCabecalhos(opcoes)
		if erro != nil {
			return erro
		}
		cabecalhos = cab
		if tipoCorpo != "" && !temCabecalho(cabecalhos, "Content-Type") {
			cabecalhos["Content-Type"] = tipoCorpo
		}
		if t, erro := opcaoTimeout(opcoes); erro != nil {
			return erro
		} else if t > 0 {
			timeout = t
		}
		if cfgTLS, erro = tlsDoClienteHTTP("busca", opcoes); erro != nil {
			return erro
		}
	}

	req, err := http.NewRequest(metodo, urlObj.Value, corpoReq)
	if err != nil {
		return erroBuiltin("nao consegui montar a requisicao pra %q: %v", urlObj.Value, err)
	}
	for k, v := range cabecalhos {
		req.Header.Set(k, v)
	}

	cliente := &http.Client{Timeout: timeout}
	if cfgTLS != nil {
		tr, erro := transporteTLS("busca", cfgTLS)
		if erro != nil {
			return erro
		}
		defer tr.CloseIdleConnections()
		cliente.Transport = tr
	}
	resp, err := cliente.Do(req)
	if err != nil {
		if ehErroCertificado(err) {
			return erroBuiltin("deu ruim na conexao com %q: o certificado nao passou (%v) — servidor de dev com certificado proprio? passa {\"ca\": \"ca.pem\"}", urlObj.Value, err)
		}
		return erroBuiltin("deu ruim na conexao com %q: %v", urlObj.Value, err)
	}
	defer resp.Body.Close()

	corpo, err := io.ReadAll(resp.Body)
	if err != nil {
		return erroBuiltin("deu ruim lendo a resposta de %q: %v", urlObj.Value, err)
	}

	return montaResposta(resp, corpo)
}

// transporteTLS monta um transporte novo (copia do padrao do Go: proxy,
// HTTP/2, timeouts) com o tls.Config de "ca"/"inseguro". No navegador o
// fetch nao deixa mexer na validacao do certificado: erro claro em vez de
// ignorar calado.
func transporteTLS(nome string, cfg *tls.Config) (*http.Transport, *object.Erro) {
	if runtime.GOOS == "js" {
		return nil, erroBuiltin("%s(): \"ca\" e \"inseguro\" nao funcionam no navegador (wasm) — quem valida o certificado la e o proprio navegador", nome)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = cfg
	return tr, nil
}

// opcaoCorpo le o corpo da requisicao de UMA das opcoes: "corpo" (texto),
// "corpo_base64" (bytes crus, pra upload binario) ou "json" (qualquer valor,
// serializado igual ao pra_json — e ja manda o Content-Type de JSON).
func opcaoCorpo(d *object.Dicionario) ([]byte, string, *object.Erro) {
	var achadas []string
	for _, k := range []string{"corpo", "corpo_base64", "json"} {
		if _, ok := d.Pega((&object.Texto{Value: k}).ChaveHash()); ok {
			achadas = append(achadas, k)
		}
	}
	if len(achadas) > 1 {
		return nil, "", erroBuiltin("busca(): escolhe um so entre \"corpo\", \"corpo_base64\" e \"json\", veio %s", strings.Join(achadas, " e "))
	}
	if len(achadas) == 0 {
		return nil, "", nil
	}
	switch achadas[0] {
	case "corpo":
		v, erro := opcaoTexto(d, "corpo")
		if erro != nil || v == "" {
			return nil, "", erro
		}
		return []byte(v), "", nil
	case "corpo_base64":
		v, erro := opcaoTexto(d, "corpo_base64")
		if erro != nil {
			return nil, "", erro
		}
		bs, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, "", erroBuiltin("busca(): \"corpo_base64\" nao e base64 valido, parca: %v", err)
		}
		return bs, "", nil
	}
	par, _ := d.Pega((&object.Texto{Value: "json"}).ChaveHash())
	var buf bytes.Buffer
	if e := escreveJson(&buf, par.Valor); e != nil {
		return nil, "", e
	}
	return buf.Bytes(), tipoJSON, nil
}

func temCabecalho(cab map[string]string, nome string) bool {
	for k := range cab {
		if strings.EqualFold(k, nome) {
			return true
		}
	}
	return false
}

func metodoValido(m string) bool {
	switch m {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return true
	}
	return false
}

// opcaoTexto le uma chave-texto do dicionario de opcoes; "" se ausente; erro se tipo errado.
func opcaoTexto(d *object.Dicionario, chave string) (string, *object.Erro) {
	par, existe := d.Pega((&object.Texto{Value: chave}).ChaveHash())
	if !existe {
		return "", nil
	}
	t, ok := par.Valor.(*object.Texto)
	if !ok {
		return "", erroBuiltin("a opcao %q tem que ser texto, veio %s", chave, par.Valor.Type())
	}
	return t.Value, nil
}

func opcaoCabecalhos(d *object.Dicionario) (map[string]string, *object.Erro) {
	out := map[string]string{}
	par, existe := d.Pega((&object.Texto{Value: "cabecalhos"}).ChaveHash())
	if !existe {
		return out, nil
	}
	dic, ok := par.Valor.(*object.Dicionario)
	if !ok {
		return nil, erroBuiltin("a opcao \"cabecalhos\" tem que ser um dicionario, veio %s", par.Valor.Type())
	}
	for _, p := range dic.Pares() {
		chave, ok := p.Chave.(*object.Texto)
		if !ok {
			return nil, erroBuiltin("nome de cabecalho tem que ser texto, veio %s", p.Chave.Type())
		}
		valor, ok := p.Valor.(*object.Texto)
		if !ok {
			return nil, erroBuiltin("valor do cabecalho %q tem que ser texto, veio %s", chave.Value, p.Valor.Type())
		}
		out[chave.Value] = valor.Value
	}
	return out, nil
}

func opcaoTimeout(d *object.Dicionario) (time.Duration, *object.Erro) {
	par, existe := d.Pega((&object.Texto{Value: "timeout"}).ChaveHash())
	if !existe {
		return 0, nil
	}
	n, ok := par.Valor.(*object.Numero)
	if !ok {
		return 0, erroBuiltin("a opcao \"timeout\" tem que ser numero (segundos), veio %s", par.Valor.Type())
	}
	return time.Duration(n.Value * float64(time.Second)), nil
}

// montaResposta monta o dicionario de resposta. Corpo que nao e UTF-8 valido
// (imagem, zip, pdf...) tambem vem em "corpo_base64" — o "corpo" continua com
// os bytes crus, que nao imprimem direito mas servem pro escreve_arquivo.
func montaResposta(resp *http.Response, bruto []byte) object.Object {
	corpo := string(bruto)
	dic := object.NovoDicionario()
	set := func(chave string, valor object.Object) {
		k := &object.Texto{Value: chave}
		dic.Bota(k.ChaveHash(), object.ParDic{Chave: k, Valor: valor})
	}
	set("status", &object.Numero{Value: float64(resp.StatusCode)})
	set("ok", boolDoNativo(resp.StatusCode >= 200 && resp.StatusCode <= 299))
	set("corpo", &object.Texto{Value: corpo})

	cab := object.NovoDicionario()
	for _, nome := range chavesOrdenadas(resp.Header) {
		k := &object.Texto{Value: nome}
		cab.Bota(k.ChaveHash(), object.ParDic{Chave: k, Valor: &object.Texto{Value: strings.Join(resp.Header[nome], ", ")}})
	}
	set("cabecalhos", cab)
	if !utf8.Valid(bruto) {
		set("corpo_base64", &object.Texto{Value: base64.StdEncoding.EncodeToString(bruto)})
	}

	return dic
}

// chavesOrdenadas devolve os nomes de um multimap (cabecalhos, query) em ordem
// alfabetica. O map do Go entrega ordem embaralhada, e cabecalho/query viram
// dicionario da linguagem — que agora tem ordem estavel.
func chavesOrdenadas(m map[string][]string) []string {
	nomes := make([]string, 0, len(m))
	for k := range m {
		nomes = append(nomes, k)
	}
	sort.Strings(nomes)
	return nomes
}
