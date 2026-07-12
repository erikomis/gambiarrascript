package interpreter

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"

	"gambiarrascript/object"
)

// Lib padrão — compressao.
//
//	gzip_comprime(texto)      → texto (base64 dos bytes comprimidos com gzip)
//	gzip_descomprime(texto)   → texto original (texto base64 -> descomprime -> texto)
//
// Como a linguagem ainda nao tem tipo "bytes", o resultado da compressao e
// codificado em base64 pra poder guardar/transmitir como texto. A
// descomprime faz o caminho inverso.

// builtinGzipComprime comprime um texto com gzip e devolve a saida em base64.
func builtinGzipComprime(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("gzip_comprime() quer 1 arg (texto), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("gzip_comprime: texto esperado, veio %s", args[0].Type())
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(t.Value)); err != nil {
		return erroBuiltin("gzip_comprime falhou: %v", err)
	}
	if err := w.Close(); err != nil {
		return erroBuiltin("gzip_comprime falhou ao fechar: %v", err)
	}
	return &object.Texto{Value: base64.StdEncoding.EncodeToString(buf.Bytes())}
}

// builtinGzipDescomprime recebe um texto base64 com saida gzip_comprime e
// devolve o texto original.
func builtinGzipDescomprime(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("gzip_descomprime() quer 1 arg (texto), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("gzip_descomprime: texto esperado, veio %s", args[0].Type())
	}
	dados, err := base64.StdEncoding.DecodeString(t.Value)
	if err != nil {
		return erroBuiltin("gzip_descomprime: base64 invalido: %v", err)
	}
	r, err := gzip.NewReader(bytes.NewReader(dados))
	if err != nil {
		return erroBuiltin("gzip_descomprime: nao e gzip valido: %v", err)
	}
	defer r.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return erroBuiltin("gzip_descomprime falhou: %v", err)
	}
	return &object.Texto{Value: buf.String()}
}