package cobertura

import (
	"bufio"
	"fmt"
	"html"
	"io"
	"path/filepath"
	"strings"
)

// Nome devolve o caminho relativo a `base` (o diretorio de onde o gs foi
// chamado) quando da; senao o absoluto.
func Nome(caminho, base string) string {
	if base == "" {
		return caminho
	}
	if rel, err := filepath.Rel(base, caminho); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return caminho
}

// EscreveResumo imprime a tabela por arquivo + total:
//
//	cobertura (VM):
//	  calc.gs        8/10 linhas   80.0%
//	  total          8/10 linhas   80.0%
func (r *Relatorio) EscreveResumo(w io.Writer, engine, base string) {
	fmt.Fprintf(w, "cobertura (%s):\n", engine)
	if len(r.Arquivos) == 0 {
		fmt.Fprintln(w, "  nenhum arquivo fora os *_test.gs rodou")
		return
	}
	larg := len("total")
	for _, a := range r.Arquivos {
		if n := len(Nome(a.Caminho, base)); n > larg {
			larg = n
		}
	}
	linha := func(nome string, cob, exe int) {
		fmt.Fprintf(w, "  %-*s  %5d/%-5d linhas  %5.1f%%\n", larg, nome, cob, exe, Pct(cob, exe))
	}
	for _, a := range r.Arquivos {
		linha(Nome(a.Caminho, base), a.Cobertas(), len(a.Executaveis))
	}
	cob, exe := r.Totais()
	linha("total", cob, exe)
}

// EscrevePerfil grava o perfil legivel por maquina: um cabecalho e uma linha
// `arquivo:linha contagem` por linha executavel (contagem 0 = nao rodou), em
// ordem de arquivo e linha.
func (r *Relatorio) EscrevePerfil(w io.Writer, base string) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintln(bw, "modo: contagem")
	for _, a := range r.Arquivos {
		nome := Nome(a.Caminho, base)
		for _, l := range a.Executaveis {
			fmt.Fprintf(bw, "%s:%d %d\n", nome, l, a.Hits[l])
		}
	}
	return bw.Flush()
}

// EscreveHTML grava um relatorio HTML autocontido: indice com a % de cada
// arquivo e a fonte com as linhas coloridas (rodou / nao rodou / nao conta).
func (r *Relatorio) EscreveHTML(w io.Writer, engine, base string) error {
	bw := bufio.NewWriter(w)
	cob, exe := r.Totais()
	fmt.Fprintf(bw, `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cobertura GambiarraScript</title>
<style>
:root { --fundo: #ffffff; --texto: #1f2328; --borda: #d0d7de; --sutil: #656d76;
  --rodou: #dafbe1; --rodou-n: #1a7f37; --faltou: #ffebe9; --faltou-n: #cf222e; }
@media (prefers-color-scheme: dark) {
  :root { --fundo: #0d1117; --texto: #e6edf3; --borda: #30363d; --sutil: #8d96a0;
    --rodou: #12261e; --rodou-n: #3fb950; --faltou: #2d1215; --faltou-n: #f85149; }
}
body { background: var(--fundo); color: var(--texto); margin: 0; padding: 16px;
  font: 14px/1.45 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
h1 { font-size: 20px; margin: 0 0 12px; }
h2 { font-size: 16px; margin: 24px 0 8px; }
table.indice { border-collapse: collapse; margin-bottom: 8px; }
table.indice td, table.indice th { padding: 4px 12px; border-bottom: 1px solid var(--borda); text-align: left; }
table.indice td.num { text-align: right; font-variant-numeric: tabular-nums; }
a { color: inherit; }
.fonte { border: 1px solid var(--borda); border-radius: 6px; overflow-x: auto; }
.fonte pre { margin: 0; font: 12px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace; }
.l { display: block; white-space: pre; padding-right: 12px; }
.l .n { display: inline-block; width: 4em; text-align: right; color: var(--sutil); padding-right: 8px; user-select: none; }
.l .c { display: inline-block; width: 4em; text-align: right; padding-right: 12px; user-select: none; }
.l.rodou { background: var(--rodou); } .l.rodou .c { color: var(--rodou-n); }
.l.faltou { background: var(--faltou); } .l.faltou .c { color: var(--faltou-n); }
.sutil { color: var(--sutil); }
</style>
</head>
<body>
<h1>Cobertura de linhas</h1>
<p class="sutil">engine: %s &middot; total: %d/%d linhas (%.1f%%)</p>
<table class="indice"><tr><th>arquivo</th><th>linhas</th><th>%%</th></tr>
`, html.EscapeString(engine), cob, exe, Pct(cob, exe))
	for i, a := range r.Arquivos {
		fmt.Fprintf(bw, "<tr><td><a href=\"#a%d\">%s</a></td><td class=\"num\">%d/%d</td><td class=\"num\">%.1f%%</td></tr>\n",
			i, html.EscapeString(Nome(a.Caminho, base)), a.Cobertas(), len(a.Executaveis), Pct(a.Cobertas(), len(a.Executaveis)))
	}
	fmt.Fprintln(bw, "</table>")
	for i, a := range r.Arquivos {
		exec := map[int]bool{}
		for _, l := range a.Executaveis {
			exec[l] = true
		}
		fmt.Fprintf(bw, "<h2 id=\"a%d\">%s <span class=\"sutil\">%.1f%%</span></h2>\n<div class=\"fonte\"><pre>",
			i, html.EscapeString(Nome(a.Caminho, base)), Pct(a.Cobertas(), len(a.Executaveis)))
		linhas := strings.Split(strings.TrimRight(string(a.Fonte), "\n"), "\n")
		for k, txt := range linhas {
			n := k + 1
			classe, cont := "", ""
			if exec[n] {
				if h := a.Hits[n]; h > 0 {
					classe, cont = " rodou", fmt.Sprintf("%d×", h)
				} else {
					classe, cont = " faltou", "0"
				}
			}
			fmt.Fprintf(bw, "<span class=\"l%s\"><span class=\"n\">%d</span><span class=\"c\">%s</span>%s</span>",
				classe, n, cont, html.EscapeString(strings.TrimRight(txt, "\r")))
		}
		fmt.Fprintln(bw, "</pre></div>")
	}
	fmt.Fprintln(bw, "</body>\n</html>")
	return bw.Flush()
}
