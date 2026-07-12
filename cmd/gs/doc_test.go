package main

import (
	"strings"
	"testing"
)

func TestGeraDoc(t *testing.T) {
	fonte := `# soma dois numeros
# e devolve o resultado
gambiarra soma(a, b)
    funciona a + b
acabou_finalmente

gambiarra semdoc(x)
    funciona x
acabou_finalmente`
	md, err := geraDoc(fonte)
	if err != nil {
		t.Fatal(err)
	}
	for _, esperado := range []string{
		"soma(a, b)", "soma dois numeros", "e devolve o resultado", "semdoc(x)",
	} {
		if !strings.Contains(md, esperado) {
			t.Fatalf("markdown nao contem %q:\n%s", esperado, md)
		}
	}
}

func TestGeraDocIgnoraComentarioSolto(t *testing.T) {
	// comentario separado por linha em branco NAO e doc da gambiarra
	fonte := `# comentario solto no topo

gambiarra f()
    funciona 1
acabou_finalmente`
	md, err := geraDoc(fonte)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "comentario solto") {
		t.Fatalf("comentario solto nao devia virar doc:\n%s", md)
	}
	if !strings.Contains(md, "f()") {
		t.Fatalf("faltou a gambiarra f:\n%s", md)
	}
}

func TestGeraDocParametrosDefaultEVariadico(t *testing.T) {
	fonte := `gambiarra g(a, b = 10, ...resto)
    funciona a
acabou_finalmente`
	md, err := geraDoc(fonte)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "g(a, b = 10, ...resto)") {
		t.Fatalf("assinatura com default/variadico errada:\n%s", md)
	}
}
