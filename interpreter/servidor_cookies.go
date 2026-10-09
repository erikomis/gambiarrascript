//go:build !js

package interpreter

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"

	"gambiarrascript/object"
)

// Cookies na resposta: define_cookie(resposta, nome, valor, [opcoes]) bota um
// Set-Cookie no dicionario de resposta (o do responde_html/responde_json, o
// `resposta` do depois() ou um {} vazio) e devolve o proprio dicionario.
//
//	opcoes: expira_em (segundos; 0 ou menos apaga o cookie), http_only
//	        (padrao deu_bom), seguro (so HTTPS), mesmo_site ("Lax" padrao,
//	        "Strict", "None"), caminho ("/" padrao), dominio, segredo (assina
//	        com HMAC-SHA256: le_cookie(pedido, nome, segredo) confere).
//
// O padrao e o seguro: HttpOnly (JavaScript da pagina nao le) e SameSite=Lax
// (outro site nao manda o cookie num POST).

// tamMinSegredo: segredo de HMAC curto e chutavel. 32 caracteres = o minimo
// que o gera_chave() (44) passa com folga.
const tamMinSegredo = 32

func builtinDefineCookie(args []object.Object) object.Object {
	if len(args) < 3 || len(args) > 4 {
		return erroBuiltin("define_cookie() quer 3 ou 4 argumentos (resposta, nome, valor, [opcoes]), veio %d", len(args))
	}
	resp, ok := args[0].(*object.Dicionario)
	if !ok || (len(resp.Pares()) > 0 && !ehDicResposta(resp)) {
		return erroBuiltin("define_cookie(): o 1o argumento tem que ser um dicionario de resposta ({\"status\", \"corpo\", \"cabecalhos\"}, tipo o do responde_html) ou {} vazio, veio %s", args[0].Inspect())
	}
	nome, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("define_cookie(): o nome tem que ser texto, veio %s", object.NomeTipo(args[1]))
	}
	valor, ok := args[2].(*object.Texto)
	if !ok {
		return erroBuiltin("define_cookie(): o valor tem que ser texto, veio %s (pra dado use pra_json ou a usa_sessao)", object.NomeTipo(args[2]))
	}
	c := &http.Cookie{Name: nome.Value, Value: valor.Value, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode}
	var segredo string
	if len(args) == 4 {
		var erro *object.Erro
		if segredo, erro = leOpcoesCookie("define_cookie", args[3], c, true); erro != nil {
			return erro
		}
	}
	if segredo != "" {
		c.Value = c.Value + "." + assinaCookie(segredo, c.Name, c.Value)
	}
	if err := c.Valid(); err != nil {
		return erroBuiltin("define_cookie(): cookie invalido (%v) — valor de cookie nao aceita aspas, ponto e virgula, barra invertida nem acento; passa por base64_codifica", err)
	}
	botaSetCookie(resp, c.String())
	return resp
}

// leOpcoesCookie le o dicionario de opcoes de cookie (define_cookie e
// usa_sessao). comSegredo: aceita a chave "segredo" (devolvida a parte).
func leOpcoesCookie(fn string, o object.Object, c *http.Cookie, comSegredo bool) (string, *object.Erro) {
	d, ok := o.(*object.Dicionario)
	if !ok {
		return "", erroBuiltin("%s(): as opcoes tem que ser dicionario, veio %s", fn, object.NomeTipo(o))
	}
	var segredo string
	for _, par := range d.Pares() {
		k, _ := par.Chave.(*object.Texto)
		if k == nil {
			return "", erroBuiltin("%s(): as opcoes tem que ter chave texto, veio %s", fn, par.Chave.Inspect())
		}
		erroTipo := func(esperado string) (string, *object.Erro) {
			return "", erroBuiltin("%s(): %q tem que ser %s, veio %s", fn, k.Value, esperado, par.Valor.Inspect())
		}
		switch k.Value {
		case "expira_em":
			n, ok := par.Valor.(*object.Numero)
			if !ok || !ehInteiro(n) {
				return erroTipo("numero inteiro de segundos")
			}
			c.MaxAge = int(n.Value)
			if c.MaxAge <= 0 {
				c.MaxAge = -1 // Max-Age=0: o navegador apaga
			}
		case "http_only", "seguro":
			b, ok := par.Valor.(*object.Booleano)
			if !ok {
				return erroTipo("deu_bom ou deu_ruim")
			}
			if k.Value == "http_only" {
				c.HttpOnly = b.Value
			} else {
				c.Secure = b.Value
			}
		case "mesmo_site":
			t, _ := par.Valor.(*object.Texto)
			if t == nil {
				return erroTipo("\"Lax\", \"Strict\" ou \"None\"")
			}
			switch strings.ToLower(t.Value) {
			case "lax":
				c.SameSite = http.SameSiteLaxMode
			case "strict":
				c.SameSite = http.SameSiteStrictMode
			case "none":
				c.SameSite = http.SameSiteNoneMode
			default:
				return erroTipo("\"Lax\", \"Strict\" ou \"None\"")
			}
		case "caminho", "dominio":
			t, _ := par.Valor.(*object.Texto)
			if t == nil {
				return erroTipo("texto")
			}
			if k.Value == "caminho" {
				c.Path = t.Value
			} else {
				c.Domain = t.Value
			}
		case "segredo":
			if !comSegredo {
				return "", erroBuiltin("%s(): opcao \"segredo\" nao existe aqui", fn)
			}
			t, _ := par.Valor.(*object.Texto)
			if t == nil || len(t.Value) < tamMinSegredo {
				return "", erroBuiltin("%s(): \"segredo\" tem que ser texto de pelo menos %d caracteres (usa gera_chave())", fn, tamMinSegredo)
			}
			segredo = t.Value
		default:
			outras := "expira_em, http_only, seguro, mesmo_site, caminho, dominio"
			if comSegredo {
				outras += ", segredo"
			}
			return "", erroBuiltin("%s(): opcao %q nao existe (as que existem: %s)", fn, k.Value, outras)
		}
	}
	if c.SameSite == http.SameSiteNoneMode && !c.Secure {
		return "", erroBuiltin("%s(): \"mesmo_site\": \"None\" so vale com \"seguro\": deu_bom (o navegador recusa o cookie)", fn)
	}
	return segredo, nil
}

