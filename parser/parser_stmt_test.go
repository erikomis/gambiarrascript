package parser

import (
	"strings"
	"testing"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
)

func TestParseBota(t *testing.T) {
	prog := parse(t, `bota nome = "Jurandir"`)
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
	if len(stmt.Quebrous) != 1 || stmt.Quebrous[0].Nome.Value != "erro" || stmt.Quebrous[0].Filtro != nil {
		t.Fatalf("quebrou errado: %+v", stmt.Quebrous)
	}
}

func TestParseMultiCatch(t *testing.T) {
	input := `arruma
    bota x = 1
quebrou erro se erro_tipo(erro) == "rede"
    mostra 1
quebrou e2 se erro_tipo(e2) == "jwt" ou erro_msg(e2) == "x"
    mostra 2
quebrou resto
    mostra 3
finalmente
    mostra 4
acabou_finalmente`
	prog := parse(t, input)
	stmt := prog.Statements[0].(*ast.ArrumaStatement)
	if len(stmt.Quebrous) != 3 {
		t.Fatalf("esperava 3 quebrou, veio %d", len(stmt.Quebrous))
	}
	nomes := []string{"erro", "e2", "resto"}
	filtros := []string{`(erro_tipo(erro) == "rede")`, `((erro_tipo(e2) == "jwt") ou (erro_msg(e2) == "x"))`, ""}
	for i, q := range stmt.Quebrous {
		if q.Nome.Value != nomes[i] {
			t.Fatalf("clausula %d: nome %q", i, q.Nome.Value)
		}
		f := ""
		if q.Filtro != nil {
			f = q.Filtro.String()
		}
		if f != filtros[i] {
			t.Fatalf("clausula %d: filtro %q, esperava %q", i, f, filtros[i])
		}
		if len(q.Corpo.Statements) != 1 {
			t.Fatalf("clausula %d: corpo com %d statements", i, len(q.Corpo.Statements))
		}
	}
	if stmt.Finally == nil {
		t.Fatal("cade o finalmente")
	}
}

func TestParseMultiCatchSoFiltrados(t *testing.T) {
	// todos com filtro (sem pega-resto) e valido: o que sobrar sobe
	prog := parse(t, `arruma
    bota x = 1
quebrou a se a
    mostra 1
quebrou b se nao b
    mostra 2
acabou_finalmente`)
	if n := len(prog.Statements[0].(*ast.ArrumaStatement).Quebrous); n != 2 {
		t.Fatalf("esperava 2 quebrou, veio %d", n)
	}
}

func TestParseSeContextual(t *testing.T) {
	// `se` so e filtro na mesma linha do nome; fora dali e nome comum
	prog := parse(t, `gambiarra se(x)
    funciona x
acabou_finalmente
bota se2 = 1
arruma
    bota se = 1
quebrou erro
    se(erro)
acabou_finalmente
mostra se`)
	arr := prog.Statements[2].(*ast.ArrumaStatement)
	q := arr.Quebrous[0]
	if q.Filtro != nil || len(q.Corpo.Statements) != 1 {
		t.Fatalf("`se` na linha de baixo e corpo, nao filtro: %+v", q)
	}
}

func TestParseQuebrouSemFiltroNoMeio(t *testing.T) {
	p := New(lexer.New(`arruma
    bota x = 1
quebrou erro
    mostra 1
quebrou outro se erro_tipo(outro) == "rede"
    mostra 2
acabou_finalmente`))
	p.ParseProgram()
	errs := p.ErrosDetalhados()
	if len(errs) != 1 || !strings.Contains(errs[0].Msg, "quebrou sem filtro tem que ser o ultimo") || errs[0].Linha != 3 {
		t.Fatalf("esperava erro do quebrou sem filtro na linha 3: %+v", errs)
	}
}

func TestErrosDetalhados(t *testing.T) {
	// 'bota' sem identificador: erro na posicao do '='
	p := New(lexer.New("bota = 5"))
	p.ParseProgram()
	detalhes := p.ErrosDetalhados()
	if len(detalhes) == 0 {
		t.Fatal("esperava ao menos um ErroParse")
	}
	e := detalhes[0]
	if e.Linha != 1 {
		t.Fatalf("linha do erro: got %d, esperado 1", e.Linha)
	}
	if e.Coluna != 6 {
		t.Fatalf("coluna do erro: got %d, esperado 6", e.Coluna)
	}
	if e.Msg == "" {
		t.Fatal("mensagem do erro vazia")
	}
	// Errors() continua funcionando e prefixa a linha
	strs := p.Errors()
	if len(strs) == 0 || !strings.HasPrefix(strs[0], "linha 1:") {
		t.Fatalf("Errors() deveria prefixar 'linha 1:', got %v", strs)
	}
}

func TestParseDicionario(t *testing.T) {
	prog := parse(t, `bota d = {"nome": "Jurandir", "idade": 25}`)
	stmt := prog.Statements[0].(*ast.BotaStatement)
	dic, ok := stmt.Value.(*ast.DicionarioLiteral)
	if !ok {
		t.Fatalf("esperava DicionarioLiteral, got %T", stmt.Value)
	}
	if len(dic.Pares) != 2 {
		t.Fatalf("esperava 2 pares, got %d", len(dic.Pares))
	}
}

func TestParseDicionarioVazio(t *testing.T) {
	prog := parse(t, `bota d = {}`)
	stmt := prog.Statements[0].(*ast.BotaStatement)
	dic, ok := stmt.Value.(*ast.DicionarioLiteral)
	if !ok || len(dic.Pares) != 0 {
		t.Fatalf("esperava dicionario vazio, got %T", stmt.Value)
	}
}
