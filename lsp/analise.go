package lsp

import (
	"os"
	"path/filepath"
	"reflect"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/token"
)

// ---- analise de nomes (base de definicao/referencias/renomear) ----
//
// Escopo igual ao runtime: so gambiarra (nomeada ou lambda) abre escopo novo;
// blocos (se_colar, enquanto, pra_cada, arruma, escolhe) dividem o escopo da
// funcao, estilo Python. `bota x` dentro de uma gambiarra SEMPRE escreve no
// escopo da gambiarra (Environment.Set), entao:
//
//   - uma ligacao (bota/crava/param/pra_cada/quebrou/desestrutura/importa como)
//     pertence ao simbolo (escopo, nome) do escopo onde aparece;
//   - um uso resolve pro escopo atual se o nome ja foi ligado nele ANTES (na
//     ordem de avaliacao: o lado direito do bota roda antes da ligacao);
//     senao, pro primeiro escopo de fora que liga o nome em qualquer ponto
//     (a closure roda depois); senao, pra uma ligacao mais adiante no proprio
//     escopo; senao, pros `importa "x.gs"` sem alias; senao e builtin/indefinido.
//
// A declaracao de um simbolo e a primeira ligacao dele no escopo.

type tipoLigacao int

const (
	ligUso tipoLigacao = iota
	ligBota
	ligCrava
	ligGambiarra
	ligParam
	ligPraCada
	ligQuebrou
	ligDesestrutura
	ligDesestruturaChave // bota {x, y} = dict: o nome E a chave
	ligImporta           // alias do `importa "x.gs" como m`
	ligComposta          // x += 1: le e escreve
	ligTipo              // treta/combinado (POO)
)

type escopo struct {
	a        *analise
	pai      *escopo
	ligacoes map[string][]*ocorrencia // ligacoes deste escopo, na ordem
	vistos   map[string]bool          // ja ligados durante a resolucao
	simbolos map[string]*simbolo
}

func novoEscopo(a *analise, pai *escopo) *escopo {
	return &escopo{a: a, pai: pai, ligacoes: map[string][]*ocorrencia{}, vistos: map[string]bool{}, simbolos: map[string]*simbolo{}}
}

// simbolo devolve (criando se preciso) o simbolo do nome neste escopo.
func (e *escopo) simbolo(nome string, padrao *ocorrencia) *simbolo {
	if s, ok := e.simbolos[nome]; ok {
		return s
	}
	decl := padrao
	if ls := e.ligacoes[nome]; len(ls) > 0 {
		decl = ls[0]
	}
	s := &simbolo{nome: nome, escopo: e, decl: decl}
	if e.pai == nil && e.a.caminho != "" {
		s.chave = e.a.caminho + "#" + nome
	}
	e.simbolos[nome] = s
	return s
}

// contem diz se o escopo `e` e `alvo` ou esta dentro dele.
func (e *escopo) contem(alvo *escopo) bool {
	for x := e; x != nil; x = x.pai {
		if x == alvo {
			return true
		}
	}
	return false
}

// liga diz se o nome tem ligacao neste escopo ou em algum de fora.
func (e *escopo) ligaNaCadeia(nome string) bool {
	for x := e; x != nil; x = x.pai {
		if len(x.ligacoes[nome]) > 0 {
			return true
		}
	}
	return false
}

type simbolo struct {
	nome   string
	escopo *escopo     // escopo dono (no arquivo da declaracao)
	decl   *ocorrencia // primeira ligacao
	// chave identifica simbolos de topo entre arquivos: "<caminho>#<nome>".
	// Vazia pra simbolo local de gambiarra (so vale no proprio documento).
	chave string
	// externo: o simbolo foi achado num arquivo importado (decl e escopo sao
	// de la; mod e a analise daquele arquivo).
	mod *analise
}

type ocorrencia struct {
	nome   string
	linha  int // 1-based, absoluta no documento
	col    int // 1-based, em runes
	tam    int // em runes
	escopo *escopo
	lig    tipoLigacao
	simb   *simbolo
	// campo: nome depois do ponto em `alias.nome` (aliasDe = ocorrencia do alias)
	campo   bool
	aliasDe *ocorrencia
	// params: assinatura quando a ligacao e uma gambiarra (ou bota f = lambda)
	params []*ast.Parametro
	ehFunc bool
}

