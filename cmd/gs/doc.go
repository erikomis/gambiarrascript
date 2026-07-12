package main

import (
	"fmt"
	"os"
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

// geraDoc parseia a fonte e gera markdown de referencia: para cada gambiarra de
// topo, a assinatura e o bloco de comentarios `#` imediatamente acima dela.
func geraDoc(fonte string) (string, error) {
	p := parser.New(lexer.New(fonte))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return "", fmt.Errorf("erro de parse: %s", strings.Join(errs, "; "))
	}
	linhas := strings.Split(fonte, "\n")
	var b strings.Builder
	for _, stmt := range prog.Statements {
		g, ok := stmt.(*ast.GambiarraStatement)
		if !ok || g.Name == nil {
			continue
		}
		params := make([]string, len(g.Parameters))
		for i, pr := range g.Parameters {
			params[i] = pr.String()
		}
		sig := g.Name.Value + "(" + strings.Join(params, ", ") + ")"
		doc := comentariosAcima(linhas, g.Token.Line)

		fmt.Fprintf(&b, "### `%s`\n\n", sig)
		if doc != "" {
			b.WriteString(doc)
			b.WriteString("\n\n")
		}
	}
	return b.String(), nil
}

// comentariosAcima coleta os comentarios `#` contiguos imediatamente acima da
// linha 1-based dada (para no primeiro branco ou nao-comentario), devolvendo o
// texto (sem o `#`) de cima pra baixo.
func comentariosAcima(linhas []string, linha1based int) string {
	start := linha1based - 2 // index 0-based da linha ACIMA da gambiarra
	if start >= len(linhas) {
		start = len(linhas) - 1
	}
	var col []string
	for i := start; i >= 0; i-- {
		t := strings.TrimSpace(linhas[i])
		if !strings.HasPrefix(t, "#") {
			break
		}
		col = append(col, strings.TrimSpace(strings.TrimPrefix(t, "#")))
	}
	for i, j := 0, len(col)-1; i < j; i, j = i+1, j-1 {
		col[i], col[j] = col[j], col[i]
	}
	return strings.Join(col, "\n")
}

// comandoDoc implementa `gs doc <arquivo|dir>...`: imprime no stdout o markdown
// de referencia de cada arquivo .gs (default: diretorio atual).
func comandoDoc(args []string) {
	var alvos []string
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Println("uso: gs doc <arquivo.gs | diretorio>...")
			fmt.Println("  gera markdown de referencia (comentarios # acima de cada gambiarra) no stdout.")
			os.Exit(0)
		}
		alvos = append(alvos, a)
	}
	if len(alvos) == 0 {
		alvos = []string{"."}
	}
	arquivos, err := coletaArquivosGs(alvos)
	if err != nil {
		fmt.Printf("nao consegui listar os arquivos: %v\n", err)
		os.Exit(1)
	}
	for _, arq := range arquivos {
		fonte, err := os.ReadFile(arq)
		if err != nil {
			fmt.Printf("nao consegui abrir %q: %v\n", arq, err)
			os.Exit(1)
		}
		md, err := geraDoc(string(fonte))
		if err != nil {
			fmt.Printf("%s: %v\n", arq, err)
			continue
		}
		if strings.TrimSpace(md) == "" {
			continue // arquivo sem gambiarra documentavel
		}
		fmt.Printf("## %s\n\n%s", arq, md)
	}
}
