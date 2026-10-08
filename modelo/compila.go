// Package modelo e o motor de templates (HTML/texto) do GambiarraScript: o que
// tem por tras de renderiza(), renderiza_arquivo() e companhia.
//
// Sintaxe (docs completas em web/content/docs/modelos.mdx):
//
//	{{ usuario.nome }}              valor, escapado pra HTML
//	{{{ trecho }}} / {{ x | cru }}  valor cru (sem escapar)
//	{{ preco | formata "%.2f" }}    filtros: maiusculo minusculo tamanho json
//	                                formata padrao junta cru
//	{{ pra_cada i, item em itens }} ... {{ se_nao_colar }} ... {{ acabou }}
//	{{ se_colar x > 0 e nao y }} ... {{ se_nao_colar se_colar ... }} ... {{ se_nao_colar }} ... {{ acabou }}
//	{{# comentario }}
//	{{ inclui "parcial.html" }}
//	{{ usa "base.html" }} + {{ bloco "conteudo" }} ... {{ acabou }}
//
// O pacote so conhece object.Object; o que depende do interpretador (pra_json,
// formata) entra por Opcoes.
package modelo

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"gambiarrascript/object"
)

// Erro de modelo: sempre com o nome do modelo e, quando da, a linha.
type Erro struct {
	Modelo string
	Linha  int
	Msg    string
	IO     bool // arquivo que nao deu pra ler (vira erro "io" no builtin)
}

func (e *Erro) Error() string {
	if e.Linha > 0 {
		return fmt.Sprintf("modelo %s linha %d: %s", e.Modelo, e.Linha, e.Msg)
	}
	return fmt.Sprintf("modelo %s: %s", e.Modelo, e.Msg)
}

// Modelo e um template ja compilado (imutavel: da pra usar de varias
// goroutines ao mesmo tempo).
type Modelo struct {
	Nome     string // o que aparece nas mensagens de erro
	Caminho  string // arquivo absoluto ("" pra modelo de texto)
	nos      []no
	usa      string // layout (`{{ usa "base.html" }}`), "" se nao usa
	usaLinha int
	blocos   map[string]*noBloco
}

// ---------------------------------------------------------------- nos

type no interface{}

type noTexto struct{ s string }

type noValor struct {
	ex    expr
	cru   bool
	linha int
}

type noPraCada struct {
	chave, valor string // chave "" quando so tem um nome
	col          expr
	corpo, vazio []no
	linha        int
}

type ramo struct {
	cond  expr
	corpo []no
}

type noSe struct {
	ramos []ramo
	senao []no
	linha int
}

type noInclui struct {
	caminho string
	linha   int
}

type noBloco struct {
	nome  string
	corpo []no
	linha int
}

// ---------------------------------------------------------------- expressoes

type expr interface{}

type exCaminho struct {
	raiz   string
	passos []passo
	texto  string // como foi escrito (pras mensagens)
}

type passo struct {
	nome string // .nome
	idx  expr   // [idx] (quando nome == "")
}

type exLiteral struct{ v object.Object }

type exFiltro struct {
	base  expr
	nome  string
	arg   expr
	linha int
}

type exNao struct{ x expr }

type exBin struct {
	op   string // e ou == != > < >= <=
	a, b expr
}

// filtros conhecidos: nome -> leva argumento?
var filtros = map[string]bool{
	"maiusculo": false,
	"minusculo": false,
	"tamanho":   false,
	"json":      false,
	"cru":       false,
	"formata":   true,
	"padrao":    true,
	"junta":     true,
}

// ---------------------------------------------------------------- lexer de etiquetas

type tipoEtiqueta int

const (
	etValor tipoEtiqueta = iota
	etCru
	etComentario
)

type pedaco struct {
	texto bool
	s     string // texto cru, ou o miolo da etiqueta
	tipo  tipoEtiqueta
	linha int
}