type importacao struct {
	caminho   string // como escrito no fonte
	resolvido string // absoluto ("" se nao deu pra resolver)
	alias     *ocorrencia
	linha     int
	col       int
	tam       int // texto do caminho, com as aspas
}

// mapaPos converte posicao do token (sistema de coordenadas atual) pra
// absoluta no documento. Identidade no topo; dentro de `${...}` desloca pro
// lugar da interpolacao.
type mapaPos func(l, c int) (int, int)

func mapaIdentidade(l, c int) (int, int) { return l, c }

type analise struct {
	ctx         *contextoAnalise
	caminho     string // arquivo do documento ("" se nao for arquivo)
	uri         string // URI do editor, quando veio de um doc aberto
	texto       string
	linhas      linhasDoc
	prog        *ast.Program
	errosParse  int
	raiz        *escopo
	ocorrencias []*ocorrencia
	importacoes []*importacao
	porAlias    map[*ocorrencia]*importacao
	ternarios   map[[2]int]bool // posicoes de `se_colar` que sao ternario
	posVistas   map[[2]int]bool
	externos    map[string]*simbolo
}

// contextoAnalise guarda as analises de modulos de uma requisicao (cada
// arquivo e analisado uma vez so, inclusive com importa ciclico).
type contextoAnalise struct {
	carregar func(caminho string) (string, bool)
	modulos  map[string]*analise
}

func novoContexto(carregar func(string) (string, bool)) *contextoAnalise {
	if carregar == nil {
		carregar = func(c string) (string, bool) {
			b, err := os.ReadFile(c)
			return string(b), err == nil
		}
	}
	return &contextoAnalise{carregar: carregar, modulos: map[string]*analise{}}
}

// resolveImporta acha o arquivo de um `importa` relativo com a mesma regra
// dos engines: do lado de quem importa e, se nao tiver, em gs_modulos/
// subindo pelos diretorios. Sem achar nada, fica o caminho do lado.
func (ctx *contextoAnalise) resolveImporta(dir, caminho string) string {
	candidatos := object.CandidatosModulo(dir, caminho)
	for _, c := range candidatos {
		if _, ok := ctx.carregar(c); ok {
			return c
		}
	}
	return candidatos[0]
}

// modulo devolve a analise do arquivo (cacheada), ou nil se nao der pra ler.
func (ctx *contextoAnalise) modulo(caminho string) *analise {
	if caminho == "" {
		return nil
	}
	caminho = filepath.Clean(caminho)
	if a, ok := ctx.modulos[caminho]; ok {
		return a
	}
	texto, ok := ctx.carregar(caminho)
	if !ok {
		ctx.modulos[caminho] = nil
		return nil
	}
	return ctx.analisar(texto, caminho)
}

// analisar parseia e resolve os nomes do texto. caminho pode ser "" (doc sem
// arquivo: importa relativo nao resolve).
func (ctx *contextoAnalise) analisar(texto, caminho string) *analise {
	if caminho != "" {
		caminho = filepath.Clean(caminho)
	}
	p := parser.New(lexer.New(texto))
	prog := p.ParseProgram()
	a := &analise{
		ctx:        ctx,
		caminho:    caminho,
		texto:      texto,
		linhas:     novasLinhas(texto),
		prog:       prog,
		errosParse: len(p.Errors()),
		porAlias:   map[*ocorrencia]*importacao{},
		ternarios:  map[[2]int]bool{},
		posVistas:  map[[2]int]bool{},
		externos:   map[string]*simbolo{},
	}
	a.raiz = novoEscopo(a, nil)
	for _, s := range prog.Statements {
		a.andaStmt(s, a.raiz, mapaIdentidade)
	}
	// entra no cache ANTES de resolver: importa ciclico enxerga as ligacoes
	// de topo (ja completas) sem recursao infinita.
	if caminho != "" {
		ctx.modulos[caminho] = a
	}
	a.resolver()
	return a
}

// ---- passe 1: coleta ocorrencias e ligacoes ----

