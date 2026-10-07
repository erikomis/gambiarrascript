package formatter

import (
	"fmt"
	"sort"
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
	"gambiarrascript/token"
)

// ---- comentarios e linhas em branco ----
//
// O AST nao tem comentario: o lexer (modo trivia) junta os comentarios com
// posicao e o parser (GuardaPosicoes) diz onde cada statement/bloco comeca e
// acaba. O formatter reimprime o AST e vai soltando cada comentario no lugar:
//
//   - comentario sozinho na linha: linha propria, no nivel do bloco onde esta
//     (antes de se_nao_colar/caso/quebrou/finalmente/acabou_finalmente fica
//     no nivel de dentro do bloco que fecha);
//   - comentario depois do codigo: fica no fim da linha, com 2 espacos antes
//     (varios na mesma linha: 1 espaco entre eles);
//   - comentario no meio de uma expressao que vira uma linha so (chamada
//     quebrada em linhas, condicao comprida): sobe pra linha propria antes do
//     statement; se tava na ultima linha dele, vai pro fim da linha;
//   - /* */ sai verbatim (as linhas de dentro nao sao reindentadas);
//   - linha em branco entre statements/comentarios: no maximo 1, nunca no
//     comeco ou fim de bloco nem no comeco do arquivo.
//
// Lambda escrita em varias linhas sai em bloco; lista/dicionario que quebra
// linha logo depois do `[`/`{` sai um item por linha (sem virgula no fim).

type ponto struct{ l, c int }

func (a ponto) antes(b ponto) bool { return a.l < b.l || (a.l == b.l && a.c < b.c) }

func pontoDe(t token.Token) ponto { return ponto{t.Line, t.Coluna} }

var fimDoArquivo = ponto{l: 1 << 62}

type comentario struct {
	texto   string
	ini     ponto
	ancora  ponto // token antes dele
	sozinho bool
	branco  bool
	linha   bool // `#` (vai ate o fim da linha)
	usado   bool
}

// FormataFonte formata o codigo-fonte guardando comentarios e linhas em
// branco. Com erro de parse devolve os erros e nao formata.
func FormataFonte(src string) (string, []string) {
	l := lexer.NewComTrivia(src)
	p := parser.New(l)
	pos := p.GuardaPosicoes()
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return "", errs
	}
	tr := l.Trivia()
	f := novoFormatter()
	f.pos, f.branco = pos, tr.BrancoAntes
	for _, c := range tr.Comentarios {
		f.coms = append(f.coms, comentario{
			texto:   textoDoComentario(c.Texto),
			ini:     ponto{c.Linha, c.Coluna},
			ancora:  ponto{c.AntesLinha, c.AntesColuna},
			sozinho: c.Sozinho,
			branco:  c.BrancoAntes,
			linha:   strings.HasPrefix(c.Texto, "#"),
		})
	}
	f.emitStmts(prog.Statements, 0, fimDoArquivo)
	return f.out.String(), nil
}

// textoDoComentario: `#` perde o espaco/\r do fim; /* */ fica como veio
// (o `/*` que nunca fecha vai ate o EOF: perde so o branco do fim).
func textoDoComentario(s string) string {
	if strings.HasPrefix(s, "#") {
		return strings.TrimRight(s, " \t\r")
	}
	if !strings.HasSuffix(s, "*/") || len(s) < 4 {
		return strings.TrimRight(s, " \t\r\n")
	}
	return s
}

