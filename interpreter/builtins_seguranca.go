package interpreter

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/scrypt"

	"gambiarrascript/object"
)

// Lib padrao — seguranca (crypto parte 2). md5/sha* sao pra checksum; pra
// senha e segredo de verdade o caminho e esse aqui:
//
//	hash_senha(senha, [custo])   → hash bcrypt (texto "$2a$12$...")
//	confere_senha(senha, hash)   → booleano (hash lixo = deu_ruim, sem crash)
//	encripta(texto, chave)       → base64 de AES-256-GCM (nonce aleatorio)
//	decripta(cifrado, chave)     → texto (chave errada/adulterado = erro)
//	gera_chave()                 → chave aleatoria de 32 bytes em base64
//	token_aleatorio([bytes=32])  → base64url sem padding (crypto/rand)

// custoSenhaPadrao e o custo do bcrypt quando ninguem fala nada. 12 = ~250ms
// num notebook de 2024: lento pra quem chuta senha em massa, rapido o bastante
// pra um login. Da pra passar outro (4..31) no 2o arg do hash_senha.
const custoSenhaPadrao = 12

// limiteSenhaBcrypt: o bcrypt so olha os primeiros 72 bytes. Em vez de cortar
// calado (duas senhas diferentes virando o mesmo hash), recusa.
const limiteSenhaBcrypt = 72

func builtinHashSenha(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("hash_senha() quer (senha, [custo]), veio %d", len(args))
	}
	senha, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("hash_senha: senha tem que ser texto, veio %s", args[0].Type())
	}
	custo := custoSenhaPadrao
	if len(args) == 2 {
		n, ok := args[1].(*object.Numero)
		if !ok || !ehInteiro(n) {
			return erroBuiltin("hash_senha: custo tem que ser numero inteiro, veio %s", args[1].Inspect())
		}
		custo = int(n.Value)
		if custo < bcrypt.MinCost || custo > bcrypt.MaxCost {
			return erroBuiltin("hash_senha: custo vai de %d a %d, veio %d", bcrypt.MinCost, bcrypt.MaxCost, custo)
		}
	}
	if len(senha.Value) > limiteSenhaBcrypt {
		return erroBuiltin("hash_senha: senha passa de %d bytes (limite do bcrypt), veio %d", limiteSenhaBcrypt, len(senha.Value))
	}
	h, err := bcrypt.GenerateFromPassword([]byte(senha.Value), custo)
	if err != nil {
		return erroBuiltin("hash_senha falhou: %v", err)
	}
	return &object.Texto{Value: string(h)}
}

// builtinConfereSenha compara a senha com o hash do bcrypt. A comparacao do
// bcrypt ja e em tempo constante; qualquer problema com o hash (lixo, formato
// desconhecido, custo zoado) vira deu_ruim — quem chama so quer saber se loga.
func builtinConfereSenha(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("confere_senha() quer (senha, hash), veio %d", len(args))
	}
	senha, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("confere_senha: senha tem que ser texto, veio %s", args[0].Type())
	}
	hash, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("confere_senha: hash tem que ser texto, veio %s", args[1].Type())
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash.Value), []byte(senha.Value))
	return boolDoNativo(err == nil)
}

// ---- AES-256-GCM ----

const (
	tamChaveAES  = 32 // AES-256
	tamSalScrypt = 16
)

// parametros do scrypt pra chave que e senha/frase (N=2^15, r=8, p=1: o
// recomendado pra uso interativo — ~50ms e 32 MiB por derivacao).
const (
	scryptN = 1 << 15
	scryptR = 8
	scryptP = 1
)

// msgDecriptaFalhou e a mensagem unica pra chave errada e texto adulterado:
// o GCM nao distingue os dois (e nem deve — seria um oraculo pra atacante).
const msgDecriptaFalhou = "nao deu pra decriptar: chave errada ou texto adulterado"

// chaveCrua tenta ler a chave como 32 bytes "de verdade": 64 caracteres hex
// ou base64 (padrao ou url, com ou sem '=') de exatamente 32 bytes — o que o
// gera_chave() devolve. Se nao tiver cara de chave, devolve nil e quem chama
// trata como senha/frase (deriva com scrypt + sal aleatorio).
func chaveCrua(chave string) []byte {
	if len(chave) == 64 {
		if b, err := hex.DecodeString(chave); err == nil {
			return b
		}
	}
	if len(chave) == 43 || len(chave) == 44 {
		sem := strings.TrimRight(chave, "=")
		for _, enc := range []*base64.Encoding{base64.RawStdEncoding, base64.RawURLEncoding} {
			if b, err := enc.DecodeString(sem); err == nil && len(b) == tamChaveAES {
				return b
			}
		}
	}
	return nil
}

func derivaChave(senha string, sal []byte) ([]byte, error) {
	return scrypt.Key([]byte(senha), sal, scryptN, scryptR, scryptP, tamChaveAES)
}

func novoGCM(chave []byte) (cipher.AEAD, error) {
	bloco, err := aes.NewCipher(chave)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(bloco)
}

