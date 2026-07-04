package interpreter

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"strings"

	"gambiarrascript/object"
)

// Lib padrão — crypto / codificação.
//
//	md5(texto)                 → hash em hex
//	sha1(texto)               → hash em hex
//	sha256(texto)             → hash em hex
//	sha512(texto)             → hash em hex
//	hmac_sha256(chave, texto) → em hex
//	base64_codifica(texto)    → base64
//	base64_decodifica(texto)  → texto
//	base32_codifica(texto)    → base32
//	base32_decodifica(texto)  → texto
//	hex_codifica(texto)       → hex
//	hex_decodifica(texto)     → texto

func builtinMd5(args []object.Object) object.Object {
	return hashHex(md5.New(), args, "md5")
}
func builtinSha1(args []object.Object) object.Object {
	return hashHex(sha1.New(), args, "sha1")
}
func builtinSha256(args []object.Object) object.Object {
	return hashHex(sha256.New(), args, "sha256")
}
func builtinSha512(args []object.Object) object.Object {
	return hashHex(sha512.New(), args, "sha512")
}

func builtinHmacSha256(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("hmac_sha256() quer (chave, texto), veio %d", len(args))
	}
	t1, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("hmac_sha256: chave como texto, veio %s", args[0].Type())
	}
	t2, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("hmac_sha256: texto como texto, veio %s", args[1].Type())
	}
	mac := hmac.New(sha256.New, []byte(t1.Value))
	mac.Write([]byte(t2.Value))
	return &object.Texto{Value: hex.EncodeToString(mac.Sum(nil))}
}

func hashHex(h hash.Hash, args []object.Object, nome string) object.Object {
	if len(args) != 1 {
		return erroBuiltin("%s() quer 1 (texto), veio %d", nome, len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("%s: texto esperado, veio %s", nome, args[0].Type())
	}
	h.Write([]byte(t.Value))
	return &object.Texto{Value: hex.EncodeToString(h.Sum(nil))}
}

func builtinBase64Codifica(args []object.Object) object.Object {
	return codifica(args, "base64_codifica", base64.StdEncoding.EncodeToString)
}
func builtinBase64Decodifica(args []object.Object) object.Object {
	return decodifica(args, "base64_decodifica", base64.StdEncoding.DecodeString)
}
func builtinBase32Codifica(args []object.Object) object.Object {
	return codifica(args, "base32_codifica", base32.StdEncoding.EncodeToString)
}
func builtinBase32Decodifica(args []object.Object) object.Object {
	return decodifica(args, "base32_decodifica", base32.StdEncoding.DecodeString)
}
func builtinHexCodifica(args []object.Object) object.Object {
	return codifica(args, "hex_codifica", hex.EncodeToString)
}
func builtinHexDecodifica(args []object.Object) object.Object {
	return decodifica(args, "hex_decodifica", func(s string) ([]byte, error) {
		return hex.DecodeString(s)
	})
}

func codifica(args []object.Object, nome string, fn func([]byte) string) object.Object {
	if len(args) != 1 {
		return erroBuiltin("%s() quer 1 (texto), veio %d", nome, len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("%s: texto esperado, veio %s", nome, args[0].Type())
	}
	return &object.Texto{Value: fn([]byte(t.Value))}
}

func decodifica(args []object.Object, nome string, fn func(string) ([]byte, error)) object.Object {
	if len(args) != 1 {
		return erroBuiltin("%s() quer 1 (texto), veio %d", nome, len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("%s: texto esperado, veio %s", nome, args[0].Type())
	}
	b, err := fn(t.Value)
	if err != nil {
		return erroBuiltin("%s falhou: %v", nome, err)
	}
	return &object.Texto{Value: string(b)}
}

// usado pra evitar imports sem uso em builds seletivos
var _ = strings.TrimSpace
