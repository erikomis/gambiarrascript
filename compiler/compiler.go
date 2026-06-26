package compiler

import (
	"fmt"

	"gambiarrascript/ast"
	"gambiarrascript/code"
	"gambiarrascript/object"
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
	if sym, ok := s.symbols[nome]; ok {
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
// externo (nao global), marcamos como "free" no escopo atual — uma variavel
// capturada que vira freevar na closure.
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
	// outer existe — vira freevar neste escopo
	switch outer.Scope {
	case GlobalScope, BuiltinScope:
		// globals/builtins nao precisam ser freevars: alcançamos elas direto
		// via OpGetGlobal/OpGetBuiltin no escopo interno.
		return outer, true
	case FreeScope:
		// ja e freevar num nivel acima — continua freevar
		return outer, true
	default:
		// Local do escopo externo -> vira freevar aqui
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
	"separa", "junta", "maiusculo", "minusculo",
	"substitui", "fatia", "contem", "comeca_com", "termina_com", "tira_espaco",
	"adiciona", "remove", "ordena", "inverte",
	"raiz", "aleatorio", "arredonda", "teto", "chao", "abs", "min", "max",
	"le_arquivo", "escreve_arquivo", "anexa_arquivo",
	"quebra", "erro_msg", "erro_linha", "erro_tipo", "erro_pilha", "erro_causa", "envolve_erro",
	"mapeia", "filtra", "paralelo",
	"pergunta", "argumentos", "le_tudo", "le_linhas", "escreve", "escreve_erro", "env",
	"espera", "afirma",
	"busca", "rota", "escuta",
}

// BuiltinNomes expoe a lista canonica de nomes de builtins (em ordem -> idx).
// A VM usa isso pra despachar OpCallBuiltin/OpGetBuiltin.
func BuiltinNomes() []string { return nomesBuiltins }

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
	// CompiledFns  — funcoes presentes no programa (a VM copia pro pool).
	Functions []compiledFn
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.instructions,
		Constants:    c.constants,
		Functions:    c.compiledFns,
	}
}

func (c *Compiler) Compile(node ast.Node) error {
	return c.compile(node)
}

