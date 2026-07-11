package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"gambiarrascript/ast"
	"gambiarrascript/code"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/token"
)

// SymbolScope marca onde um simbolo vive.
type SymbolScope int

const (
	GlobalScope SymbolScope = iota
	LocalScope
	FreeScope // capturada por closure
	BuiltinScope
)

type Symbol struct {
	Name  string
	Index int
	Scope SymbolScope
}

type SymbolTable struct {
	symbols map[string]Symbol
	count   int
	outer   *SymbolTable
	free    []Symbol // freeVars coletadas
}

func NewSymbolTable() *SymbolTable {
	return &SymbolTable{symbols: map[string]Symbol{}}
}

func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	return &SymbolTable{symbols: map[string]Symbol{}, outer: outer}
}

func (s *SymbolTable) Define(nome string) Symbol {
	// Reusa o slot so quando o nome ja e uma variavel DESTE escopo (redefinir
	// `bota x` no mesmo bloco). Se o nome so existe como FREESCOPE (freevar
	// capturada de um escopo externo, registrada por Resolve), um `bota` tem que
	// criar um LOCAL novo que SOMBREIA a freevar — igual ao tree-walker. Sem
	// isso, `bota n = n + 1` numa closure escrevia na freevar em vez de criar
	// um local, e `funciona n` lia o valor errado.
	// BuiltinScope entra junto com FreeScope: um `bota`/param/`gambiarra` com
	// nome de builtin cria um binding novo que SOMBREIA o builtin — igual ao
	// tree-walker (evalIdentifier checa env antes dos builtins). Sem isso a VM
	// resolvia pro builtin e `bota soma = 0`/`gambiarra soma(...)` quebravam.
	if sym, ok := s.symbols[nome]; ok && sym.Scope != FreeScope && sym.Scope != BuiltinScope {
		return sym
	}
	sym := Symbol{Name: nome, Index: s.count, Scope: GlobalScope}
	if s.outer != nil {
		sym.Scope = LocalScope
	}
	s.symbols[nome] = sym
	s.count++
	return sym
}

// DefineBuiltin registra uma builtin por indice (resolver antes do lookup
// cair em "nao existe").
func (s *SymbolTable) DefineBuiltin(nome string, idx int) Symbol {
	sym := Symbol{Name: nome, Index: idx, Scope: BuiltinScope}
	s.symbols[nome] = sym
	return sym
}

// Resolve caminha pela cadeia de escopos. Quando um nome existe num escopo
// externo (nao global/builtin), marcamos como "free" no escopo atual — uma
// variavel capturada que vira freevar na closure. FreeScope de niveis acima
// tambem precisa ser re-exportado como freevar em cada nivel intermediario,
// senao a OpGetFree num nivel interno nao encontra o slot populado (a OpClosure
// so popula o Free do frame que ela cria — cada nivel tem que repassar).
func (s *SymbolTable) Resolve(nome string) (Symbol, bool) {
	sym, ok := s.symbols[nome]
	if ok {
		return sym, true
	}
	if s.outer == nil {
		return Symbol{}, false
	}
	outer, ok := s.outer.Resolve(nome)
	if !ok {
		return Symbol{}, false
	}
	switch outer.Scope {
	case GlobalScope, BuiltinScope:
		// globals/builtins nao precisam ser freevars: alcançamos elas direto
		// via OpGetGlobal/OpGetBuiltin no escopo interno.
		return outer, true
	default:
		// Local ou Free de nivel acima -> vira freevar AQUI tambem, pra poder
		// repassar pra dentro. Cada nivel cria seu proprio freevar e repassa
		// o valor na hora de empilhar antes de OpClosure.
		free := Symbol{Name: nome, Index: len(s.free), Scope: FreeScope}
		s.free = append(s.free, free)
		s.symbols[nome] = free
		return free, true
	}
}

func (s *SymbolTable) Free() []Symbol { return s.free }

// --- Compiler ---

type loopFrame struct {
	breakJumps    []int
	continueJumps []int
}

type Compiler struct {
	instructions code.Instructions
	constants    []object.Object
	scopes       []*SymbolTable
	scope        *SymbolTable
	loopStack    []loopFrame

	// funcoes compiladas
	compiledFns []compiledFn

	// tabela pc->linha do buffer de instrucoes ATUAL (trocada junto com
	// instructions ao compilar corpo de funcao) + linha do node sendo
	// compilado agora. Vai parar em CompiledFunction.Linhas / Bytecode.Linhas
	// pra VM reportar erro com posicao (igual o tree-walker).
	linhas     []object.LinhaPC
	linhaAtual int

	// importa (VM): diretorio base pra resolver imports e mapa de caminhos
	// absolutos ja importados (deteccao de ciclo). Quando dirBase e "" (REPL,
	// disasm ad-hoc), `importa` devolve erro explicando.
	DirBase    string
	importados map[string]bool
}

