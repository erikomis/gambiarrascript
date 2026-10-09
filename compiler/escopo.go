package compiler

import (
	"fmt"

	"gambiarrascript/ast"
	"gambiarrascript/code"
	"gambiarrascript/object"
)

// Escopo de funcao estilo Python — a mesma regra do tree-walker:
//
//   - todo nome botado numa gambiarra (`bota`, `+=`, `crava`, desestruturacao,
//     variavel do pra_cada, nome do `quebrou`, gambiarra aninhada, alias do
//     importa) e LOCAL na funcao inteira, nao so a partir da linha do `bota`;
//   - ler um nome enxerga o escopo mais de dentro em que ele JA tem valor: o
//     local, se ja foi botado; senao o da funcao que envolve; senao a global;
//     senao o builtin. Nenhum deles -> "cade o `x`? voce nao botou isso ainda";
//   - closure enxerga a VARIAVEL de fora (celula), nao uma copia do valor.
//
// Antes a VM resolvia o nome pela ordem do texto: um laco que lia antes de
// escrever lia sempre a global, um ramo que nao rodou deixava o slot com lixo
// da pilha, e a closure guardava o valor da hora em que foi criada.

// infoFuncao e o que a varredura descobre de um corpo de funcao antes de
// compilar.
type infoFuncao struct {
	declaradas []string        // nomes botados no corpo (fora params), em ordem
	capturadas map[string]bool // nomes lidos por alguma gambiarra aninhada
}

// varreFuncao levanta os nomes locais de uma funcao e os que alguma funcao de
// dentro le (esses viram celula). Superestimar capturadas e inofensivo: a
// celula so custa uma indirecao.
func varreFuncao(params []*ast.Parametro, corpo *ast.BlockStatement) *infoFuncao {
	info := &infoFuncao{capturadas: map[string]bool{}}
	visto := map[string]bool{}
	for _, p := range params {
		visto[p.Nome.Value] = true
	}
	v := &varredura{
		declara: func(id *ast.Identifier) {
			if id != nil && !visto[id.Value] {
				visto[id.Value] = true
				info.declaradas = append(info.declaradas, id.Value)
			}
		},
		funcao: func(params []*ast.Parametro, corpo *ast.BlockStatement) {
			// tudo que a funcao de dentro (em qualquer profundidade) le
			lidos := &varredura{lido: func(nome string) { info.capturadas[nome] = true }}
			lidos.funcao = func(ps []*ast.Parametro, c *ast.BlockStatement) { lidos.corpo(ps, c) }
			lidos.corpo(params, corpo)
		},
	}
	for _, p := range params {
		if p.Padrao != nil {
			v.expr(p.Padrao)
		}
	}
	v.bloco(corpo)
	return info
}

// nomesDeclarados devolve os nomes que um trecho de topo bota (sem entrar nas
// funcoes) — e o que vira global antes de rodar a primeira linha.
func nomesDeclarados(stmts []ast.Statement) []string {
	visto := map[string]bool{}
	var nomes []string
	v := &varredura{declara: func(id *ast.Identifier) {
		if id != nil && !visto[id.Value] {
			visto[id.Value] = true
			nomes = append(nomes, id.Value)
		}
	}}
	for _, s := range stmts {
		v.stmt(s)
	}
	return nomes
}

// varredura anda pelos statements/expressoes de UM escopo. `declara` recebe
// cada nome amarrado nele; `lido` cada identificador lido; `funcao` cada
// gambiarra aninhada (nomeada ou lambda), que nao e descida automaticamente.
type varredura struct {
	declara func(*ast.Identifier)
	lido    func(string)
	funcao  func([]*ast.Parametro, *ast.BlockStatement)
}

func (v *varredura) amarra(id *ast.Identifier) {
	if v.declara != nil && id != nil {
		v.declara(id)
	}
}

func (v *varredura) aninhada(params []*ast.Parametro, corpo *ast.BlockStatement) {
	if v.funcao != nil {
		v.funcao(params, corpo)
	}
}

// corpo varre params (defaults) e corpo como se fossem deste escopo — usado
// pra coletar leituras de uma funcao de dentro inteira.
func (v *varredura) corpo(params []*ast.Parametro, corpo *ast.BlockStatement) {
	for _, p := range params {
		if p != nil && p.Padrao != nil {
			v.expr(p.Padrao)
		}
	}
	v.bloco(corpo)
}

func (v *varredura) bloco(b *ast.BlockStatement) {
	if b == nil {
		return
	}
	for _, s := range b.Statements {
		v.stmt(s)
	}
}

