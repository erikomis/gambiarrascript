package vm

import (
	"strings"
	"testing"
)

const fonteGeradorDep = `gambiarra conta(n)
    pra_cada i de 1 ate n
        bota dobro = i * 2
        rende dobro
    acabou_finalmente
    mostra "fim"
acabou_finalmente
pra_cada x em conta(3)
    mostra x
acabou_finalmente
gambiarra nunca()
    rende 1
acabou_finalmente
bota g = nunca()
`

// A cobertura conta as linhas do corpo do gerador (cada volta), e a linha de
// um gerador criado mas nunca consumido fica com zero.
func TestCoberturaGerador(t *testing.T) {
	c := cobreAmbos(t, map[string]string{"main.gs": fonteGeradorDep}, "main.gs")
	esperaHits(t, c, "main.gs", map[int]int64{
		1: 1, 2: 1, 3: 3, 4: 3, 6: 1, 8: 1, 9: 3, 11: 1, 12: 0, 14: 1,
	})
}

// Breakpoint dentro do corpo do gerador para nos dois engines e enxerga os
// locais dele. Na VM o corpo roda em cima do fluxo de quem pediu; no
// tree-walker ele e um fluxo proprio (a goroutine produtora).
func TestDepuradorBreakpointNoGerador(t *testing.T) {
	for _, eng := range []string{"tree", "vm"} {
		g := novoGravador([]string{"dobro + n"}, 4)
		if eng == "tree" {
			rodaDepTree(t, fonteGeradorDep, g)
		} else {
			rodaDepVM(t, fonteGeradorDep, g)
		}
		rt := g.texto()
		if n := strings.Count(rt, "linha 4 fluxo"); n != 3 {
			t.Errorf("%s: breakpoint na linha 4 parou %d vez(es), esperava 3:\n%s", eng, n, rt)
		}
		if !strings.Contains(rt, "#0 conta:4 | locais: ") || !strings.Contains(rt, "dobro=6") {
			t.Errorf("%s: retrato sem os locais do gerador:\n%s", eng, rt)
		}
		if !strings.Contains(rt, "ve dobro + n => 9") {
			t.Errorf("%s: avalia no quadro do gerador:\n%s", eng, rt)
		}
	}
}
