package interpreter

import (
	"io"
	"net/http"
	"strings"
	"time"

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
		if v, erro := opcaoTexto(opcoes, "corpo"); erro != nil {
			return erro
		} else if v != "" {
			corpoReq = strings.NewReader(v)
		}
		cab, erro := opcaoCabecalhos(opcoes)
		if erro != nil {
			return erro
		}
		cabecalhos = cab
		if t, erro := opcaoTimeout(opcoes); erro != nil {
			return erro
		} else if t > 0 {
			timeout = t
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
	resp, err := cliente.Do(req)
	if err != nil {
		return erroBuiltin("deu ruim na conexao com %q: %v", urlObj.Value, err)
	}
	defer resp.Body.Close()

	corpo, err := io.ReadAll(resp.Body)
	if err != nil {
		return erroBuiltin("deu ruim lendo a resposta de %q: %v", urlObj.Value, err)
	}

	return montaResposta(resp, string(corpo))
}

func metodoValido(m string) bool {
	switch m {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
		return true
	}
	return false
}

// opcaoTexto le uma chave-texto do dicionario de opcoes; "" se ausente; erro se tipo errado.
func opcaoTexto(d *object.Dicionario, chave string) (string, *object.Erro) {
	par, existe := d.Pares[(&object.Texto{Value: chave}).ChaveHash()]
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
	par, existe := d.Pares[(&object.Texto{Value: "cabecalhos"}).ChaveHash()]
	if !existe {
		return out, nil
	}
	dic, ok := par.Valor.(*object.Dicionario)
	if !ok {
		return nil, erroBuiltin("a opcao \"cabecalhos\" tem que ser um dicionario, veio %s", par.Valor.Type())
	}
	for _, p := range dic.Pares {
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
	par, existe := d.Pares[(&object.Texto{Value: "timeout"}).ChaveHash()]
	if !existe {
		return 0, nil
	}
	n, ok := par.Valor.(*object.Numero)
	if !ok {
		return 0, erroBuiltin("a opcao \"timeout\" tem que ser numero (segundos), veio %s", par.Valor.Type())
	}
	return time.Duration(n.Value * float64(time.Second)), nil
}

func montaResposta(resp *http.Response, corpo string) object.Object {
	pares := map[object.HashKey]object.ParDic{}
	set := func(chave string, valor object.Object) {
		k := &object.Texto{Value: chave}
		pares[k.ChaveHash()] = object.ParDic{Chave: k, Valor: valor}
	}
	set("status", &object.Numero{Value: float64(resp.StatusCode)})
	set("ok", boolDoNativo(resp.StatusCode >= 200 && resp.StatusCode <= 299))
	set("corpo", &object.Texto{Value: corpo})

	cab := map[object.HashKey]object.ParDic{}
	for nome, valores := range resp.Header {
		k := &object.Texto{Value: nome}
		cab[k.ChaveHash()] = object.ParDic{Chave: k, Valor: &object.Texto{Value: strings.Join(valores, ", ")}}
	}
	set("cabecalhos", &object.Dicionario{Pares: cab})

	return &object.Dicionario{Pares: pares}
}