type compiledFn struct {
	name      string
	numArgs   int
	numLocals int
	bytecode  []byte
	free      []Symbol
}

func New() *Compiler {
	main := NewSymbolTable()
	c := &Compiler{scope: main, scopes: []*SymbolTable{main}}
	// registra builtins no escopo global — idx 0..N-1. A ordem aqui determina
	// o indice que a VM usa pra despachar a builtin (veja vm.Builtins()).
	for i, nome := range nomesBuiltins {
		main.DefineBuiltin(nome, i)
	}
	return c
}

// nomesBuiltins — ordem canonica. Deve casar com vm.builtinsInstanciaNames.
// Apenas nomes das builtins suportadas pela VM (as mesmas do interpreter).
var nomesBuiltins = []string{
	"tamanho", "chaves", "tem", "texto", "numero",
	"de_json", "pra_json",
	"formata",
	"separa", "junta", "maiusculo", "minusculo",
	"substitui", "fatia", "contem", "comeca_com", "termina_com", "tira_espaco",
	"adiciona", "remove", "ordena", "inverte",
	"reduz", "acha", "acha_indice", "unicos", "achatada",
	"soma", "media", "zip", "enumera", "ordena_por", "agrupa_por",
	"raiz", "aleatorio", "arredonda", "teto", "chao", "abs", "min", "max",
	"semente", "embaralha", "escolhe_um", "uuid",
	"le_arquivo", "escreve_arquivo", "anexa_arquivo",
	"existe", "eh_dir", "deleta", "cria_dir", "le_dir",
	"caminho_junta", "caminho_base", "caminho_dir", "caminho_ext", "caminho_abs",
	"quebra", "erro_msg", "erro_linha", "erro_tipo", "erro_pilha", "erro_causa", "envolve_erro",
	"mapeia", "filtra", "paralelo",
	"ordena_com",
	"pergunta", "argumentos", "le_tudo", "le_linhas", "escreve", "escreve_erro", "env",
	"espera", "afirma",
	"busca", "rota", "escuta",
	"cano", "envia", "recebe", "fecha",
	"conecta", "consulta", "executa",
	// regex
	"busca_regex", "acha_regex", "combina_regex", "substitui_regex", "separa_regex",
	// tempo
	"agora", "agora_num", "agora_ns", "formata_tempo", "parse_tempo", "duracao", "espera_ms",
	// crypto
	"md5", "sha1", "sha256", "sha512", "hmac_sha256",
	"base64_codifica", "base64_decodifica",
	"base32_codifica", "base32_decodifica",
	"hex_codifica", "hex_decodifica",
	// set
	"conjunto", "contem_conjunto", "adiciona_conjunto", "remove_conjunto",
	"uniao", "intersecao", "diferenca",
}

// BuiltinNomes expoe a lista canonica de nomes de builtins (em ordem -> idx).
// A VM usa isso pra despachar OpCallBuiltin/OpGetBuiltin.
func BuiltinNomes() []string { return nomesBuiltins }

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
	// CompiledFns  — funcoes presentes no programa (a VM copia pro pool).
	Functions []compiledFn
	// Linhas e a tabela pc->linha do fluxo principal (funcoes carregam a
	// propria tabela dentro da CompiledFunction).
	Linhas []object.LinhaPC
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.instructions,
		Constants:    c.constants,
		Functions:    c.compiledFns,
		Linhas:       c.linhas,
	}
}

func (c *Compiler) Compile(node ast.Node) error {
	return c.compile(node)
}

// linhaDe extrai a linha do token de qualquer node do AST. Todos os nodes
// tem um campo `Token token.Token`; em vez de um type-switch com 30 casos
// (que quebra silenciosamente quando nasce um node novo), usamos reflection —
// isso so roda em compile time do script, nao no hot path da VM.
func linhaDe(node ast.Node) int {
	v := reflect.ValueOf(node)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return 0
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return 0
	}
	f := v.FieldByName("Token")
	if !f.IsValid() {
		return 0
	}
	if tok, ok := f.Interface().(token.Token); ok {
		return tok.Line
	}
	return 0
}

