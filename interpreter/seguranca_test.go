package interpreter

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"gambiarrascript/object"
)

func txt(s string) *object.Texto { return &object.Texto{Value: s} }

func precisaTexto(t *testing.T, o object.Object) string {
	t.Helper()
	tx, ok := o.(*object.Texto)
	if !ok {
		t.Fatalf("esperava texto, veio %s: %s", o.Type(), o.Inspect())
	}
	return tx.Value
}

func precisaErro(t *testing.T, o object.Object, kind, trecho string) {
	t.Helper()
	e, ok := o.(*object.Erro)
	if !ok {
		t.Fatalf("esperava erro %q, veio %s: %s", trecho, o.Type(), o.Inspect())
	}
	if e.Kind != kind || !strings.Contains(e.Message, trecho) {
		t.Fatalf("erro errado: kind=%q msg=%q (queria kind=%q contendo %q)", e.Kind, e.Message, kind, trecho)
	}
}

func TestHashSenhaEConfere(t *testing.T) {
	h := precisaTexto(t, builtinHashSenha([]object.Object{txt("hunter2"), object.NumInt(4)}))
	if !strings.HasPrefix(h, "$2a$04$") || len(h) != 60 {
		t.Fatalf("hash bcrypt estranho: %q", h)
	}
	casos := []struct {
		senha, hash string
		ok          bool
	}{
		{"hunter2", h, true},
		{"hunter3", h, false},
		{"hunter2", "lixo", false},
		{"hunter2", "", false},
		{"hunter2", "$2a$99$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", false},
		// vetor conhecido (OpenBSD / passlib)
		{"U*U", "$2a$05$CCCCCCCCCCCCCCCCCCCCC.E5YPO9kmyuRGyh0XouQYb4YMJKvyOeW", true},
		{"U*U*", "$2a$05$CCCCCCCCCCCCCCCCCCCCC.VGOzA784oUp/Z0DY336zx7pLYAy0lwK", true},
		// $2y$ (PHP/htpasswd) tambem
		{"U*U", "$2y$05$CCCCCCCCCCCCCCCCCCCCC.E5YPO9kmyuRGyh0XouQYb4YMJKvyOeW", true},
	}
	for _, c := range casos {
		res := builtinConfereSenha([]object.Object{txt(c.senha), txt(c.hash)})
		if b, ok := res.(*object.Booleano); !ok || b.Value != c.ok {
			t.Errorf("confere_senha(%q, %q) = %s, queria %v", c.senha, c.hash, res.Inspect(), c.ok)
		}
	}
	// custo padrao e 12
	if custoSenhaPadrao != 12 {
		t.Fatalf("custo padrao mudou: %d", custoSenhaPadrao)
	}
	precisaErro(t, builtinHashSenha([]object.Object{txt(strings.Repeat("a", 73))}), KindBuiltin, "72 bytes")
	precisaErro(t, builtinHashSenha([]object.Object{txt("x"), object.NumInt(3)}), KindBuiltin, "custo vai de 4 a 31")
	precisaErro(t, builtinHashSenha([]object.Object{object.NumInt(1)}), KindBuiltin, "senha tem que ser texto")
	precisaErro(t, builtinConfereSenha([]object.Object{txt("x")}), KindBuiltin, "quer (senha, hash)")
}

