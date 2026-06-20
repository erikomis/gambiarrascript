package parser

import (
	"testing"

	"gambiarrascript/ast"
)

func TestParseBota(t *testing.T) {
	prog := parse(t, `bota nome = "Erik"`)
	if len(prog.Statements) != 1 {
		t.Fatalf("esperava 1 statement, got %d", len(prog.Statements))
	}
	stmt, ok := prog.Statements[0].(*ast.BotaStatement)
	if !ok {
		t.Fatalf("esperava *ast.BotaStatement, got %T", prog.Statements[0])
	}
	if stmt.Name.Value != "nome" {
		t.Fatalf("nome errado: %q", stmt.Name.Value)
	}
}

func TestParseSeColarChain(t *testing.T) {
	input := `se_colar x == 1
    mostra "um"
se_nao_colar se_colar x == 2
    mostra "dois"
se_nao_colar
    mostra "outro"
acabou_finalmente`
	prog := parse(t, input)
	stmt, ok := prog.Statements[0].(*ast.SeColarStatement)
	if !ok {
		t.Fatalf("esperava *ast.SeColarStatement, got %T", prog.Statements[0])
	}
	if len(stmt.Conditions) != 2 {
		t.Fatalf("esperava 2 condicoes, got %d", len(stmt.Conditions))
	}
	if stmt.Alternative == nil {
		t.Fatalf("esperava bloco else (Alternative)")
	}
}

func TestParseGambiarraEPraCada(t *testing.T) {
	input := `gambiarra dobro(n)
    funciona n * 2
acabou_finalmente

pra_cada i de 1 ate 3
    mostra dobro(i)
acabou_finalmente`
	prog := parse(t, input)
	if len(prog.Statements) != 2 {
		t.Fatalf("esperava 2 statements, got %d", len(prog.Statements))
	}
	if _, ok := prog.Statements[0].(*ast.GambiarraStatement); !ok {
		t.Fatalf("statement 0 deveria ser GambiarraStatement, got %T", prog.Statements[0])
	}
	laco, ok := prog.Statements[1].(*ast.PraCadaNumStatement)
	if !ok {
		t.Fatalf("statement 1 deveria ser PraCadaNumStatement, got %T", prog.Statements[1])
	}
	if laco.Var.Value != "i" {
		t.Fatalf("variavel do laco errada: %q", laco.Var.Value)
	}
}

func TestParseArruma(t *testing.T) {
	input := `arruma
    bota x = 1
quebrou erro
    mostra erro
acabou_finalmente`
	prog := parse(t, input)
	stmt, ok := prog.Statements[0].(*ast.ArrumaStatement)
	if !ok {
		t.Fatalf("esperava *ast.ArrumaStatement, got %T", prog.Statements[0])
	}
	if stmt.ErrName.Value != "erro" {
		t.Fatalf("nome do erro errado: %q", stmt.ErrName.Value)
	}
}
