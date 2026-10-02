package lsp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// ---- formatacao ----

func formatarDoc(t *testing.T, s *Servidor, uri string) []EdicaoTexto {
	t.Helper()
	res, e := pedir(t, s, "textDocument/formatting", map[string]interface{}{
		"textDocument": map[string]string{"uri": uri},
		"options":      map[string]interface{}{"tabSize": 4, "insertSpaces": true},
	})
	if e != nil {
		t.Fatalf("formatting deu erro: %s", e.Message)
	}
	var eds []EdicaoTexto
	if err := json.Unmarshal(res, &eds); err != nil {
		t.Fatalf("formatting: %v (%s)", err, res)
	}
	return eds
}

func TestFormatarDocumentoInteiro(t *testing.T) {
	src := "bota preço=1\ngambiarra f(a,b=2,...resto)\nmostra \"😀 ${a+preço}\"\nacabou_finalmente\n"
	s := servidorCom(map[string]string{uriMain: src})
	eds := formatarDoc(t, s, uriMain)
	if len(eds) != 1 {
		t.Fatalf("esperava 1 edicao do doc inteiro, veio %+v", eds)
	}
	want := "bota preço = 1\ngambiarra f(a, b = 2, ...resto)\n    mostra \"😀 ${a+preço}\"\nacabou_finalmente\n"
	if eds[0].NewText != want {
		t.Fatalf("texto formatado:\n%q\nwant:\n%q", eds[0].NewText, want)
	}
	// cobre o doc todo: do inicio ate o fim da ultima linha (vazia, apos o \n)
	if eds[0].Range.Start != (Posicao{0, 0}) || eds[0].Range.End != (Posicao{4, 0}) {
		t.Fatalf("range errado: %+v", eds[0].Range)
	}
	// ja formatado: nada a fazer
	s.docs[uriMain] = want
	if eds := formatarDoc(t, s, uriMain); len(eds) != 0 {
		t.Fatalf("doc ja formatado nao deveria ter edicao: %+v", eds)
	}
}

func TestFormatarComErroDeParseNaoMexe(t *testing.T) {
	s := servidorCom(map[string]string{uriMain: "bota = 5\ngambiarra f(\n"})
	if eds := formatarDoc(t, s, uriMain); len(eds) != 0 {
		t.Fatalf("com erro de parse nao pode editar: %+v", eds)
	}
}

func TestFormatarGuardaComentarios(t *testing.T) {
	src := "# importante\nbota x=1\nmostra \"#nao e comentario\" /* esse e */\n"
	s := servidorCom(map[string]string{uriMain: src})
	eds := formatarDoc(t, s, uriMain)
	if len(eds) != 1 {
		t.Fatalf("esperava 1 edicao, veio %+v", eds)
	}
	want := "# importante\nbota x = 1\nmostra \"#nao e comentario\"  /* esse e */\n"
	if eds[0].NewText != want {
		t.Fatalf("texto formatado:\n%q\nwant:\n%q", eds[0].NewText, want)
	}
	if strings.Contains(s.out.(*bytes.Buffer).String(), "window/showMessage") {
		t.Fatal("nao devia avisar nada: formatou de boa")
	}
}

// ---- signature help ----

func assinaturaEm(t *testing.T, s *Servidor, uri string, p Posicao) *AjudaAssinatura {
	t.Helper()
	res, e := pedir(t, s, "textDocument/signatureHelp", posDoc(uri, p.Line, p.Character))
	if e != nil {
		t.Fatalf("signatureHelp deu erro: %s", e.Message)
	}
	if string(res) == "null" {
		return nil
	}
	var a AjudaAssinatura
	json.Unmarshal(res, &a)
	return &a
}

// fimDe e a posicao logo depois da ultima linha do texto (cursor no fim).
func fimDe(texto string) Posicao {
	linhas := strings.Split(texto, "\n")
	return Posicao{Line: len(linhas) - 1, Character: tamUTF16(linhas[len(linhas)-1])}
}

