package main

import (
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"os"

	"gambiarrascript/code"
	"gambiarrascript/compiler"
	"gambiarrascript/object"
)

// Cache de bytecode (.gsc): `gs roda --vm --cache arquivo.gs` grava o
// bytecode compilado ao lado da fonte e reusa enquanto a fonte (e a versao
// do gs) nao mudar. Vale a pena pra scripts grandes chamados toda hora.

func init() {
	// tipos concretos que aparecem em Bytecode.Constants
	gob.Register(&object.Numero{})
	gob.Register(&object.Texto{})
	gob.Register(&object.Booleano{})
	gob.Register(&object.Nada{})
	gob.Register(&object.CompiledFunction{})
}

type cacheGSC struct {
	Versao       string   // versao do gs que gravou (invalida em upgrade)
	NumBuiltins  int      // guarda contra mudanca nos indices de builtin
	HashFonte    [32]byte // sha256 da fonte
	Constants    []object.Object
	Instructions []byte
	Linhas       []object.LinhaPC // tabela pc->linha do fluxo principal
}

// carregaCache tenta ler um .gsc valido pro arquivo/fonte. Devolve nil se
// nao existir ou estiver invalido (fonte mudou, versao diferente...).
func carregaCache(caminhoGSC string, fonte []byte) *compiler.Bytecode {
	f, err := os.Open(caminhoGSC)
	if err != nil {
		return nil
	}
	defer f.Close()
	var c cacheGSC
	if err := gob.NewDecoder(f).Decode(&c); err != nil {
		return nil
	}
	if c.Versao != Versao || c.NumBuiltins != len(compiler.BuiltinNomes()) {
		return nil
	}
	if c.HashFonte != sha256.Sum256(fonte) {
		return nil
	}
	return &compiler.Bytecode{
		Instructions: code.Instructions(c.Instructions),
		Constants:    c.Constants,
		Linhas:       c.Linhas,
	}
}

// gravaCache serializa o bytecode no .gsc. Falha e so aviso — cache e
// otimizacao, nao requisito.
func gravaCache(caminhoGSC string, fonte []byte, bc *compiler.Bytecode) {
	f, err := os.Create(caminhoGSC)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aviso: nao consegui gravar o cache %s: %v\n", caminhoGSC, err)
		return
	}
	defer f.Close()
	c := cacheGSC{
		Versao:       Versao,
		NumBuiltins:  len(compiler.BuiltinNomes()),
		HashFonte:    sha256.Sum256(fonte),
		Constants:    bc.Constants,
		Instructions: []byte(bc.Instructions),
		Linhas:       bc.Linhas,
	}
	if err := gob.NewEncoder(f).Encode(&c); err != nil {
		fmt.Fprintf(os.Stderr, "aviso: cache nao gravado: %v\n", err)
	}
}