// racha separa a fonte em texto e etiquetas `{{ }}` / `{{{ }}}` / `{{# }}`.
func racha(nome, fonte string) ([]pedaco, error) {
	var out []pedaco
	linha := 1
	i := 0
	for i < len(fonte) {
		j := strings.Index(fonte[i:], "{{")
		if j < 0 {
			out = append(out, pedaco{texto: true, s: fonte[i:], linha: linha})
			break
		}
		if j > 0 {
			out = append(out, pedaco{texto: true, s: fonte[i : i+j], linha: linha})
			linha += strings.Count(fonte[i:i+j], "\n")
		}
		ini := i + j
		switch {
		case strings.HasPrefix(fonte[ini:], "{{{"):
			fim := strings.Index(fonte[ini+3:], "}}}")
			if fim < 0 {
				return nil, &Erro{Modelo: nome, Linha: linha, Msg: "`{{{` sem `}}}`"}
			}
			miolo := fonte[ini+3 : ini+3+fim]
			out = append(out, pedaco{s: miolo, tipo: etCru, linha: linha})
			linha += strings.Count(miolo, "\n")
			i = ini + 3 + fim + 3
		case strings.HasPrefix(fonte[ini:], "{{#"):
			fim := strings.Index(fonte[ini+3:], "}}")
			if fim < 0 {
				return nil, &Erro{Modelo: nome, Linha: linha, Msg: "comentario `{{#` sem `}}`"}
			}
			miolo := fonte[ini+3 : ini+3+fim]
			out = append(out, pedaco{s: miolo, tipo: etComentario, linha: linha})
			linha += strings.Count(miolo, "\n")
			i = ini + 3 + fim + 2
		default:
			// acha o `}}` fora de string ("}}" dentro de aspas nao fecha)
			k := ini + 2
			emTexto := false
			fim := -1
			for k < len(fonte) {
				c := fonte[k]
				if emTexto {
					if c == '\\' {
						k += 2
						continue
					}
					if c == '"' {
						emTexto = false
					}
				} else if c == '"' {
					emTexto = true
				} else if c == '}' && k+1 < len(fonte) && fonte[k+1] == '}' {
					fim = k
					break
				}
				k++
			}
			if fim < 0 {
				return nil, &Erro{Modelo: nome, Linha: linha, Msg: "`{{` sem `}}`"}
			}
			miolo := fonte[ini+2 : fim]
			out = append(out, pedaco{s: miolo, linha: linha})
			linha += strings.Count(miolo, "\n")
			i = fim + 2
		}
	}
	return out, nil
}

// ---------------------------------------------------------------- tokens da expressao

type tok struct {
	tipo byte // 'i' ident, 's' texto, 'n' numero, 'p' pontuacao, 0 fim
	s    string
	v    object.Object
}

func tokeniza(s string) ([]tok, string) {
	var out []tok
	rs := []rune(s)
	i := 0
	for i < len(rs) {
		c := rs[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case c == '_' || unicode.IsLetter(c):
			j := i
			for j < len(rs) && (rs[j] == '_' || unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j])) {
				j++
			}
			out = append(out, tok{tipo: 'i', s: string(rs[i:j])})
			i = j
		case unicode.IsDigit(c) || (c == '-' && i+1 < len(rs) && unicode.IsDigit(rs[i+1])):
			j := i + 1
			for j < len(rs) && (unicode.IsDigit(rs[j]) || rs[j] == '.' || rs[j] == '_') {
				j++
			}
			lit := strings.ReplaceAll(string(rs[i:j]), "_", "")
			if n, err := strconv.ParseInt(lit, 10, 64); err == nil {
				out = append(out, tok{tipo: 'n', s: lit, v: object.NumInt(n)})
			} else if f, err := strconv.ParseFloat(lit, 64); err == nil {
				out = append(out, tok{tipo: 'n', s: lit, v: object.NumFloat(f)})
			} else {
				return nil, "numero estranho: " + lit
			}
			i = j
		case c == '"':
			var b strings.Builder
			j := i + 1
			for ; j < len(rs) && rs[j] != '"'; j++ {
				if rs[j] == '\\' && j+1 < len(rs) {
					j++
					switch rs[j] {
					case 'n':
						b.WriteRune('\n')
					case 't':
						b.WriteRune('\t')
					case '"', '\\':
						b.WriteRune(rs[j])
					default:
						b.WriteRune('\\')
						b.WriteRune(rs[j])
					}
					continue
				}
				b.WriteRune(rs[j])
			}
			if j >= len(rs) {
				return nil, "texto sem fechar as aspas"
			}
			out = append(out, tok{tipo: 's', s: b.String(), v: &object.Texto{Value: b.String()}})
			i = j + 1
		default:
			dois := ""
			if i+1 < len(rs) {
				dois = string(rs[i : i+2])
			}
			switch dois {
			case "==", "!=", ">=", "<=":
				out = append(out, tok{tipo: 'p', s: dois})
				i += 2
				continue
			}
			if strings.ContainsRune(".[]|,<>()", c) {
				out = append(out, tok{tipo: 'p', s: string(c)})
				i++
				continue
			}
			return nil, fmt.Sprintf("nao sei o que e `%c` aqui", c)
		}
	}
	return out, ""
}

