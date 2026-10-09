package compiler

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

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
	// Param: parametro da funcao (sempre tem valor, a leitura dispensa a
	// checagem). Celula: local capturado por closure, mora numa celula.
	Param  bool
	Celula bool
}

// livreRef e uma freevar: o nome `Name` do escopo de funcao `dono`.
type livreRef struct {
	Name string
	dono *SymbolTable
}

// posBuiltin: nome do builtin -> indice (o mesmo de nomesBuiltins).
var posBuiltin = func() map[string]int {
	m := make(map[string]int, len(nomesBuiltins))
	for i, n := range nomesBuiltins {
		m[n] = i
	}
	return m
}()

type SymbolTable struct {
	symbols map[string]Symbol
	count   int
	outer   *SymbolTable
	free    []livreRef // freeVars coletadas (na ordem do OpClosure)
	// globais: contador de slots globais COMPARTILHADO entre a tabela do
	// principal e as tabelas dos modulos (cada modulo tem nomes proprios, mas
	// os slots moram no mesmo array de globais da VM). nil = usa count.
	globais *int
}

// NumGlobais conta os simbolos do escopo mais externo (as globais). Chamada no
// fim da compilacao, quando c.scope ja voltou pro topo.
func (s *SymbolTable) NumGlobais() int {
	topo := s
	for topo.outer != nil {
		topo = topo.outer
	}
	if topo.globais != nil {
		return *topo.globais
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
	} else if s.globais != nil {
		sym.Index = *s.globais
		*s.globais++
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

	// importa (VM): DirBase e o diretorio pra resolver caminho relativo do
	// programa principal ("" = diretorio atual); Arquivo e o .gs principal
	// (opcional: entra na cadeia do import circular e, se DirBase estiver
	// vazio, da o diretorio). Cada modulo e compilado UMA vez (modulos, pelo
	// caminho absoluto) com tabela de nomes propria; moduloAtual e o arquivo
	// sendo compilado agora ("" = principal).
	DirBase       string
	Arquivo       string
	modulos       map[string]*moduloInfo
	moduloAtual   string
	progPrincipal *ast.Program

	// interning de constantes escalares (Numero/Texto/Booleano): mesma
	// constante literal repetida reusa o mesmo indice no pool.
	constDedupe map[string]int

	// funcAtual: nome da funcao sendo compilada (pra detectar self-tail-call).
	funcAtual string

	// cravadas: nomes cravados no escopo global ate o ponto da compilacao
	// (sobrevive entre entradas do REPL e recebe o que os modulos cravam).
	cravadas map[string]bool

	// Instrumentar liga o gancho de linha (object/gancho.go): antes de cada
	// statement executavel (ast.StatementsExecutaveis) do principal e dos
	// modulos sai um OpLinha com o indice do sitio em Bytecode.Sitios. Fica
	// desligado no caminho normal — ai o bytecode e byte a byte o de sempre
	// e a VM nao paga nada. Liga antes do Compile.
	Instrumentar bool
	sitios       []*object.SitioLinha
	sitioDe      map[ast.Statement]int // statement do usuario -> indice do sitio
	// globaisDep: com Instrumentar, a tabela de globais (pro depurador) de
	// cada arquivo compilado ("" = principal). As gambiarras guardam o
	// ponteiro; os nomes entram no fim da compilacao do arquivo.
	globaisDep map[string]*object.TabelaGlobais
}

type compiledFn struct {
	name      string
	numArgs   int
	minArgs   int // argumentos requeridos (sem default e sem varargs)
	numLocals int
	bytecode  []byte
	free      []livreRef
	variadic  bool
}

func New() *Compiler {
	main := NewSymbolTable()
	main.globais = new(int)
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
	// introspeccao (sempre no FIM: indice novo nao mexe nos de antes)
	"tipo",
	// rede baixo nivel (TCP/UDP)
	"conecta_tcp", "escuta_tcp", "endereco", "escuta_udp", "envia_udp", "conecta_udp",
	// seguranca (Tier 5b)
	"hash_senha", "confere_senha", "encripta", "decripta", "gera_chave",
	"token_aleatorio", "jwt_assina", "jwt_confere",
	// logging, flags e .env (Tier 5b)
	"log_debug", "log_info", "log_aviso", "log_erro", "opcoes", "carrega_env",
	// servidor parte 2 + websocket
	"responde_json", "antes", "depois", "cors", "serve_pasta", "rota_ws", "conecta_ws",
	// concorrencia: lock explicito
	"trava", "com_trava",
	// tls: certificado autoassinado pra dev
	"gera_certificado",
	// POO (Tier 8): satisfacao de combinado e type assertion
	"satisfaz", "como_tipo",
	// banco: migracoes; validacao de entrada
	"migra", "valida",
	// tarefas agendadas e datas amigaveis
	"a_cada", "depois_de", "agenda", "cancela", "formata_data", "le_data",
}

// indiceBuiltin devolve o indice canonico da builtin (pros desugars que
// chamam builtin direto via OpCallBuiltin).
func indiceBuiltin(nome string) int {
	for i, n := range nomesBuiltins {
		if n == nome {
			return i
		}
	}
	panic("builtin desconhecida: " + nome)
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
	// Modulos: caminhos absolutos dos modulos compilados junto (o cache .gsc
	// confere se nenhum mudou; o `gs build` embute as fontes).
	Modulos []string
	// MaxStack e o teto da pilha de operandos do fluxo principal (as
	// gambiarras levam o delas na CompiledFunction). 0 = desconhecido.
	MaxStack int
	// Sitios: so com Instrumentar — o operando de cada OpLinha indexa aqui.
	// nil no bytecode normal (e bytecode com sitio nunca vai pro .gsc).
	Sitios []*object.SitioLinha
	// Depura: so com Instrumentar — arquivo e globais do fluxo principal pro
	// depurador (as gambiarras levam o delas na CompiledFunction).
	Depura *object.InfoDepuracao
}

func (c *Compiler) Bytecode() *Bytecode {
	var dep *object.InfoDepuracao
	if c.Instrumentar {
		tab := c.tabelaGlobais("")
		preencheGlobais(tab, c.scopes[0])
		dep = &object.InfoDepuracao{Arquivo: c.arquivoPrincipal(), Globais: tab}
	}
	return &Bytecode{
		Depura:       dep,
		Instructions: c.instructions,
		Constants:    c.constants,
		Functions:    c.compiledFns,
		Linhas:       c.linhas,
		NumGlobals:   c.scope.NumGlobais(),
		Modulos:      c.caminhosModulos(),
		MaxStack:     MaxPilha(c.instructions, c.constants),
		Sitios:       c.sitios,
	}
}

// maxSitios e o teto de statements instrumentados (operando de 2 bytes).
const maxSitios = 1 << 16

// registraSitios cria um sitio pra cada statement executavel do programa (o
// principal ou um modulo). Os nodes sao a chave: statement sintetico de
// desugar nao esta na arvore do parser, entao nunca dispara o gancho.
func (c *Compiler) registraSitios(prog *ast.Program, arquivo string) error {
	if c.sitioDe == nil {
		c.sitioDe = map[ast.Statement]int{}
	}
	for _, s := range ast.StatementsExecutaveis(prog) {
		if _, ok := c.sitioDe[s]; ok {
			continue
		}
		if len(c.sitios) >= maxSitios {
			return fmt.Errorf("instrumentacao: programa grande demais (mais de %d statements)", maxSitios)
		}
		c.sitioDe[s] = len(c.sitios)
		c.sitios = append(c.sitios, &object.SitioLinha{Arquivo: arquivo, Linha: ast.LinhaDoStatement(s)})
	}
	return nil
}

// caminhosModulos lista (ordenado) os modulos compilados de verdade — o
// principal, quando importado de volta, nao conta.
func (c *Compiler) caminhosModulos() []string {
	var out []string
	for caminho, info := range c.modulos {
		if info.desc.Corpo != nil {
			out = append(out, caminho)
		}
	}
	sort.Strings(out)
	return out
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
		// `__*` sao os temporarios que o proprio compilador cria (__it_gs0,
		// __seq_gs0, __erro_gsN, __esc_gs...): moram no global mas nao sao do
		// usuario, e apareciam no TAB do REPL
		if sym.Scope == GlobalScope && !strings.HasPrefix(nome, "__") {
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
	if c.sitioDe != nil {
		if s, ok := node.(ast.Statement); ok {
			if id, ok := c.sitioDe[s]; ok {
				c.emit(code.OpLinha, id)
			}
		}
	}
	switch node := node.(type) {
	case *ast.Program:
		if c.Instrumentar && c.moduloAtual == "" {
			if err := c.registraSitios(node, c.arquivoPrincipal()); err != nil {
				return err
			}
		}
		// crava: checa pelo texto antes de compilar (a mesma regra que o
		// tree-walker usa). Se a entrada falhar, o que ela cravou nao fica.
		if errs := ast.ChecaCravadas(node, copiaCravadas(c.cravadas)); len(errs) > 0 {
			return fmt.Errorf("linha %d: %s", errs[0].Linha, errs[0].Msg)
		}
		antes := copiaCravadas(c.cravadas)
		if c.moduloAtual == "" {
			c.progPrincipal = node
		}
		c.declaraNoTopo(node.Statements)
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
		// Concatena via OpAdd (string + string). Se a primeira parte nao e
		// texto garantido, comeca de "" — senao `"${1}${2}"` somava (3) e
		// `"${x}"` devolvia o valor cru em vez de texto.
		jaTemTexto := false
		if len(node.Parts) > 0 {
			_, ehTexto := node.Parts[0].(*ast.TextoLiteral)
			jaTemTexto = ehTexto || node.FormatoDa(0) != ""
		}
		if !jaTemTexto {
			c.emit(code.OpConstant, c.addConstant(&object.Texto{Value: ""}))
		}
		for i, p := range node.Parts {
			if f := node.FormatoDa(i); f != "" {
				// `${v:fmt}` = formata("%fmt", v) — pelo indice, sem passar
				// pelo nome (que o usuario pode ter sombreado)
				c.emit(code.OpConstant, c.addConstant(&object.Texto{Value: "%" + f}))
				if err := c.compile(p); err != nil {
					return err
				}
				c.emit(code.OpCallBuiltin, indiceBuiltin("formata"), 2)
			} else if err := c.compile(p); err != nil {
				return err
			}
			if i > 0 || !jaTemTexto {
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
			// Chamada com `...lista` nao vira tail call (argc so se sabe em runtime).
			if call, ok := node.Value.(*ast.CallExpression); ok && len(c.arrumas) == 0 && call.Espalhados == nil && ehSelfCall(call, c.funcAtual) {
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
	// POO (poo.go)
	case *ast.TretaDecl:
		return c.compileTretaDecl(node)
	case *ast.CombinadoDecl:
		return c.compileCombinadoDecl(node)
	case *ast.MetodoDecl:
		return c.compileMetodoDecl(node)
	case *ast.TretaLiteral:
		return c.compileTretaLiteral(node)
	default:
		return fmt.Errorf("a VM ainda nao sabe compilar %T", node)
	}
	return nil
}

func (c *Compiler) compileIdent(node *ast.Identifier) error {
	return c.emitLeitura(node.Value, node.Token.Line)
}

func (c *Compiler) emitVarSet(sym Symbol) {
	switch sym.Scope {
	case GlobalScope:
		c.emit(code.OpSetGlobal, sym.Index)
	case LocalScope:
		if sym.Celula {
			c.emit(code.OpSetCelula, sym.Index)
		} else {
			c.emit(code.OpSetLocal, sym.Index)
		}
	}
	// FreeScope nunca e escrito: todo nome botado numa funcao e local dela
	// (escopo de funcao, ver escopo.go).
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
	c.emit(code.OpCallBuiltin, indiceBuiltin("tamanho"), 1)
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
	// escopo de funcao (escopo.go): params e todo nome botado no corpo ja
	// nascem locais; o que alguma funcao de dentro le vira celula.
	info := varreFuncao(params, body)
	paramSyms := make([]Symbol, len(params))
	minArgs := 0
	temVariadic := false
	for i, p := range params {
		sym := newScope.Define(p.Nome.Value)
		sym.Param = true
		sym.Celula = info.capturadas[p.Nome.Value]
		newScope.symbols[p.Nome.Value] = sym
		paramSyms[i] = sym
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

	for _, nome := range info.declaradas {
		sym := newScope.Define(nome)
		sym.Celula = info.capturadas[nome]
		newScope.symbols[nome] = sym
	}
	// Prologo 1: local capturado por closure vira celula (param embrulha o
	// argumento; o resto nasce celula vazia). O slot sem valor e nil: a VM
	// limpa os locals em toda entrada de funcao.
	for _, nome := range append(nomesParams(params), info.declaradas...) {
		if sym := newScope.symbols[nome]; sym.Celula {
			c.emit(code.OpCelula, sym.Index)
		}
	}

	// Prologo 2: pra cada param com valor padrao, se o slot veio NADA (nao
	// preenchido pela VM), substitui pelo default. A VM poe NADA nos slots
	// nao fornecidos (ver OpCall: padding com NADA quando argc < NumArgs).
	for i, p := range params {
		if p.Padrao == nil || p.Variadico {
			continue
		}
		// if param[i] == nada then param[i] = default
		c.emitVarGet(paramSyms[i])
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
		c.emitVarSet(paramSyms[i])
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
	free := newScope.free
	var dep *object.InfoDepuracao
	if c.Instrumentar {
		dep = c.infoFuncao(newScope)
	}
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
		MaxStack:  MaxPilha(cf.bytecode, c.constants),
		Depura:    dep,
	})
	// pra cada freevar, empilha a CELULA dela antes do OpClosure: o local do
	// escopo dono (se e quem esta criando a closure) ou a freevar que este
	// escopo ja recebeu de fora (repasse). Local que nao e celula (nome que
	// so o importa sem alias criou) vai por valor, como antes.
	for _, fv := range free {
		if fv.dono == outer {
			sym, ok := outer.symbols[fv.Name]
			if !ok || sym.Scope != LocalScope {
				return fmt.Errorf("freevar %q sumiu do escopo externo", fv.Name)
			}
			c.emit(code.OpGetLocal, sym.Index)
			continue
		}
		c.emit(code.OpGetFreeCelula, outer.livre(fv.Name, fv.dono).Index)
	}
	c.emit(code.OpClosure, fnIdx, len(free))
	return nil
}

// nomesParams lista os nomes dos parametros, na ordem.
func nomesParams(params []*ast.Parametro) []string {
	out := make([]string, len(params))
	for i, p := range params {
		out[i] = p.Nome.Value
	}
	return out
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
	if node.Espalhados != nil {
		c.emit(code.OpCallEspalha, c.mascaraEspalha(node))
		return nil
	}
	c.emit(code.OpCall, len(node.Arguments))
	return nil
}

// mascaraEspalha guarda no pool a mascara dos args espalhados ("010": 1 = o
// arg e um `...lista`) e devolve o indice dela pro OpCallEspalha/OpBoraEspalha.
func (c *Compiler) mascaraEspalha(call *ast.CallExpression) int {
	m := make([]byte, len(call.Arguments))
	for i := range m {
		m[i] = '0'
		if call.Espalha(i) {
			m[i] = '1'
		}
	}
	return c.addConstant(&object.Texto{Value: string(m)})
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
	if call.Espalhados != nil {
		c.emit(code.OpBoraEspalha, c.mascaraEspalha(call))
		return nil
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
	//     com quebrou:  [set tmp]; [OpTry <relanca>]
	//       cada clausula: [get tmp]; set nome; [<filtro>; OpJumpIfFalse <prox>]
	//                      <corpo>; [OpTryEnd]; OpJump <fim>
	//       nenhuma pegou (so com filtro na ultima): get tmp; OpThrow
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
	if len(node.Quebrous) > 0 {
		// multi-catch: o erro fica num temporario pra cada clausula amarrar
		// e pra relancar se nenhum filtro colar. Um quebrou sem filtro so
		// amarra direto (o bytecode de sempre).
		temFiltro := false
		for _, q := range node.Quebrous {
			temFiltro = temFiltro || q.Filtro != nil
		}
		var erroTmp Symbol
		if temFiltro {
			c.numTemps++
			erroTmp = c.defineVar("__erro_gs" + strconv.Itoa(c.numTemps))
			c.emitVarSet(erroTmp)
		}
		// com finalmente, erro dentro do quebrou (ou do filtro, ou nenhum
		// filtro colou) roda o finalmente e sobe
		relancaOp := -1
		if node.Finally != nil {
			relancaOp = c.emit(code.OpTry, 9999)
		}
		for _, q := range node.Quebrous {
			if temFiltro {
				c.emitVarGet(erroTmp)
			}
			if q.Nome != nil {
				c.emitVarSet(c.defineVar(q.Nome.Value))
			} else {
				c.emit(code.OpPop) // descarta erro sem nome
			}
			jmpProx := -1
			if q.Filtro != nil {
				if err := c.compile(q.Filtro); err != nil {
					return err
				}
				jmpProx = c.emit(code.OpJumpIfFalse, 9999)
			}
			c.arrumas = append(c.arrumas, arrumaAtiva{handler: node.Finally != nil, finally: node.Finally, numLoops: len(c.loopStack)})
			err := c.compile(q.Corpo)
			c.arrumas = c.arrumas[:len(c.arrumas)-1]
			if err != nil {
				return err
			}
			if node.Finally != nil {
				c.emit(code.OpTryEnd)
			}
			jmpsFim = append(jmpsFim, c.emit(code.OpJump, 9999))
			if jmpProx >= 0 {
				c.backpatch(jmpProx, len(c.instructions))
			}
		}
		if node.Quebrous[len(node.Quebrous)-1].Filtro != nil {
			// nenhum filtro colou: relanca o erro original (com finalmente,
			// cai no relanca la embaixo, que roda o finalmente antes)
			c.emitVarGet(erroTmp)
			c.emit(code.OpThrow)
		}
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
		if sym.Celula {
			c.emit(code.OpGetCelula, sym.Index)
		} else {
			c.emit(code.OpGetLocal, sym.Index)
		}
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
// iguais ao runtime: texto+texto e inteiro exato +/-/* (estouro vira real,
// object.SomaInt & cia, igual o runtime). Divisao, modulo, float, comparacao,
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
	// mesma conta do runtime: inteiro que estoura o int64 vira real
	var conta func(a, b int64) (int64, bool)
	var emReal func(a, b float64) float64
	switch op {
	case "+":
		conta, emReal = object.SomaInt, func(a, b float64) float64 { return a + b }
	case "-":
		conta, emReal = object.SubInt, func(a, b float64) float64 { return a - b }
	case "*":
		conta, emReal = object.MulInt, func(a, b float64) float64 { return a * b }
	case "**":
		// mesma conta do runtime (object.Potencia); so dobra resultado inteiro
		iv, _, ehInt, err := object.Potencia(ln, rn)
		if err != nil || !ehInt {
			return nil, false
		}
		return object.NumInt(iv), true
	default:
		return nil, false
	}
	if v, ok := conta(ln.Int, rn.Int); ok {
		return object.NumInt(v), true
	}
	return object.NumFloat(emReal(ln.Value, rn.Value)), true
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

// moduloInfo e o que o compilador sabe de um modulo ja compilado (ou sendo
// compilado, no caso de import circular).
type moduloInfo struct {
	desc     *object.Modulo
	constIdx int
	nomes    []string        // nomes de topo (o que o importa sem alias copia)
	cravadas map[string]bool // o que o modulo cravou no topo
}

// compileImporta: o modulo e compilado UMA vez (com tabela de nomes propria,
// sem enxergar as globais de quem importa) e vira um descritor no pool de
// constantes. No lugar do importa sai um OpImporta, que roda o corpo uma vez
// so por processo e empilha o namespace (dicionario). Com `como`, o
// namespace vai pro alias; sem, cada nome e copiado pro escopo atual.
// Modulo que nao existe/nao parseia/nao compila e erro de compilacao (a VM
// nao tem como seguir sem saber os nomes); import circular so da pra saber
// rodando, entao e erro de runtime (igual o tree-walker).
// Suportamos somente caminho literal de texto (`importa "x.gs"`).
func (c *Compiler) compileImporta(node *ast.ImportaStatement) error {
	tx, ok := node.Path.(*ast.TextoLiteral)
	if !ok {
		return fmt.Errorf("importa na VM so aceita texto literal (veio %T)", node.Path)
	}
	linha := node.Token.Line
	abs, fonte, errRes := object.ResolveModulo(c.dirImporta(), tx.Value)
	if errRes != nil {
		return errors.New(object.ComLinha(errRes, linha).Message)
	}
	info, err := c.moduloPra(abs, fonte, tx.Value, linha)
	if err != nil {
		return err
	}
	atual := c.moduloAtual
	if atual == "" {
		atual = c.arquivoPrincipal()
	}
	c.emit(code.OpImporta, info.constIdx, c.addConstant(&object.Texto{Value: atual}))

	if node.Alias != nil {
		c.emitVarSet(c.defineVar(node.Alias.Value))
		return nil
	}
	// sem alias: copia cada nome do namespace pro escopo atual. No topo, nome
	// cravado por quem importa nao pode vir do modulo; o que o modulo cravou
	// passa a valer aqui tambem.
	topo := c.scope.outer == nil
	if topo {
		for _, nome := range info.nomes {
			if c.cravadas[nome] {
				return fmt.Errorf("deu ruim na linha %d: o modulo %q ta com perrengue: `%s` foi cravada, nao da pra mudar", linha, tx.Value, nome)
			}
		}
	}
	for _, nome := range info.nomes {
		c.emit(code.OpDup)
		c.emit(code.OpConstant, c.addConstant(&object.Texto{Value: nome}))
		c.emit(code.OpIndexOuNada)
		c.emitVarSet(c.defineVar(nome))
	}
	c.emit(code.OpPop)
	if topo {
		for nome := range info.cravadas {
			c.cravadas[nome] = true
		}
	}
	return nil
}

// arquivoPrincipal devolve o caminho absoluto do .gs principal ("" se nao
// foi informado).
func (c *Compiler) arquivoPrincipal() string {
	if c.Arquivo == "" {
		return ""
	}
	if abs, err := filepath.Abs(c.Arquivo); err == nil {
		return abs
	}
	return c.Arquivo
}

// dirImporta e o diretorio contra o qual o importa sendo compilado resolve
// caminho relativo: o do arquivo onde ele esta escrito.
func (c *Compiler) dirImporta() string {
	if c.moduloAtual != "" {
		return filepath.Dir(c.moduloAtual)
	}
	if c.DirBase != "" {
		return c.DirBase
	}
	if p := c.arquivoPrincipal(); p != "" {
		return filepath.Dir(p)
	}
	return ""
}

// moduloPra devolve o modulo ja compilado ou compila agora.
func (c *Compiler) moduloPra(abs string, fonte []byte, escrito string, linha int) (*moduloInfo, error) {
	if c.modulos == nil {
		c.modulos = map[string]*moduloInfo{}
	}
	if info, ok := c.modulos[abs]; ok {
		// ja compilado — ou no meio da compilacao (ciclo): os nomes vem da
		// varredura do topo, e o runtime acusa o ciclo antes de usar.
		return info, nil
	}
	novoInfo := func(nomes []string) *moduloInfo {
		info := &moduloInfo{desc: &object.Modulo{Caminho: abs}, nomes: nomes}
		info.constIdx = c.addConstant(info.desc)
		c.modulos[abs] = info
		return info
	}
	if p := c.arquivoPrincipal(); p != "" && abs == p {
		// o principal esta no comeco de toda cadeia: importar ele e sempre
		// circular, o runtime acusa. Os nomes so servem pra compilar o resto.
		return novoInfo(nomesDeTopo(c.progPrincipal)), nil
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return nil, errors.New(object.ComLinha(object.ErroDeModulo(escrito, errs[0]), linha).Message)
	}
	if errs := ast.ChecaCravadas(prog, nil); len(errs) > 0 {
		det := fmt.Sprintf("linha %d: %s", errs[0].Linha, errs[0].Msg)
		return nil, errors.New(object.ComLinha(object.ErroDeModulo(escrito, det), linha).Message)
	}
	info := novoInfo(nomesDeTopo(prog))
	if err := c.compilaCorpoModulo(info, prog); err != nil {
		delete(c.modulos, abs)
		return nil, err
	}
	return info, nil
}

// compilaCorpoModulo compila o topo do modulo como uma funcao sem argumentos
// numa tabela de nomes PROPRIA (so builtins de fora). Os slots das globais
// do modulo saem do mesmo contador das globais do principal.
func (c *Compiler) compilaCorpoModulo(info *moduloInfo, prog *ast.Program) error {
	abs := info.desc.Caminho
	escopo, inst, linhas, linhaAtual := c.scope, c.instructions, c.linhas, c.linhaAtual
	loops, arrumas, funcAtual, cravadas := c.loopStack, c.arrumas, c.funcAtual, c.cravadas
	dirBase, moduloAtual := c.DirBase, c.moduloAtual
	defer func() {
		c.scope, c.instructions, c.linhas, c.linhaAtual = escopo, inst, linhas, linhaAtual
		c.loopStack, c.arrumas, c.funcAtual, c.cravadas = loops, arrumas, funcAtual, cravadas
		c.DirBase, c.moduloAtual = dirBase, moduloAtual
	}()

	tab := NewSymbolTable()
	tab.globais = c.scopes[0].globais
	for i, nome := range nomesBuiltins {
		tab.DefineBuiltin(nome, i)
	}
	for nome := range object.Predefinidas {
		tab.symbols[nome] = Symbol{Name: nome, Scope: PredefinidaScope}
	}
	c.scope = tab
	c.instructions, c.linhas, c.linhaAtual = code.Instructions{}, nil, 0
	c.loopStack, c.arrumas, c.funcAtual = nil, nil, ""
	c.cravadas = map[string]bool{}
	c.DirBase, c.moduloAtual = filepath.Dir(abs), abs
	c.declaraNoTopo(prog.Statements)
	if c.Instrumentar {
		if err := c.registraSitios(prog, abs); err != nil {
			return err
		}
	}

	for _, s := range prog.Statements {
		if err := c.compile(s); err != nil {
			e := object.AnotaModulo(&object.Erro{Message: err.Error()}, object.NomeCurto(abs, c.arquivoPrincipal()))
			return errors.New(e.Message)
		}
	}
	c.emit(code.OpReturnNada)
	info.desc.Corpo = &object.CompiledFunction{Name: "<modulo>", Bytecode: c.instructions, Linhas: c.linhas,
		MaxStack: MaxPilha(c.instructions, c.constants)}

	nomes := make([]string, 0, len(tab.symbols))
	for nome, sym := range tab.symbols {
		if sym.Scope == GlobalScope && !strings.HasPrefix(nome, "__") {
			nomes = append(nomes, nome)
		}
	}
	sort.Strings(nomes)
	slots := make([]int, len(nomes))
	for i, nome := range nomes {
		slots[i] = tab.symbols[nome].Index
	}
	info.desc.Nomes, info.desc.Slots = nomes, slots
	if c.Instrumentar {
		tg := c.tabelaGlobais(abs)
		tg.Nomes, tg.Slots = nomes, slots
		info.desc.Corpo.Depura = &object.InfoDepuracao{Arquivo: abs, Globais: tg}
	}
	info.nomes = nomes
	info.cravadas = c.cravadas
	return nil
}

// nomesDeTopo varre o topo do programa atras dos nomes que ele define — usado
// so pra um modulo que ainda esta compilando (import circular), quando a
// tabela de nomes dele ainda nao fechou.
func nomesDeTopo(prog *ast.Program) []string {
	if prog == nil {
		return nil
	}
	visto := map[string]bool{}
	var nomes []string
	add := func(id *ast.Identifier) {
		if id != nil && !visto[id.Value] {
			visto[id.Value] = true
			nomes = append(nomes, id.Value)
		}
	}
	for _, s := range prog.Statements {
		switch n := s.(type) {
		case *ast.BotaStatement:
			add(n.Name)
		case *ast.CravaStatement:
			add(n.Name)
		case *ast.GambiarraStatement:
			add(n.Name)
		case *ast.ImportaStatement:
			add(n.Alias)
		case *ast.TretaDecl:
			add(n.Nome)
		case *ast.CombinadoDecl:
			add(n.Nome)
		}
	}
	sort.Strings(nomes)
	return nomes
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
