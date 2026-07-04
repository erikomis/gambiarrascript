package parser

import (
	"testing"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
)

func TestParseImporta(t *testing.T) {
	prog := parse(t, `importa "util.gs"`)
	if len(prog.Statements) != 1 {
		t.Fatalf("esperava 1 statement, got %d", len(prog.Statements))
	}
	stmt, ok := prog.Statements[0].(*ast.ImportaStatement)
	if !ok {
		t.Fatalf("esperava *ast.ImportaStatement, got %T", prog.Statements[0])
	}
	tl, ok := stmt.Path.(*ast.TextoLiteral)
	if !ok {
		t.Fatalf("Path deveria ser TextoLiteral, got %T", stmt.Path)
	}
	if tl.Value != "util.gs" {
		t.Fatalf("caminho errado: %q", tl.Value)
	}
}

// sanity: importa nao e tratado como expressao/identificador
func TestImportaNaoEIdent(t *testing.T) {
	l := lexer.New("importa")
	tok := l.NextToken()
	if tok.Type != "IMPORTA" {
		t.Fatalf("esperava IMPORTA, got %q", tok.Type)
	}
}