func (v *varredura) stmt(s ast.Statement) {
	switch n := s.(type) {
	case *ast.CravaStatement:
		v.expr(n.Value)
		v.amarra(n.Name)
	case *ast.BotaStatement:
		v.expr(n.Value)
		if n.Indice != nil {
			v.expr(n.Indice)
		}
		v.amarra(n.Name)
	case *ast.DesestruturaStatement:
		v.expr(n.Value)
		for _, nome := range n.Names {
			v.amarra(nome)
		}
	case *ast.MostraStatement:
		v.expr(n.Value)
	case *ast.FuncionaStatement:
		v.expr(n.Value)
	case *ast.RendeStatement:
		v.expr(n.Value)
	case *ast.ExpressionStatement:
		v.expr(n.Expression)
	case *ast.GambiarraStatement:
		v.amarra(n.Name)
		v.aninhada(n.Parameters, n.Body)
	case *ast.SeColarStatement:
		for i, c := range n.Conditions {
			v.expr(c)
			if i < len(n.Consequences) {
				v.bloco(n.Consequences[i])
			}
		}
		v.bloco(n.Alternative)
	case *ast.EnquantoStatement:
		v.expr(n.Condition)
		v.bloco(n.Body)
	case *ast.PraCadaNumStatement:
		v.expr(n.Start)
		v.expr(n.End)
		v.amarra(n.Var)
		v.bloco(n.Body)
	case *ast.PraCadaListStatement:
		v.expr(n.Iterable)
		for _, nome := range n.Vars {
			v.amarra(nome)
		}
		v.bloco(n.Body)
	case *ast.EscolheStatement:
		v.expr(n.Subject)
		for _, braco := range n.Casos {
			// padrao: os nomes soltos dentro dele amarram (locais da funcao)
			for _, val := range braco.Values {
				ast.PercorrePadrao(val, v.amarra, v.expr)
			}
			v.expr(braco.Guarda)
			v.bloco(braco.Body)
		}
		v.bloco(n.Default)
	case *ast.ArrumaStatement:
		v.bloco(n.Try)
		// cada quebrou amarra o nome dele como local (o filtro ja enxerga)
		for _, q := range n.Quebrous {
			v.amarra(q.Nome)
			v.expr(q.Filtro)
			v.bloco(q.Corpo)
		}
		v.bloco(n.Finally)
	case *ast.ImportaStatement:
		v.expr(n.Path)
		v.amarra(n.Alias)
	case *ast.BlockStatement:
		v.bloco(n)
	// POO (so no topo): treta/combinado amarram o nome; o padrao de campo vira
	// thunk e o metodo e uma gambiarra com o receiver de primeiro parametro
	case *ast.TretaDecl:
		v.amarra(n.Nome)
		for _, c := range n.Campos {
			v.expr(c.Embutida)
			if c.Padrao != nil {
				v.aninhada(nil, &ast.BlockStatement{Statements: []ast.Statement{&ast.FuncionaStatement{Value: c.Padrao}}})
			}
		}
	case *ast.CardapioDecl:
		v.amarra(n.Nome)
	case *ast.CombinadoDecl:
		v.amarra(n.Nome)
		for _, m := range n.Metodos {
			v.expr(m.Embutido)
		}
	case *ast.MetodoDecl:
		v.expr(n.Tipo)
		v.aninhada(n.ParametrosComReceptor(), n.Body)
	}
}

func (v *varredura) expr(e ast.Expression) {
	switch n := e.(type) {
	case nil:
	case *ast.Identifier:
		if v.lido != nil && n != nil {
			v.lido(n.Value)
		}
	case *ast.FuncaoLiteral:
		v.aninhada(n.Parameters, n.Body)
	case *ast.PrefixExpression:
		v.expr(n.Right)
	case *ast.InfixExpression:
		v.expr(n.Left)
		v.expr(n.Right)
	case *ast.CallExpression:
		v.expr(n.Function)
		for _, a := range n.Arguments {
			v.expr(a)
		}
	case *ast.BoraExpression:
		if n.Call != nil {
			v.expr(n.Call)
		}
	case *ast.IndexExpression:
		v.expr(n.Left)
		v.expr(n.Index)
	case *ast.FatiaExpression:
		v.expr(n.Left)
		v.expr(n.Inicio)
		v.expr(n.Fim)
	case *ast.ListaLiteral:
		for _, el := range n.Elements {
			v.expr(el)
		}
	case *ast.DicionarioLiteral:
		for _, p := range n.Pares {
			v.expr(p.Chave)
			v.expr(p.Valor)
		}
	case *ast.RangeExpression:
		v.expr(n.Start)
		v.expr(n.End)
	case *ast.TernarioExpression:
		v.expr(n.Cond)
		v.expr(n.SeVerdadeiro)
		v.expr(n.SeFalso)
	case *ast.CoalesceExpression:
		v.expr(n.Left)
		v.expr(n.Right)
	case *ast.TextoInterpolado:
		for _, p := range n.Parts {
			v.expr(p)
		}
	case *ast.TretaLiteral:
		v.expr(n.Tipo)
		for _, val := range n.Valores {
			v.expr(val)
		}
	}
}