func (c *Compiler) compile(node ast.Node) error {
	if l := linhaDe(node); l > 0 {
		c.linhaAtual = l
	}
	switch node := node.(type) {
	case *ast.Program:
		for _, s := range node.Statements {
			if err := c.compile(s); err != nil {
				return err
			}
		}
		c.emit(code.OpHalt)
	case *ast.ExpressionStatement:
		if err := c.compile(node.Expression); err != nil {
			return err
		}
		c.emit(code.OpPop)
	case *ast.MostraStatement:
		if err := c.compile(node.Value); err != nil {
			return err
		}
		c.emit(code.OpMostra)
	case *ast.NumeroLiteral:
		// Preserva flag EhInt pra VM usar aritimetica inteira exata quando
		// ambos operandos forem inteiros (evita perda de precisao em inteiros
		// grandes que nao cabem em float64 — limiar 2^53).
		idx := c.addConstant(&object.Numero{Value: node.Value, Int: node.Int, EhInt: node.EhInt})
		c.emit(code.OpConstant, idx)
	case *ast.TextoLiteral:
		idx := c.addConstant(&object.Texto{Value: node.Value})
		c.emit(code.OpConstant, idx)
	case *ast.TextoInterpolado:
		// empilha cada part; TextoLiteral => constante, expr => compilada.
		// Concatena via OpAdd (string + string).
		if len(node.Parts) == 0 {
			c.emit(code.OpConstant, c.addConstant(&object.Texto{Value: ""}))
			break
		}
		for i, p := range node.Parts {
			if err := c.compile(p); err != nil {
				return err
			}
			if i > 0 {
				c.emit(code.OpAdd)
			}
		}
	case *ast.BooleanoLiteral:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}
	case *ast.NadaLiteral:
		c.emit(code.OpNada)
	case *ast.BotaStatement:
		if node.Name != nil {
			if err := c.compile(node.Value); err != nil {
				return err
			}
			sym := c.defineVar(node.Name.Value)
			c.emitVarSet(sym)
			return nil
		}
		// atribuicao por indice: empilha cont, idx, val nessa ordem (a VM faz
		// val=pop, idx=pop, cont=pop) e emite OpIndexSet.
		if err := c.compile(node.Indice.Left); err != nil {
			return err
		}
		if err := c.compile(node.Indice.Index); err != nil {
			return err
		}
		if err := c.compile(node.Value); err != nil {
			return err
		}
		c.emit(code.OpIndexSet)
		return nil
	case *ast.Identifier:
		return c.compileIdent(node)
	case *ast.PrefixExpression:
		if err := c.compile(node.Right); err != nil {
			return err
		}
		switch node.Operator {
		case "-":
			c.emit(code.OpMinus)
		case "nao":
			c.emit(code.OpNao)
		case "~":
			c.emit(code.OpBNot)
		default:
			return fmt.Errorf("operador prefixo desconhecido na VM: %s", node.Operator)
		}
	case *ast.InfixExpression:
		return c.compileInfix(node)
	case *ast.BlockStatement:
		for _, s := range node.Statements {
			if err := c.compile(s); err != nil {
				return err
			}
		}
	case *ast.SeColarStatement:
		return c.compileSeColar(node)
	case *ast.EnquantoStatement:
		return c.compileEnquanto(node)
	case *ast.PraCadaNumStatement:
		return c.compilePraCadaNum(node)
	case *ast.PraCadaListStatement:
		return c.compilePraCadaList(node)
	case *ast.VazaStatement:
		if len(c.loopStack) == 0 {
			return fmt.Errorf("vaza fora de loop")
		}
		frame := &c.loopStack[len(c.loopStack)-1]
		jmpPos := c.emit(code.OpJump, 9999)
		frame.breakJumps = append(frame.breakJumps, jmpPos)
	case *ast.ContinuaStatement:
		if len(c.loopStack) == 0 {
			return fmt.Errorf("continua fora de loop")
		}
		frame := &c.loopStack[len(c.loopStack)-1]
		jmpPos := c.emit(code.OpJump, 9999)
		frame.continueJumps = append(frame.continueJumps, jmpPos)
	case *ast.FuncionaStatement:
		if node.Value != nil {
			if err := c.compile(node.Value); err != nil {
				return err
			}
			c.emit(code.OpReturn)
		} else {
			c.emit(code.OpReturnNada)
		}
	case *ast.GambiarraStatement:
		return c.compileGambiarra(node)
	case *ast.CallExpression:
		return c.compileCall(node)
	case *ast.ArrumaStatement:
		return c.compileArruma(node)
	case *ast.ListaLiteral:
		return c.compileLista(node)
	case *ast.DicionarioLiteral:
		return c.compileDicionario(node)
	case *ast.IndexExpression:
		if err := c.compile(node.Left); err != nil {
			return err
		}
		if err := c.compile(node.Index); err != nil {
			return err
		}
		c.emit(code.OpIndex)
	case *ast.FuncaoLiteral:
		// lambda anonima: closure fica na pilha como valor de expressao.
		return c.compileFuncaoValor("<anonima>", node.Parameters, node.Body)
	case *ast.DesestruturaStatement:
		return c.compileDesestrutura(node)
	case *ast.EscolheStatement:
		return c.compileEscolhe(node)
	case *ast.RangeExpression:
		if err := c.compile(node.Start); err != nil {
			return err
		}
		if err := c.compile(node.End); err != nil {
			return err
		}
		c.emit(code.OpRange)
	case *ast.ImportaStatement:
		return c.compileImporta(node)
	case *ast.BoraExpression:
		return c.compileBora(node)
	default:
		return fmt.Errorf("a VM ainda nao sabe compilar %T", node)
	}
	return nil
}