// ---------------------------------------------------------------- parser de expressao

type parserEx struct {
	toks  []tok
	pos   int
	linha int
}

func (p *parserEx) olha() tok {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return tok{}
}

func (p *parserEx) anda() tok { t := p.olha(); p.pos++; return t }

func (p *parserEx) eh(tipo byte, s string) bool {
	t := p.olha()
	return t.tipo == tipo && t.s == s
}

type erroEx string

// ou := e ("ou" e)*
func (p *parserEx) ou() expr {
	a := p.e()
	for p.eh('i', "ou") {
		p.anda()
		a = &exBin{op: "ou", a: a, b: p.e()}
	}
	return a
}

func (p *parserEx) e() expr {
	a := p.nao()
	for p.eh('i', "e") {
		p.anda()
		a = &exBin{op: "e", a: a, b: p.nao()}
	}
	return a
}

func (p *parserEx) nao() expr {
	if p.eh('i', "nao") {
		p.anda()
		return &exNao{x: p.nao()}
	}
	return p.compara()
}

func (p *parserEx) compara() expr {
	a := p.filtrado()
	t := p.olha()
	if t.tipo == 'p' {
		switch t.s {
		case "==", "!=", ">", "<", ">=", "<=":
			p.anda()
			return &exBin{op: t.s, a: a, b: p.filtrado()}
		}
	}
	return a
}

func (p *parserEx) filtrado() expr {
	x := p.primario()
	for p.eh('p', "|") {
		p.anda()
		t := p.anda()
		if t.tipo != 'i' {
			panic(erroEx("depois do `|` vem o nome do filtro"))
		}
		levaArg, ok := filtros[t.s]
		if !ok {
			panic(erroEx(fmt.Sprintf("filtro `%s` nao existe (tem: maiusculo, minusculo, tamanho, json, formata, padrao, junta, cru)", t.s)))
		}
		f := &exFiltro{base: x, nome: t.s, linha: p.linha}
		if levaArg {
			if p.olha().tipo == 0 {
				panic(erroEx(fmt.Sprintf("o filtro `%s` quer um argumento (ex.: `| %s \"...\"`)", t.s, t.s)))
			}
			f.arg = p.primario()
		}
		x = f
	}
	return x
}

