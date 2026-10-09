package main

import (
	"fmt"
	"os"
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

// geraDoc parseia a fonte e gera markdown de referencia: para cada gambiarra (e
// cada constante `crava`) de topo, a assinatura e o bloco de comentarios `#`
// imediatamente acima dela.
func geraDoc(fonte string) (string, error) {
	p := parser.New(lexer.New(fonte))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return "", fmt.Errorf("erro de parse: %s", strings.Join(errs, "; "))
	}
	linhas := strings.Split(fonte, "\n")
	var b strings.Builder
	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *ast.CravaStatement:
			escreveDocCrava(&b, n, linhas)
			continue
		case *ast.TretaDecl, *ast.CombinadoDecl, *ast.CardapioDecl:
			escreveDocTipo(&b, n, linhas)
			continue
		case *ast.MetodoDecl:
			params := make([]string, len(n.Parameters))
			for i, pr := range n.Parameters {
				params[i] = pr.String()
			}
			sig := "(" + n.Receptor.Value + " " + n.Tipo.Value + ") " + n.Nome.Value + "(" + strings.Join(params, ", ") + ")"
			fmt.Fprintf(&b, "### `%s`\n\n", sig)
			if doc := comentariosAcima(linhas, n.Token.Line); doc != "" {
				b.WriteString(doc + "\n\n")
			}
			continue
		}
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

// escreveDocCrava documenta uma constante de topo. Valor curto entra na
// assinatura; valor grande (lista/dict enorme) fica so o nome.
func escreveDocCrava(b *strings.Builder, c *ast.CravaStatement, linhas []string) {
	sig := "crava " + c.Name.Value
	if c.Value != nil {
		if v := c.Value.String(); len(v) <= 40 {
			sig += " = " + v
		}
	}
	fmt.Fprintf(b, "### `%s`\n\n", sig)
	if doc := comentariosAcima(linhas, c.Token.Line); doc != "" {
		b.WriteString(doc)
		b.WriteString("\n\n")
	}
}

// escreveDocTipo documenta uma treta (campos) ou combinado (assinaturas):
// titulo, comentario de cima e a declaracao num bloco de codigo.
func escreveDocTipo(b *strings.Builder, s ast.Statement, linhas []string) {
	var kw, nome string
	var corpo []string
	var linha int
	switch n := s.(type) {
	case *ast.TretaDecl:
		kw, nome, linha = "treta", n.Nome.Value, n.Token.Line
		for _, c := range n.Campos {
			corpo = append(corpo, campoDoc(c))
		}
	case *ast.CardapioDecl:
		kw, nome, linha = "cardapio", n.Nome.Value, n.Token.Line
		corpo = n.NomesMembros()
	case *ast.CombinadoDecl:
		kw, nome, linha = "combinado", n.Nome.Value, n.Token.Line
		for _, m := range n.Metodos {
			if m.Embutido != nil {
				corpo = append(corpo, textoEmbutido(m.Embutido))
			} else {
				corpo = append(corpo, m.String())
			}
		}
	}
	fmt.Fprintf(b, "### `%s %s`\n\n", kw, nome)
	if doc := comentariosAcima(linhas, linha); doc != "" {
		b.WriteString(doc + "\n\n")
	}
	b.WriteString("```\n" + kw + " " + nome + "\n")
	for _, l := range corpo {
		b.WriteString("    " + l + "\n")
	}
	b.WriteString("acabou_finalmente\n```\n\n")
}

func campoDoc(c *ast.CampoTreta) string {
	switch {
	case c.Embutida != nil:
		return textoEmbutido(c.Embutida) + "  # puxadinho"
	case c.Padrao != nil:
		return c.Nome.Value + " = " + c.Padrao.String()
	}
	return c.Nome.Value
}

// textoEmbutido: `Animal` ou `geo.Animal` (sem os parenteses do String()).
func textoEmbutido(e ast.Expression) string {
	if ix, ok := e.(*ast.IndexExpression); ok {
		if t, ok := ix.Index.(*ast.TextoLiteral); ok {
			return ix.Left.String() + "." + t.Value
		}
	}
	return e.String()
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
