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
	PredefinidaScope // valor que ja nasce definido (pi): vira OpConstant
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

// NumGlobais conta os simbolos do escopo mais externo (as globais). Chamada no
// fim da compilacao, quando c.scope ja voltou pro topo.
func (s *SymbolTable) NumGlobais() int {
	topo := s
	for topo.outer != nil {
		topo = topo.outer
	}
	return topo.count
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
	// PredefinidaScope (pi) segue a mesma regra do builtin: da pra sombrear.
	if sym, ok := s.symbols[nome]; ok && sym.Scope != FreeScope && sym.Scope != BuiltinScope && sym.Scope != PredefinidaScope {
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
	case GlobalScope, BuiltinScope, PredefinidaScope:
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
	// arrumaBase: quantos arrumas ja estavam abertos quando o laco comecou.
	// vaza/continua fecham so os arrumas abertos DENTRO do laco.
	arrumaBase int
}

// arrumaAtiva e um arruma aberto no ponto da compilacao. Saida antecipada
// (funciona/vaza/continua) tem que desarmar o handler e rodar o finalmente
// antes de pular.
type arrumaAtiva struct {
	handler  bool                // tem OpTry armado (precisa de OpTryEnd)
	finally  *ast.BlockStatement // finalmente pra rodar na saida (ou nil)
	numLoops int                 // lacos abertos quando o arruma comecou
}

type Compiler struct {
	instructions code.Instructions
	constants    []object.Object
	scopes       []*SymbolTable
	scope        *SymbolTable
	loopStack    []loopFrame
	arrumas      []arrumaAtiva // arrumas abertos na funcao atual
	numTemps     int           // contador pra nomes de temporarios unicos

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

	// cravadas: nomes cravados no escopo global ate o ponto da compilacao
	// (sobrevive entre entradas do REPL e recebe o que os modulos cravam).
	cravadas map[string]bool
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
	c := &Compiler{scope: main, scopes: []*SymbolTable{main}, constDedupe: map[string]int{}, cravadas: map[string]bool{}}
	// registra builtins no escopo global — idx 0..N-1. A ordem aqui determina
	// o indice que a VM usa pra despachar a builtin (veja vm.Builtins()).
	for i, nome := range nomesBuiltins {
		main.DefineBuiltin(nome, i)
	}
	// valores predefinidos (pi): nao tem slot, o compileIdent vira constante.
	for nome := range object.Predefinidas {
		main.symbols[nome] = Symbol{Name: nome, Scope: PredefinidaScope}
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
	"seno", "cosseno", "tangente", "log", "log10", "exp",
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
	// NumGlobals e quantas variaveis globais o programa declara. A VM aloca
	// EXATAMENTE isso: o slice de globais e compartilhado com os clones do
	// `bora`, entao ele nao pode ser realocado depois (os clones ficariam com
	// o array velho e as escritas parariam de se enxergar).
	NumGlobals int
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.instructions,
		Constants:    c.constants,
		Functions:    c.compiledFns,
		Linhas:       c.linhas,
		NumGlobals:   c.scope.NumGlobais(),
	}
}

// NovaEntrada prepara o compilador pra compilar MAIS um pedaco de programa
// reusando o estado acumulado — e o que faz o REPL rodar na VM: o pool de
// constantes, a tabela de simbolos globais e as funcoes ja definidas
// sobrevivem, so o buffer de instrucoes recomeca. Assim `bota x = 1` numa
// linha e `mostra x` na seguinte enxergam o mesmo indice global.
func (c *Compiler) NovaEntrada() {
	c.instructions = nil
	c.linhas = nil
	c.linhaAtual = 0
	c.loopStack = nil
	c.arrumas = nil
	c.funcAtual = ""
	// volta pro escopo global: se a entrada anterior morreu no meio de um
	// corpo de funcao, o scope podia ter ficado aninhado.
	c.scope = c.scopes[0]
	c.scopes = c.scopes[:1]
}

// NomesGlobais lista os nomes definidos pelo USUARIO no escopo global (fora os
// builtins, que quem completa ja conhece por outro caminho). O REPL usa pra
// completar com TAB — na VM nao existe Environment pra perguntar.
func (c *Compiler) NomesGlobais() []string {
	topo := c.scopes[0]
	nomes := make([]string, 0, len(topo.symbols))
	for nome, sym := range topo.symbols {
		if sym.Scope == GlobalScope {
			nomes = append(nomes, nome)
		}
	}
	return nomes
}

// NumGlobaisDefinidas diz quantas globais o compilador ja conhece. O REPL usa
// pra saber se precisa de mais espaco no array de globais da sessao.
func (c *Compiler) NumGlobaisDefinidas() int { return c.scope.NumGlobais() }

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
		// crava: checa pelo texto antes de compilar (a mesma regra que o
		// tree-walker usa). Se a entrada falhar, o que ela cravou nao fica.
		if errs := ast.ChecaCravadas(node, copiaCravadas(c.cravadas)); len(errs) > 0 {
			return fmt.Errorf("linha %d: %s", errs[0].Linha, errs[0].Msg)
		}
		antes := copiaCravadas(c.cravadas)
		for _, s := range node.Statements {
			if err := c.compile(s); err != nil {
				c.cravadas = antes
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
	case *ast.CravaStatement:
		if err := c.compile(node.Value); err != nil {
			return err
		}
		sym := c.defineVar(node.Name.Value)
		c.emitVarSet(sym)
		if c.scope.outer == nil {
			c.cravadas[node.Name.Value] = true
		}
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
			return fmt.Errorf("linha %d: `vaza` so funciona dentro de um laco", node.Token.Line)
		}
		if err := c.saiDosArrumas(c.loopStack[len(c.loopStack)-1].arrumaBase); err != nil {
			return err
		}
		frame := &c.loopStack[len(c.loopStack)-1]
		jmpPos := c.emit(code.OpJump, 9999)
		frame.breakJumps = append(frame.breakJumps, jmpPos)
	case *ast.ContinuaStatement:
		if len(c.loopStack) == 0 {
			return fmt.Errorf("linha %d: `continua` so funciona dentro de um laco", node.Token.Line)
		}
		if err := c.saiDosArrumas(c.loopStack[len(c.loopStack)-1].arrumaBase); err != nil {
			return err
		}
		frame := &c.loopStack[len(c.loopStack)-1]
		jmpPos := c.emit(code.OpJump, 9999)
		frame.continueJumps = append(frame.continueJumps, jmpPos)
	case *ast.FuncionaStatement:
		if node.Value != nil {
			// tail call: `funciona f(args)` DENTRO de uma funcao vira OpTailCall,
			// que reusa o frame atual — recursao em cauda nao estoura os frames.
			// Dentro de arruma nao: a chamada tem que rodar protegida pelo try.
			if call, ok := node.Value.(*ast.CallExpression); ok && len(c.arrumas) == 0 && ehSelfCall(call, c.funcAtual) {
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
				// valor fica na pilha enquanto os finalmentes rodam
				if err := c.saiDosArrumas(0); err != nil {
					return err
				}
				c.emit(code.OpReturn)
			}
		} else {
			if err := c.saiDosArrumas(0); err != nil {
				return err
			}
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
		// nao e nada: left fica na pilha como resultado (OpIsNada ja consumiu
		// o dup — um OpPop aqui esvaziava a pilha e a VM estourava)
		jmpFim := c.emit(code.OpJump, 9999)
		c.backpatch(jmpNada, len(c.instructions))
		// e nada: descarta o left e empilha right
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
		// erro do PROGRAMA, nao da VM: quem le tem que saber a linha e o que
		// fazer, nao que existe um compilador por baixo.
		return fmt.Errorf("linha %d: nao existe nenhum `%s` por aqui — confere o nome ou declara com `bota %s = ...`", node.Token.Line, node.Value, node.Value)
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
	case PredefinidaScope:
		c.emit(code.OpConstant, c.addConstant(object.Predefinidas[sym.Name]))
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

// opcodeBinario devolve o opcode da operacao infixa e se ela pode virar
// OpBinConst (operacao simples entre dois valores, sem fluxo de controle).
func opcodeBinario(operador string) (code.Opcode, bool) {
	switch operador {
	case "+":
		return code.OpAdd, true
	case "-":
		return code.OpSub, true
	case "*":
		return code.OpMul, true
	case "/":
		return code.OpDiv, true
	case "%":
		return code.OpMod, true
	case "**":
		return code.OpPow, true
	case "<":
		return code.OpMenor, true
	case "<=":
		return code.OpMenorEqual, true
	case ">":
		return code.OpGreaterThan, true
	case ">=":
		return code.OpGreaterEqual, true
	case "==":
		return code.OpEqual, true
	case "!=":
		return code.OpNotEqual, true
	case "&":
		return code.OpBAnd, true
	case "|":
		return code.OpBOr, true
	case "^":
		return code.OpBXor, true
	case "<<":
		return code.OpLShift, true
	case ">>":
		return code.OpRShift, true
	}
	return 0, false
}

// literalConstante devolve o valor se a expressao for um literal simples
// (numero ou texto). So esses viram operando de OpBinConst: sao imutaveis e
// nao tem efeito colateral, entao fundir nao muda ordem de avaliacao.
func literalConstante(e ast.Expression) (object.Object, bool) {
	switch n := e.(type) {
	case *ast.NumeroLiteral:
		return &object.Numero{Value: n.Value, Int: n.Int, EhInt: n.EhInt}, true
	case *ast.TextoLiteral:
		return &object.Texto{Value: n.Value}, true
	}
	return nil, false
}

// tentaBinConst emite `left` seguido de OpBinConst quando o lado direito e um
// literal. Devolve false se nao der (o chamador segue pelo caminho normal).
func (c *Compiler) tentaBinConst(node *ast.InfixExpression) (bool, error) {
	op, ok := opcodeBinario(node.Operator)
	if !ok {
		return false, nil
	}
	val, ok := literalConstante(node.Right)
	if !ok {
		return false, nil
	}
	if err := c.compile(node.Left); err != nil {
		return true, err
	}
	c.emit(code.OpBinConst, c.addConstant(val), int(op))
	return true, nil
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
		if feito, err := c.tentaBinConst(node); feito {
			return err
		}
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
	if feito, err := c.tentaBinConst(node); feito {
		return err
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
	case "**":
		c.emit(code.OpPow)
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

// compilePraCadaNum compila `pra_cada i de A ate B` igual o tree-walker: A e B
// avaliados uma vez, contador escondido (mexer em `i` no corpo nao muda a
// contagem) e `i` so recebe valor quando o corpo vai rodar — depois do laco
// fica com o ultimo valor iterado (faixa vazia nao mexe nele).
//
//	__cont = A; __fim = B
//	inicio: se __cont > __fim pula pro fim
//	  i = __cont; <corpo>
//	  __cont = __cont + 1; volta pro inicio
func (c *Compiler) compilePraCadaNum(node *ast.PraCadaNumStatement) error {
	sufixo := strconv.Itoa(len(c.loopStack)) // laco aninhado tem os seus
	if err := c.compile(node.Start); err != nil {
		return err
	}
	contSym := c.defineVar("__cont_gs" + sufixo)
	c.emitVarSet(contSym)
	// fim literal vira constante direto (sem temporario no laco quente)
	fimConst, fimEhConst := literalConstante(node.End)
	var fimSym Symbol
	if !fimEhConst {
		if err := c.compile(node.End); err != nil {
			return err
		}
		fimSym = c.defineVar("__fim_gs" + sufixo)
		c.emitVarSet(fimSym)
	}

	startPos := len(c.instructions)
	c.emitVarGet(contSym)
	if fimEhConst {
		c.emit(code.OpBinConst, c.addConstant(fimConst), int(code.OpGreaterThan))
	} else {
		c.emitVarGet(fimSym)
		c.emit(code.OpGreaterThan)
	}
	jmpFim := c.emit(code.OpJumpIfTrue, 9999)
	c.emitVarGet(contSym)
	c.emitVarSet(c.defineVar(node.Var.Value))

	c.pushLoop(loopFrame{})
	idx := len(c.loopStack) - 1
	if err := c.compile(node.Body); err != nil {
		c.popLoop()
		return err
	}

	// incremento: continua pula pra ca
	incrementoAddr := len(c.instructions)
	for _, j := range c.loopStack[idx].continueJumps {
		c.backpatch(j, incrementoAddr)
	}

	c.emitVarGet(contSym)
	// incremento inteiro exato: NumInt (EhInt=true) mantem o contador em int64.
	// Com object.Numero{Value:1} (float) o `i + 1` caia no caminho float da VM
	// e o contador perdia exatidao acima de 2^53.
	c.emit(code.OpConstant, c.addConstant(object.NumInt(1)))
	c.emit(code.OpAdd)
	c.emitVarSet(contSym)
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
	// sufixo pela profundidade: laco aninhado tem os proprios temporarios
	// (com nome fixo o de dentro zerava o contador do de fora)
	sufixo := strconv.Itoa(len(c.loopStack))
	seqNome := "__seq_gs" + sufixo
	itNome := "__it_gs" + sufixo
	lenNome := "__len_gs" + sufixo
	origNome := "__orig_gs" + sufixo

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
	// lacos e arrumas de fora nao valem dentro do corpo: `vaza` numa funcao
	// nao pode pular pro laco de quem a declarou (o jump caia no bytecode
	// errado e a VM panicava).
	loopsSalvos, arrumasSalvos := c.loopStack, c.arrumas
	c.loopStack, c.arrumas = nil, nil
	defer func() {
		c.funcAtual = funcSalva
		c.loopStack, c.arrumas = loopsSalvos, arrumasSalvos
	}()
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
	// Layout:
	//   OpTry <catchAddr>
	//   <try>
	//   OpTryEnd
	//   OpJump <fim>
	//   catchAddr:             (a VM empilha o erro)
	//     com quebrou:  set err; [OpTry <relanca>]; <catch>; [OpTryEnd]; OpJump <fim>
	//     relanca (so com finalmente): set tmp; <finally>; get tmp; OpThrow
	//   fim: <finally>
	//
	// Os blocos rodam no escopo de quem tem o arruma (igual o tree-walker):
	// params/locals da funcao sao lidos e escritos direto, e o nome do erro
	// vira variavel desse mesmo escopo. Nada de escopo fechado aqui — o
	// bytecode roda no frame da funcao, nao tem frame proprio pra freevar.
	tryOp := c.emit(code.OpTry, 9999)
	c.arrumas = append(c.arrumas, arrumaAtiva{handler: true, finally: node.Finally, numLoops: len(c.loopStack)})
	err := c.compile(node.Try)
	c.arrumas = c.arrumas[:len(c.arrumas)-1]
	if err != nil {
		return err
	}
	c.emit(code.OpTryEnd)
	jmpsFim := []int{c.emit(code.OpJump, 9999)}

	c.backpatch(tryOp, len(c.instructions))
	if node.Catch != nil {
		if node.ErrName != nil {
			c.emitVarSet(c.defineVar(node.ErrName.Value))
		} else {
			c.emit(code.OpPop) // descarta erro sem nome
		}
		// com finalmente, erro dentro do quebrou roda o finalmente e sobe
		relancaOp := -1
		if node.Finally != nil {
			relancaOp = c.emit(code.OpTry, 9999)
		}
		c.arrumas = append(c.arrumas, arrumaAtiva{handler: node.Finally != nil, finally: node.Finally, numLoops: len(c.loopStack)})
		err := c.compile(node.Catch)
		c.arrumas = c.arrumas[:len(c.arrumas)-1]
		if err != nil {
			return err
		}
		if node.Finally != nil {
			c.emit(code.OpTryEnd)
		}
		jmpsFim = append(jmpsFim, c.emit(code.OpJump, 9999))
		if relancaOp >= 0 {
			c.backpatch(relancaOp, len(c.instructions))
		}
	}
	if node.Finally != nil {
		// relanca: erro sem quebrou (ou estourado no quebrou) roda o
		// finalmente e continua subindo
		c.numTemps++
		tmp := c.defineVar("__erro_gs" + strconv.Itoa(c.numTemps))
		c.emitVarSet(tmp)
		if err := c.compile(node.Finally); err != nil {
			return err
		}
		c.emitVarGet(tmp)
		c.emit(code.OpThrow)
	}

	for _, j := range jmpsFim {
		c.backpatch(j, len(c.instructions))
	}
	if node.Finally != nil {
		if err := c.compile(node.Finally); err != nil {
			return err
		}
	}
	return nil
}

// saiDosArrumas prepara uma saida antecipada (funciona/vaza/continua): fecha,
// de dentro pra fora, os arrumas abertos a partir de `ate` — desarma o
// handler (OpTryEnd) e roda o finalmente inline. O finalmente compila como se
// estivesse no lugar dele: so com os arrumas e lacos de fora.
func (c *Compiler) saiDosArrumas(ate int) error {
	salvos := c.arrumas
	defer func() { c.arrumas = salvos }()
	for k := len(salvos) - 1; k >= ate; k-- {
		a := salvos[k]
		if a.handler {
			c.emit(code.OpTryEnd)
		}
		if a.finally == nil {
			continue
		}
		c.arrumas = salvos[:k:k]
		// lacos abertos dentro do arruma nao valem no finalmente: copia os de
		// fora (os jumps registrados neles voltam pro original depois)
		loops := c.loopStack
		c.loopStack = append([]loopFrame(nil), loops[:a.numLoops]...)
		err := c.compile(a.finally)
		copy(loops, c.loopStack[:a.numLoops])
		c.loopStack = loops
		if err != nil {
			return err
		}
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
	case PredefinidaScope:
		c.emit(code.OpConstant, c.addConstant(object.Predefinidas[sym.Name]))
	}
}

// copiaCravadas devolve uma copia do conjunto de nomes cravados.
func copiaCravadas(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// --- helpers ---

func (c *Compiler) pushLoop(f loopFrame) {
	f.arrumaBase = len(c.arrumas)
	c.loopStack = append(c.loopStack, f)
}
func (c *Compiler) popLoop() { c.loopStack = c.loopStack[:len(c.loopStack)-1] }

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
	case "**":
		// mesma conta do runtime (object.Potencia); so dobra resultado inteiro
		iv, _, ehInt, err := object.Potencia(ln, rn)
		if err != nil || !ehInt {
			return nil, false
		}
		return object.NumInt(iv), true
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
	// o modulo compila no escopo de quem importa: nao pode mexer no que foi
	// cravado ate aqui, e o que ele cravar passa a valer pra frente.
	if errs := ast.ChecaCravadas(prog, c.cravadas); len(errs) > 0 {
		return fmt.Errorf("importa: modulo %q com perrengue: linha %d: %s", tx.Value, errs[0].Linha, errs[0].Msg)
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
