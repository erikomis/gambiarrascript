package formatter

import (
	"strings"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

func formataFonte(t *testing.T, src string) string {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("erros de parse: %v", errs)
	}
	return Formata(prog)
}

func TestFormataBasico(t *testing.T) {
	src := `gambiarra dobro(n)
funciona n * 2
acabou_finalmente
mostra dobro(5)`
	out := formataFonte(t, src)
	esperado := `gambiarra dobro(n)
    funciona n * 2
acabou_finalmente
mostra dobro(5)
`
	if out != esperado {
		t.Fatalf("got:\n%s\nesperado:\n%s", out, esperado)
	}
}

func TestFormataSeColarIndentado(t *testing.T) {
	src := `bota x = 10
se_colar x > 5
mostra "grande"
se_nao_colar
mostra "pequeno"
acabou_finalmente`
	out := formataFonte(t, src)
	if !strings.Contains(out, "    mostra \"grande\"") {
		t.Fatalf("esperava indentacao no se_colar, got:\n%s", out)
	}
	if !strings.Contains(out, "    mostra \"pequeno\"") {
		t.Fatalf("esperava indentacao no se_nao_colar, got:\n%s", out)
	}
}

func TestFormetaReparser(t *testing.T) {
	// o resultado do formatter tem que voltar a parsear sem erros
	src := `pra_cada i de 1 ate 3
se_colar i == 2
continua
acabou_finalmente
mostra i
acabou_finalmente`
	out := formataFonte(t, src)
	p := parser.New(lexer.New(out))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("saida formatada nao reparseou: %v\n%s", errs, out)
	}
	if len(prog.Statements) == 0 {
		t.Fatalf("saida formatada vazia")
	}
}