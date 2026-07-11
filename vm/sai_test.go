package vm

import (
	"bytes"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

// rodaVMSai compila e roda; devolve (saida, codigoDeSaida, saiu?).
func rodaVMSai(t *testing.T, src string) (string, int, bool) {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	var buf bytes.Buffer
	maq := New(comp.Bytecode(), &buf)
	err := maq.Run()
	if sr, ok := err.(SaiRequisicao); ok {
		return buf.String(), sr.Codigo, true
	}
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	return buf.String(), 0, false
}

func TestVMSaiParaEDevolveCodigo(t *testing.T) {
	saida, codigo, saiu := rodaVMSai(t, `mostra "antes"
sai(7)
mostra "depois"`)
	if !saiu || codigo != 7 {
		t.Fatalf("esperava sai com codigo 7, veio saiu=%v codigo=%d", saiu, codigo)
	}
	if saida != "antes\n" {
		t.Fatalf("depois de sai nao devia rodar; saida=%q", saida)
	}
}

func TestVMSaiDentroDeFuncao(t *testing.T) {
	saida, codigo, saiu := rodaVMSai(t, `gambiarra f()
    sai(2)
acabou_finalmente
mostra "a"
f()
mostra "b"`)
	if !saiu || codigo != 2 {
		t.Fatalf("esperava sai 2 de dentro da funcao, veio saiu=%v codigo=%d", saiu, codigo)
	}
	if saida != "a\n" {
		t.Fatalf("desenrolar da funcao falhou; saida=%q", saida)
	}
}

func TestVMSaiDefaultZero(t *testing.T) {
	_, codigo, saiu := rodaVMSai(t, `sai()`)
	if !saiu || codigo != 0 {
		t.Fatalf("sai() default: saiu=%v codigo=%d", saiu, codigo)
	}
}
