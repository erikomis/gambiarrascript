package formatter

import (
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

// POO: treta/combinado com um campo por linha, metodo com receiver e o
// literal `Tipo{...}` (sem espaco antes do `{`), guardando comentario.
func TestFormataPOO(t *testing.T) {
	src := `# o ponto
treta   Ponto   # cabecalho
  x      # abscissa
    y=0
  # sozinho antes do fim
acabou_finalmente
treta Cachorro
Animal   # puxadinho
  geo.Ponto
acabou_finalmente

combinado Forma
area( )
      Nomeavel
  escala(f,  c=1)   # com padrao
acabou_finalmente
gambiarra   ( p   Ponto )  distancia( )   # metodo
funciona raiz(p.x*p.x+p.y*p.y)
acabou_finalmente
bota p=Ponto{x:1,y:2}
bota q = geo.Ponto{1,2}.soma()
bota r = Ponto{
x: 1,   # um
  y: Ponto{}
}
mostra -Ponto{}.x
`
	esperado := `# o ponto
treta Ponto  # cabecalho
    x  # abscissa
    y = 0
    # sozinho antes do fim
acabou_finalmente
treta Cachorro
    Animal  # puxadinho
    geo.Ponto
acabou_finalmente

combinado Forma
    area()
    Nomeavel
    escala(f, c = 1)  # com padrao
acabou_finalmente
gambiarra (p Ponto) distancia()  # metodo
    funciona raiz(p.x * p.x + p.y * p.y)
acabou_finalmente
bota p = Ponto{x: 1, y: 2}
bota q = geo.Ponto{1, 2}.soma()
bota r = Ponto{
    x: 1,  # um
    y: Ponto{}
}
mostra -Ponto{}.x
`
	confereIgual(t, src, esperado)
}

// Formata (so AST, sem fonte) tambem reimprime igual e reparseia.
func TestFormataPOOSoAST(t *testing.T) {
	src := "treta A\n    B\n    x = [1]\nacabou_finalmente\nmostra A{x: [2]}\n"
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	if out := Formata(prog); out != src {
		t.Fatalf("veio:\n%s", out)
	}
}
