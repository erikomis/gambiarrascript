package interpreter

import (
	"strings"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

func TestBoraDevolucoesFuturosEAguardaSincrono(t *testing.T) {
	out := rodar(t, `
gambiarra dobra(n)
    funciona n * 2
acabou_finalmente

bota f1 = bora dobra(21)
bota f2 = bora dobra(50)
mostra espera(f1)
mostra espera(f2)
`)
	if !strings.Contains(out, "42\n100\n") {
		t.Fatalf("saida errada: %q", out)
	}
}

func TestBoraErroPropagaNoFuturo(t *testing.T) {
	out := rodar(t, `
gambiarra com_erro(n)
    se_colar n < 0
        funciona 10 / 0
    acabou_finalmente
    funciona n
acabou_finalmente

bota f = bora com_erro(-1)
arruma
    mostra espera(f)
quebrou erro
    mostra "capturou"
acabou_finalmente
`)
	if !strings.Contains(out, "capturou\n") {
		t.Fatalf("esperava captura do erro, saida=%q", out)
	}
}

func TestEsperaListaDeFuturos(t *testing.T) {
	out := rodar(t, `
gambiarra dobra(n)
    funciona n * 2
acabou_finalmente

bota fs = [bora dobra(1), bora dobra(2), bora dobra(3)]
mostra espera(fs)
`)
	if !strings.Contains(out, "[2, 4, 6]") {
		t.Fatalf("saida errada: %q", out)
	}
}

func TestCanoProdutorConsumidor(t *testing.T) {
	out := rodar(t, `
gambiarra produtor(c)
    pra_cada i de 1 ate 3
        envia(c, i)
    acabou_finalmente
    fecha(c)
acabou_finalmente

bota c = cano(3)
bora produtor(c)

bota soma = 0
enquanto deu_bom
    bota v = recebe(c)
    se_colar v == nada
        vaza
    acabou_finalmente
    bota soma = soma + v
acabou_finalmente
mostra soma
`)
	if !strings.Contains(out, "6\n") {
		t.Fatalf("soma esperada 6, saida=%q", out)
	}
}

func TestEsperaAssertCompatNaoQuebra(t *testing.T) {
	// assert antigo: 2 args. Continua funcionando sem escrever "FALHA".
	var buf strings.Builder
	i := New(&buf)
	p := parser.New(lexer.New("espera(1 + 1, 2)"))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse: %v", errs)
	}
	i.Eval(prog, object.NewEnvironment())
	if strings.Contains(buf.String(), "FALHA") {
		t.Fatalf("assert deveria passar, mas falhou: %q", buf.String())
	}
	total, ok := i.TotaisTeste()
	if total != 1 || ok != 1 {
		t.Fatalf("contadores (total=%d ok=%d), esperava 1/1", total, ok)
	}
}