func (c *Compiler) compileIdent(node *ast.Identifier) error {
	sym, ok := c.scope.Resolve(node.Value)
	if !ok {
		return fmt.Errorf("VM nao conhece `%s`", node.Value)
	}
	switch sym.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobal, sym.Index)
	case LocalScope:
		c.emit(code.OpGetLocal, sym.Index)
	case FreeScope:
		c.emit(code.OpGetFree, sym.Index)
	case BuiltinScope:
		c.emit(code.OpGetBuiltin, sym.Index)
	}
	return nil
}

func (c *Compiler) emitVarSet(sym Symbol) {
	switch sym.Scope {
	case GlobalScope:
		c.emit(code.OpSetGlobal, sym.Index)
	case LocalScope:
		c.emit(code.OpSetLocal, sym.Index)
	case FreeScope:
		// assignment pra freevar: nao suportado naturalmente (closures
		// capturam valores, nao enderecos aqui). empurra via op especial;
		// por enquanto trata como SetLocal do frame atual (comum: shadow).
		// Se realmente quisermos mutar outer, precisariamos de boxes. Hoje
		// mantemos simples e consistente: escreve no local atual (pq a
		// Resolve/Define garante que se a var existe neste escopo e local).
		c.emit(code.OpSetLocal, sym.Index)
	}
}

// defineVar registra um novo simbolo no escopo atual usando Define (cresce
// indice). Nao procura outer — pra re-bota em loop reatribui no local.
func (c *Compiler) defineVar(nome string) Symbol {
	return c.scope.Define(nome)
}

func (c *Compiler) compileInfix(node *ast.InfixExpression) error {
	switch node.Operator {
	case "e":
		if err := c.compile(node.Left); err != nil {
			return err
		}
		jmpEsqFalso := c.emit(code.OpJumpIfFalse, 9999)
		if err := c.compile(node.Right); err != nil {
			return err
		}
		jmpDirFalso := c.emit(code.OpJumpIfFalse, 9999)
		c.emit(code.OpTrue)
		jmpFim := c.emit(code.OpJump, 9999)
		endFalso := len(c.instructions)
		c.emit(code.OpFalse)
		c.backpatch(jmpEsqFalso, endFalso)
		c.backpatch(jmpDirFalso, endFalso)
		c.backpatch(jmpFim, len(c.instructions))
		return nil
	case "ou":
		if err := c.compile(node.Left); err != nil {
			return err
		}
		jmpEsqTrue := c.emit(code.OpJumpIfTrue, 9999)
		if err := c.compile(node.Right); err != nil {
			return err
		}
		jmpDirTrue := c.emit(code.OpJumpIfTrue, 9999)
		c.emit(code.OpFalse)
		jmpFim := c.emit(code.OpJump, 9999)
		endTrue := len(c.instructions)
		c.emit(code.OpTrue)
		c.backpatch(jmpEsqTrue, endTrue)
		c.backpatch(jmpDirTrue, endTrue)
		c.backpatch(jmpFim, len(c.instructions))
		return nil
	case "<", "<=":
		if err := c.compile(node.Left); err != nil {
			return err
		}
		if err := c.compile(node.Right); err != nil {
			return err
		}
		if node.Operator == "<" {
			c.emit(code.OpMenor)
		} else {
			c.emit(code.OpMenorEqual)
		}
		return nil
	}
	if err := c.compile(node.Left); err != nil {
		return err
	}
	if err := c.compile(node.Right); err != nil {
		return err
	}
	switch node.Operator {
	case "+":
		c.emit(code.OpAdd)
	case "-":
		c.emit(code.OpSub)
	case "*":
		c.emit(code.OpMul)
	case "/":
		c.emit(code.OpDiv)
	case "%":
		c.emit(code.OpMod)
	case ">":
		c.emit(code.OpGreaterThan)
	case ">=":
		c.emit(code.OpGreaterEqual)
	case "==":
		c.emit(code.OpEqual)
	case "!=":
		c.emit(code.OpNotEqual)
	case "&":
		c.emit(code.OpBAnd)
	case "|":
		c.emit(code.OpBOr)
	case "^":
		c.emit(code.OpBXor)
	case "<<":
		c.emit(code.OpLShift)
	case ">>":
		c.emit(code.OpRShift)
	default:
		return fmt.Errorf("operador infixo desconhecido: %s", node.Operator)
	}
	return nil
}

