package vm

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Templates (renderiza / renderiza_arquivo / responde_html) nos dois motores.

func TestRenderizaParidade(t *testing.T) {
	casos := []struct{ src, saida, erro string }{
		{`mostra renderiza("Ola, {{ nome }}!", {"nome": "Ze"})`, "Ola, Ze!\n", ""},
		{`mostra renderiza("{{ u.nome }} {{ u.tags[0] }} {{ u.tags[-1] }}", {"u": {"nome": "Bia", "tags": ["a", "b"]}})`, "Bia a b\n", ""},
		{`mostra renderiza("[{{ falta }}][{{ x.y.z }}]", {"x": 1})`, "[][]\n", ""},
		// XSS: escapado por padrao, cru so quando pede
		{`mostra renderiza("{{ x }}", {"x": "<script>alert('oi')</script>&\""})`,
			"&lt;script&gt;alert(&#39;oi&#39;)&lt;/script&gt;&amp;&#34;\n", ""},
		{`mostra renderiza("{{{ x }}}|{{ x | cru }}", {"x": "<b>"})`, "<b>|<b>\n", ""},
		{`mostra renderiza("<a title=\"{{ t }}\">", {"t": "\" onclick=\"roubo()"})`,
			"<a title=\"&#34; onclick=&#34;roubo()\">\n", ""},
		{`mostra renderiza("{{ x }}", {"x": "<b>"}, {"escapa": deu_ruim})`, "<b>\n", ""},
		// filtros
		{`mostra renderiza("{{ n | maiusculo }} {{ n | minusculo }} {{ n | tamanho }}", {"n": "Zé"})`, "ZÉ zé 2\n", ""},
		{`mostra renderiza("{{ p | formata \"%.2f\" }} {{ i | formata \"%03d\" }}", {"p": 3.14159, "i": 7})`, "3.14 007\n", ""},
		{`mostra renderiza("{{ falta | padrao \"anonimo\" }}", {})`, "anonimo\n", ""},
		{`mostra renderiza("{{ l | junta \", \" }}", {"l": [1, 2, 3]})`, "1, 2, 3\n", ""},
		{`mostra renderiza("{{ d | json | cru }}", {"d": {"a": [1, "</script>"]}})`, "{\"a\":[1,\"\\u003c/script\\u003e\"]}\n", ""},
		{`mostra renderiza("{{ d | json }}", {"d": {"a": 1}})`, "{&#34;a&#34;:1}\n", ""},
		// laco
		{`mostra renderiza("{{ pra_cada i, x em l }}{{ i }}={{ x }};{{ acabou }}", {"l": ["a", "b"]})`, "0=a;1=b;\n", ""},
		{`mostra renderiza("{{ pra_cada k, v em d }}{{ k }}:{{ v }} {{ acabou }}", {"d": {"x": 1, "y": 2}})`, "x:1 y:2 \n", ""},
		{`mostra renderiza("{{ pra_cada x em l }}{{ x }}{{ se_nao_colar }}vazio{{ acabou }}", {"l": []})`, "vazio\n", ""},
		{"mostra renderiza(`<ul>\n  {{ pra_cada x em l }}\n  <li>{{ x }}</li>\n  {{ acabou }}\n</ul>`, {\"l\": [1, 2]})",
			"<ul>\n  <li>1</li>\n  <li>2</li>\n</ul>\n", ""},
		// condicao
		{`mostra renderiza("{{ se_colar n > 3 e nao f }}sim{{ se_nao_colar }}nao{{ acabou }}", {"n": 5, "f": deu_ruim})`, "sim\n", ""},
		{`mostra renderiza("{{ se_colar s == \"a\" }}A{{ se_nao_colar se_colar s == \"b\" }}B{{ se_nao_colar }}?{{ acabou }}", {"s": "b"})`, "B\n", ""},
		{`mostra renderiza("{{ se_colar l | tamanho > 0 }}tem{{ se_nao_colar }}nada{{ acabou }}", {"l": []})`, "nada\n", ""},
		{`mostra renderiza("a{{# comentario }}b")`, "ab\n", ""},
		// treta como dados
		{"treta P\n    x\n    y\nacabou_finalmente\nmostra renderiza(\"{{ x }},{{ y }}\", P{1, 2})", "1,2\n", ""},
		// erros com nome e linha
		{"mostra renderiza(\"a\\n{{ pra_cada x em l }}\", {})", "", "modelo (texto) linha 2: `pra_cada` sem `acabou`"},
		{`mostra renderiza("{{ x | sei_la }}", {})`, "", "filtro `sei_la` nao existe"},
		{`mostra renderiza("{{ u.idade }}", {"u": {}}, {"estrito": deu_bom})`, "", "`u.idade` nao existe (modo estrito)"},
		{`mostra renderiza(1)`, "", "o modelo tem que ser texto"},
		{`mostra renderiza("x", [1])`, "", "os dados tem que ser dicionario"},
		{`mostra renderiza("x", {}, {"sei": 1})`, "", "opcao \"sei\" nao existe"},
		{"arruma\n    renderiza(\"{{ se_colar x }}\")\nquebrou erro\n    mostra erro_tipo(erro)\nacabou_finalmente", "builtin\n", ""},
		// responde_html
		{`bota r = responde_html("<h1>oi</h1>")
mostra r.status
mostra r.corpo
mostra r.cabecalhos["Content-Type"]`, "200\n<h1>oi</h1>\ntext/html; charset=utf-8\n", ""},
		{`mostra responde_html("x", 404).status`, "404\n", ""},
		{`responde_html("x", 99)`, "", "status tem que ser numero inteiro de 100 a 599"},
		{`responde_html(1)`, "", "o html tem que ser texto"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, c.erro)
	}
}

