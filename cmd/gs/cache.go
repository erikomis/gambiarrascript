package main

import (
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"os"
	"sort"

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
	gob.Register(&object.Modulo{})
	// POO: descritores de treta/combinado/literal
	gob.Register(&object.DescTreta{})
	gob.Register(&object.DescCombinado{})
	gob.Register(&object.DescLiteral{})
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
// 8 = trava e com_trava no fim da lista.
// 9 = gera_certificado no fim da lista.
// 10 = OpImporta: modulo compilado a parte (descritor object.Modulo), roda uma
// vez so; o .gsc guarda o hash de cada modulo importado.
// 11 = escopo de funcao (OpGet*Ou/OpGet*Chk, celulas), estouro de inteiro
// vira real no folding, traco com frame de builtin.
// 12 = POO: OpTreta/OpCombinado/OpMetodo/OpInstancia (descritores no pool) e
// builtins satisfaz/como_tipo no fim da lista.
// 13 = multi-catch: varios `quebrou NOME se COND` (temporario do erro +
// OpJumpIfFalse por filtro e OpThrow quando nenhum cola).
// 14 = MaxStack: teto da pilha de operandos de cada gambiarra (campo novo na
// CompiledFunction) e do fluxo principal (cacheGSC.MaxStack). Cache velho
// viria com 0 em tudo e cairia no caminho checado da VM — funciona, mas
// invalida pra nao rodar lento a toa.
// 15 = builtins migra e valida no fim da lista.
// 16 = a_cada, depois_de, agenda, cancela, formata_data e le_data no fim da
// lista de builtins.
// 17 = templates: renderiza, renderiza_arquivo e responde_html no fim da lista.
const formatoGSC = 17

type cacheGSC struct {
	Formato      int      // formatoGSC de quem gravou (cache sem o campo = 0)
	Versao       string   // versao do gs que gravou (invalida em upgrade)
	NumBuiltins  int      // guarda contra mudanca nos indices de builtin
	HashFonte    [32]byte // sha256 da fonte
	Constants    []object.Object
	Instructions []byte
	Linhas       []object.LinhaPC // tabela pc->linha do fluxo principal
	NumGlobals   int              // quantas globais o programa declara
	MaxStack     int              // teto da pilha do fluxo principal
	// HashModulos: sha256 da fonte de cada modulo importado (caminho
	// absoluto). Modulo que mudou (ou sumiu) invalida o cache — antes so a
	// fonte principal contava e o .gsc rodava o modulo velho.
	HashModulos map[string][32]byte
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
	modulos := make([]string, 0, len(c.HashModulos))
	for caminho, hash := range c.HashModulos {
		fonteMod, err := object.LeModulo(caminho)
		if err != nil || sha256.Sum256(fonteMod) != hash {
			return nil
		}
		modulos = append(modulos, caminho)
	}
	sort.Strings(modulos)
	return &compiler.Bytecode{
		Instructions: code.Instructions(c.Instructions),
		Constants:    c.Constants,
		Linhas:       c.Linhas,
		NumGlobals:   c.NumGlobals,
		MaxStack:     c.MaxStack,
		Modulos:      modulos,
	}
}

// gravaCache serializa o bytecode no .gsc. Falha e so aviso — cache e
// otimizacao, nao requisito.
func gravaCache(caminhoGSC string, fonte []byte, bc *compiler.Bytecode) {
	if len(bc.Sitios) > 0 {
		return // bytecode instrumentado (OpLinha) nunca vai pro cache
	}
	hashModulos := map[string][32]byte{}
	for _, caminho := range bc.Modulos {
		fonteMod, err := object.LeModulo(caminho)
		if err != nil {
			return // modulo sumiu entre compilar e gravar: nao vale cachear
		}
		hashModulos[caminho] = sha256.Sum256(fonteMod)
	}
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
		MaxStack:     bc.MaxStack,
		HashModulos:  hashModulos,
	}
	if err := gob.NewEncoder(f).Encode(&c); err != nil {
		fmt.Fprintf(os.Stderr, "aviso: cache nao gravado: %v\n", err)
	}
}