func (c *Compiler) compileSeColar(node *ast.SeColarStatement) error {
	var jmpParaFim []int
	for idx := range node.Conditions {
		if err := c.compile(node.Conditions[idx]); err != nil {
			return err
		}
		jmpSeFalso := c.emit(code.OpJumpIfFalse, 9999)
		if err := c.compile(node.Consequences[idx]); err != nil {
			return err
		}
		jmpParaFim = append(jmpParaFim, c.emit(code.OpJump, 9999))
		c.backpatch(jmpSeFalso, len(c.instructions))
	}
	if node.Alternative != nil {
		if err := c.compile(node.Alternative); err != nil {
			return err
		}
	}
	for _, jmp := range jmpParaFim {
		c.backpatch(jmp, len(c.instructions))
	}
	return nil
}

func (c *Compiler) compileEnquanto(node *ast.EnquantoStatement) error {
	startPos := len(c.instructions)
	if err := c.compile(node.Condition); err != nil {
		return err
	}
	jmpSeFalso := c.emit(code.OpJumpIfFalse, 9999)
	c.pushLoop(loopFrame{})
	idx := len(c.loopStack) - 1
	if err := c.compile(node.Body); err != nil {
		c.popLoop()
		return err
	}
	frame := c.loopStack[idx]
	c.popLoop()
	c.emit(code.OpJump, startPos)
	endAddr := len(c.instructions)
	c.backpatch(jmpSeFalso, endAddr)
	for _, j := range frame.breakJumps {
		c.backpatch(j, endAddr)
	}
	for _, j := range frame.continueJumps {
		c.backpatch(j, startPos)
	}
	return nil
}

func (c *Compiler) compilePraCadaNum(node *ast.PraCadaNumStatement) error {
	if err := c.compile(node.Start); err != nil {
		return err
	}
	sym := c.defineVar(node.Var.Value)
	c.emitVarSet(sym)

	startPos := len(c.instructions)
	c.emitVarGet(sym)
	if err := c.compile(node.End); err != nil {
		return err
	}
	c.emit(code.OpGreaterThan)
	jmpFim := c.emit(code.OpJumpIfTrue, 9999)

	c.pushLoop(loopFrame{})
	idx := len(c.loopStack) - 1
	if err := c.compile(node.Body); err != nil {
		c.popLoop()
		return err
	}

	// Bloco de incremento - endereco conhecido so agora.
	// Importante: continueJumps coletados durante o body apontam pra `9999`.
	// Agora backpatchamos todos pro inicio do bloco de incremento (aqui).
	incrementoAddr := len(c.instructions)
	for _, j := range c.loopStack[idx].continueJumps {
		c.backpatch(j, incrementoAddr)
	}

	c.emitVarGet(sym)
	// incremento inteiro exato: NumInt (EhInt=true) mantem o contador em int64.
	// Com object.Numero{Value:1} (float) o `i + 1` caia no caminho float da VM
	// e o contador perdia exatidao acima de 2^53.
	c.emit(code.OpConstant, c.addConstant(object.NumInt(1)))
	c.emit(code.OpAdd)
	c.emitVarSet(sym)
	c.emit(code.OpJump, startPos)
	endAddr := len(c.instructions)
	c.backpatch(jmpFim, endAddr)
	frame := c.loopStack[idx]
	c.popLoop()
	for _, j := range frame.breakJumps {
		c.backpatch(j, endAddr)
	}
	return nil
}

// compilePraCadaList compila `pra_cada x em lista ... ` gerando um iterador:
// transforma no equivalente:
//
//	bota __it = 0
//	bota __len = tamanho(iter)
//	enquanto __it < __len
//	  bota x = iter[__it]
//	  <body>
//	  bota __it = __it + 1
//	acabou_finalmente
func (c *Compiler) compilePraCadaList(node *ast.PraCadaListStatement) error {
	// nomes unicos (nao podem colidir com user)
	const seqNome = "__seq_gs"
	const itNome = "__it_gs"
	const lenNome = "__len_gs"

	// __seq = sequencia de iteracao: elementos (lista) ou chaves (dicionario).
	// Avaliamos o iteravel UMA vez (evita reexecutar efeito colateral por
	// iteracao) e normalizamos com OpIterSeq.
	if err := c.compile(node.Iterable); err != nil {
		return err
	}
	c.emit(code.OpIterSeq)
	seqSym := c.defineVar(seqNome)
	c.emitVarSet(seqSym)

	// __it = 0  (indice inteiro exato — ver nota no loop numerico)
	c.emit(code.OpConstant, c.addConstant(object.NumInt(0)))
	itSym := c.defineVar(itNome)
	c.emitVarSet(itSym)
	// __len = tamanho(__seq)
	c.emitVarGet(seqSym)
	tamanhoSym, _ := c.scope.Resolve("tamanho")
	c.emit(code.OpCallBuiltin, tamanhoSym.Index, 1)
	lenSym := c.defineVar(lenNome)
	c.emitVarSet(lenSym)

	startPos := len(c.instructions)
	c.emitVarGet(itSym)
	c.emitVarGet(lenSym)
	c.emit(code.OpMenor)
	jmpFim := c.emit(code.OpJumpIfFalse, 9999)

	// x = __seq[__it]
	c.emitVarGet(seqSym)
	c.emitVarGet(itSym)
	c.emit(code.OpIndex)
	xSym := c.defineVar(node.Var.Value)
	c.emitVarSet(xSym)

	c.pushLoop(loopFrame{})
	idx := len(c.loopStack) - 1
	if err := c.compile(node.Body); err != nil {
		c.popLoop()
		return err
	}
	// bloco de incremento - endereço so conhecido agora
	incrementoAddr := len(c.instructions)
	for _, j := range c.loopStack[idx].continueJumps {
		c.backpatch(j, incrementoAddr)
	}
	// __it = __it + 1  (indice inteiro exato — ver nota no loop numerico)
	c.emitVarGet(itSym)
	c.emit(code.OpConstant, c.addConstant(object.NumInt(1)))
	c.emit(code.OpAdd)
	c.emitVarSet(itSym)
	c.emit(code.OpJump, startPos)
	endAddr := len(c.instructions)
	c.backpatch(jmpFim, endAddr)
	frame := c.loopStack[idx]
	c.popLoop()
	for _, j := range frame.breakJumps {
		c.backpatch(j, endAddr)
	}
	return nil
}