func (p *parserEx) primario() expr {
	t := p.anda()
	switch t.tipo {
	case 's', 'n':
		return &exLiteral{v: t.v}
	case 'p':
		if t.s == "(" {
			x := p.ou()
			if !p.eh('p', ")") {
				panic(erroEx("faltou fechar o `)`"))
			}
			p.anda()
			return x
		}
	case 'i':
		switch t.s {
		case "deu_bom":
			return &exLiteral{v: &object.Booleano{Value: true}}
		case "deu_ruim":
			return &exLiteral{v: &object.Booleano{Value: false}}
		case "nada":
			return &exLiteral{v: &object.Nada{}}
		case "e", "ou", "nao", "em":
			panic(erroEx(fmt.Sprintf("`%s` fora do lugar", t.s)))
		}
		c := &exCaminho{raiz: t.s}
		texto := t.s
		for {
			if p.eh('p', ".") {
				p.anda()
				n := p.anda()
				if n.tipo != 'i' {
					panic(erroEx(fmt.Sprintf("depois de `%s.` vem um nome", texto)))
				}
				c.passos = append(c.passos, passo{nome: n.s})
				texto += "." + n.s
				continue
			}
			if p.eh('p', "[") {
				ini := p.pos
				p.anda()
				idx := p.ou()
				if !p.eh('p', "]") {
					panic(erroEx(fmt.Sprintf("faltou fechar o `[` em `%s`", texto)))
				}
				p.anda()
				c.passos = append(c.passos, passo{idx: idx})
				texto += "[" + juntaToks(p.toks[ini+1:p.pos-1]) + "]"
				continue
			}
			break
		}
		c.texto = texto
		return c
	case 0:
		panic(erroEx("faltou um valor"))
	}
	panic(erroEx(fmt.Sprintf("nao esperava `%s` aqui", t.s)))
}

func juntaToks(ts []tok) string {
	partes := make([]string, len(ts))
	for i, t := range ts {
		if t.tipo == 's' {
			partes[i] = strconv.Quote(t.s)
		} else {
			partes[i] = t.s
		}
	}
	return strings.Join(partes, "")
}

// compilaExpr compila a expressao inteira de toks (tem que consumir tudo).
func compilaExpr(toks []tok, linha int) (x expr, msg string) {
	p := &parserEx{toks: toks, linha: linha}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(erroEx)
			if !ok {
				panic(r)
			}
			x, msg = nil, string(e)
		}
	}()
	x = p.ou()
	if p.pos < len(p.toks) {
		return nil, fmt.Sprintf("sobrou `%s` no fim da expressao", juntaToks(p.toks[p.pos:]))
	}
	return x, ""
}

// ---------------------------------------------------------------- compilador

// abertura guarda o bloco aberto (pra casar com o `acabou`).
type abertura struct {
	tipo    string // pra_cada, se_colar, bloco
	linha   int
	praC    *noPraCada
	se      *noSe
	bloco   *noBloco
	destino *[]no // onde os nos estao indo agora
}