func (c *Compiler) compile(node ast.Node) error {
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
	case *ast.BooleanoLiteral:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}
	case *ast.NadaLiteral:
		c.emit(code.OpNada)
	case *ast.BotaStatement:
		if err := c.compile(node.Value); err != nil {
			return err
		}
		if node.Name != nil {
			sym := c.defineVar(node.Name.Value)
			c.emitVarSet(sym)
			return nil
		}
		// atribuicao por indice: bota lista[i] = v -> eval left, idx, val, OpIndexSet
		if err := c.compile(node.Indice.Left); err != nil {
			return err
		}
		if err := c.compile(node.Indice.Index); err != nil {
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
	case *ast.ImportaStatement:
		// importa na VM: hoje totalmente禁ado — dependemos do tree-walker
		// pra imports. A VM nao enxerga o sistema de arquivos.
		return fmt.Errorf("a VM ainda nao faz `importa` (use sem --vm)")
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
	c.emit(code.OpConstant, c.addConstant(&object.Numero{Value: 1}))
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
//   bota __it = 0
//   bota __len = tamanho(iter)
//   enquanto __it < __len
//     bota x = iter[__it]
//     <body>
//     bota __it = __it + 1
//   acabou_finalmente
func (c *Compiler) compilePraCadaList(node *ast.PraCadaListStatement) error {
	// empilha iteravel
	if err := c.compile(node.Iterable); err != nil {
		return err
	}
	// __iter e __len: nomes unicos (nao podem colidir com user)
	const itNome = "__it_gs"
	const lenNome = "__len_gs"
	// __iter = 0
	c.emit(code.OpConstant, c.addConstant(&object.Numero{Value: 0}))
	itSym := c.defineVar(itNome)
	c.emitVarSet(itSym)
	// __len = tamanho(iteravel)  --> chama builtin tamanho
	// (mais simples: empilha o iteravel de novo e OpCallBuiltin tamanho idx)
	if err := c.compile(node.Iterable); err != nil {
		return err
	}
	// builtin tamanho — qual idx?
	tamanhoSym, _ := c.scope.Resolve("tamanho")
	c.emit(code.OpCallBuiltin, tamanhoSym.Index, 1)
	lenSym := c.defineVar(lenNome)
	c.emitVarSet(lenSym)

	startPos := len(c.instructions)
	c.emitVarGet(itSym)
	c.emitVarGet(lenSym)
	c.emit(code.OpMenor)
	jmpFim := c.emit(code.OpJumpIfFalse, 9999)

	// x = iteravel[__it]
	if err := c.compile(node.Iterable); err != nil {
		return err
	}
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
	// __it = __it + 1
	c.emitVarGet(itSym)
	c.emit(code.OpConstant, c.addConstant(&object.Numero{Value: 1}))
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

func (c *Compiler) compileGambiarra(node *ast.GambiarraStatement) error {
	// Reserva o simbolo da funcao ANTES de compilar o body (permite recursao).
	fnSym := c.defineVar(node.Name.Value)

	// Empurra um novo escopo (novo symbol table) — params viram locals.
	outer := c.scope
	newScope := NewEnclosedSymbolTable(outer)
	c.scope = newScope
	for _, p := range node.Parameters {
		newScope.Define(p.Value)
	}

	// SALVA o bytecode do fluxo principal e troca por um buffer vazio so
	// pra compilar o body da funcao. Isso garante que os opcodes do corpo
	// NAO sejam executados como parte do programa principal.
	savedInst := c.instructions
	c.instructions = code.Instructions{}

	if err := c.compile(node.Body); err != nil {
		c.instructions = savedInst
		c.scope = outer
		return err
	}
	c.emit(code.OpReturnNada)

	bc := c.instructions               // body bytecode isolado
	c.instructions = savedInst         // restaura fluxo principal
	free := newScope.Free()
	c.scope = outer

	cf := compiledFn{
		name:      node.Name.Value,
		numArgs:   len(node.Parameters),
		numLocals: newScope.count,
		bytecode:  bc,
		free:      free,
	}
	c.compiledFns = append(c.compiledFns, cf)

	// empilha a CompiledFunction como constante; emite OpClosure pro
	// fluxo principal e atribui à variável global.
	fnIdx := c.addConstant(&object.CompiledFunction{
		Name:      cf.name,
		NumArgs:   cf.numArgs,
		NumLocals: cf.numLocals,
		Bytecode:  cf.bytecode,
		Free:      nil,
	})
	c.emit(code.OpClosure, fnIdx)
	c.emitVarSet(fnSym)
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

func (c *Compiler) compileArruma(node *ast.ArrumaStatement) error {
	// errSym tem que aparecer no escopo do catch
	// compilacao:
	//   OpTry <catchAddr>
	//   <try>
	//   OpTryEnd
	//   OpJump <end>
	//   <catch>   (ErrName como local 0 deste escopo filho)
	//   end:
	catchScope := NewEnclosedSymbolTable(c.scope)
	errSym := catchScope.Define(node.ErrName.Value)
	c.scope = catchScope

	tryOp := c.emit(code.OpTry, 9999)
	if err := c.compile(node.Try); err != nil {
		c.scope = catchScope.outer
		return err
	}
	c.emit(code.OpTryEnd)
	jmpFim := c.emit(code.OpJump, 9999)
	catchAddr := len(c.instructions)
	c.backpatch(tryOp, catchAddr)
	// binding do erro capturado: a VM empurra o erro na pilha antes de saltar
	// pra catchAddr. Salvamos em local.
	c.emitVarSet(errSym)
	if err := c.compile(node.Catch); err != nil {
		c.scope = catchScope.outer
		return err
	}
	end := len(c.instructions)
	c.backpatch(jmpFim, end)
	c.scope = catchScope.outer
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
	return pos
}