// compileEscolhe vira uma cadeia if-else com jumps:
//
//	<subject> -> __esc_gs
//	caso N: pra cada valor { get tmp; <valor>; OpEqual; JumpIfTrue corpoN }
//	        Jump proximoCaso
//	corpoN: <corpo>; Jump fim
//	default (se_nao_colar): <corpo>
//	fim:
func (c *Compiler) compileEscolhe(node *ast.EscolheStatement) error {
	if err := c.compile(node.Subject); err != nil {
		return err
	}
	tmp := c.defineVar("__esc_gs")
	c.emitVarSet(tmp)

	var jmpsFim []int
	for _, braco := range node.Casos {
		var jmpsCorpo []int
		for _, v := range braco.Values {
			c.emitVarGet(tmp)
			if err := c.compile(v); err != nil {
				return err
			}
			c.emit(code.OpEqual)
			jmpsCorpo = append(jmpsCorpo, c.emit(code.OpJumpIfTrue, 9999))
		}
		jmpProximo := c.emit(code.OpJump, 9999)
		corpoAddr := len(c.instructions)
		for _, j := range jmpsCorpo {
			c.backpatch(j, corpoAddr)
		}
		if err := c.compile(braco.Body); err != nil {
			return err
		}
		jmpsFim = append(jmpsFim, c.emit(code.OpJump, 9999))
		c.backpatch(jmpProximo, len(c.instructions))
	}
	if node.Default != nil {
		if err := c.compile(node.Default); err != nil {
			return err
		}
	}
	fim := len(c.instructions)
	for _, j := range jmpsFim {
		c.backpatch(j, fim)
	}
	return nil
}

// compileDesestrutura: avalia o valor UMA vez num temp e amarra cada nome via
// OpIndexOuNada (indice/chave ausente vira nada — mesma semantica lenient do
// tree-walker).
func (c *Compiler) compileDesestrutura(node *ast.DesestruturaStatement) error {
	if err := c.compile(node.Value); err != nil {
		return err
	}
	tmp := c.defineVar("__des_gs")
	c.emitVarSet(tmp)
	for idx, n := range node.Names {
		c.emitVarGet(tmp)
		if node.DeDict {
			c.emit(code.OpConstant, c.addConstant(&object.Texto{Value: n.Value}))
		} else {
			c.emit(code.OpConstant, c.addConstant(object.NumInt(int64(idx))))
		}
		c.emit(code.OpIndexOuNada)
		sym := c.defineVar(n.Value)
		c.emitVarSet(sym)
	}
	return nil
}

func (c *Compiler) compileGambiarra(node *ast.GambiarraStatement) error {
	// Reserva o simbolo da funcao ANTES de compilar o body (permite recursao).
	fnSym := c.defineVar(node.Name.Value)
	if err := c.compileFuncaoValor(node.Name.Value, node.Parameters, node.Body); err != nil {
		return err
	}
	c.emitVarSet(fnSym)
	return nil
}