// botaSetCookie acrescenta uma linha Set-Cookie nos cabecalhos da resposta
// (vira lista quando tem mais de uma).
func botaSetCookie(resp *object.Dicionario, linha string) {
	var cab *object.Dicionario
	if v, ok := dicPega(resp, "cabecalhos"); ok {
		cab, _ = v.(*object.Dicionario)
	}
	if cab == nil {
		cab = object.NovoDicionario()
		dicBota(resp, "cabecalhos", cab)
	}
	chave := "Set-Cookie"
	for _, par := range cab.Pares() {
		if k, ok := par.Chave.(*object.Texto); ok && strings.EqualFold(k.Value, chave) {
			chave = k.Value
		}
	}
	novo := &object.Texto{Value: linha}
	atual, _ := dicPega(cab, chave)
	switch v := atual.(type) {
	case *object.Lista:
		v.Adiciona(novo)
	case *object.Texto:
		dicBota(cab, chave, object.NovaLista([]object.Object{v, novo}))
	default:
		dicBota(cab, chave, novo)
	}
}

// assinaCookie: HMAC-SHA256 de "nome=valor" (o nome entra pra assinatura de
// um cookie nao valer em outro) em base64url sem padding.
func assinaCookie(segredo, nome, valor string) string {
	m := hmac.New(sha256.New, []byte(segredo))
	m.Write([]byte(nome + "=" + valor))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// confereCookie separa valor e assinatura e confere em tempo constante.
func confereCookie(segredo, nome, bruto string) (string, bool) {
	i := strings.LastIndexByte(bruto, '.')
	if i < 0 {
		return "", false
	}
	valor, ass := bruto[:i], bruto[i+1:]
	esperado := assinaCookie(segredo, nome, valor)
	if !hmac.Equal([]byte(ass), []byte(esperado)) {
		return "", false
	}
	return valor, true
}

// builtinLeCookie: le_cookie(pedido, nome, [segredo]) → o valor do cookie ou
// nada. Com segredo so devolve se a assinatura do define_cookie bater
// (adulterado = nada, sem erro).
func builtinLeCookie(args []object.Object) object.Object {
	if len(args) < 2 || len(args) > 3 {
		return erroBuiltin("le_cookie() quer 2 ou 3 argumentos (pedido, nome, [segredo]), veio %d", len(args))
	}
	pedido, ok := args[0].(*object.Dicionario)
	if !ok {
		return erroBuiltin("le_cookie(): o 1o argumento tem que ser o pedido, veio %s", object.NomeTipo(args[0]))
	}
	nome, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("le_cookie(): o nome tem que ser texto, veio %s", object.NomeTipo(args[1]))
	}
	v, _ := dicPega(pedido, "cookies")
	cookies, ok := v.(*object.Dicionario)
	if !ok {
		return erroBuiltin("le_cookie(): esse dicionario nao tem \"cookies\" — passa o pedido que a rota recebeu")
	}
	bruto, _ := dicPega(cookies, nome.Value)
	t, ok := bruto.(*object.Texto)
	if !ok {
		return NADA
	}
	if len(args) == 2 {
		return t
	}
	segredo, ok := args[2].(*object.Texto)
	if !ok || len(segredo.Value) < tamMinSegredo {
		return erroBuiltin("le_cookie(): o segredo tem que ser texto de pelo menos %d caracteres (o mesmo do define_cookie)", tamMinSegredo)
	}
	valor, ok := confereCookie(segredo.Value, nome.Value, t.Value)
	if !ok {
		return NADA
	}
	return &object.Texto{Value: valor}
}