func TestEncriptaDecripta(t *testing.T) {
	chave := precisaTexto(t, builtinGeraChave(nil))
	if b, err := base64.StdEncoding.DecodeString(chave); err != nil || len(b) != 32 {
		t.Fatalf("gera_chave nao e base64 de 32 bytes: %q", chave)
	}
	hex64 := strings.Repeat("ab", 32)
	frase := "minha frase de passe"
	for _, k := range []string{chave, hex64, frase, strings.TrimRight(chave, "=")} {
		for _, msg := range []string{"", "salve", "ç acentuado \n e quebra", strings.Repeat("x", 5000)} {
			c := precisaTexto(t, builtinEncripta([]object.Object{txt(msg), txt(k)}))
			c2 := precisaTexto(t, builtinEncripta([]object.Object{txt(msg), txt(k)}))
			if c == c2 {
				t.Fatalf("nonce repetido: duas encriptacoes iguais com a chave %q", k)
			}
			if got := precisaTexto(t, builtinDecripta([]object.Object{txt(c), txt(k)})); got != msg {
				t.Fatalf("round-trip com chave %q: %q != %q", k, got, msg)
			}
			if k == frase && len(msg) > 100 {
				break // scrypt e lento de proposito; um texto grande basta
			}
		}
	}

	c := precisaTexto(t, builtinEncripta([]object.Object{txt("segredo"), txt(chave)}))
	// chave errada
	precisaErro(t, builtinDecripta([]object.Object{txt(c), txt(hex64)}), KindBuiltin,
		"nao deu pra decriptar: chave errada ou texto adulterado")
	// adulterado: vira 1 bit no meio do cifrado
	raw, _ := base64.StdEncoding.DecodeString(c)
	raw[len(raw)/2] ^= 1
	precisaErro(t, builtinDecripta([]object.Object{txt(base64.StdEncoding.EncodeToString(raw)), txt(chave)}),
		KindBuiltin, "chave errada ou texto adulterado")
	// curto demais, base64 lixo, chave vazia
	precisaErro(t, builtinDecripta([]object.Object{txt("AAAA"), txt(chave)}), KindBuiltin, "texto adulterado")
	precisaErro(t, builtinDecripta([]object.Object{txt("%%%"), txt(chave)}), KindBuiltin, "base64 invalido")
	precisaErro(t, builtinEncripta([]object.Object{txt("x"), txt("")}), KindBuiltin, "chave vazia")
	// texto cifrado com frase nao abre com chave crua (e vice-versa)
	cf := precisaTexto(t, builtinEncripta([]object.Object{txt("x"), txt(frase)}))
	precisaErro(t, builtinDecripta([]object.Object{txt(cf), txt(frase + "!")}), KindBuiltin, "chave errada")
}

func TestChaveCrua(t *testing.T) {
	b32 := make([]byte, 32)
	for i := range b32 {
		b32[i] = byte(i * 7)
	}
	crua := []string{
		hex.EncodeToString(b32),
		strings.ToUpper(hex.EncodeToString(b32)),
		base64.StdEncoding.EncodeToString(b32),
		base64.RawStdEncoding.EncodeToString(b32),
		base64.URLEncoding.EncodeToString(b32),
		base64.RawURLEncoding.EncodeToString(b32),
	}
	for _, k := range crua {
		if got := chaveCrua(k); string(got) != string(b32) {
			t.Errorf("chaveCrua(%q) nao leu os 32 bytes", k)
		}
	}
	for _, k := range []string{"senha", strings.Repeat("z", 64), base64.StdEncoding.EncodeToString(b32[:31]), ""} {
		if chaveCrua(k) != nil {
			t.Errorf("chaveCrua(%q) devia cair no scrypt", k)
		}
	}
}

func TestTokenAleatorio(t *testing.T) {
	vistos := map[string]bool{}
	for i := 0; i < 50; i++ {
		tok := precisaTexto(t, builtinTokenAleatorio(nil))
		if len(tok) != 43 || strings.ContainsAny(tok, "+/=") {
			t.Fatalf("token padrao estranho: %q", tok)
		}
		if vistos[tok] {
			t.Fatal("token repetido")
		}
		vistos[tok] = true
	}
	if tok := precisaTexto(t, builtinTokenAleatorio([]object.Object{object.NumInt(16)})); len(tok) != 22 {
		t.Fatalf("16 bytes -> 22 chars, veio %q", tok)
	}
	precisaErro(t, builtinTokenAleatorio([]object.Object{object.NumInt(0)}), KindBuiltin, "vai de 1 a")
	precisaErro(t, builtinTokenAleatorio([]object.Object{object.NumFloat(1.5)}), KindBuiltin, "inteiro")
}

// token de exemplo do jwt.io (HS256, segredo "your-256-bit-secret")
const jwtIO = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

func congelaJWT(t *testing.T, unix int64) {
	t.Helper()
	antes := agoraJWT
	agoraJWT = func() time.Time { return time.Unix(unix, 0) }
	t.Cleanup(func() { agoraJWT = antes })
}

func tokenNaMao(cab, payload, segredo string) string {
	b := base64.RawURLEncoding
	conteudo := b.EncodeToString([]byte(cab)) + "." + b.EncodeToString([]byte(payload))
	mac := hmac.New(sha256.New, []byte(segredo))
	mac.Write([]byte(conteudo))
	return conteudo + "." + b.EncodeToString(mac.Sum(nil))
}