func (a *analise) anota(id *ast.Identifier, esc *escopo, lig tipoLigacao, m mapaPos) *ocorrencia {
	if id == nil || id.Value == "" {
		return nil
	}
	l, c := m(id.Token.Line, id.Token.Coluna)
	chave := [2]int{l, c}
	if a.posVistas[chave] {
		return nil // no compartilhado no AST (ex: desugar do +=)
	}
	a.posVistas[chave] = true
	o := &ocorrencia{nome: id.Value, linha: l, col: c, tam: len([]rune(id.Value)), escopo: esc, lig: lig}
	a.ocorrencias = append(a.ocorrencias, o)
	if lig != ligUso && lig != ligComposta {
		esc.ligacoes[o.nome] = append(esc.ligacoes[o.nome], o)
	}
	return o
}

func (a *analise) andaBloco(b *ast.BlockStatement, esc *escopo, m mapaPos) {
	if b == nil {
		return
	}
	for _, s := range b.Statements {
		a.andaStmt(s, esc, m)
	}
}

func (a *analise) andaParams(ps []*ast.Parametro, esc *escopo, m mapaPos) {
	for _, p := range ps {
		if p == nil {
			continue
		}
		// o padrao roda na hora da chamada e enxerga os params anteriores
		a.andaExpr(p.Padrao, esc, m)
		a.anota(p.Nome, esc, ligParam, m)
	}
}

