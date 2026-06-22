package interpreter

import (
	"testing"

	"gambiarrascript/object"
)

func TestDeJsonTipos(t *testing.T) {
	casos := []struct{ input, esp string }{
		{`mostra de_json("42")`, "42"},
		{`mostra de_json("\"oi\"")`, "oi"},
		{`mostra de_json("true")`, "deu_bom"},
		{`mostra de_json("false")`, "deu_ruim"},
		{`mostra de_json("null")`, "nada"},
		{`mostra de_json("[1, 2, 3]")`, "[1, 2, 3]"},
		{`mostra de_json("{\"nome\": \"Erik\"}")["nome"]`, "Erik"},
	}
	for _, c := range casos {
		out := rodar(t, c.input)
		if out != c.esp+"\n" {
			t.Errorf("%q => got %q, esperado %q", c.input, out, c.esp+"\n")
		}
	}
}

func TestPraJsonTipos(t *testing.T) {
	casos := []struct{ input, esp string }{
		{`mostra pra_json(42)`, "42"},
		{`mostra pra_json("oi")`, `"oi"`},
		{`mostra pra_json(deu_bom)`, "true"},
		{`mostra pra_json(nada)`, "null"},
		{`mostra pra_json([1, 2])`, "[1,2]"},
		{`mostra pra_json({"nome": "Erik"})`, `{"nome":"Erik"}`},
	}
	for _, c := range casos {
		out := rodar(t, c.input)
		if out != c.esp+"\n" {
			t.Errorf("%q => got %q, esperado %q", c.input, out, c.esp+"\n")
		}
	}
}

func TestJsonRoundTrip(t *testing.T) {
	out := rodar(t, `bota original = {"nome": "Erik", "tags": ["dev", "br"], "ativo": deu_bom}
bota voltou = de_json(pra_json(original))
mostra original == voltou`)
	if out != "deu_bom\n" {
		t.Fatalf("round-trip falhou: got %q", out)
	}
}

func TestDeJsonInvalido(t *testing.T) {
	if got := eval(t, `de_json("{quebrado")`); got.Type() != object.ERRO_OBJ {
		t.Fatalf("json invalido deveria dar erro, got %s", got.Type())
	}
	if got := eval(t, `de_json(42)`); got.Type() != object.ERRO_OBJ {
		t.Fatalf("argumento nao-texto deveria dar erro, got %s", got.Type())
	}
}

func TestPraJsonNaoSerializavel(t *testing.T) {
	got := eval(t, `gambiarra f()
    funciona 1
acabou_finalmente
pra_json(f)`)
	if got.Type() != object.ERRO_OBJ {
		t.Fatalf("gambiarra nao serializa, deveria dar erro, got %s", got.Type())
	}
	got2 := eval(t, `gambiarra f()
    funciona 1
acabou_finalmente
pra_json([1, f])`)
	if got2.Type() != object.ERRO_OBJ {
		t.Fatalf("gambiarra dentro de lista deveria dar erro, got %s", got2.Type())
	}
}

func TestPraJsonChaveNumerica(t *testing.T) {
	out := rodar(t, `mostra pra_json({1: "a"})`)
	if out != `{"1":"a"}`+"\n" {
		t.Fatalf("chave numerica deveria virar string: got %q", out)
	}
}

func TestPraJsonNaoSerializavelEmDict(t *testing.T) {
	got := eval(t, `gambiarra f()
    funciona 1
acabou_finalmente
pra_json({"fn": f})`)
	if got.Type() != object.ERRO_OBJ {
		t.Fatalf("gambiarra dentro de dict deveria dar erro, got %s", got.Type())
	}
}