// Compila transforma a fonte num Modelo. nome e o que aparece nos erros.
func Compila(nome, fonte string) (*Modelo, error) {
	pedacos, err := racha(nome, fonte)
	if err != nil {
		return nil, err
	}
	tiraLinhasSozinhas(pedacos)
	m := &Modelo{Nome: nome, blocos: map[string]*noBloco{}}
	errAt := func(linha int, f string, a ...interface{}) error {
		return &Erro{Modelo: nome, Linha: linha, Msg: fmt.Sprintf(f, a...)}
	}
	raiz := &m.nos
	var pilha []*abertura
	destino := func() *[]no {
		if len(pilha) == 0 {
			return raiz
		}
		return pilha[len(pilha)-1].destino
	}
	poe := func(n no) { d := destino(); *d = append(*d, n) }

	for _, pd := range pedacos {
		if pd.texto {
			if pd.s != "" {
				poe(&noTexto{s: pd.s})
			}
			continue
		}
		if pd.tipo == etComentario {
			continue
		}
		toks, msg := tokeniza(pd.s)
		if msg != "" {
			return nil, errAt(pd.linha, "%s", msg)
		}
		if len(toks) == 0 {
			return nil, errAt(pd.linha, "`{{ }}` vazio")
		}
		if pd.tipo == etCru {
			x, msg := compilaExpr(toks, pd.linha)
			if msg != "" {
				return nil, errAt(pd.linha, "%s", msg)
			}
			poe(&noValor{ex: x, cru: true, linha: pd.linha})
			continue
		}
		cab := toks[0]
		if cab.tipo != 'i' {
			cab.s = ""
		}
		switch cab.s {
		case "pra_cada":
			// pra_cada item em lista | pra_cada i, item em lista
			n := &noPraCada{linha: pd.linha}
			resto := toks[1:]
			if len(resto) >= 1 && resto[0].tipo == 'i' {
				n.valor = resto[0].s
				resto = resto[1:]
			} else {
				return nil, errAt(pd.linha, "`pra_cada` quer um nome: `pra_cada item em itens`")
			}
			if len(resto) >= 2 && resto[0].tipo == 'p' && resto[0].s == "," && resto[1].tipo == 'i' {
				n.chave, n.valor = n.valor, resto[1].s
				resto = resto[2:]
			}
			if len(resto) == 0 || resto[0].tipo != 'i' || resto[0].s != "em" {
				return nil, errAt(pd.linha, "`pra_cada` sem o `em`: `pra_cada item em itens`")
			}
			x, msg := compilaExpr(resto[1:], pd.linha)
			if msg != "" {
				return nil, errAt(pd.linha, "pra_cada: %s", msg)
			}
			n.col = x
			poe(n)
			pilha = append(pilha, &abertura{tipo: "pra_cada", linha: pd.linha, praC: n, destino: &n.corpo})
		case "se_colar":
			x, msg := compilaExpr(toks[1:], pd.linha)
			if msg != "" {
				return nil, errAt(pd.linha, "se_colar: %s", msg)
			}
			n := &noSe{linha: pd.linha, ramos: []ramo{{cond: x}}}
			poe(n)
			pilha = append(pilha, &abertura{tipo: "se_colar", linha: pd.linha, se: n, destino: &n.ramos[0].corpo})
		case "se_nao_colar":
			if len(pilha) == 0 {
				return nil, errAt(pd.linha, "`se_nao_colar` sem `se_colar` (ou `pra_cada`) antes")
			}
			ab := pilha[len(pilha)-1]
			switch ab.tipo {
			case "se_colar":
				if ab.destino == &ab.se.senao {
					return nil, errAt(pd.linha, "`se_nao_colar` repetido (o `se_colar` da linha %d ja tem um)", ab.linha)
				}
				if len(toks) > 1 && toks[1].tipo == 'i' && toks[1].s == "se_colar" {
					x, msg := compilaExpr(toks[2:], pd.linha)
					if msg != "" {
						return nil, errAt(pd.linha, "se_nao_colar se_colar: %s", msg)
					}
					ab.se.ramos = append(ab.se.ramos, ramo{cond: x})
					ab.destino = &ab.se.ramos[len(ab.se.ramos)-1].corpo
					// os ponteiros dos ramos anteriores mudam com o append,
					// mas so o ultimo recebe nos daqui pra frente.
				} else if len(toks) > 1 {
					return nil, errAt(pd.linha, "`se_nao_colar` nao leva condicao (pra isso: `se_nao_colar se_colar ...`)")
				} else {
					ab.destino = &ab.se.senao
				}
			case "pra_cada":
				if len(toks) > 1 {
					return nil, errAt(pd.linha, "`se_nao_colar` do `pra_cada` nao leva condicao")
				}
				if ab.destino == &ab.praC.vazio {
					return nil, errAt(pd.linha, "`se_nao_colar` repetido no `pra_cada` da linha %d", ab.linha)
				}
				ab.destino = &ab.praC.vazio
			default:
				return nil, errAt(pd.linha, "`se_nao_colar` dentro de `%s` (linha %d) sem `se_colar`", ab.tipo, ab.linha)
			}
		case "acabou", "acabou_finalmente":
			if len(toks) > 1 {
				return nil, errAt(pd.linha, "`%s` nao leva nada depois", cab.s)
			}
			if len(pilha) == 0 {
				return nil, errAt(pd.linha, "`%s` sobrando (nao tem bloco aberto)", cab.s)
			}
			pilha = pilha[:len(pilha)-1]
		case "inclui", "usa", "bloco":
			if len(toks) != 2 || toks[1].tipo != 's' || toks[1].s == "" {
				return nil, errAt(pd.linha, "`%s` quer um texto entre aspas: `{{ %s \"nome\" }}`", cab.s, cab.s)
			}
			arg := toks[1].s
			switch cab.s {
			case "inclui":
				poe(&noInclui{caminho: arg, linha: pd.linha})
			case "usa":
				if len(pilha) > 0 {
					return nil, errAt(pd.linha, "`usa` tem que ficar fora de qualquer bloco")
				}
				if m.usa != "" {
					return nil, errAt(pd.linha, "`usa` repetido (ja usa %q desde a linha %d)", m.usa, m.usaLinha)
				}
				m.usa, m.usaLinha = arg, pd.linha
			case "bloco":
				if b, ok := m.blocos[arg]; ok {
					return nil, errAt(pd.linha, "bloco %q repetido (ja tem na linha %d)", arg, b.linha)
				}
				n := &noBloco{nome: arg, linha: pd.linha}
				m.blocos[arg] = n
				poe(n)
				pilha = append(pilha, &abertura{tipo: "bloco", linha: pd.linha, bloco: n, destino: &n.corpo})
			}
		default:
			x, msg := compilaExpr(toks, pd.linha)
			if msg != "" {
				return nil, errAt(pd.linha, "%s", msg)
			}
			poe(&noValor{ex: x, linha: pd.linha})
		}
	}
	if len(pilha) > 0 {
		ab := pilha[len(pilha)-1]
		return nil, errAt(ab.linha, "`%s` sem `acabou`", ab.tipo)
	}
	return m, nil
}