// compileFuncaoValor compila params+corpo e deixa a CLOSURE no topo da pilha.
// Usado pela gambiarra nomeada (que em seguida amarra num simbolo) e pela
// lambda anonima (que usa o valor direto como expressao).
func (c *Compiler) compileFuncaoValor(nome string, params []*ast.Identifier, body *ast.BlockStatement) error {
	// Empurra um novo escopo (novo symbol table) — params viram locals.
	outer := c.scope
	newScope := NewEnclosedSymbolTable(outer)
	c.scope = newScope
	for _, p := range params {
		newScope.Define(p.Value)
	}

	// SALVA o bytecode (e a tabela de linhas) do fluxo principal e troca por
	// buffers vazios so pra compilar o body da funcao. Isso garante que os
	// opcodes do corpo NAO sejam executados como parte do programa principal.
	savedInst := c.instructions
	savedLinhas := c.linhas
	c.instructions = code.Instructions{}
	c.linhas = nil

	if err := c.compile(body); err != nil {
		c.instructions = savedInst
		c.linhas = savedLinhas
		c.scope = outer
		return err
	}
	c.emit(code.OpReturnNada)

	bc := c.instructions       // body bytecode isolado
	fnLinhas := c.linhas       // tabela pc->linha do corpo
	c.instructions = savedInst // restaura fluxo principal
	c.linhas = savedLinhas
	free := newScope.Free()
	c.scope = outer

	cf := compiledFn{
		name:      nome,
		numArgs:   len(params),
		numLocals: newScope.count,
		bytecode:  bc,
		free:      free,
	}
	c.compiledFns = append(c.compiledFns, cf)

	// empilha a CompiledFunction na pool de constantes (sem free ainda —
	// a VM vai popular `Free` em runtime quando executar OpClosure).
	fnIdx := c.addConstant(&object.CompiledFunction{
		Name:      cf.name,
		NumArgs:   cf.numArgs,
		NumLocals: cf.numLocals,
		Bytecode:  cf.bytecode,
		Free:      nil,
		Linhas:    fnLinhas,
	})
	// pra cada freevar, empilhamos o valor capturado ANTES do OpClosure.
	// o `free[i]` e um Symbol com scope=FreeScope que Resolve devolveu
	// pra esse escopo-filho; precisamos achar o simbolo "original" do
	// outer pega o valor agora. Cada free guarda o .Name original —
	// pedimos pro escopo atual (c.scope, que e o outer) resolver de novo.
	for _, fv := range free {
		// refaz o lookup no escopo externo: o freevar veio daqui (ou de
		// cima). Resolve deve devolver o mesmo Symbol do escopo externo.
		orig, ok := outer.Resolve(fv.Name)
		if !ok {
			return fmt.Errorf("freevar %q sumiu do escopo externo", fv.Name)
		}
		c.emitVarGet(orig)
	}
	c.emit(code.OpClosure, fnIdx, len(free))
	return nil
}

// compileCall: gambiarra(...) -> OpCall argc
func (c *Compiler) compileCall(node *ast.CallExpression) error {
	// avalia arguments primeiro (okers emocionantes: tem que empilhar em ordem)
	for _, a := range node.Arguments {
		if err := c.compile(a); err != nil {
			return err
		}
	}
	// agora resolve a funcao sendo chamada
	if err := c.compile(node.Function); err != nil {
		return err
	}
	c.emit(code.OpCall, len(node.Arguments))
	return nil
}

// compileBora: `bora fn(args)` dispara a chamada em paralelo e devolve Futuro.
// Mesmo empilhamento de OpCall (args primeiro, depois a fn), mas emite OpBoraCall.
func (c *Compiler) compileBora(node *ast.BoraExpression) error {
	call := node.Call
	if call == nil {
		return fmt.Errorf("bora sem chamada — isso nao devia acontecer")
	}
	for _, a := range call.Arguments {
		if err := c.compile(a); err != nil {
			return err
		}
	}
	if err := c.compile(call.Function); err != nil {
		return err
	}
	c.emit(code.OpBoraCall, len(call.Arguments))
	return nil
}

func (c *Compiler) compileArruma(node *ast.ArrumaStatement) error {
	// Layout com finally + catch opcional:
	//   OpTry <catchAddr>
	//   <try>
	//   OpTryEnd
	//   OpJump <finallyAddr>
	//   <catch>     (se houver; ErrName amarrado)
	//   OpJump <finallyAddr>
	//   finallyAddr: <finally>
	//   end:
	///
	// ErrName: no top-level (c.scope.outer == nil) amarramos a GLOBAL
	// (frame principal nao tem locals alocados na VM). Em funcoes, vira
	// local de um enclosed scope (igual antes).
	var catchScope *SymbolTable
	var errSym Symbol
	if node.Catch != nil {
		if c.scope.outer == nil {
			// top-level: amarra no proprio scope global
			if node.ErrName != nil {
				errSym = c.scope.Define(node.ErrName.Value)
			}
			catchScope = nil
		} else {
			catchScope = NewEnclosedSymbolTable(c.scope)
			if node.ErrName != nil {
				errSym = catchScope.Define(node.ErrName.Value)
			}
			c.scope = catchScope
		}
	}

	tryOp := c.emit(code.OpTry, 9999)
	if err := c.compile(node.Try); err != nil {
		if catchScope != nil {
			c.scope = catchScope.outer
		}
		return err
	}
	c.emit(code.OpTryEnd)
	jmpAposTry := c.emit(code.OpJump, 9999) // pula catch quando try OK

	catchAddr := len(c.instructions)
	c.backpatch(tryOp, catchAddr)
	if node.Catch != nil {
		// a VM empurra o erro na pilha antes de saltar pra catchAddr.
		if node.ErrName != nil {
			c.emitVarSet(errSym)
		} else {
			c.emit(code.OpPop) // descarta erro sem nome
		}
		if err := c.compile(node.Catch); err != nil {
			if catchScope != nil {
				c.scope = catchScope.outer
			}
			return err
		}
	}
	jmpAposCatch := c.emit(code.OpJump, 9999) // pula finally daqui (catch OK)
	c.backpatch(jmpAposTry, len(c.instructions))

	if catchScope != nil {
		c.scope = catchScope.outer
	}

	// finally
	if node.Finally != nil {
		c.backpatch(jmpAposCatch, len(c.instructions))
		if err := c.compile(node.Finally); err != nil {
			return err
		}
	} else {
		c.backpatch(jmpAposCatch, len(c.instructions))
	}
	return nil
}