func (a *analise) andaStmt(s ast.Statement, esc *escopo, m mapaPos) {
	if ehNil(s) {
		return
	}
	switch n := s.(type) {
	case *ast.BotaStatement:
		if n.OpComposto != "" {
			// `x += 1` virou Value = x + 1 com o MESMO no do alvo na esquerda
			if inf, ok := n.Value.(*ast.InfixExpression); ok {
				a.andaExpr(inf.Right, esc, m)
			} else {
				a.andaExpr(n.Value, esc, m)
			}
			if n.Name != nil {
				a.anota(n.Name, esc, ligComposta, m)
			} else {
				a.andaExpr(n.Indice, esc, m)
			}
			return
		}
		a.andaExpr(n.Value, esc, m)
		if n.Name != nil {
			if o := a.anota(n.Name, esc, ligBota, m); o != nil {
				if fl, ok := n.Value.(*ast.FuncaoLiteral); ok && fl != nil {
					o.params, o.ehFunc = fl.Parameters, true
				}
			}
		} else {
			a.andaExpr(n.Indice, esc, m)
		}
	case *ast.CravaStatement:
		a.andaExpr(n.Value, esc, m)
		if o := a.anota(n.Name, esc, ligCrava, m); o != nil {
			if fl, ok := n.Value.(*ast.FuncaoLiteral); ok && fl != nil {
				o.params, o.ehFunc = fl.Parameters, true
			}
		}
	case *ast.MostraStatement:
		a.andaExpr(n.Value, esc, m)
	case *ast.FuncionaStatement:
		a.andaExpr(n.Value, esc, m)
	case *ast.ExpressionStatement:
		a.andaExpr(n.Expression, esc, m)
	case *ast.BlockStatement:
		a.andaBloco(n, esc, m)
	case *ast.GambiarraStatement:
		if o := a.anota(n.Name, esc, ligGambiarra, m); o != nil {
			o.params, o.ehFunc = n.Parameters, true
		}
		dentro := novoEscopo(a, esc)
		a.andaParams(n.Parameters, dentro, m)
		a.andaBloco(n.Body, dentro, m)
	case *ast.SeColarStatement:
		for i, c := range n.Conditions {
			a.andaExpr(c, esc, m)
			if i < len(n.Consequences) {
				a.andaBloco(n.Consequences[i], esc, m)
			}
		}
		a.andaBloco(n.Alternative, esc, m)
	case *ast.EnquantoStatement:
		a.andaExpr(n.Condition, esc, m)
		a.andaBloco(n.Body, esc, m)
	case *ast.PraCadaNumStatement:
		a.andaExpr(n.Start, esc, m)
		a.andaExpr(n.End, esc, m)
		a.anota(n.Var, esc, ligPraCada, m)
		a.andaBloco(n.Body, esc, m)
	case *ast.PraCadaListStatement:
		a.andaExpr(n.Iterable, esc, m)
		for _, v := range n.Vars {
			a.anota(v, esc, ligPraCada, m)
		}
		a.andaBloco(n.Body, esc, m)
	case *ast.ArrumaStatement:
		a.andaBloco(n.Try, esc, m)
		for _, q := range n.Quebrous {
			a.anota(q.Nome, esc, ligQuebrou, m)
			if q.Filtro != nil {
				a.andaExpr(q.Filtro, esc, m)
			}
			a.andaBloco(q.Corpo, esc, m)
		}
		a.andaBloco(n.Finally, esc, m)
	case *ast.DesestruturaStatement:
		a.andaExpr(n.Value, esc, m)
		lig := ligDesestrutura
		if n.DeDict {
			lig = ligDesestruturaChave
		}
		for _, nome := range n.Names {
			a.anota(nome, esc, lig, m)
		}
	case *ast.EscolheStatement:
		a.andaExpr(n.Subject, esc, m)
		for _, braco := range n.Casos {
			for _, v := range braco.Values {
				a.andaExpr(v, esc, m)
			}
			a.andaBloco(braco.Body, esc, m)
		}
		a.andaBloco(n.Default, esc, m)
	case *ast.ImportaStatement:
		imp := &importacao{}
		if t, ok := n.Path.(*ast.TextoLiteral); ok && t != nil {
			l, c := m(t.Token.Line, t.Token.Coluna)
			imp.caminho, imp.linha, imp.col, imp.tam = t.Value, l, c, len([]rune(t.Value))+2
			if filepath.IsAbs(t.Value) {
				imp.resolvido = filepath.Clean(t.Value)
			} else if a.caminho != "" {
				imp.resolvido = a.ctx.resolveImporta(filepath.Dir(a.caminho), t.Value)
			}
		} else {
			a.andaExpr(n.Path, esc, m)
		}
		if n.Alias != nil {
			imp.alias = a.anota(n.Alias, esc, ligImporta, m)
			if imp.alias != nil {
				a.porAlias[imp.alias] = imp
			}
		}
		a.importacoes = append(a.importacoes, imp)
	case *ast.VazaStatement, *ast.ContinuaStatement:
	// POO: treta/combinado ligam o nome no escopo; o metodo e uma gambiarra
	// com o receiver de primeiro parametro (usa o tipo, nao liga nome)
	case *ast.TretaDecl:
		a.anota(n.Nome, esc, ligTipo, m)
		for _, c := range n.Campos {
			if c == nil {
				continue
			}
			a.andaExpr(c.Embutida, esc, m)
			a.andaExpr(c.Padrao, esc, m)
		}
	case *ast.CombinadoDecl:
		a.anota(n.Nome, esc, ligTipo, m)
		for _, ass := range n.Metodos {
			if ass != nil {
				a.andaExpr(ass.Embutido, esc, m)
			}
		}
	case *ast.MetodoDecl:
		a.andaExpr(n.Tipo, esc, m)
		dentro := novoEscopo(a, esc)
		a.andaParams(n.ParametrosComReceptor(), dentro, m)
		a.andaBloco(n.Body, dentro, m)
	default:
		a.andaGenerico(s, esc, m)
	}
}

