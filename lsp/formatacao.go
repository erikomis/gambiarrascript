package lsp

import (
	"strings"

	"gambiarrascript/formatter"
)

// ---- textDocument/formatting ----
//
// Reaproveita o formatter do `gs formata` (que guarda comentarios). Duas
// travas pra nunca destruir codigo: (1) com erro de parse, nenhuma edicao;
// (2) o resultado passa no formatter.Confere (reparseia no mesmo AST e com os
// mesmos comentarios); se nao passar, avisa uma vez por arquivo.

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
	novo, errs := formatter.FormataFonte(texto)
	if len(errs) > 0 {
		return texto, ""
	}
	// mesma regra do `gs formata -w`: preserva a quebra de linha final
	if strings.HasSuffix(texto, "\n") && !strings.HasSuffix(novo, "\n") {
		novo += "\n"
	}
	if !strings.HasSuffix(texto, "\n") {
		novo = strings.TrimSuffix(novo, "\n")
	}
	if err := formatter.Confere(texto, novo); err != nil {
		return texto, "formatar pulado: " + err.Error() + " (bug do formatter, avisa a gente)"
	}
	return novo, ""
}
