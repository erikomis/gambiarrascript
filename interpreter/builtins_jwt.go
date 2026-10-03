package interpreter

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"

	"gambiarrascript/object"
)

// JWT (HS256) na mao, so com a stdlib: hmac + sha256 + base64url.
//
//	jwt_assina(dados, segredo, [{"expira_em": segundos}]) → token
//	jwt_confere(token, segredo) → dicionario com os claims
//
// O jwt_confere so aceita alg HS256 — "none" e qualquer outro alg sao
// recusados ANTES de olhar a assinatura (o golpe classico do alg:none e o de
// trocar pra RS256 usando a chave publica como segredo). Falha de token
// (assinatura, expirado, mal formado, alg) vira erro do tipo "jwt": da pra
// pegar no arruma e devolver 401 sem confundir com bug do codigo.

// KindJWT e o tipo de erro de token invalido (veja erro_tipo).
const KindJWT = "jwt"

// agoraJWT e o relogio do JWT (variavel pros testes congelarem o tempo).
var agoraJWT = time.Now

// cabecalhoJWT e fixo: sempre HS256. Ordem alg/typ igual a da RFC 7519.
const cabecalhoJWT = `{"alg":"HS256","typ":"JWT"}`

func erroJWT(formato string, args ...interface{}) *object.Erro {
	return erroBuiltinKind(KindJWT, "jwt: "+formato, args...)
}

func assinaHS256(segredo, conteudo string) []byte {
	mac := hmac.New(sha256.New, []byte(segredo))
	mac.Write([]byte(conteudo))
	return mac.Sum(nil)
}

func builtinJwtAssina(args []object.Object) object.Object {
	if len(args) < 2 || len(args) > 3 {
		return erroBuiltin("jwt_assina() quer (dados, segredo, [opcoes]), veio %d", len(args))
	}
	dados, ok := args[0].(*object.Dicionario)
	if !ok {
		return erroBuiltin("jwt_assina: dados tem que ser dicionario, veio %s", args[0].Type())
	}
	segredo, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("jwt_assina: segredo tem que ser texto, veio %s", args[1].Type())
	}
	if segredo.Value == "" {
		return erroBuiltin("jwt_assina: segredo vazio nao rola (usa gera_chave())")
	}

	claims := dados
	if len(args) == 3 {
		op, ok := args[2].(*object.Dicionario)
		if !ok {
			return erroBuiltin("jwt_assina: opcoes tem que ser dicionario, veio %s", args[2].Type())
		}
		var erro *object.Erro
		claims, erro = aplicaOpcoesJWT(dados, op)
		if erro != nil {
			return erro
		}
	}

	var payload bytes.Buffer
	if erro := escreveJson(&payload, claims); erro != nil {
		return erro
	}
	b64 := base64.RawURLEncoding
	conteudo := b64.EncodeToString([]byte(cabecalhoJWT)) + "." + b64.EncodeToString(payload.Bytes())
	return &object.Texto{Value: conteudo + "." + b64.EncodeToString(assinaHS256(segredo.Value, conteudo))}
}

// aplicaOpcoesJWT devolve uma COPIA dos dados com os claims das opcoes
// (nao mexe no dicionario de quem chamou). Hoje: expira_em → exp (e iat).
func aplicaOpcoesJWT(dados, op *object.Dicionario) (*object.Dicionario, *object.Erro) {
	claims := object.NovoDicionario()
	dados.Itera(func(par object.ParDic) {
		claims.Bota(par.Chave.(object.Chaveavel).ChaveHash(), par)
	})
	var erro *object.Erro
	op.Itera(func(par object.ParDic) {
		if erro != nil {
			return
		}
		nome, _ := par.Chave.(*object.Texto)
		if nome == nil || nome.Value != "expira_em" {
			erro = erroBuiltin("jwt_assina: opcao desconhecida %s (a que rola: expira_em)", par.Chave.Inspect())
			return
		}
		n, ok := par.Valor.(*object.Numero)
		if !ok || !ehInteiro(n) {
			erro = erroBuiltin("jwt_assina: expira_em quer segundos (numero inteiro), veio %s", par.Valor.Inspect())
			return
		}
		agora := agoraJWT().Unix()
		botaTexto(claims, "iat", object.NumInt(agora))
		botaTexto(claims, "exp", object.NumInt(agora+int64(n.Value)))
	})
	return claims, erro
}

