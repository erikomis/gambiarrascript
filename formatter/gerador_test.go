package formatter

import "testing"

// rende formatado no lugar, com comentario em volta preservado e idempotente.
func TestFormataRende(t *testing.T) {
	confereIgual(t, `# conta de n em n
gambiarra conta(n)
  pra_cada i de 1 ate n
      rende   i*n   # o valor
  acabou_finalmente
  # acabou
acabou_finalmente
bota g = gambiarra()
rende [1,2]
acabou_finalmente
treta Caixa
  itens
acabou_finalmente
gambiarra (c Caixa) itera()
    /* percorre */
    pra_cada x em c.itens
        rende x
    acabou_finalmente
acabou_finalmente
`, `# conta de n em n
gambiarra conta(n)
    pra_cada i de 1 ate n
        rende i * n  # o valor
    acabou_finalmente
    # acabou
acabou_finalmente
bota g = gambiarra()
    rende [1, 2]
acabou_finalmente
treta Caixa
    itens
acabou_finalmente
gambiarra (c Caixa) itera()
    /* percorre */
    pra_cada x em c.itens
        rende x
    acabou_finalmente
acabou_finalmente
`)
}
