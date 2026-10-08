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
				Value: &TextoLiteral{Token: token.Token{Type: token.TEXTO, Literal: "Jurandir"}, Value: "Jurandir"},
			},
		},
	}

	if programa.String() != `bota nome = "Jurandir"` {
		t.Fatalf("String() errado: got %q", programa.String())
	}
}
