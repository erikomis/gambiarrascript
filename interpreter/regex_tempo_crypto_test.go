package interpreter

import (
	"strings"
	"testing"

	"gambiarrascript/object"
)

func TestBuscaEAchaRegex(t *testing.T) {
	if got := eval(t, `busca_regex("\\d+", "oi 42!")`); got.Type() != object.BOOLEANO_OBJ {
		t.Fatalf("esperava booleano")
	}
	if eval(t, `busca_regex("x", "y")`).Inspect() != "deu_ruim" {
		t.Fatal("busca_regex deve dar deu_ruim quando nao acha")
	}
	if got := eval(t, `acha_regex("\\d+", "abc42de")`).Inspect(); got != "42" {
		t.Fatalf("acha_regex: %q", got)
	}
	if got := eval(t, `acha_regex("\\d+", "abc")`); got.Type() != object.NADA_OBJ {
		t.Fatalf("acha_regex sem match: %s", got.Type())
	}
}

func TestCombinaRegex(t *testing.T) {
	r := eval(t, `combina_regex("(\\w+)@(\\w+)", "Ze@xx Rita@yy")`)
	l, ok := r.(*object.Lista)
	if !ok {
		t.Fatalf("esperava lista")
	}
	if len(l.Elements) != 2 {
		t.Fatalf("esperava 2 matches, veio %d", len(l.Elements))
	}
}

func TestSubstituiRegex(t *testing.T) {
	out := rodar(t, `mostra substitui_regex("(\\w+)@(\\w+)", "$2/$1", "ze@xx")`)
	if out != "xx/ze\n" {
		t.Fatalf("saida %q", out)
	}
	out = rodar(t, `mostra substitui_regex("\\d+", "#", "1 2 3", 2)`)
	if out != "# # 3\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestSeparaRegex(t *testing.T) {
	out := rodar(t, `mostra separa_regex("\\s+", "oi  tropa   do  bem")`)
	if out != "[oi, tropa, do, bem]\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestAgoraEFormataTempo(t *testing.T) {
	out := rodar(t, `mostra tamanho(agora()) > 0`)
	if out != "deu_bom\n" {
		t.Fatalf("saida %q", out)
	}
	out2 := rodar(t, `bota t = parse_tempo("2006-01-02", "2024-06-15")
mostra formata_tempo("02/01/2006", t)`)
	if !strings.Contains(out2, "15/06/2024") {
		t.Fatalf("formata_tempo: %q", out)
	}
}

func TestDuracao(t *testing.T) {
	d1 := eval(t, `duracao({"h": 1, "m": 30})`)
	n, ok := d1.(*object.Numero)
	if !ok || !n.EhInt || n.Int != int64(5400000000000) {
		t.Fatalf("duracao dicionario: %v", d1)
	}
	d2 := eval(t, `duracao(parse_tempo("2006-01-02 15:04:05", "2024-01-01 12:00:00"), parse_tempo("2006-01-02 15:04:05", "2024-01-01 12:05:00"))`)
	n, ok = d2.(*object.Numero)
	if !ok || !n.EhInt || n.Int != int64(300000000000) {
		t.Fatalf("duracao inst: %v", d2)
	}
}

func TestHashesEHex(t *testing.T) {
	sha := eval(t, `sha256("gambiarra")`).Inspect()
	const esperadoSHA = "469f3d53a3fcff775d9487630f13a26f65766d0db121505b347ef26d05ef9986"
	if sha != esperadoSHA {
		t.Fatalf("sha256: %q", sha)
	}
	md5 := eval(t, `md5("gambiarra")`).Inspect()
	if md5 != "f964fedd110c1d0d999b9592473a05c5" {
		t.Fatalf("md5: %q", md5)
	}
	hmac := eval(t, `hmac_sha256("k", "m")`).Inspect()
	wantHmac := "b60090e3052297aeb5a080889ce2fc4bca957e756faeb4df7d31800ca1e771ec"
	if hmac != wantHmac {
		t.Fatalf("hmac_sha256: %q esperado %q", hmac, wantHmac)
	}
}

func TestBase64Hex(t *testing.T) {
	if eval(t, `base64_decodifica(base64_codifica("oi tropa"))`).Inspect() != "oi tropa" {
		t.Fatal("base64 round-trip")
	}
	if eval(t, `hex_decodifica(hex_codifica("AB"))`).Inspect() != "AB" {
		t.Fatal("hex round-trip")
	}
	if eval(t, `hex_codifica("AB")`).Inspect() != "4142" {
		t.Fatal("hex_codifica AB")
	}
}

func TestEsperaMs(t *testing.T) {
	// apenas valida que roda e devolve nada. Sem teste temporal (flakiness).
	v := eval(t, `espera_ms(1)`)
	if v.Type() != object.NADA_OBJ {
		t.Fatalf("espera_ms deve devolver nada, veio %s", v.Type())
	}
}
