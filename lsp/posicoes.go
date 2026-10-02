package lsp

import (
	"net/url"
	"path/filepath"
	"strings"
)

// ---- posicoes: lexer (runes) <-> LSP (UTF-16) ----
//
// O lexer conta coluna em RUNES, 1-based. O LSP manda/espera `character` em
// unidades UTF-16, 0-based. Pra acento (ç, ã, é) da no mesmo (BMP = 1 unidade),
// mas emoji e afins fora do BMP ocupam 2 unidades UTF-16 e 1 rune — entao toda
// posicao que sai ou entra pelos recursos de navegacao passa por aqui.

// linhasDoc e o texto do documento quebrado em linhas de runes.
type linhasDoc [][]rune

func novasLinhas(texto string) linhasDoc {
	partes := strings.Split(texto, "\n")
	out := make(linhasDoc, len(partes))
	for i, p := range partes {
		out[i] = []rune(p)
	}
	return out
}

// larguraUTF16 e quantas unidades UTF-16 a rune ocupa.
func larguraUTF16(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// runeParaUTF16 converte a coluna em runes (0-based) da linha (0-based) pra
// unidades UTF-16. Coluna alem do fim da linha conta 1 unidade por rune.
func (ls linhasDoc) runeParaUTF16(linha, col int) int {
	if linha < 0 || linha >= len(ls) {
		return col
	}
	rs := ls[linha]
	n := 0
	for i := 0; i < col; i++ {
		if i >= len(rs) {
			n += col - i
			break
		}
		n += larguraUTF16(rs[i])
	}
	return n
}

// utf16ParaRune faz o caminho inverso: unidades UTF-16 (0-based) -> coluna em
// runes (0-based). Se cair no meio de um par surrogate, arredonda pra rune
// seguinte; alem do fim da linha, satura no tamanho da linha.
func (ls linhasDoc) utf16ParaRune(linha, u int) int {
	if linha < 0 || linha >= len(ls) {
		return u
	}
	n := 0
	for i, r := range ls[linha] {
		if n >= u {
			return i
		}
		n += larguraUTF16(r)
	}
	return len(ls[linha])
}

// posLSP converte (linha, coluna) 1-based do lexer numa Posicao do LSP.
func (ls linhasDoc) posLSP(linha, col int) Posicao {
	return Posicao{Line: linha - 1, Character: ls.runeParaUTF16(linha-1, col-1)}
}

// faixa devolve a Faixa LSP de `tam` runes a partir de (linha, col) 1-based.
func (ls linhasDoc) faixa(linha, col, tam int) Faixa {
	return Faixa{Start: ls.posLSP(linha, col), End: ls.posLSP(linha, col+tam)}
}

// fimDaLinha e a Posicao LSP do fim da linha 1-based.
func (ls linhasDoc) fimDaLinha(linha int) Posicao {
	if linha-1 < 0 || linha-1 >= len(ls) {
		return Posicao{Line: linha - 1}
	}
	return Posicao{Line: linha - 1, Character: ls.runeParaUTF16(linha-1, len(ls[linha-1]))}
}

// cursorFonte anda rune a rune pelo texto, atravessando quebras de linha (que
// viram '\n'). Usado pra reescanear strings e achar onde cada `${` comeca.
type cursorFonte struct {
	ls     linhasDoc
	li, ci int // 0-based
}

func (c *cursorFonte) atual() rune {
	if c.li < 0 || c.li >= len(c.ls) {
		return 0
	}
	if c.ci < len(c.ls[c.li]) {
		return c.ls[c.li][c.ci]
	}
	if c.li+1 < len(c.ls) {
		return '\n'
	}
	return 0
}

func (c *cursorFonte) avanca() {
	if c.li < 0 || c.li >= len(c.ls) {
		return
	}
	if c.ci < len(c.ls[c.li]) {
		c.ci++
		return
	}
	c.li++
	c.ci = 0
}

func (c *cursorFonte) espia() rune {
	copia := *c
	copia.avanca()
	return copia.atual()
}

// pos devolve a posicao atual 1-based (linha, coluna em runes).
func (c *cursorFonte) pos() [2]int { return [2]int{c.li + 1, c.ci + 1} }

// ---- URIs ----

// uriParaCaminho converte `file:///a/b.gs` no caminho do SO. Devolve "" pra
// esquemas que nao sao arquivo (untitled:, etc).
func uriParaCaminho(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	p := u.Path
	// windows: /C:/x.gs -> C:/x.gs
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.Clean(filepath.FromSlash(p))
}

// caminhoParaURI e o inverso: caminho do SO -> `file://` com escape.
func caminhoParaURI(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}