func (a *analise) andaExpr(e ast.Expression, esc *escopo, m mapaPos) *ocorrencia {
	if ehNil(e) {
		return nil
	}
	switch n := e.(type) {
	case *ast.Identifier:
		return a.anota(n, esc, ligUso, m)
	case *ast.PrefixExpression:
		a.andaExpr(n.Right, esc, m)
	case *ast.InfixExpression:
		a.andaExpr(n.Left, esc, m)
		a.andaExpr(n.Right, esc, m)
	case *ast.CallExpression:
		a.andaExpr(n.Function, esc, m)
		for _, arg := range n.Arguments {
			a.andaExpr(arg, esc, m)
		}
	case *ast.IndexExpression:
		esq := a.andaExpr(n.Left, esc, m)
		if n.Dot {
			// `alias.nome`: o nome so vira simbolo se o alias for um importa
			// (decidido na resolucao). Campo de dicionario comum fica de fora.
			if t, ok := n.Index.(*ast.TextoLiteral); ok && t != nil && esq != nil {
				l, c := m(t.Token.Line, t.Token.Coluna)
				if !a.posVistas[[2]int{l, c}] {
					a.posVistas[[2]int{l, c}] = true
					a.ocorrencias = append(a.ocorrencias, &ocorrencia{
						nome: t.Value, linha: l, col: c, tam: len([]rune(t.Value)),
						escopo: esc, lig: ligUso, campo: true, aliasDe: esq,
					})
				}
			}
		} else {
			a.andaExpr(n.Index, esc, m)
		}
	case *ast.FatiaExpression:
		a.andaExpr(n.Left, esc, m)
		a.andaExpr(n.Inicio, esc, m)
		a.andaExpr(n.Fim, esc, m)
	case *ast.ListaLiteral:
		for _, el := range n.Elements {
			a.andaExpr(el, esc, m)
		}
	case *ast.DicionarioLiteral:
		for _, p := range n.Pares {
			a.andaExpr(p.Chave, esc, m)
			a.andaExpr(p.Valor, esc, m)
		}
	case *ast.BoraExpression:
		if n.Call != nil {
			a.andaExpr(n.Call, esc, m)
		}
	case *ast.FuncaoLiteral:
		dentro := novoEscopo(a, esc)
		a.andaParams(n.Parameters, dentro, m)
		a.andaBloco(n.Body, dentro, m)
	case *ast.RangeExpression:
		a.andaExpr(n.Start, esc, m)
		a.andaExpr(n.End, esc, m)
	case *ast.TernarioExpression:
		l, c := m(n.Token.Line, n.Token.Coluna)
		a.ternarios[[2]int{l, c}] = true
		a.andaExpr(n.Cond, esc, m)
		a.andaExpr(n.SeVerdadeiro, esc, m)
		a.andaExpr(n.SeFalso, esc, m)
	case *ast.CoalesceExpression:
		a.andaExpr(n.Left, esc, m)
		a.andaExpr(n.Right, esc, m)
	case *ast.TextoInterpolado:
		a.andaInterpolado(n, esc, m)
	case *ast.TretaLiteral:
		// o nome do campo (`x:` em Ponto{x: 1}) nao e variavel: so o tipo e os valores
		a.andaExpr(n.Tipo, esc, m)
		for _, v := range n.Valores {
			a.andaExpr(v, esc, m)
		}
	case *ast.NumeroLiteral, *ast.TextoLiteral, *ast.BooleanoLiteral, *ast.NadaLiteral:
	default:
		a.andaGenerico(e, esc, m)
	}
	return nil
}

// andaGenerico visita, por reflexao, os filhos de um no que esta analise nao
// conhece (no novo no AST): melhor tratar os nomes de dentro como uso do que
// sumir com eles das referencias (renomear deixaria o codigo quebrado).
func (a *analise) andaGenerico(no interface{}, esc *escopo, m mapaPos) {
	v := reflect.ValueOf(no)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	var visita func(f reflect.Value)
	visita = func(f reflect.Value) {
		if !f.IsValid() || !f.CanInterface() {
			return
		}
		switch f.Kind() {
		case reflect.Slice:
			for i := 0; i < f.Len(); i++ {
				visita(f.Index(i))
			}
			return
		case reflect.Ptr, reflect.Interface:
			if f.IsNil() {
				return
			}
		default:
			return
		}
		switch x := f.Interface().(type) {
		case ast.Expression:
			a.andaExpr(x, esc, m)
		case ast.Statement:
			a.andaStmt(x, esc, m)
		case *ast.Parametro:
			a.andaParams([]*ast.Parametro{x}, esc, m)
		}
	}
	for i := 0; i < v.NumField(); i++ {
		visita(v.Field(i))
	}
}

// andaInterpolado visita as expressoes de `"... ${expr} ..."`. O sub-parser
// do `${}` devolve tokens com posicao relativa ao pedaco interpolado; aqui a
// gente reescaneia o fonte da string pra saber onde cada `${` comeca e monta
// o mapa de posicao certo pra cada parte.
func (a *analise) andaInterpolado(n *ast.TextoInterpolado, esc *escopo, m mapaPos) {
	l, c := m(n.Token.Line, n.Token.Coluna)
	inicios := inicioInterpolacoes(a.linhas, l, c)
	k := 0
	for _, parte := range n.Parts {
		if t, ok := parte.(*ast.TextoLiteral); ok && t != nil && t.Token == n.Token {
			continue // pedaco literal (o parser usa o token da string toda)
		}
		if k >= len(inicios) {
			break // fonte e AST discordam: melhor nao inventar posicao
		}
		l0, c0 := inicios[k][0], inicios[k][1]
		sub := func(ll, cc int) (int, int) {
			if ll == 1 {
				return l0, c0 + cc - 1
			}
			return l0 + ll - 1, cc
		}
		a.andaExpr(parte, esc, sub)
		k++
	}
}