// ehControle diz se a etiqueta some inteira quando fica sozinha na linha
// (igual o Mustache): bloco, laco, condicao, comentario, inclui, usa.
func ehControle(pd pedaco) bool {
	if pd.texto {
		return false
	}
	if pd.tipo == etComentario {
		return true
	}
	if pd.tipo == etCru {
		return false
	}
	campos := strings.Fields(pd.s)
	if len(campos) == 0 {
		return false
	}
	switch campos[0] {
	case "pra_cada", "se_colar", "se_nao_colar", "acabou", "acabou_finalmente", "bloco", "usa", "inclui":
		return true
	}
	return false
}

// tiraLinhasSozinhas: etiqueta de controle sozinha na linha (so espaco em
// volta) leva a linha junto — assim `{{ pra_cada }}` numa linha propria nao
// deixa linha em branco no HTML. A decisao olha o texto ORIGINAL (antes de
// cortar qualquer coisa), igual o Mustache.
func tiraLinhasSozinhas(ps []pedaco) {
	ini := make([]int, len(ps)) // corte no comeco de cada texto
	fim := make([]int, len(ps)) // corte no fim
	for i := range ps {
		fim[i] = len(ps[i].s)
	}
	for i := range ps {
		if !ehControle(ps[i]) {
			continue
		}
		antesOk, cortaAntes := false, -1
		if i == 0 {
			antesOk = true
		} else if ps[i-1].texto {
			s := ps[i-1].s
			k := len(s)
			for k > 0 && (s[k-1] == ' ' || s[k-1] == '\t') {
				k--
			}
			if (k == 0 && i-1 == 0) || (k > 0 && s[k-1] == '\n') {
				antesOk, cortaAntes = true, k
			}
		}
		if !antesOk {
			continue
		}
		depoisOk, cortaDepois := false, -1
		if i == len(ps)-1 {
			depoisOk = true
		} else if ps[i+1].texto {
			s := ps[i+1].s
			k := 0
			for k < len(s) && (s[k] == ' ' || s[k] == '\t') {
				k++
			}
			switch {
			case k == len(s) && i+1 == len(ps)-1:
				depoisOk, cortaDepois = true, k
			case strings.HasPrefix(s[k:], "\r\n"):
				depoisOk, cortaDepois = true, k+2
			case strings.HasPrefix(s[k:], "\n"):
				depoisOk, cortaDepois = true, k+1
			}
		}
		if !depoisOk {
			continue
		}
		if cortaAntes >= 0 {
			fim[i-1] = cortaAntes
		}
		if cortaDepois >= 0 {
			ini[i+1] = cortaDepois
		}
	}
	for i := range ps {
		if !ps[i].texto {
			continue
		}
		if ini[i] >= fim[i] {
			ps[i].s = ""
		} else {
			ps[i].s = ps[i].s[ini[i]:fim[i]]
		}
	}
}
