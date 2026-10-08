package compiler

import (
	"fmt"
	"sort"
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/object"
)

// Informacao de depuracao (gs debug). So sai com Instrumentar: o bytecode
// normal nao muda nem um byte e as CompiledFunction ficam com Depura nil.

// tabelaGlobais devolve (criando) a tabela de globais do arquivo ("" =
// principal). As gambiarras guardam o ponteiro ja durante a compilacao; os
// nomes entram quando o arquivo termina de compilar (preencheGlobais).
func (c *Compiler) tabelaGlobais(modulo string) *object.TabelaGlobais {
	if c.globaisDep == nil {
		c.globaisDep = map[string]*object.TabelaGlobais{}
	}
	t := c.globaisDep[modulo]
	if t == nil {
		t = &object.TabelaGlobais{}
		c.globaisDep[modulo] = t
	}
	return t
}

// preencheGlobais poe na tabela as globais do usuario da tabela de simbolos
// (fora builtins, predefinidas e os temporarios `__*` do compilador).
func preencheGlobais(t *object.TabelaGlobais, tab *SymbolTable) {
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
	t.Nomes, t.Slots = nomes, slots
}

// arquivoAtual e o .gs do codigo sendo compilado agora (modulo ou principal).
func (c *Compiler) arquivoAtual() string {
	if c.moduloAtual != "" {
		return c.moduloAtual
	}
	return c.arquivoPrincipal()
}

// infoFuncao monta a InfoDepuracao da gambiarra cujo escopo acabou de
// compilar: slot -> nome dos locais e a ordem das freevars.
func (c *Compiler) infoFuncao(escopo *SymbolTable) *object.InfoDepuracao {
	locais := make([]string, escopo.count)
	for nome, sym := range escopo.symbols {
		if sym.Scope == LocalScope && sym.Index < len(locais) && !strings.HasPrefix(nome, "__") {
			locais[sym.Index] = nome
		}
	}
	livres := make([]string, len(escopo.free))
	for i, fv := range escopo.free {
		livres[i] = fv.Name
	}
	return &object.InfoDepuracao{
		Arquivo: c.arquivoAtual(),
		Locais:  locais,
		Livres:  livres,
		Globais: c.tabelaGlobais(c.moduloAtual),
	}
}

// ---- avaliacao com o programa parado (depurador na VM) ----

// Avaliacao e o resultado de CompilaAvaliacao: o bytecode do topo (que deixa
// a closure da expressao como ultimo valor) e a ordem dos parametros dela.
type Avaliacao struct {
	Bytecode *Bytecode
	Params   []string
}

// CompilaAvaliacao compila `fonte` (uma expressao) pra rodar com o programa
// parado num quadro da VM. A expressao vira o corpo de uma lambda cujos
// parametros sao os locais visiveis no quadro (`locais`, por valor: atribuir
// neles nao muda o quadro); as globais do arquivo do quadro ja nascem nos
// slots de la, e o pool de constantes continua o do programa (os indices das
// gambiarras do programa continuam valendo). Criar global nova e erro: o
// array de globais da VM nao cresce com o programa rodando.
func CompilaAvaliacao(expr ast.Expression, locais []string, constantes []object.Object,
	globais *object.TabelaGlobais, numGlobais int) (*Avaliacao, error) {
	c := New()
	main := c.scopes[0]
	if globais != nil {
		for i, nome := range globais.Nomes {
			main.symbols[nome] = Symbol{Name: nome, Index: globais.Slots[i], Scope: GlobalScope}
		}
	}
	*main.globais = numGlobais
	c.constants = append([]object.Object(nil), constantes...)

	params := make([]*ast.Parametro, len(locais))
	for i, n := range locais {
		params[i] = &ast.Parametro{Nome: &ast.Identifier{Value: n}}
	}
	lambda := &ast.FuncaoLiteral{Parameters: params, Body: &ast.BlockStatement{
		Statements: []ast.Statement{&ast.FuncionaStatement{Value: expr}},
	}}
	prog := &ast.Program{Statements: []ast.Statement{&ast.ExpressionStatement{Expression: lambda}}}
	if err := c.Compile(prog); err != nil {
		return nil, err
	}
	if c.scope.NumGlobais() > numGlobais {
		return nil, fmt.Errorf("avaliando nao da pra criar variavel global nova")
	}
	return &Avaliacao{Bytecode: c.Bytecode(), Params: locais}, nil
}
