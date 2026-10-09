//go:build !js

package interpreter

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gambiarrascript/object"
)

// Sessao sem banco: usa_sessao({"segredo": ...}) faz de pedido["sessao"] um
// dicionario que volta no proximo pedido. Ele mora inteiro num cookie
// assinado com HMAC-SHA256 (e, com "encripta": deu_bom, cifrado com
// AES-256-GCM — sem isso o usuario LE o conteudo, so nao consegue mudar).
// Cookie adulterado, de outro segredo ou vencido = sessao vazia, sem erro.
//
// Formato do cookie: base64url(carga) "." base64url(hmac), com
// carga = "<unix>|<json>" (ou nonce||AES-GCM disso). O unix e a hora em que o
// cookie foi emitido: passou de expira_em, nao vale mais, mesmo que o
// navegador insista em mandar.

const (
	nomeSessaoPadrao   = "gs_sessao"
	expiraSessaoPadrao = 7 * 24 * 60 * 60 // 7 dias
	// tamMaxCookie: o navegador joga fora cookie maior que ~4 KB calado.
	tamMaxCookie = 4000
)

type configSessao struct {
	nome     string
	chaveMAC []byte
	chaveAES []byte // nil = sem criptografia
	expira   int64  // segundos
	modelo   http.Cookie
}

// derivaChaveSessao separa o segredo em chaves independentes por uso.
func derivaChaveSessao(segredo, uso string) []byte {
	m := hmac.New(sha256.New, []byte(segredo))
	m.Write([]byte("gambiarrascript/sessao/" + uso))
	return m.Sum(nil)
}

func (s *servidorEstado) builtinUsaSessao(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("usa_sessao() quer 1 argumento (opcoes com \"segredo\"), veio %d", len(args))
	}
	d, ok := args[0].(*object.Dicionario)
	if !ok {
		return erroBuiltin("usa_sessao(): as opcoes tem que ser dicionario, tipo {\"segredo\": env(\"SEGREDO\")}, veio %s", object.NomeTipo(args[0]))
	}
	cfg := &configSessao{nome: nomeSessaoPadrao, expira: expiraSessaoPadrao}
	cfg.modelo = http.Cookie{Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode}
	resto := object.NovoDicionario()
	cripto := false
	for _, par := range d.Pares() {
		k, _ := par.Chave.(*object.Texto)
		switch {
		case k != nil && k.Value == "nome":
			t, _ := par.Valor.(*object.Texto)
			if t == nil || t.Value == "" {
				return erroBuiltin("usa_sessao(): \"nome\" tem que ser texto nao vazio, veio %s", par.Valor.Inspect())
			}
			cfg.nome = t.Value
		case k != nil && k.Value == "encripta":
			b, _ := par.Valor.(*object.Booleano)
			if b == nil {
				return erroBuiltin("usa_sessao(): \"encripta\" tem que ser deu_bom ou deu_ruim, veio %s", par.Valor.Inspect())
			}
			cripto = b.Value
		case k != nil && k.Value == "expira_em":
			n, _ := par.Valor.(*object.Numero)
			if n == nil || !ehInteiro(n) || n.Value < 1 {
				return erroBuiltin("usa_sessao(): \"expira_em\" tem que ser um numero inteiro de segundos maior que 0, veio %s", par.Valor.Inspect())
			}
			cfg.expira = int64(n.Value)
		default:
			resto.Bota(par.Chave.(object.Chaveavel).ChaveHash(), par)
		}
	}
	segredo, erro := leOpcoesCookie("usa_sessao", resto, &cfg.modelo, true)
	if erro != nil {
		return erro
	}
	if segredo == "" {
		return erroBuiltin("usa_sessao(): falta o \"segredo\" (texto de pelo menos %d caracteres; usa gera_chave() e guarda no .env)", tamMinSegredo)
	}
	cfg.modelo.Name = cfg.nome
	if (&http.Cookie{Name: cfg.nome, Value: "x"}).Valid() != nil {
		return erroBuiltin("usa_sessao(): %q nao serve de nome de cookie", cfg.nome)
	}
	cfg.modelo.MaxAge = int(cfg.expira)
	cfg.chaveMAC = derivaChaveSessao(segredo, "mac")
	if cripto {
		cfg.chaveAES = derivaChaveSessao(segredo, "aes")
	}
	s.mu.Lock()
	s.sessao = cfg
	s.mu.Unlock()
	return NADA
}

// sessaoCarregada e o que o pedido trouxe: o json original (pra saber se
// mudou) e quando o cookie foi emitido.
type sessaoCarregada struct {
	cfg      *configSessao
	original string // "" = nao veio cookie valido
	emitido  int64
}