func (c *Compiler) compileLista(node *ast.ListaLiteral) error {
	for _, e := range node.Elements {
		if err := c.compile(e); err != nil {
			return err
		}
	}
	c.emit(code.OpArray, len(node.Elements))
	return nil
}

func (c *Compiler) compileDicionario(node *ast.DicionarioLiteral) error {
	for _, par := range node.Pares {
		if err := c.compile(par.Chave); err != nil {
			return err
		}
		if err := c.compile(par.Valor); err != nil {
			return err
		}
	}
	c.emit(code.OpHash, len(node.Pares))
	return nil
}

func (c *Compiler) emitVarGet(sym Symbol) {
	switch sym.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobal, sym.Index)
	case LocalScope:
		c.emit(code.OpGetLocal, sym.Index)
	case FreeScope:
		c.emit(code.OpGetFree, sym.Index)
	case BuiltinScope:
		c.emit(code.OpGetBuiltin, sym.Index)
	}
}

// --- helpers ---

func (c *Compiler) pushLoop(f loopFrame) { c.loopStack = append(c.loopStack, f) }
func (c *Compiler) popLoop()             { c.loopStack = c.loopStack[:len(c.loopStack)-1] }

func (c *Compiler) pushFn(name string, numArgs int) {
	// placeholder — só usamos pra delimitar `startFn` via len(instructions).
}
func (c *Compiler) popFn() {}

func (c *Compiler) backpatch(jumpPos int, alvo int) {
	c.instructions[jumpPos+1] = byte(alvo >> 8)
	c.instructions[jumpPos+2] = byte(alvo)
}

func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := len(c.instructions)
	c.instructions = append(c.instructions, ins...)
	// tabela pc->linha: so grava quando a linha muda (tabela esparsa)
	if c.linhaAtual > 0 &&
		(len(c.linhas) == 0 || c.linhas[len(c.linhas)-1].Linha != c.linhaAtual) {
		c.linhas = append(c.linhas, object.LinhaPC{PC: pos, Linha: c.linhaAtual})
	}
	return pos
}

// compileImporta resolve o caminho relativo ao dirBase, le o arquivo, faz
// parse e compila cada statement do modulo INLINE no mesmo Compiler. Assim
// as globals definidas no modulo (`bota`, `gambiarra`) passam a existir no
// programa principal e a VM as acessa via OpGetGlobal. Imports recursivos
// sao detidos via mapa de caminhos ja visitados (ciclo vira no-op).
// Suportamos somente caminho literal de texto (`importa "x.gs"`).
func (c *Compiler) compileImporta(node *ast.ImportaStatement) error {
	tx, ok := node.Path.(*ast.TextoLiteral)
	if !ok {
		return fmt.Errorf("importa na VM so aceita texto literal (veio %T)", node.Path)
	}
	resolvido := tx.Value
	if !filepath.IsAbs(resolvido) && c.DirBase != "" {
		resolvido = filepath.Join(c.DirBase, resolvido)
	}
	if c.importados == nil {
		c.importados = map[string]bool{}
	}
	if c.importados[resolvido] {
		return nil // ja importado — ciclo
	}
	c.importados[resolvido] = true

	fonte, err := os.ReadFile(resolvido)
	if err != nil {
		return fmt.Errorf("importa: nao consegui ler %q: %v", tx.Value, err)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return fmt.Errorf("importa: modulo %q com perrengue: %s", tx.Value, errs[0])
	}

	dirAntes := c.DirBase
	c.DirBase = filepath.Dir(resolvido)
	for _, s := range prog.Statements {
		if err := c.compile(s); err != nil {
			c.DirBase = dirAntes
			return err
		}
	}
	c.DirBase = dirAntes
	return nil
}