func botaTexto(d *object.Dicionario, chave string, v object.Object) {
	k := &object.Texto{Value: chave}
	d.Bota(k.ChaveHash(), object.ParDic{Chave: k, Valor: v})
}

func pegaTexto(d *object.Dicionario, chave string) (object.Object, bool) {
	k := &object.Texto{Value: chave}
	par, ok := d.Pares[k.ChaveHash()]
	if !ok {
		return nil, false
	}
	return par.Valor, true
}

// decodificaParteJWT aceita base64url sem padding (o certo) e tolera '=' no
// fim, que tem gerador por ai que bota.
func decodificaParteJWT(s string) ([]byte, bool) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	return b, err == nil
}

func builtinJwtConfere(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("jwt_confere() quer (token, segredo), veio %d", len(args))
	}
	token, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("jwt_confere: token tem que ser texto, veio %s", args[0].Type())
	}
	segredo, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("jwt_confere: segredo tem que ser texto, veio %s", args[1].Type())
	}
	if segredo.Value == "" {
		return erroBuiltin("jwt_confere: segredo vazio nao rola")
	}

	partes := strings.Split(strings.TrimSpace(token.Value), ".")
	if len(partes) != 3 {
		return erroJWT("token mal formado (quer 3 partes separadas por ponto, veio %d)", len(partes))
	}

	// 1) cabecalho: tem que ser JSON com alg HS256 — antes de tudo
	cabBytes, ok := decodificaParteJWT(partes[0])
	if !ok {
		return erroJWT("token mal formado (cabecalho nao e base64url)")
	}
	cab, err := parseJson(string(cabBytes))
	cabDic, ehDic := cab.(*object.Dicionario)
	if err != nil || !ehDic {
		return erroJWT("token mal formado (cabecalho nao e JSON de objeto)")
	}
	alg, _ := pegaTexto(cabDic, "alg")
	if algT, ok := alg.(*object.Texto); !ok || algT.Value != "HS256" {
		mostrado := "nenhum"
		if alg != nil {
			mostrado = alg.Inspect()
		}
		return erroJWT("alg %s nao rola, so HS256", mostrado)
	}

	// 2) assinatura, comparada em tempo constante
	assinatura, ok := decodificaParteJWT(partes[2])
	if !ok {
		return erroJWT("token mal formado (assinatura nao e base64url)")
	}
	if !hmac.Equal(assinatura, assinaHS256(segredo.Value, partes[0]+"."+partes[1])) {
		return erroJWT("assinatura invalida (segredo errado ou token adulterado)")
	}

	// 3) payload + validade (exp / nbf)
	pBytes, ok := decodificaParteJWT(partes[1])
	if !ok {
		return erroJWT("token mal formado (payload nao e base64url)")
	}
	payload, err := parseJson(string(pBytes))
	claims, ehDic := payload.(*object.Dicionario)
	if err != nil || !ehDic {
		return erroJWT("token mal formado (payload nao e JSON de objeto)")
	}
	agora := float64(agoraJWT().Unix())
	if exp, tem := pegaTexto(claims, "exp"); tem {
		n, ok := exp.(*object.Numero)
		if !ok {
			return erroJWT("claim exp tem que ser numero, veio %s", exp.Inspect())
		}
		if agora >= n.Value {
			return erroJWT("token expirado")
		}
	}
	if nbf, tem := pegaTexto(claims, "nbf"); tem {
		n, ok := nbf.(*object.Numero)
		if !ok {
			return erroJWT("claim nbf tem que ser numero, veio %s", nbf.Inspect())
		}
		if agora < n.Value {
			return erroJWT("token ainda nao vale (nbf no futuro)")
		}
	}
	return claims
}
