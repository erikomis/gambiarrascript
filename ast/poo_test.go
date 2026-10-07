package ast_test

import (
	"strings"
	"testing"
)

// treta/combinado ligam o nome igual gambiarra: cravado antes = erro. O
// metodo abre escopo proprio (receiver e params podem sombrear cravada).
func TestChecaCravadasPOO(t *testing.T) {
	pega := []string{
		"crava Ponto = 1\ntreta Ponto\n    x\nacabou_finalmente",
		"crava Forma = 1\ncombinado Forma\n    area()\nacabou_finalmente",
	}
	for _, src := range pega {
		errs := checa(t, src)
		if len(errs) == 0 || !strings.Contains(errs[0].Msg, "cravada") {
			t.Errorf("%q: queria erro de cravada, veio %v", src, errs)
		}
	}
	libera := []string{
		"crava P = 1\ntreta T\nacabou_finalmente\ngambiarra (P T) m(x)\n    bota P = 2\nacabou_finalmente",
		"treta T\n    x\nacabou_finalmente\nbota T = 2",
	}
	for _, src := range libera {
		if errs := checa(t, src); len(errs) != 0 {
			t.Errorf("%q: nao devia reclamar, veio %v", src, errs)
		}
	}
}