// carrega le o cookie do pedido. Qualquer coisa errada = sessao vazia.
func (cfg *configSessao) carrega(r *http.Request) (*object.Dicionario, *sessaoCarregada) {
	sc := &sessaoCarregada{cfg: cfg}
	vazia := object.NovoDicionario()
	c, err := r.Cookie(cfg.nome)
	if err != nil {
		return vazia, sc
	}
	i := strings.LastIndexByte(c.Value, '.')
	if i < 0 {
		return vazia, sc
	}
	carga64, ass64 := c.Value[:i], c.Value[i+1:]
	m := hmac.New(sha256.New, cfg.chaveMAC)
	m.Write([]byte(cfg.nome + "=" + carga64))
	ass, err := base64.RawURLEncoding.DecodeString(ass64)
	if err != nil || !hmac.Equal(ass, m.Sum(nil)) {
		return vazia, sc
	}
	carga, err := base64.RawURLEncoding.DecodeString(carga64)
	if err != nil {
		return vazia, sc
	}
	if cfg.chaveAES != nil {
		gcm, err := novoGCM(cfg.chaveAES)
		if err != nil || len(carga) < gcm.NonceSize() {
			return vazia, sc
		}
		carga, err = gcm.Open(nil, carga[:gcm.NonceSize()], carga[gcm.NonceSize():], []byte(cfg.nome))
		if err != nil {
			return vazia, sc
		}
	}
	unix, js, ok := strings.Cut(string(carga), "|")
	if !ok {
		return vazia, sc
	}
	emitido, err := strconv.ParseInt(unix, 10, 64)
	if err != nil || time.Now().Unix()-emitido > cfg.expira {
		return vazia, sc
	}
	v, err := parseJson(js)
	d, ehDic := v.(*object.Dicionario)
	if err != nil || !ehDic {
		return vazia, sc
	}
	sc.original, sc.emitido = js, emitido
	return d, sc
}

// cookie monta o valor assinado (e cifrado) do json.
func (cfg *configSessao) cookie(js string) (string, error) {
	carga := []byte(strconv.FormatInt(time.Now().Unix(), 10) + "|" + js)
	if cfg.chaveAES != nil {
		gcm, err := novoGCM(cfg.chaveAES)
		if err != nil {
			return "", err
		}
		nonce := make([]byte, gcm.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return "", err
		}
		carga = gcm.Seal(nonce, nonce, carga, []byte(cfg.nome))
	}
	carga64 := base64.RawURLEncoding.EncodeToString(carga)
	m := hmac.New(sha256.New, cfg.chaveMAC)
	m.Write([]byte(cfg.nome + "=" + carga64))
	return carga64 + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil)), nil
}

// salva olha o pedido["sessao"] depois da rota/depois() e, se mudou (ou ta
// na metade da validade), manda o Set-Cookie novo. Sessao esvaziada (ou
// trocada por nada) apaga o cookie. Erro = 500 (detalhe no stderr).
func (sc *sessaoCarregada) salva(pedido *object.Dicionario, resp *respostaHTTP) *object.Erro {
	cfg := sc.cfg
	v, _ := dicPega(pedido, "sessao")
	var js string
	switch d := v.(type) {
	case *object.Dicionario:
		var buf bytes.Buffer
		if e := escreveJson(&buf, d); e != nil {
			return erroBuiltin("pedido.sessao nao virou json (so da pra guardar texto, numero, booleano, nada, lista e dicionario): %s", e.Message)
		}
		js = buf.String()
	case *object.Nada, nil:
		js = "{}"
	default:
		return erroBuiltin("pedido.sessao tem que ser dicionario (ou nada pra sair), virou %s", object.NomeTipo(v))
	}
	c := cfg.modelo
	switch {
	case js == "{}" && sc.original == "":
		return nil // nunca teve sessao e continua sem
	case js == "{}":
		c.MaxAge = -1 // apaga
	case js == sc.original && time.Now().Unix()-sc.emitido < cfg.expira/2:
		return nil // nada mudou e o cookie ainda ta novo
	default:
		valor, err := cfg.cookie(js)
		if err != nil {
			return erroBuiltin("usa_sessao: nao consegui montar o cookie: %v", err)
		}
		if len(valor) > tamMaxCookie {
			return erroBuiltin("pedido.sessao ficou grande demais pro cookie (%d bytes, o navegador aguenta uns %d): guarda so o id e o resto no banco", len(valor), tamMaxCookie)
		}
		c.Value = valor
	}
	resp.cabecalhos.Add("Set-Cookie", c.String())
	return nil
}
