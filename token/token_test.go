package token

import "testing"

func TestLookupIdent(t *testing.T) {
	casos := map[string]TokenType{
		"bota":              BOTA,
		"crava":             CRAVA,
		"mostra":            MOSTRA,
		"se_colar":          SE_COLAR,
		"se_nao_colar":      SE_NAO_COLAR,
		"acabou_finalmente": ACABOU,
		"pra_cada":          PRA_CADA,
		"deu_bom":           DEU_BOM,
		"nao":               NAO,
		"cardapio":          CARDAPIO,
		"jurandir":          IDENT, // nao-keyword vira identificador
	}
	for entrada, esperado := range casos {
		if got := LookupIdent(entrada); got != esperado {
			t.Fatalf("LookupIdent(%q) = %q, esperado %q", entrada, got, esperado)
		}
	}
}
