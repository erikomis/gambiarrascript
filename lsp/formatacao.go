package lsp

import (
	"strings"

	"gambiarrascript/formatter"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

// ---- textDocument/formatting ----
//
// Reaproveita o formatter do `gs formata`. Tres travas pra nunca destruir
// codigo: (1) com erro de parse, nenhuma edicao; (2) o formatter descarta
// comentarios, entao doc com comentario nao e formatado (avisa uma vez por
// arquivo); (3) o resultado e reparseado e precisa dar o mesmo AST.

func (s *Servidor) formatar(uri string) []EdicaoTexto {
	texto, ok := s.docs[uri]
	if !ok {
		return []EdicaoTexto{}
	}
	novo, motivo := formatarTexto(texto)
	if motivo != "" {
		if s.avisos == nil {
			s.avisos = map[string]bool{}
		}
		if !s.avisos[uri] {
			s.avisos[uri] = true
			s.notificar("window/showMessage", map[string]interface{}{
				"type":    2, // Warning
				"message": "GambiarraScript: " + motivo,
			})
		}
		return []EdicaoTexto{}
	}
	if novo == texto {
		return []EdicaoTexto{}
	}
	ls := novasLinhas(texto)
	fim := ls.fimDaLinha(len(ls))
	return []EdicaoTexto{{
		Range:   Faixa{Start: Posicao{}, End: fim},
		NewText: novo,
	}}
}

// formatarTexto devolve o texto formatado, ou o motivo de nao formatar ("" =
// ok). Erro de parse nao gera aviso: o editor ja sublinha o erro.
func formatarTexto(texto string) (string, string) {
	p := parser.New(lexer.New(texto))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return texto, ""
	}
	if temComentario(texto) {
		return texto, "formatar pulado: o formatter ainda descarta comentarios (# e /* */) e ia apagar os teus"
	}
	novo := formatter.Formata(prog)
	// mesma regra do `gs formata -w`: preserva a quebra de linha final
	if strings.HasSuffix(texto, "\n") && !strings.HasSuffix(novo, "\n") {
		novo += "\n"
	}
	if !strings.HasSuffix(texto, "\n") {
		novo = strings.TrimSuffix(novo, "\n")
	}
	p2 := parser.New(lexer.New(novo))
	prog2 := p2.ParseProgram()
	if len(p2.Errors()) > 0 || prog2.String() != prog.String() {
		return texto, "formatar pulado: o resultado mudaria o significado do codigo (bug do formatter, avisa a gente)"
	}
	return novo, ""
}

// temComentario procura `#` ou `/*` fora de string, imitando o lexer (string
// com aspas tem escape e `${...}` balanceado; crase e crua).
func temComentario(texto string) bool {
	rs := []rune(texto)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '#':
			return true
		case '/':
			if i+1 < len(rs) && rs[i+1] == '*' {
				return true
			}
		case '`':
			for i++; i < len(rs) && rs[i] != '`'; i++ {
			}
		case '"':
			for i++; i < len(rs) && rs[i] != '"'; i++ {
				if rs[i] == '\\' && i+1 < len(rs) && strings.ContainsRune("\"\\nt", rs[i+1]) {
					i++
					continue
				}
				if rs[i] == '$' && i+1 < len(rs) && rs[i+1] == '{' {
					prof := 0
					for ; i < len(rs); i++ {
						if rs[i] == '{' {
							prof++
						} else if rs[i] == '}' {
							prof--
							if prof == 0 {
								break
							}
						}
					}
				}
			}
		}
	}
	return false
}