func rotuloParam(sig InfoAssinatura, i int) string {
	u := []uint16{}
	for _, r := range sig.Label {
		if r >= 0x10000 {
			r -= 0x10000
			u = append(u, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			u = append(u, uint16(r))
		}
	}
	p := sig.Parameters[i].Label
	var out []rune
	for _, c := range u[p[0]:p[1]] {
		out = append(out, rune(c))
	}
	return string(out)
}

func TestAssinaturaGambiarraDoUsuario(t *testing.T) {
	base := `# soma tudo que vier
gambiarra saudação(nome, ênfase = "!", ...extras)
    funciona nome + ênfase
acabou_finalmente
`
	casos := []struct {
		digitado string
		ativo    int
	}{
		{"saudação(", 0},
		{"saudação(\"ana\", ", 1},
		{"saudação(\"ana\", \"?\", 1, 2, ", 2}, // varargs: fica no ultimo
		{"bota r = saudação([1, 2], tamanho(\"a, b\"), ", 2},
	}
	for _, c := range casos {
		src := base + c.digitado
		s := servidorCom(map[string]string{uriMain: src})
		a := assinaturaEm(t, s, uriMain, fimDe(src))
		if a == nil || len(a.Signatures) != 1 {
			t.Fatalf("%q: sem assinatura", c.digitado)
		}
		sig := a.Signatures[0]
		if sig.Label != `saudação(nome, ênfase = "!", ...extras)` {
			t.Fatalf("label errado: %q", sig.Label)
		}
		if a.ActiveParameter != c.ativo {
			t.Errorf("%q: parametro ativo %d, want %d", c.digitado, a.ActiveParameter, c.ativo)
		}
		if len(sig.Parameters) != 3 || rotuloParam(sig, 1) != `ênfase = "!"` || rotuloParam(sig, 2) != "...extras" {
			t.Fatalf("params errados: %+v", sig.Parameters)
		}
		if sig.Documentation != "soma tudo que vier" {
			t.Errorf("doc do comentario: %q", sig.Documentation)
		}
	}
}

func TestAssinaturaBuiltin(t *testing.T) {
	src := "bota s = formata(\"%d 😀\", 1, "
	s := servidorCom(map[string]string{uriMain: src})
	a := assinaturaEm(t, s, uriMain, fimDe(src))
	if a == nil {
		t.Fatal("formata deveria ter assinatura")
	}
	sig := a.Signatures[0]
	if sig.Label != "formata(modelo, valores...) -> texto" || rotuloParam(sig, 0) != "modelo" || rotuloParam(sig, 1) != "valores..." {
		t.Fatalf("assinatura do formata errada: %+v", sig)
	}
	if a.ActiveParameter != 1 {
		t.Fatalf("varargs do builtin: ativo %d, want 1", a.ActiveParameter)
	}
	src = "mostra busca(\"http://x\", "
	s.docs[uriMain] = src
	a = assinaturaEm(t, s, uriMain, fimDe(src))
	if a == nil || rotuloParam(a.Signatures[0], 1) != "[opcoes]" || a.ActiveParameter != 1 {
		t.Fatalf("busca: %+v", a)
	}
}

func TestAssinaturaModuloEForaDeChamada(t *testing.T) {
	_, uri, _, s := projetoComUtil(t)
	src := s.docs[uri] + "\nmostra u.dobro("
	s.docs[uri] = src
	a := assinaturaEm(t, s, uri, fimDe(src))
	if a == nil || a.Signatures[0].Label != "dobro(n)" || a.Signatures[0].Documentation != "dobra o numero" {
		t.Fatalf("u.dobro deveria ter a assinatura do util.gs: %+v", a)
	}
	s.docs[uri] = "mostra 1 + (2"
	if a := assinaturaEm(t, s, uri, fimDe(s.docs[uri])); a != nil {
		t.Fatalf("parentese de agrupamento nao e chamada: %+v", a)
	}
	s.docs[uri] = "gambiarra nova("
	if a := assinaturaEm(t, s, uri, fimDe(s.docs[uri])); a != nil {
		t.Fatalf("declaracao de gambiarra nao e chamada: %+v", a)
	}
}

// ---- simbolos do documento ----

func TestSimbolosDoDocumento(t *testing.T) {
	src := `importa "util.gs" como u
crava LIMITE = 10
bota contagem = 0
bota contagem = contagem + 1
se_colar LIMITE > 5
    bota grande = deu_bom
acabou_finalmente
gambiarra fora(x)
    bota local = se_colar x entao 1 se_nao_colar 2
    gambiarra dentro(y)
        se_colar y > 0
            funciona y
        se_nao_colar se_colar y < 0
            funciona 0
        acabou_finalmente
    acabou_finalmente
    funciona dentro(x)
acabou_finalmente
bota dobra = gambiarra(v)
    funciona v * 2
acabou_finalmente`
	s := servidorCom(map[string]string{uriMain: src})
	res, e := pedir(t, s, "textDocument/documentSymbol", map[string]interface{}{
		"textDocument": map[string]string{"uri": uriMain},
	})
	if e != nil {
		t.Fatalf("documentSymbol deu erro: %s", e.Message)
	}
	var syms []SimboloDoc
	json.Unmarshal(res, &syms)
	nomes := []string{}
	porNome := map[string]SimboloDoc{}
	for _, sy := range syms {
		nomes = append(nomes, sy.Name)
		porNome[sy.Name] = sy
	}
	if strings.Join(nomes, ",") != "u,LIMITE,contagem,grande,fora,dobra" {
		t.Fatalf("simbolos de topo: %v", nomes)
	}
	if porNome["LIMITE"].Kind != kindConstante || porNome["contagem"].Kind != kindVariavel ||
		porNome["u"].Kind != kindModulo || porNome["dobra"].Kind != kindFuncao {
		t.Fatalf("kinds errados: %+v", syms)
	}
	fora := porNome["fora"]
	if fora.Kind != kindFuncao || fora.Detail != "(x)" {
		t.Fatalf("fora: %+v", fora)
	}
	// o range vai do `gambiarra` ate o acabou_finalmente da propria (linha 18),
	// atravessando o se_colar de statement, o senao-se e o ternario
	if fora.Range.Start != (Posicao{7, 0}) || fora.Range.End != (Posicao{17, 17}) {
		t.Fatalf("range de fora: %+v", fora.Range)
	}
	if len(fora.Children) != 1 || fora.Children[0].Name != "dentro" || fora.Children[0].Range.End != (Posicao{15, 21}) {
		t.Fatalf("filho dentro: %+v", fora.Children)
	}
	if porNome["dobra"].Range.End != (Posicao{20, 17}) {
		t.Fatalf("range da lambda: %+v", porNome["dobra"].Range)
	}
}