// inicioInterpolacoes reproduz o lexer (readString/readRawString) e o
// parser (interpolar) sobre o fonte da string que comeca em (l, c), 1-based,
// e devolve a posicao absoluta do primeiro char de cada expressao `${...}`.
func inicioInterpolacoes(ls linhasDoc, l, c int) [][2]int {
	cur := cursorFonte{ls: ls, li: l - 1, ci: c - 1}
	abre := cur.atual()
	var dec []rune
	var pos [][2]int
	add := func(r rune, p [2]int) {
		dec = append(dec, r)
		pos = append(pos, p)
	}
	switch abre {
	case '`':
		cur.avanca()
		for ch := cur.atual(); ch != '`' && ch != 0; ch = cur.atual() {
			add(ch, cur.pos())
			cur.avanca()
		}
	case '"':
		cur.avanca()
		for {
			ch := cur.atual()
			if ch == 0 || ch == '"' {
				break
			}
			if ch == '$' && cur.espia() == '{' {
				add('$', cur.pos())
				cur.avanca()
				add('{', cur.pos())
				cur.avanca()
				prof := 1
				for prof > 0 && cur.atual() != 0 {
					ch = cur.atual()
					if ch == '{' {
						prof++
					} else if ch == '}' {
						prof--
					}
					add(ch, cur.pos())
					cur.avanca()
				}
				continue
			}
			if ch == '\\' {
				switch cur.espia() {
				case '"', '\\', 'n', 't':
					p := cur.pos()
					cur.avanca()
					add(cur.atual(), p)
					cur.avanca()
				default:
					add('\\', cur.pos())
					cur.avanca()
				}
				continue
			}
			add(ch, cur.pos())
			cur.avanca()
		}
	default:
		return nil
	}
	var out [][2]int
	for i := 0; i < len(dec); {
		if i+2 < len(dec) && dec[i] == '\\' && dec[i+1] == '$' && dec[i+2] == '{' {
			i += 3
			continue
		}
		if i+1 < len(dec) && dec[i] == '$' && dec[i+1] == '{' {
			inicio := i + 2
			prof := 1
			j := inicio
			for j < len(dec) && prof > 0 {
				if dec[j] == '{' {
					prof++
				} else if dec[j] == '}' {
					prof--
				}
				if prof == 0 {
					break
				}
				j++
			}
			if prof != 0 {
				break
			}
			if inicio < len(pos) {
				out = append(out, pos[inicio])
			}
			i = j + 1
			continue
		}
		i++
	}
	return out
}