// Confere e a trava antes de escrever: a saida tem que parsear, dar o mesmo
// AST e ter exatamente os mesmos comentarios da fonte. Erro = nao escreve.
func Confere(fonte, saida string) error {
	prog1, coms1, errs := parseComComentarios(fonte)
	if len(errs) != 0 {
		return fmt.Errorf("a fonte tem erro de parse (%s)", errs[0])
	}
	prog2, coms2, errs := parseComComentarios(saida)
	if len(errs) != 0 {
		return fmt.Errorf("a saida formatada nao parseia (%s)", errs[0])
	}
	if prog1.String() != prog2.String() || Formata(prog1) != Formata(prog2) {
		return fmt.Errorf("a saida formatada mudaria o significado do codigo")
	}
	if len(coms1) != len(coms2) {
		return fmt.Errorf("a saida formatada tem %d comentarios, a fonte tem %d", len(coms2), len(coms1))
	}
	sort.Strings(coms1)
	sort.Strings(coms2)
	for i := range coms1 {
		if coms1[i] != coms2[i] {
			return fmt.Errorf("a saida formatada mudou o comentario %q", coms1[i])
		}
	}
	return nil
}

func parseComComentarios(src string) (*ast.Program, []string, []string) {
	l := lexer.NewComTrivia(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	var coms []string
	for _, c := range l.Trivia().Comentarios {
		coms = append(coms, textoDoComentario(c.Texto))
	}
	return prog, coms, p.Errors()
}

// ---- posicoes (zeradas quando nao tem fonte) ----

func (f *formatter) inicio(s ast.Statement) ponto {
	if f.pos == nil {
		return ponto{}
	}
	return pontoDe(f.pos.Inicio[s])
}

func (f *formatter) fim(s ast.Statement) ponto {
	if f.pos == nil {
		return ponto{}
	}
	return pontoDe(f.pos.Fim[s])
}

func (f *formatter) cabeca(n ast.Node) ponto {
	if f.pos == nil {
		return ponto{}
	}
	return pontoDe(f.pos.Cabeca[n])
}

func (f *formatter) fimBloco(b *ast.BlockStatement) ponto {
	if f.pos == nil {
		return ponto{}
	}
	return pontoDe(f.pos.FimBloco[b])
}

func (f *formatter) caso(b *ast.BlockStatement) ponto {
	if f.pos == nil {
		return ponto{}
	}
	return pontoDe(f.pos.Caso[b])
}

// ---- soltando comentarios ----

// soltaComentarios escreve, em linha propria, os comentarios ainda nao usados
// que vem antes de `limite` (e depois do piso).
func (f *formatter) soltaComentarios(limite ponto, nivel int, primeiro *bool) {
	for f.prox < len(f.coms) && f.coms[f.prox].usado {
		f.prox++
	}
	for i := f.prox; i < len(f.coms); i++ {
		c := &f.coms[i]
		if !c.ini.antes(limite) {
			break
		}
		if c.usado || !f.piso.antes(c.ini) {
			continue
		}
		c.usado = true
		if c.branco && !*primeiro {
			f.out.WriteString("\n")
		}
		f.escreve(nivel, c.texto)
		*primeiro = false
	}
}

// abre escreve a primeira linha de um statement (`cab`, que termina no token
// `fimCab`): linha em branco se tinha, os comentarios do meio dela que nao
// cabem no fim da linha (sobem) e o rabo.
func (f *formatter) abre(s ast.Statement, nivel int, primeiro *bool, cab string, fimCab ponto) {
	ini := f.inicio(s)
	if !*primeiro && f.branco[[2]int{ini.l, ini.c}] {
		f.out.WriteString("\n")
	}
	*primeiro = false
	var naLinha []string // do meio, mas na ultima linha: vao pro rabo
	for i := f.prox; i < len(f.coms); i++ {
		c := &f.coms[i]
		if !c.ini.antes(fimCab) {
			break
		}
		if c.usado || !ini.antes(c.ini) {
			continue
		}
		c.usado = true
		if !c.sozinho && c.ini.l == fimCab.l {
			naLinha = append(naLinha, c.texto)
			continue
		}
		f.escreve(nivel, c.texto)
	}
	f.escreve(nivel, cab+rabo(append(naLinha, f.pendurados(fimCab)...)))
}

// linha escreve uma linha de cabecalho/terminador com o rabo pendurado nela.
func (f *formatter) linha(nivel int, txt string, ancora ponto) {
	f.escreve(nivel, txt+rabo(f.pendurados(ancora)))
}

// rabo e o fim de linha com os comentarios: "  c1 c2" (ou "").
func rabo(partes []string) string {
	if len(partes) == 0 {
		return ""
	}
	return "  " + strings.Join(partes, " ")
}

// pendurados pega os comentarios pendurados no token `ancora` (depois dele,
// antes do proximo token, sem quebra de linha antes).
func (f *formatter) pendurados(ancora ponto) []string {
	if ancora == (ponto{}) {
		return nil
	}
	i := sort.Search(len(f.coms), func(i int) bool { return ancora.antes(f.coms[i].ini) })
	var partes []string
	for ; i < len(f.coms); i++ {
		c := &f.coms[i]
		if c.ancora != ancora || c.sozinho {
			break
		}
		if c.usado {
			continue
		}
		c.usado = true
		partes = append(partes, c.texto)
		if c.linha {
			break // `#` vai ate o fim da linha
		}
	}
	return partes
}

// raboFaixa e o rabo de um item de lista/dicionario em linhas: os
// comentarios nao-sozinhos entre o comeco dele e o do proximo (inclui o da
// virgula). Para no primeiro `#`; o resto vira linha propria depois.
func (f *formatter) raboFaixa(de, ate ponto) string {
	var partes []string
	for i := f.prox; i < len(f.coms); i++ {
		c := &f.coms[i]
		if !c.ini.antes(ate) {
			break
		}
		if c.usado || c.sozinho || !de.antes(c.ini) {
			continue
		}
		c.usado = true
		partes = append(partes, c.texto)
		if c.linha {
			break
		}
	}
	return rabo(partes)
}

// ---- lambda e lista/dicionario em varias linhas ----

func (f *formatter) lambdaEmBloco(n *ast.FuncaoLiteral, cab string) (string, bool) {
	if f.pos == nil || n.Body == nil {
		return "", false
	}
	fimTok, ok := f.pos.FimBloco[n.Body]
	if !ok || fimTok.Line <= n.Token.Line {
		return "", false
	}
	cab += rabo(f.pendurados(f.cabeca(n.Body)))
	velhoOut, velhoPiso, nivel := f.out, f.piso, f.nivel
	var corpo strings.Builder
	f.out, f.piso = &corpo, pontoDe(n.Token)
	f.emitStmts(n.Body.Statements, nivel+1, pontoDe(fimTok))
	f.out, f.piso, f.nivel = velhoOut, velhoPiso, nivel
	return cab + "\n" + corpo.String() + strings.Repeat(f.indent, nivel) + "acabou_finalmente", true
}

type itemEmLinha struct {
	ini   ponto
	texto func() string
}

func (f *formatter) listaEmLinhas(n *ast.ListaLiteral) (string, bool) {
	if f.pos == nil || len(n.Elements) == 0 {
		return "", false
	}
	itens := make([]itemEmLinha, len(n.Elements))
	for i, el := range n.Elements {
		el := el
		itens[i] = itemEmLinha{primeiroTok(el), func() string { return f.emitExpr(el) }}
	}
	return f.emLinhas(n, "[", "]", itens)
}

func (f *formatter) dictEmLinhas(n *ast.DicionarioLiteral) (string, bool) {
	if f.pos == nil || len(n.Pares) == 0 {
		return "", false
	}
	itens := make([]itemEmLinha, len(n.Pares))
	for i, p := range n.Pares {
		p := p
		itens[i] = itemEmLinha{primeiroTok(p.Chave), func() string {
			return f.emitExpr(p.Chave) + ": " + f.emitExpr(p.Valor)
		}}
	}
	return f.emLinhas(n, "{", "}", itens)
}

// tretaEmLinhas e o dictEmLinhas do literal `Tipo{...}`.
func (f *formatter) tretaEmLinhas(n *ast.TretaLiteral, tipo string) (string, bool) {
	if f.pos == nil || len(n.Valores) == 0 {
		return "", false
	}
	itens := make([]itemEmLinha, len(n.Valores))
	for i, v := range n.Valores {
		v := v
		ini := primeiroTok(v)
		txt := func() string { return f.emitExpr(v) }
		if n.Nomes != nil {
			nome := n.Nomes[i]
			ini = pontoDe(nome.Token)
			txt = func() string { return nome.Value + ": " + f.emitExpr(v) }
		}
		itens[i] = itemEmLinha{ini, txt}
	}
	return f.emLinhas(n, tipo+"{", "}", itens)
}

// emLinhas monta `[`/`{` + um item por linha + `]`/`}` quando o autor quebrou
// a linha logo depois de abrir (senao fica tudo numa linha).
func (f *formatter) emLinhas(n ast.Expression, abre, fecha string, itens []itemEmLinha) (string, bool) {
	fechaTok, ok := f.pos.Fecha[n]
	tokAbre := primeiroTok(n)
	if tl, ehTreta := n.(*ast.TretaLiteral); ehTreta {
		tokAbre = pontoDe(tl.Token) // o `{` (o comeco da expressao e o tipo)
	}
	if !ok || itens[0].ini.l <= tokAbre.l {
		return "", false
	}
	velhoOut, velhoPiso, nivel := f.out, f.piso, f.nivel
	var sb strings.Builder
	f.out, f.piso, f.nivel = &sb, tokAbre, nivel+1
	sb.WriteString(abre + rabo(f.pendurados(tokAbre)) + "\n")
	primeiro := true
	for i, it := range itens {
		f.soltaComentarios(it.ini, f.nivel, &primeiro)
		txt := it.texto()
		f.nivel = nivel + 1 // o item pode ter mexido (lambda em bloco restaura, mas garante)
		ate := pontoDe(fechaTok)
		if i+1 < len(itens) {
			txt += ","
			ate = itens[i+1].ini
		}
		f.escreve(f.nivel, txt+f.raboFaixa(it.ini, ate))
		primeiro = false
	}
	f.soltaComentarios(pontoDe(fechaTok), f.nivel, &primeiro)
	f.out, f.piso, f.nivel = velhoOut, velhoPiso, nivel
	return sb.String() + strings.Repeat(f.indent, nivel) + fecha, true
}

// primeiroTok e a posicao do token mais a esquerda da expressao (o Token de
// infix/chamada/indice e o operador, nao o comeco).
func primeiroTok(e ast.Expression) ponto {
	switch n := e.(type) {
	case *ast.InfixExpression:
		return primeiroTok(n.Left)
	case *ast.CallExpression:
		return primeiroTok(n.Function)
	case *ast.IndexExpression:
		return primeiroTok(n.Left)
	case *ast.FatiaExpression:
		return primeiroTok(n.Left)
	case *ast.RangeExpression:
		return primeiroTok(n.Start)
	case *ast.CoalesceExpression:
		return primeiroTok(n.Left)
	case *ast.ListaLiteral:
		return pontoDe(n.Token)
	case *ast.DicionarioLiteral:
		return pontoDe(n.Token)
	case *ast.Identifier:
		return pontoDe(n.Token)
	case *ast.NumeroLiteral:
		return pontoDe(n.Token)
	case *ast.TextoLiteral:
		return pontoDe(n.Token)
	case *ast.TextoInterpolado:
		return pontoDe(n.Token)
	case *ast.BooleanoLiteral:
		return pontoDe(n.Token)
	case *ast.NadaLiteral:
		return pontoDe(n.Token)
	case *ast.PrefixExpression:
		return pontoDe(n.Token)
	case *ast.FuncaoLiteral:
		return pontoDe(n.Token)
	case *ast.TernarioExpression:
		return pontoDe(n.Token)
	case *ast.BoraExpression:
		return pontoDe(n.Token)
	case *ast.TretaLiteral:
		return primeiroTok(n.Tipo)
	}
	return ponto{}
}
