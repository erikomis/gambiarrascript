package formatter

import (
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

// Pattern matching e cardapio: padroes numa linha so, guarda com ` se `,
// cardapio com uma opcao por linha — guardando comentario e idempotente.
func TestFormataPadraoECardapio(t *testing.T) {
	src := `# cores
cardapio   Cor   # cabecalho
vermelho  # quente
  azul
  # antes do fim
acabou_finalmente
escolhe   v   # o valor
caso [ a,b , ...resto ]   se  a>b   # guarda
  mostra a
caso {"tipo":"erro" ,msg}
mostra msg
# antes do caso
caso Ponto{ x:0,y },geo.Ponto{z:Cor.azul}, [_, ..._]
  mostra y
caso _
mostra -1
acabou_finalmente
`
	esperado := `# cores
cardapio Cor  # cabecalho
    vermelho  # quente
    azul
    # antes do fim
acabou_finalmente
escolhe v  # o valor
caso [a, b, ...resto] se a > b  # guarda
    mostra a
caso {"tipo": "erro", msg}
    mostra msg
    # antes do caso
caso Ponto{x: 0, y}, geo.Ponto{z: Cor.azul}, [_, ..._]
    mostra y
caso _
    mostra -1
acabou_finalmente
`
	confereIgual(t, src, esperado)
	confereIgual(t, esperado, esperado)
}

// Formata (so AST, sem fonte) tambem reimprime igual e reparseia.
func TestFormataPadraoSoAST(t *testing.T) {
	src := "cardapio Cor\n    a\nacabou_finalmente\nescolhe x\ncaso [a, {\"k\": -1, b}] se b\n    mostra a\nacabou_finalmente\n"
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	if out := Formata(prog); out != src {
		t.Fatalf("veio:\n%s", out)
	}
}
