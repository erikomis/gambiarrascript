package interpreter

import (
	"encoding/json"
	"testing"

	"gambiarrascript/object"
)

// TestParseJsonBateComEncodingJson: pro parser proprio nao divergir da
// biblioteca padrao, todo caso valido tem que ser aceito pelos dois e todo
// invalido tem que ser recusado pelos dois.
func TestParseJsonBateComEncodingJson(t *testing.T) {
	casos := []string{
		`null`, `true`, `false`, `0`, `-0`, `42`, `-42`, `3.14`, `-3.14`,
		`1e3`, `1E3`, `1e+3`, `1e-3`, `-1.5e-7`, `9007199254740993`,
		`""`, `"salve"`, `"com \"aspas\""`, `"barra\\invertida"`,
		`"\n\t\r\b\f\/"`, `"é acentuado"`, `"😀 emoji"`,
		`"acentuado direto: ção"`, `[]`, `{}`, `[1,2,3]`, `[[1],[2,[3]]]`,
		`{"a":1}`, `{"a":{"b":{"c":[1,2,{"d":null}]}}}`,
		`  {  "a"  :  1  ,  "b"  :  [ ]  }  `,
		`{"repetida":1,"repetida":2}`,
		`[{"n":1},{"n":2}]`, "\t\n {\"x\": true} \r\n",
	}
	for _, c := range casos {
		var alvo interface{}
		errPadrao := json.Unmarshal([]byte(c), &alvo)
		_, errNosso := parseJson(c)
		if (errPadrao == nil) != (errNosso == nil) {
			t.Errorf("%q: encoding/json err=%v, nosso err=%v", c, errPadrao, errNosso)
		}
	}
}

func TestParseJsonRecusaLixo(t *testing.T) {
	ruins := []string{
		``, `{`, `}`, `[`, `]`, `{"a"}`, `{"a":}`, `{a:1}`, `{'a':1}`,
		`[1,]`, `{"a":1,}`, `nulo`, `tru`, `01`, `--1`, `"sem fim`,
		`{} lixo`, `[1] [2]`, `"\x41"`, `"\u12"`, "\"quebra\nlinha\"",
	}
	for _, c := range ruins {
		var alvo interface{}
		errPadrao := json.Unmarshal([]byte(c), &alvo)
		_, errNosso := parseJson(c)
		if errPadrao == nil {
			t.Fatalf("caso %q devia ser invalido pro encoding/json tambem", c)
		}
		if errNosso == nil {
			t.Errorf("%q: nosso parser aceitou json invalido", c)
		}
	}
}

// TestParseJsonValores confere que o conteudo decodificado bate, nao so a
// aceitacao.
func TestParseJsonValores(t *testing.T) {
	casos := []struct{ entrada, querido string }{
		{`{"b":1,"a":2}`, `{"b": 1, "a": 2}`},
		{`"é"`, `é`},
		{`"😀"`, `😀`},
		{`[1,"dois",true,null]`, `[1, dois, deu_bom, nada]`},
		{`9007199254740993`, `9007199254740993`},
		{`1.5`, `1.5`},
	}
	for _, c := range casos {
		v, err := parseJson(c.entrada)
		if err != nil {
			t.Fatalf("%q: %v", c.entrada, err)
		}
		if got := v.Inspect(); got != c.querido {
			t.Errorf("%q virou %q, queria %q", c.entrada, got, c.querido)
		}
	}
}

// TestJsonRoundTripPreservaOrdem: de_json -> pra_json devolve o documento com
// as chaves na MESMA ordem em que apareceram.
func TestJsonRoundTripPreservaOrdem(t *testing.T) {
	entrada := `{"z":1,"a":{"n":2,"m":3},"b":[{"y":4,"x":5}]}`
	for i := 0; i < 30; i++ {
		v, err := parseJson(entrada)
		if err != nil {
			t.Fatal(err)
		}
		saida := builtinPraJson([]object.Object{v})
		txt, ok := saida.(*object.Texto)
		if !ok {
			t.Fatalf("pra_json devolveu %s", saida.Type())
		}
		if txt.Value != entrada {
			t.Fatalf("rodada %d: round-trip deu %q, queria %q", i, txt.Value, entrada)
		}
	}
}