func argsCripto(nome string, args []object.Object) (string, string, *object.Erro) {
	if len(args) != 2 {
		return "", "", erroBuiltin("%s() quer (texto, chave), veio %d", nome, len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return "", "", erroBuiltin("%s: 1o arg tem que ser texto, veio %s", nome, args[0].Type())
	}
	k, ok := args[1].(*object.Texto)
	if !ok {
		return "", "", erroBuiltin("%s: chave tem que ser texto, veio %s", nome, args[1].Type())
	}
	if k.Value == "" {
		return "", "", erroBuiltin("%s: chave vazia nao rola (usa gera_chave())", nome)
	}
	return t.Value, k.Value, nil
}

// builtinEncripta: AES-256-GCM com nonce aleatorio. Saida em base64 padrao:
//
//	chave crua (gera_chave, 64 hex):  nonce(12) || cifrado+tag
//	chave senha/frase:                sal(16) || nonce(12) || cifrado+tag
//
// Com senha o sal vai junto pra cada mensagem ter chave propria e o scrypt
// nao virar tabela pronta. Custa ~50ms por chamada de proposito; em caminho
// quente (toda requisicao) usa uma chave do gera_chave().
func builtinEncripta(args []object.Object) object.Object {
	texto, chave, erro := argsCripto("encripta", args)
	if erro != nil {
		return erro
	}
	var prefixo []byte
	k := chaveCrua(chave)
	if k == nil {
		sal := make([]byte, tamSalScrypt)
		if _, err := rand.Read(sal); err != nil {
			return erroBuiltin("encripta: sem aleatoriedade no sistema: %v", err)
		}
		var err error
		if k, err = derivaChave(chave, sal); err != nil {
			return erroBuiltin("encripta: derivar a chave falhou: %v", err)
		}
		prefixo = sal
	}
	gcm, err := novoGCM(k)
	if err != nil {
		return erroBuiltin("encripta falhou: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return erroBuiltin("encripta: sem aleatoriedade no sistema: %v", err)
	}
	saida := append(prefixo, nonce...)
	saida = gcm.Seal(saida, nonce, []byte(texto), nil)
	return &object.Texto{Value: base64.StdEncoding.EncodeToString(saida)}
}

func builtinDecripta(args []object.Object) object.Object {
	cifrado, chave, erro := argsCripto("decripta", args)
	if erro != nil {
		return erro
	}
	dados, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cifrado))
	if err != nil {
		return erroBuiltin("decripta: isso nao e texto cifrado (base64 invalido)")
	}
	k := chaveCrua(chave)
	if k == nil {
		if len(dados) < tamSalScrypt {
			return erroBuiltin(msgDecriptaFalhou)
		}
		if k, err = derivaChave(chave, dados[:tamSalScrypt]); err != nil {
			return erroBuiltin("decripta: derivar a chave falhou: %v", err)
		}
		dados = dados[tamSalScrypt:]
	}
	gcm, err := novoGCM(k)
	if err != nil {
		return erroBuiltin("decripta falhou: %v", err)
	}
	if len(dados) < gcm.NonceSize()+gcm.Overhead() {
		return erroBuiltin(msgDecriptaFalhou)
	}
	nonce, corpo := dados[:gcm.NonceSize()], dados[gcm.NonceSize():]
	claro, err := gcm.Open(nil, nonce, corpo, nil)
	if err != nil {
		return erroBuiltin(msgDecriptaFalhou)
	}
	return &object.Texto{Value: string(claro)}
}

// builtinGeraChave devolve 32 bytes aleatorios em base64 (44 caracteres) —
// formato que o encripta/decripta usa direto, sem scrypt.
func builtinGeraChave(args []object.Object) object.Object {
	if len(args) != 0 {
		return erroBuiltin("gera_chave() nao quer argumento, veio %d", len(args))
	}
	b, err := bytesAleatorios(tamChaveAES)
	if err != nil {
		return erroBuiltin("gera_chave: %v", err)
	}
	return &object.Texto{Value: base64.StdEncoding.EncodeToString(b)}
}

// limiteTokenAleatorio e so pra ninguem pedir 1 GB de token sem querer.
const limiteTokenAleatorio = 4096

// builtinTokenAleatorio devolve n bytes (default 32) do crypto/rand em
// base64url sem padding: serve em URL, cookie e cabecalho sem escapar nada.
// Diferente do uuid()/aleatorio(), que vem do gerador semeavel (previsivel).
func builtinTokenAleatorio(args []object.Object) object.Object {
	if len(args) > 1 {
		return erroBuiltin("token_aleatorio() quer 0 ou 1 arg (bytes), veio %d", len(args))
	}
	n := 32
	if len(args) == 1 {
		num, ok := args[0].(*object.Numero)
		if !ok || !ehInteiro(num) {
			return erroBuiltin("token_aleatorio: quantos bytes tem que ser numero inteiro, veio %s", args[0].Inspect())
		}
		n = int(num.Value)
		if n < 1 || n > limiteTokenAleatorio {
			return erroBuiltin("token_aleatorio: bytes vai de 1 a %d, veio %d", limiteTokenAleatorio, n)
		}
	}
	b, err := bytesAleatorios(n)
	if err != nil {
		return erroBuiltin("token_aleatorio: %v", err)
	}
	return &object.Texto{Value: base64.RawURLEncoding.EncodeToString(b)}
}

func bytesAleatorios(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, errors.New("sem aleatoriedade no sistema: " + err.Error())
	}
	return b, nil
}

// ehInteiro: numero sem parte fracionaria (inteiro exato ou float redondo).
func ehInteiro(n *object.Numero) bool {
	if n.EhInt {
		return true
	}
	return n.Value == float64(int64(n.Value))
}