func TestJWTVetorConhecido(t *testing.T) {
	// assina igualzinho o jwt.io (header compacto alg,typ + payload na ordem)
	tok := eval(t, `jwt_assina({"sub": "1234567890", "name": "John Doe", "iat": 1516239022}, "your-256-bit-secret")`)
	if precisaTexto(t, tok) != jwtIO {
		t.Fatalf("jwt_assina divergiu do jwt.io:\n%s\n%s", tok.Inspect(), jwtIO)
	}
	res := builtinJwtConfere([]object.Object{txt(jwtIO), txt("your-256-bit-secret")})
	if res.Inspect() != `{"sub": "1234567890", "name": "John Doe", "iat": 1516239022}` {
		t.Fatalf("claims: %s", res.Inspect())
	}
}

func TestJWTRecusas(t *testing.T) {
	congelaJWT(t, 1_000_000)
	seg := "segredo"
	confere := func(tok string) object.Object {
		return builtinJwtConfere([]object.Object{txt(tok), txt(seg)})
	}
	valido := precisaTexto(t, builtinJwtAssina([]object.Object{
		eval(t, `{"u": 1}`), txt(seg), eval(t, `{"expira_em": 60}`)}))
	if got := confere(valido).Inspect(); got != `{"u": 1, "iat": 1000000, "exp": 1000060}` {
		t.Fatalf("claims com expira_em: %s", got)
	}

	partes := strings.Split(valido, ".")
	adulterado := partes[0] + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"u":2,"exp":9999999999}`)) + "." + partes[2]

	casos := []struct{ nome, tok, trecho string }{
		{"segredo errado", tokenNaMao(`{"alg":"HS256"}`, `{}`, "outro"), "assinatura invalida"},
		{"payload adulterado", adulterado, "assinatura invalida"},
		{"alg none", base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." + partes[1] + ".", "alg none nao rola"},
		{"alg none com assinatura de HS256", tokenNaMao(`{"alg":"none"}`, `{}`, seg), "alg none nao rola"},
		{"alg HS512", tokenNaMao(`{"alg":"HS512"}`, `{}`, seg), "alg HS512 nao rola"},
		{"sem alg", tokenNaMao(`{"typ":"JWT"}`, `{}`, seg), "alg nenhum nao rola"},
		{"expirado", tokenNaMao(`{"alg":"HS256"}`, `{"exp":1000000}`, seg), "token expirado"},
		{"nbf no futuro", tokenNaMao(`{"alg":"HS256"}`, `{"nbf":1000001}`, seg), "ainda nao vale"},
		{"exp texto", tokenNaMao(`{"alg":"HS256"}`, `{"exp":"amanha"}`, seg), "exp tem que ser numero"},
		{"payload lista", tokenNaMao(`{"alg":"HS256"}`, `[1]`, seg), "payload nao e JSON de objeto"},
		{"cabecalho lixo", tokenNaMao(`nao e json`, `{}`, seg), "cabecalho nao e JSON"},
		{"2 partes", "a.b", "3 partes"},
		{"vazio", "", "3 partes"},
		{"base64 zoado", "!!!.b.c", "cabecalho nao e base64url"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) { precisaErro(t, confere(c.tok), KindJWT, c.trecho) })
	}
	// exp/nbf no limite certo
	if res := confere(tokenNaMao(`{"alg":"HS256"}`, `{"exp":1000001,"nbf":1000000}`, seg)); res.Type() != object.DICIONARIO_OBJ {
		t.Fatalf("token no limite devia valer: %s", res.Inspect())
	}
	// erro de uso (nao de token) continua "builtin"
	precisaErro(t, builtinJwtConfere([]object.Object{object.NumInt(1), txt(seg)}), KindBuiltin, "token tem que ser texto")
	precisaErro(t, builtinJwtAssina([]object.Object{eval(t, `[1]`), txt(seg)}), KindBuiltin, "dados tem que ser dicionario")
	precisaErro(t, builtinJwtAssina([]object.Object{eval(t, `{}`), txt("")}), KindBuiltin, "segredo vazio")
	precisaErro(t, builtinJwtAssina([]object.Object{eval(t, `{}`), txt(seg), eval(t, `{"expira": 1}`)}), KindBuiltin, "opcao desconhecida")
	// jwt_assina nao mexe no dicionario de quem chamou
	d := eval(t, `{"a": 1}`)
	builtinJwtAssina([]object.Object{d, txt(seg), eval(t, `{"expira_em": 5}`)})
	if d.Inspect() != `{"a": 1}` {
		t.Fatalf("jwt_assina mexeu nos dados: %s", d.Inspect())
	}
}