// livre devolve o indice da freevar desta tabela que aponta pro nome `nome`
// do escopo `dono` (uma funcao de fora), registrando-a aqui e em cada nivel
// do meio (cada closure repassa pra de dentro na hora do OpClosure).
func (s *SymbolTable) livre(nome string, dono *SymbolTable) Symbol {
	for i, f := range s.free {
		if f.Name == nome && f.dono == dono {
			return Symbol{Name: nome, Index: i, Scope: FreeScope}
		}
	}
	if s.outer != dono {
		s.outer.livre(nome, dono)
	}
	s.free = append(s.free, livreRef{Name: nome, dono: dono})
	return Symbol{Name: nome, Index: len(s.free) - 1, Scope: FreeScope}
}

// candidato e um lugar onde o nome pode morar, na ordem de busca.
type candidato struct {
	sym        Symbol
	podeFaltar bool // false = sempre tem valor (param, builtin): fim da cadeia
}

// candidatos lista onde procurar `nome`, do escopo mais de dentro pro mais de
// fora, parando no primeiro que sempre tem valor.
func (c *Compiler) candidatos(nome string) []candidato {
	var out []candidato
	for s := c.scope; s != nil; s = s.outer {
		sym, ok := s.symbols[nome]
		if s.outer != nil {
			if !ok || sym.Scope != LocalScope {
				continue
			}
			podeFaltar := !sym.Param
			if s != c.scope {
				sym = c.scope.livre(nome, s)
			}
			out = append(out, candidato{sym: sym, podeFaltar: podeFaltar})
			if !podeFaltar {
				return out
			}
			continue
		}
		// tabela do topo: global, builtin ou predefinida
		if ok && sym.Scope == GlobalScope {
			out = append(out, candidato{sym: sym, podeFaltar: true})
		}
		if idx, ehBuiltin := posBuiltin[nome]; ehBuiltin {
			out = append(out, candidato{sym: Symbol{Name: nome, Index: idx, Scope: BuiltinScope}})
		} else if _, ehPre := object.Predefinidas[nome]; ehPre {
			out = append(out, candidato{sym: Symbol{Name: nome, Scope: PredefinidaScope}})
		}
	}
	return out
}

// emitLeitura emite a leitura de `nome` como uma cadeia: cada lugar que pode
// estar sem valor vira um OpGet*Ou (achou -> pula pro fim); o ultimo e um
// builtin/param (sempre tem valor) ou um OpGet*Chk (sem valor -> "cade o").
func (c *Compiler) emitLeitura(nome string, linha int) error {
	cands := c.candidatos(nome)
	if len(cands) == 0 {
		// erro do PROGRAMA, nao da VM: quem le tem que saber a linha e o que
		// fazer, nao que existe um compilador por baixo.
		return fmt.Errorf("linha %d: nao existe nenhum `%s` por aqui — confere o nome ou declara com `bota %s = ...`", linha, nome, nome)
	}
	var saltos []int
	for i, cd := range cands {
		switch {
		case !cd.podeFaltar:
			c.emitVarGet(cd.sym)
		case i == len(cands)-1:
			c.emitGetChk(cd.sym, c.addConstant(&object.Texto{Value: nome}))
		default:
			saltos = append(saltos, c.emitGetOu(cd.sym))
			continue
		}
		break
	}
	fim := len(c.instructions)
	for _, pos := range saltos {
		c.backpatch(pos, fim)
	}
	return nil
}

// emitGetChk emite a leitura que da "cade o `nome`?" se o simbolo nao tiver
// valor (fim da cadeia).
func (c *Compiler) emitGetChk(sym Symbol, nomeIdx int) {
	switch sym.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobalChk, sym.Index, nomeIdx)
	case FreeScope:
		c.emit(code.OpGetFreeChk, sym.Index, nomeIdx)
	default:
		if sym.Celula {
			c.emit(code.OpGetCelulaChk, sym.Index, nomeIdx)
		} else {
			c.emit(code.OpGetLocalChk, sym.Index, nomeIdx)
		}
	}
}

// emitGetOu emite a leitura "se tiver valor, pula" do simbolo e devolve a
// posicao pro backpatch do alvo.
func (c *Compiler) emitGetOu(sym Symbol) int {
	switch sym.Scope {
	case GlobalScope:
		return c.emit(code.OpGetGlobalOu, 9999, sym.Index)
	case FreeScope:
		return c.emit(code.OpGetFreeOu, 9999, sym.Index)
	default:
		if sym.Celula {
			return c.emit(code.OpGetCelulaOu, 9999, sym.Index)
		}
		return c.emit(code.OpGetLocalOu, 9999, sym.Index)
	}
}

// declaraNoTopo cria antes de rodar a global de cada nome que o trecho de topo
// bota — assim uma gambiarra declarada antes enxerga a global declarada
// depois (igual o tree-walker, que procura o nome na hora da chamada).
func (c *Compiler) declaraNoTopo(stmts []ast.Statement) {
	if c.scope.outer != nil {
		return
	}
	for _, nome := range nomesDeclarados(stmts) {
		c.scope.Define(nome)
	}
}