func TestRenderizaArquivoParidade(t *testing.T) {
	dir := t.TempDir()
	escreve := func(nome, conteudo string) {
		p := filepath.Join(dir, filepath.FromSlash(nome))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escreve("segredo.txt", "SEGREDO")
	escreve("modelos/base.html", "<title>{{ bloco \"titulo\" }}Site{{ acabou }}</title>\n<main>\n{{ bloco \"conteudo\" }}\n{{ acabou }}\n</main>\n")
	escreve("modelos/parciais/item.html", "<li>{{ item.nome }}</li>\n")
	escreve("modelos/pagina.html", `{{ usa "base.html" }}
{{ bloco "titulo" }}{{ titulo }}{{ acabou }}
{{ bloco "conteudo" }}
<ul>
  {{ pra_cada item em itens }}
  {{ inclui "parciais/item.html" }}
  {{ acabou }}
</ul>
{{ acabou }}
`)
	escreve("modelos/fuga.html", `{{ inclui "../segredo.txt" }}`)
	escreve("modelos/quebrado.html", "linha 1\nlinha 2\n{{ se_colar x }}\n")
	pag := strconv.Quote(filepath.Join(dir, "modelos", "pagina.html"))
	fuga := strconv.Quote(filepath.Join(dir, "modelos", "fuga.html"))
	quebrado := strconv.Quote(filepath.Join(dir, "modelos", "quebrado.html"))
	sumido := strconv.Quote(filepath.Join(dir, "modelos", "sumido.html"))

	esperaNosDois(t, `mostra renderiza_arquivo(`+pag+`, {"titulo": "Loja", "itens": [{"nome": "<cafe>"}, {"nome": "pao"}]})`,
		"<title>Loja</title>\n<main>\n<ul>\n<li>&lt;cafe&gt;</li>\n<li>pao</li>\n</ul>\n</main>\n\n", "")
	esperaNosDois(t, `renderiza_arquivo(`+fuga+`, {})`, "", "sai da pasta dos modelos")
	esperaNosDois(t, `mostra renderiza_arquivo(`+fuga+`, {}, {"pasta": `+strconv.Quote(dir)+`})`, "SEGREDO\n", "")
	esperaNosDois(t, `renderiza_arquivo(`+quebrado+`, {})`, "", "modelo quebrado.html linha 3: `se_colar` sem `acabou`")
	esperaNosDois(t, "arruma\n    renderiza_arquivo("+sumido+")\nquebrou erro\n    mostra erro_tipo(erro)\nacabou_finalmente", "io\n", "")
}
