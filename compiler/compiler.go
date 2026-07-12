package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

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

	// interning de constantes escalares (Numero/Texto/Booleano): mesma
	// constante literal repetida reusa o mesmo indice no pool.
	constDedupe map[string]int

	// funcAtual: nome da funcao sendo compilada (pra detectar self-tail-call).
	funcAtual string
}

type compiledFn struct {
	name      string
	numArgs   int
	minArgs   int // argumentos requeridos (sem default e sem varargs)
	numLocals int
	bytecode  []byte
	free      []Symbol
	variadic  bool
}

func New() *Compiler {
	main := NewSymbolTable()
	c := &Compiler{scope: main, scopes: []*SymbolTable{main}, constDedupe: map[string]int{}}
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
	"copia", "move", "tamanho_arquivo", "modificado_em", "glob",
	"caminho_junta", "caminho_base", "caminho_dir", "caminho_ext", "caminho_abs",
	"quebra", "erro_msg", "erro_linha", "erro_tipo", "erro_pilha", "erro_causa", "envolve_erro",
	"mapeia", "filtra", "paralelo",
	"ordena_com",
	"pergunta", "argumentos", "le_tudo", "le_linhas", "escreve", "escreve_erro", "env",
	"roda_comando", "sai",
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
	// datas parte 2
	"soma_tempo", "sub_tempo", "dia_da_semana", "diferenca_dias", "diferenca_horas", "converte_tz",
	// csv
	"le_csv", "escreve_csv",
	// compressao
	"gzip_comprime", "gzip_descomprime",
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
			// tail call: `funciona f(args)` DENTRO de uma funcao vira OpTailCall,
			// que reusa o frame atual — recursao em cauda nao estoura os frames.
			if call, ok := node.Value.(*ast.CallExpression); ok && ehSelfCall(call, c.funcAtual) {
				for _, a := range call.Arguments {
					if err := c.compile(a); err != nil {
						return err
					}
				}
				if err := c.compile(call.Function); err != nil {
					return err
				}
				c.emit(code.OpTailCall, len(call.Arguments))
			} else {
				if err := c.compile(node.Value); err != nil {
					return err
				}
				c.emit(code.OpReturn)
			}
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
		if node.Safe {
			// obj?.campo — se left for nada, empilha nada e pula
			c.emit(code.OpDup)
			c.emit(code.OpIsNada)
			jmpNada := c.emit(code.OpJumpIfTrue, 9999)
			// left ja esta na pilha (OpIsNada consumiu o dup, OpJumpIfTrue o bool);
			// nao ha OpPop aqui — o left e justamente o que o OpIndex precisa.
			if err := c.compile(node.Index); err != nil {
				return err
			}
			c.emit(code.OpIndex)
			jmpFim := c.emit(code.OpJump, 9999)
			c.backpatch(jmpNada, len(c.instructions))
			c.emit(code.OpPop) // descarta o dup
			c.emit(code.OpNada)
			c.backpatch(jmpFim, len(c.instructions))
		} else {
			if err := c.compile(node.Index); err != nil {
				return err
			}
			c.emit(code.OpIndex)
		}
	case *ast.FatiaExpression:
		return c.compileFatia(node)
	case *ast.TernarioExpression:
		if err := c.compile(node.Cond); err != nil {
			return err
		}
		jmpF := c.emit(code.OpJumpIfFalse, 9999)
		if err := c.compile(node.SeVerdadeiro); err != nil {
			return err
		}
		jmpFim := c.emit(code.OpJump, 9999)
		c.backpatch(jmpF, len(c.instructions))
		if err := c.compile(node.SeFalso); err != nil {
			return err
		}
		c.backpatch(jmpFim, len(c.instructions))
	case *ast.CoalesceExpression:
		if err := c.compile(node.Left); err != nil {
			return err
		}
		c.emit(code.OpDup)
		c.emit(code.OpIsNada)
		jmpNada := c.emit(code.OpJumpIfTrue, 9999)
		// nao e nada: mantem left (ja na pilha 2x), descarta o dup
		c.emit(code.OpPop)
		jmpFim := c.emit(code.OpJump, 9999)
		c.backpatch(jmpNada, len(c.instructions))
		// e nada: descarta o dup e empilha right
		c.emit(code.OpPop)
		if err := c.compile(node.Right); err != nil {
			return err
		}
		c.backpatch(jmpFim, len(c.instructions))
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
	// constant folding: se a expressao inteira e uma constante segura, emite
	// uma constante so (ex.: `2 + 3` vira OpConstant 5).
	if v, ok := dobraConstante(node); ok {
		c.emit(code.OpConstant, c.addConstant(v))
		return nil
	}
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
	const seqNome = "__seq_gs"
	const itNome = "__it_gs"
	const lenNome = "__len_gs"
	const origNome = "__orig_gs"

	doisNomes := len(node.Vars) == 2

	// __orig = iteravel original; __seq = OpIterSeq(orig) (elementos p/ lista,
	// chaves p/ dict). Ambos usados quando ha 2 nomes (OpIterPar precisa do
	// original pra resolver o valor de um dict).
	if err := c.compile(node.Iterable); err != nil {
		return err
	}
	origSym := c.defineVar(origNome)
	c.emitVarSet(origSym)
	c.emitVarGet(origSym)
	c.emit(code.OpIterSeq)
	seqSym := c.defineVar(seqNome)
	c.emitVarSet(seqSym)

	c.emit(code.OpConstant, c.addConstant(object.NumInt(0)))
	itSym := c.defineVar(itNome)
	c.emitVarSet(itSym)
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

	if doisNomes {
		// OpIterPar: pop __it, pop __seq, pop __orig -> push (key, value)
		c.emitVarGet(origSym)
		c.emitVarGet(seqSym)
		c.emitVarGet(itSym)
		c.emit(code.OpIterPar)
		// pilha: [key, value] (value no topo)
		v2Sym := c.defineVar(node.Vars[1].Value)
		c.emitVarSet(v2Sym)
		v1Sym := c.defineVar(node.Vars[0].Value)
		c.emitVarSet(v1Sym)
	} else {
		// x = __seq[__it]
		c.emitVarGet(seqSym)
		c.emitVarGet(itSym)
		c.emit(code.OpIndex)
		xSym := c.defineVar(node.Vars[0].Value)
		c.emitVarSet(xSym)
	}

	c.pushLoop(loopFrame{})
	idx := len(c.loopStack) - 1
	if err := c.compile(node.Body); err != nil {
		c.popLoop()
		return err
	}
	incrementoAddr := len(c.instructions)
	for _, j := range c.loopStack[idx].continueJumps {
		c.backpatch(j, incrementoAddr)
	}
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

// compilePraCadaList original replaced by implementation above.

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
func (c *Compiler) compileFuncaoValor(nome string, params []*ast.Parametro, body *ast.BlockStatement) error {
	funcSalva := c.funcAtual
	c.funcAtual = nome
	defer func() { c.funcAtual = funcSalva }()
	// Empurra um novo escopo (novo symbol table) — params viram locals.
	outer := c.scope
	newScope := NewEnclosedSymbolTable(outer)
	c.scope = newScope
	paramSyms := make([]Symbol, len(params))
	minArgs := 0
	temVariadic := false
	for i, p := range params {
		paramSyms[i] = newScope.Define(p.Nome.Value)
		if p.Variadico {
			temVariadic = true
		} else if p.Padrao == nil {
			minArgs++
		}
	}

	// SALVA o bytecode (e a tabela de linhas) do fluxo principal e troca por
	// buffers vazios so pra compilar o body da funcao.
	savedInst := c.instructions
	savedLinhas := c.linhas
	c.instructions = code.Instructions{}
	c.linhas = nil

	// Prologo: pra cada param com valor padrao, se o slot veio NADA (nao
	// preenchido pela VM), substitui pelo default. A VM poe NADA nos slots
	// nao fornecidos (ver OpCall: padding com NADA quando argc < NumArgs).
	for i, p := range params {
		if p.Padrao == nil || p.Variadico {
			continue
		}
		// if param[i] == nada then param[i] = default
		c.emit(code.OpGetLocal, paramSyms[i].Index)
		c.emit(code.OpNada)
		c.emit(code.OpEqual)
		jmpSkip := c.emit(code.OpJumpIfFalse, 9999)
		// avalia o default no escopo da funcao (podereferenciar params anteriores)
		if err := c.compile(p.Padrao); err != nil {
			c.instructions = savedInst
			c.linhas = savedLinhas
			c.scope = outer
			return err
		}
		c.emit(code.OpSetLocal, paramSyms[i].Index)
		c.backpatch(jmpSkip, len(c.instructions))
	}

	if err := c.compile(body); err != nil {
		c.instructions = savedInst
		c.linhas = savedLinhas
		c.scope = outer
		return err
	}
	c.emit(code.OpReturnNada)

	bc := c.instructions       // body bytecode isolado (com prologo)
	fnLinhas := c.linhas       // tabela pc->linha do corpo
	c.instructions = savedInst // restaura fluxo principal
	c.linhas = savedLinhas
	free := newScope.Free()
	c.scope = outer

	cf := compiledFn{
		name:      nome,
		numArgs:   len(params),
		minArgs:   minArgs,
		numLocals: newScope.count,
		bytecode:  bc,
		free:      free,
		variadic:  temVariadic,
	}
	c.compiledFns = append(c.compiledFns, cf)

	// empilha a CompiledFunction na pool de constantes (sem free ainda —
	// a VM vai popular `Free` em runtime quando executar OpClosure).
	fnIdx := c.addConstant(&object.CompiledFunction{
		Name:      cf.name,
		NumArgs:   cf.numArgs,
		MinArgs:   cf.minArgs,
		NumLocals: cf.numLocals,
		Bytecode:  cf.bytecode,
		Free:      nil,
		Linhas:    fnLinhas,
		Variadic:  cf.variadic,
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
	// interning: constante escalar identica reusa o indice existente.
	if k, ok := chaveConstante(obj); ok {
		if idx, existe := c.constDedupe[k]; existe {
			return idx
		}
		c.constants = append(c.constants, obj)
		idx := len(c.constants) - 1
		c.constDedupe[k] = idx
		return idx
	}
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

// chaveConstante devolve uma chave canonica pra interning de constantes
// escalares. Inteiro e float sao chaves distintas (a VM usa aritimetica
// diferente pra cada), assim como textos e booleanos.
func chaveConstante(obj object.Object) (string, bool) {
	switch o := obj.(type) {
	case *object.Numero:
		if o.EhInt {
			return "i" + strconv.FormatInt(o.Int, 10), true
		}
		return "f" + strconv.FormatFloat(o.Value, 'g', -1, 64), true
	case *object.Texto:
		return "s" + o.Value, true
	case *object.Booleano:
		if o.Value {
			return "b1", true
		}
		return "b0", true
	}
	return "", false
}

// ehSelfCall diz se `call` e uma chamada direta a `nome` (a propria funcao).
// So nesse caso emitimos OpTailCall: recursao em cauda roda em profundidade
// constante SEM perder o traco de pilha de chamadas entre funcoes diferentes
// (essas continuam empilhando frame).
func ehSelfCall(call *ast.CallExpression, nome string) bool {
	if nome == "" || nome == "<anonima>" {
		return false
	}
	id, ok := call.Function.(*ast.Identifier)
	return ok && id.Value == nome
}

// dobraConstante avalia em tempo de compilacao expressoes 100% constantes
// (literais + operacoes SEGURAS), pra emitir uma constante so em vez de
// OpConstant/OpConstant/OpAdd. So dobra o que casa byte-a-byte com o runtime;
// qualquer duvida (divisao, overflow, tipo, float) devolve (nil,false) e a
// expressao compila normal — mantendo a paridade com o tree-walker.
func dobraConstante(expr ast.Expression) (object.Object, bool) {
	switch n := expr.(type) {
	case *ast.NumeroLiteral:
		return &object.Numero{Value: n.Value, Int: n.Int, EhInt: n.EhInt}, true
	case *ast.TextoLiteral:
		return &object.Texto{Value: n.Value}, true
	case *ast.InfixExpression:
		l, ok := dobraConstante(n.Left)
		if !ok {
			return nil, false
		}
		r, ok := dobraConstante(n.Right)
		if !ok {
			return nil, false
		}
		return dobraInfixo(n.Operator, l, r)
	}
	return nil, false
}

// dobraInfixo computa uma op binaria de constantes so nos casos garantidamente
// iguais ao runtime: texto+texto e inteiro exato +/-/* sem overflow (mesma
// deteccao do vmExecBinarioIntShort). Divisao, modulo, float, comparacao,
// bitwise e logico NAO sao dobrados (ficam pro runtime).
func dobraInfixo(op string, l, r object.Object) (object.Object, bool) {
	if lt, ok := l.(*object.Texto); ok {
		rt, ok := r.(*object.Texto)
		if ok && op == "+" {
			return &object.Texto{Value: lt.Value + rt.Value}, true
		}
		return nil, false
	}
	ln, lok := l.(*object.Numero)
	rn, rok := r.(*object.Numero)
	if !lok || !rok || !ln.EhInt || !rn.EhInt {
		return nil, false
	}
	switch op {
	case "+":
		res := ln.Int + rn.Int
		if (ln.Int > 0 && rn.Int > 0 && res < 0) || (ln.Int < 0 && rn.Int < 0 && res > 0) {
			return nil, false // overflow: deixa a VM cair no float
		}
		return object.NumInt(res), true
	case "-":
		res := ln.Int - rn.Int
		if (ln.Int > 0 && rn.Int < 0 && res < 0) || (ln.Int < 0 && rn.Int > 0 && res > 0) {
			return nil, false
		}
		return object.NumInt(res), true
	case "*":
		if ln.Int == 0 || rn.Int == 0 {
			return object.NumInt(0), true
		}
		res := ln.Int * rn.Int
		if res/rn.Int != ln.Int {
			return nil, false
		}
		return object.NumInt(res), true
	}
	return nil, false
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

	// registra nomes globais antes de compilar o modulo (pra saber quais
	// variaveis novas vieram dele — usado em `importa ... como alias`)
	globaisAntes := map[string]Symbol{}
	for k, v := range c.scope.symbols {
		if v.Scope == GlobalScope {
			globaisAntes[k] = v
		}
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

	// importa ... como alias: cria um dicionario com as definicoes do modulo
	// e amarra no alias. As globals tambem existem no escopo global (nao da
	// pra evitar na VM sem reescrever o sistema de modulos), mas o alias
	// resolve o acesso via alias.nome.
	if node.Alias != nil {
		// coleta nomes novos (definidos pelo modulo)
		novosNomes := []string{}
		for k, v := range c.scope.symbols {
			if v.Scope == GlobalScope {
				if _, ja := globaisAntes[k]; !ja {
					novosNomes = append(novosNomes, k)
				}
			}
		}
		// empilha OpHash com pares {nome: global}
		nPares := 0
		for _, nome := range novosNomes {
			sym, _ := c.scope.Resolve(nome)
			c.emit(code.OpConstant, c.addConstant(&object.Texto{Value: nome}))
			c.emitVarGet(sym)
			nPares++
		}
		c.emit(code.OpHash, nPares)
		// amarra no alias
		aliasSym := c.defineVar(node.Alias.Value)
		c.emitVarSet(aliasSym)
	}
	return nil
}

// compileFatia compila xs[inicio:fim]. Desugar pra chamada da builtin fatia
// — mas como fatia so trabalha com texto, implemementamos via OpIndex com
// inicio/fim no stack e um novo opcode. Por simplicidade, desugaramos pra
// uma chamada da builtin `fatia` com indices nil (0 / tamanho).
// Na verdade, mais limpo: empilhamos left, inicio (ou nada-numero), fim (ou
// nada-numero) e usamos OpFatia.
func (c *Compiler) compileFatia(node *ast.FatiaExpression) error {
	if err := c.compile(node.Left); err != nil {
		return err
	}
	// nil = NADA (sentinela); a VM cuida da normalizacao.
	if node.Inicio != nil {
		if err := c.compile(node.Inicio); err != nil {
			return err
		}
	} else {
		c.emit(code.OpNada)
	}
	if node.Fim != nil {
		if err := c.compile(node.Fim); err != nil {
			return err
		}
	} else {
		c.emit(code.OpNada)
	}
	c.emit(code.OpFatia)
	return nil
}
