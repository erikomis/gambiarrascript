package ast

import (
	"testing"

	"gambiarrascript/token"
)

func TestString(t *testing.T) {
	programa := &Program{
		Statements: []Statement{
			&BotaStatement{
				Token: token.Token{Type: token.BOTA, Literal: "bota"},
				Name:  &Identifier{Token: token.Token{Type: token.IDENT, Literal: "nome"}, Value: "nome"},
				Value: &TextoLiteral{Token: token.Token{Type: token.TEXTO, Literal: "Erik"}, Value: "Erik"},
			},
		},
	}

	if programa.String() != `bota nome = "Erik"` {
		t.Fatalf("String() errado: got %q", programa.String())
	}
}