// ehNil cobre o nil de interface e o ponteiro nil embrulhado numa interface.
func ehNil(x interface{}) bool {
	if x == nil {
		return true
	}
	v := reflect.ValueOf(x)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

// ---- passe 2: resolve cada ocorrencia ----

func (a *analise) resolver() {
	for _, o := range a.ocorrencias {
		switch {
		case o.campo:
			o.simb = a.resolveCampo(o)
		case o.lig == ligUso:
			o.simb = a.resolveUso(o)
		case o.lig == ligComposta:
			o.simb = a.resolveUso(o)
			if o.simb == nil {
				// `x += 1` sem x antes: vira ligacao local (o runtime reclama,
				// mas pro editor e o melhor palpite)
				o.escopo.ligacoes[o.nome] = append(o.escopo.ligacoes[o.nome], o)
				o.simb = o.escopo.simbolo(o.nome, o)
				o.escopo.vistos[o.nome] = true
			}
		default:
			o.simb = o.escopo.simbolo(o.nome, o)
			o.escopo.vistos[o.nome] = true
		}
	}
}

func (a *analise) resolveUso(o *ocorrencia) *simbolo {
	e := o.escopo
	if e.vistos[o.nome] {
		return e.simbolo(o.nome, o)
	}
	for p := e.pai; p != nil; p = p.pai {
		if len(p.ligacoes[o.nome]) > 0 {
			return p.simbolo(o.nome, o)
		}
	}
	if len(e.ligacoes[o.nome]) > 0 {
		return e.simbolo(o.nome, o)
	}
	return a.externo(o.nome, map[string]bool{})
}

// resolveCampo trata `alias.nome` quando o alias e de um importa.
func (a *analise) resolveCampo(o *ocorrencia) *simbolo {
	al := o.aliasDe
	if al == nil || al.simb == nil || al.simb.mod != nil || al.simb.decl == nil || al.simb.decl.lig != ligImporta {
		return nil
	}
	imp := al.simb.escopo.a.porAlias[al.simb.decl]
	if imp == nil {
		return nil
	}
	mod := a.ctx.modulo(imp.resolvido)
	if mod == nil {
		return nil
	}
	return mod.simboloDeTopo(o.nome, map[string]bool{})
}

// simboloDeTopo devolve o simbolo de topo `nome` deste modulo (ligado nele
// ou trazido por um importa sem alias), marcado como externo.
func (a *analise) simboloDeTopo(nome string, visitados map[string]bool) *simbolo {
	if len(a.raiz.ligacoes[nome]) > 0 {
		local := a.raiz.simbolo(nome, nil)
		return &simbolo{nome: nome, escopo: a.raiz, decl: local.decl, chave: local.chave, mod: a}
	}
	return a.externo(nome, visitados)
}

// externo procura `nome` nos arquivos importados sem alias (o importa
// despeja as definicoes do modulo no escopo de quem importa).
func (a *analise) externo(nome string, visitados map[string]bool) *simbolo {
	if s, ok := a.externos[nome]; ok {
		return s
	}
	if a.caminho != "" {
		if visitados[a.caminho] {
			return nil
		}
		visitados[a.caminho] = true
	}
	var achado *simbolo
	for _, imp := range a.importacoes {
		if imp.alias != nil || imp.resolvido == "" {
			continue
		}
		mod := a.ctx.modulo(imp.resolvido)
		if mod == nil {
			continue
		}
		if s := mod.simboloDeTopo(nome, visitados); s != nil {
			achado = s
			break
		}
	}
	a.externos[nome] = achado
	return achado
}

// ---- consultas ----

// ocorrenciaEm acha a ocorrencia sob a posicao LSP (aceita o cursor colado
// no fim do nome, como os editores mandam depois de digitar).
func (a *analise) ocorrenciaEm(p Posicao) *ocorrencia {
	linha := p.Line + 1
	col := a.linhas.utf16ParaRune(p.Line, p.Character) + 1
	var borda *ocorrencia
	for _, o := range a.ocorrencias {
		if o.linha != linha {
			continue
		}
		if o.col <= col && col < o.col+o.tam {
			return o
		}
		if col == o.col+o.tam && borda == nil {
			borda = o
		}
	}
	return borda
}

// importacaoEm acha o importa cujo texto do caminho esta sob a posicao.
func (a *analise) importacaoEm(p Posicao) *importacao {
	linha := p.Line + 1
	col := a.linhas.utf16ParaRune(p.Line, p.Character) + 1
	for _, imp := range a.importacoes {
		if imp.linha == linha && imp.col <= col && col <= imp.col+imp.tam {
			return imp
		}
	}
	return nil
}

// mesmoSimbolo compara simbolos dentro do documento ou entre arquivos.
func mesmoSimbolo(x, y *simbolo) bool {
	if x == nil || y == nil {
		return false
	}
	if x.chave != "" || y.chave != "" {
		return x.chave == y.chave
	}
	return x.escopo == y.escopo && x.nome == y.nome
}

// ehNomeValido diz se `nome` e um identificador comum (nao keyword) da
// linguagem, do jeito que o lexer le.
func ehNomeValido(nome string) bool {
	if nome == "" {
		return false
	}
	l := lexer.New(nome)
	t := l.NextToken()
	return t.Type == token.IDENT && t.Literal == nome && l.NextToken().Type == token.EOF
}
