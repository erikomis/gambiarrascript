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

// formatoGSC e a versao do formato do bytecode. Sobe toda vez que mudar
// opcode (numeracao ou semantica): .gsc velho deixa de valer mesmo que a
// Versao do gs nao tenha mudado (build de dev). 2 = OpPow (`**`).
// 3 = codegen novo de `??`, arruma no escopo da funcao e pra_cada com
// contador escondido (bytecode velho tem os bugs). 4 = OpCallEspalha/
// OpBoraEspalha, formato na interpolacao e builtin tipo. 5 = builtins de
// rede (conecta_tcp, escuta_tcp, endereco, escuta_udp, envia_udp, conecta_udp).
// 6 = seguranca/log/flags/.env no fim da lista (hash_senha..carrega_env).
// 7 = servidor parte 2 e websocket (responde_json..conecta_ws).
const formatoGSC = 7

type cacheGSC struct {
	Formato      int      // formatoGSC de quem gravou (cache sem o campo = 0)
	Versao       string   // versao do gs que gravou (invalida em upgrade)
	NumBuiltins  int      // guarda contra mudanca nos indices de builtin
	HashFonte    [32]byte // sha256 da fonte
	Constants    []object.Object
	Instructions []byte
	Linhas       []object.LinhaPC // tabela pc->linha do fluxo principal
	NumGlobals   int              // quantas globais o programa declara
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
	if c.Formato != formatoGSC || c.Versao != Versao || c.NumBuiltins != len(compiler.BuiltinNomes()) {
		return nil
	}
	if c.HashFonte != sha256.Sum256(fonte) {
		return nil
	}
	return &compiler.Bytecode{
		Instructions: code.Instructions(c.Instructions),
		Constants:    c.Constants,
		Linhas:       c.Linhas,
		NumGlobals:   c.NumGlobals,
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
		Formato:      formatoGSC,
		Versao:       Versao,
		NumBuiltins:  len(compiler.BuiltinNomes()),
		HashFonte:    sha256.Sum256(fonte),
		Constants:    bc.Constants,
		Instructions: []byte(bc.Instructions),
		Linhas:       bc.Linhas,
		NumGlobals:   bc.NumGlobals,
	}
	if err := gob.NewEncoder(f).Encode(&c); err != nil {
		fmt.Fprintf(os.Stderr, "aviso: cache nao gravado: %v\n", err)
	}
}
