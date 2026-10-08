package modelo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gambiarrascript/object"
)

// val converte valor Go em object.Object (map vira dicionario na ordem das
// chaves passadas em pares).
func val(v interface{}) object.Object {
	switch v := v.(type) {
	case nil:
		return &object.Nada{}
	case object.Object:
		return v
	case string:
		return &object.Texto{Value: v}
	case int:
		return object.NumInt(int64(v))
	case float64:
		return object.NumFloat(v)
	case bool:
		return &object.Booleano{Value: v}
	case []interface{}:
		els := make([]object.Object, len(v))
		for i, x := range v {
			els[i] = val(x)
		}
		return object.NovaLista(els)
	}
	panic(fmt.Sprintf("val: %T", v))
}

func dic(pares ...interface{}) *object.Dicionario {
	d := object.NovoDicionario()
	for i := 0; i < len(pares); i += 2 {
		k := &object.Texto{Value: pares[i].(string)}
		d.Bota(k.ChaveHash(), object.ParDic{Chave: k, Valor: val(pares[i+1])})
	}
	return d
}

func lista(xs ...interface{}) *object.Lista { return val(xs).(*object.Lista) }

func roda(t *testing.T, fonte string, dados object.Object, op Opcoes) (string, error) {
	t.Helper()
	m, err := Compila("t.html", fonte)
	if err != nil {
		return "", err
	}
	return m.Renderiza(dados, op)
}

func confere(t *testing.T, fonte string, dados object.Object, esperado string) {
	t.Helper()
	out, err := roda(t, fonte, dados, Opcoes{})
	if err != nil {
		t.Fatalf("erro em %q: %v", fonte, err)
	}
	if out != esperado {
		t.Errorf("modelo %q\n  veio:     %q\n  esperado: %q", fonte, out, esperado)
	}
}

func confereErro(t *testing.T, fonte string, dados object.Object, op Opcoes, pedaco string) {
	t.Helper()
	_, err := roda(t, fonte, dados, op)
	if err == nil || !strings.Contains(err.Error(), pedaco) {
		t.Errorf("modelo %q: queria erro com %q, veio %v", fonte, pedaco, err)
	}
}

func TestValoresECaminhos(t *testing.T) {
	d := dic("nome", "Ze", "n", 3, "u", dic("nome", "Bia", "tags", lista("a", "b")),
		"itens", lista(dic("t", "x"), dic("t", "y")), "k", "nome", "nulo", nil, "ok", true)
	confere(t, "Ola, {{ nome }}!", d, "Ola, Ze!")
	confere(t, "{{u.nome}} {{ u.tags[1] }} {{ itens[0].t }} {{ itens[-1].t }}", d, "Bia b x y")
	confere(t, `{{ u["nome"] }} {{ u[k] }}`, d, "Bia Bia")
	confere(t, "[{{ nao_tem }}][{{ u.nao.tem }}][{{ itens[9] }}][{{ nulo }}]", d, "[][][][]")
	confere(t, "{{ n }} {{ ok }} {{ u.tags }}", d, "3 deu_bom [a, b]")
	confere(t, `{{ "{{" }} literal`, d, "{{ literal")
	confere(t, "sem nada", nil, "sem nada")
	confere(t, `{{ falta | padrao "}}" }}!`, d, "}}!")
}

func TestEscape(t *testing.T) {
	xss := `<script>alert('x')</script>"&`
	d := dic("x", xss, "attr", `" onmouseover="alert(1)`, "l", lista(xss))
	confere(t, "{{ x }}", d, "&lt;script&gt;alert(&#39;x&#39;)&lt;/script&gt;&#34;&amp;")
	confere(t, `<a title="{{ attr }}">`, d, `<a title="&#34; onmouseover=&#34;alert(1)">`)
	confere(t, "{{ l | junta \",\" }}", d, "&lt;script&gt;alert(&#39;x&#39;)&lt;/script&gt;&#34;&amp;")
	confere(t, "{{ x | maiusculo }}", d, "&lt;SCRIPT&gt;ALERT(&#39;X&#39;)&lt;/SCRIPT&gt;&#34;&amp;")
	confere(t, "{{{ x }}}", d, xss)
	confere(t, "{{ x | cru }}", d, xss)
	out, _ := roda(t, "{{ x }}", d, Opcoes{SemEscapar: true})
	if out != xss {
		t.Errorf("SemEscapar: %q", out)
	}
	for _, c := range []string{"<", ">", `"`, "'"} {
		if out, _ := roda(t, "{{ x }}{{ attr }}", d, Opcoes{}); strings.Contains(out, c) {
			t.Errorf("escape deixou passar %q: %q", c, out)
		}
	}
}

func TestFiltros(t *testing.T) {
	d := dic("nome", "Zé", "p", 3.14159, "i", 7, "l", lista(1, 2, 3), "vazio", "")
	op := Opcoes{
		Formata: func(f string, v object.Object) (string, error) {
			if n, ok := v.(*object.Numero); ok {
				return fmt.Sprintf(f, n.Value), nil
			}
			return fmt.Sprintf(f, v.Inspect()), nil
		},
		Json: func(v object.Object) (string, error) { return `"` + v.Inspect() + `"`, nil },
	}
	casos := []struct{ fonte, esperado string }{
		{"{{ nome | maiusculo }}", "ZÉ"},
		{"{{ nome | minusculo }}", "zé"},
		{"{{ nome | tamanho }} {{ l | tamanho }} {{ falta | tamanho }}", "2 3 0"},
		{`{{ p | formata "%.2f" }}`, "3.14"},
		{`{{ falta | padrao "anonimo" }} {{ vazio | padrao "-" }} {{ nome | padrao "x" }}`, "anonimo - Zé"},
		{`{{ l | junta ", " }}`, "1, 2, 3"},
		{`{{ nome | json | cru }}`, `"Zé"`},
		{`{{ "</script>" | json | cru }}`, `"</script>"`},
		{`{{ falta | padrao "a" | maiusculo }}`, "A"},
	}
	for _, c := range casos {
		out, err := roda(t, c.fonte, d, op)
		if err != nil || out != c.esperado {
			t.Errorf("%q: veio %q (%v), queria %q", c.fonte, out, err, c.esperado)
		}
	}
	confereErro(t, "{{ i | tamanho }}", d, op, "`| tamanho` quer")
	confereErro(t, "{{ nome | sei_la }}", d, op, "filtro `sei_la` nao existe")
	confereErro(t, "{{ nome | formata }}", d, op, "quer um argumento")
}

func TestPraCada(t *testing.T) {
	d := dic("l", lista("a", "b"), "d", dic("x", 1, "y", 2), "v", lista())
	confere(t, "{{ pra_cada x em l }}[{{ x }}]{{ acabou }}", d, "[a][b]")
	confere(t, "{{ pra_cada i, x em l }}{{ i }}={{ x }};{{ acabou }}", d, "0=a;1=b;")
	confere(t, "{{ pra_cada k em d }}{{ k }}{{ acabou }}", d, "xy")
	confere(t, "{{ pra_cada k, v em d }}{{ k }}:{{ v }} {{ acabou }}", d, "x:1 y:2 ")
	confere(t, "{{ pra_cada x em v }}{{ x }}{{ se_nao_colar }}vazio{{ acabou }}", d, "vazio")
	confere(t, "{{ pra_cada x em falta }}{{ x }}{{ se_nao_colar }}vazio{{ acabou_finalmente }}", d, "vazio")
	// linha sozinha some inteira
	confere(t, "<ul>\n  {{ pra_cada x em l }}\n  <li>{{ x }}</li>\n  {{ acabou }}\n</ul>\n", d,
		"<ul>\n  <li>a</li>\n  <li>b</li>\n</ul>\n")
	confereErro(t, "{{ pra_cada x em 3 }}{{ acabou }}", d, Opcoes{}, "quer lista, dicionario ou conjunto, veio numero")
}

func TestSeColar(t *testing.T) {
	d := dic("n", 5, "s", "ze", "f", false, "z", 0, "vazio", "", "l", lista())
	casos := []struct{ fonte, esperado string }{
		{"{{ se_colar n > 3 }}sim{{ acabou }}", "sim"},
		{"{{ se_colar n < 3 }}sim{{ se_nao_colar }}nao{{ acabou }}", "nao"},
		{"{{ se_colar n == 1 }}1{{ se_nao_colar se_colar n == 5 }}5{{ se_nao_colar }}?{{ acabou }}", "5"},
		{`{{ se_colar s == "ze" e nao f }}ok{{ acabou }}`, "ok"},
		{`{{ se_colar s != "ze" ou f }}x{{ se_nao_colar }}y{{ acabou }}`, "y"},
		{"{{ se_colar falta }}x{{ se_nao_colar }}y{{ acabou }}", "y"},
		// truthiness da linguagem: 0, "" e [] sao verdade
		{"{{ se_colar z }}a{{ acabou }}{{ se_colar vazio }}b{{ acabou }}{{ se_colar l }}c{{ acabou }}", "abc"},
		{"{{ se_colar l | tamanho > 0 }}tem{{ se_nao_colar }}nao tem{{ acabou }}", "nao tem"},
		{"{{ se_colar falta == nada }}nada{{ acabou }}", "nada"},
		{"{{ se_colar n >= 5 e (f ou n <= 5) }}ok{{ acabou }}", "ok"},
	}
	for _, c := range casos {
		confere(t, c.fonte, d, c.esperado)
	}
	confereErro(t, `{{ se_colar s > 3 }}x{{ acabou }}`, d, Opcoes{}, "nao da pra comparar texto com numero")
}

func TestComentarioEStrito(t *testing.T) {
	confere(t, "a{{# isso some\n }}b", nil, "ab")
	confere(t, "a\n{{# linha toda }}\nb", nil, "a\nb")
	d := dic("u", dic("nome", "Ze"))
	confereErro(t, "x\n{{ u.idade }}", d, Opcoes{Estrito: true}, "modelo t.html linha 2: `u.idade` nao existe (modo estrito)")
	out, err := roda(t, `{{ u.idade | padrao "?" }}{{ se_colar u.idade }}x{{ acabou }}`, d, Opcoes{Estrito: true})
	if err != nil || out != "?" {
		t.Errorf("estrito com padrao/se_colar: %q %v", out, err)
	}
}

func TestErrosDeSintaxe(t *testing.T) {
	casos := []struct{ fonte, msg string }{
		{"a\nb\n{{ pra_cada x em l }}\nc", "modelo t.html linha 3: `pra_cada` sem `acabou`"},
		{"{{ se_colar x }}", "`se_colar` sem `acabou`"},
		{"{{ acabou }}", "`acabou` sobrando"},
		{"{{ se_nao_colar }}", "`se_nao_colar` sem `se_colar`"},
		{"{{ pra_cada x l }}{{ acabou }}", "sem o `em`"},
		{"{{ x", "`{{` sem `}}`"},
		{"{{{ x }}", "`{{{` sem `}}}`"},
		{"{{ }}", "vazio"},
		{"{{ a b }}", "sobrou `b`"},
		{`{{ inclui x }}`, "quer um texto entre aspas"},
		{"{{ se_colar x }}{{ se_nao_colar }}{{ se_nao_colar }}{{ acabou }}", "repetido"},
		{"{{ x[1 }}", "faltou fechar o `[`"},
		{`{{ "abc }}`, "`{{` sem `}}`"},
		{`{{ x | padrao "abc }}"`, "sem `}}`"},
	}
	for _, c := range casos {
		_, err := Compila("t.html", c.fonte)
		if err == nil || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%q: queria %q, veio %v", c.fonte, c.msg, err)
		}
	}
}

func escreve(t *testing.T, dir, nome, conteudo string) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(nome))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIncluiEUsa(t *testing.T) {
	dir := t.TempDir()
	escreve(t, dir, "base.html", "<title>{{ bloco \"titulo\" }}Site{{ acabou }}</title>\n<main>\n{{ bloco \"conteudo\" }}\n{{ acabou }}\n</main>\n{{ inclui \"parciais/rodape.html\" }}\n")
	escreve(t, dir, "parciais/rodape.html", "<footer>{{ ano }}{{ inclui \"assinatura.html\" }}</footer>\n")
	escreve(t, dir, "parciais/assinatura.html", " - {{ autor | padrao \"anonimo\" }}")
	escreve(t, dir, "parciais/item.html", "<li>{{ item }}</li>\n")
	pag := escreve(t, dir, "pagina.html", `{{ usa "base.html" }}
{{ bloco "titulo" }}Inicio{{ acabou }}
{{ bloco "conteudo" }}
<ul>
  {{ pra_cada item em itens }}
  {{ inclui "parciais/item.html" }}
  {{ acabou }}
</ul>
{{ acabou }}
`)
	m, err := CarregaArquivo(pag, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := m.Renderiza(dic("ano", 2026, "itens", lista("a", "<b>")), Opcoes{})
	if err != nil {
		t.Fatal(err)
	}
	esperado := "<title>Inicio</title>\n<main>\n<ul>\n<li>a</li>\n<li>&lt;b&gt;</li>\n</ul>\n</main>\n<footer>2026 - anonimo</footer>\n"
	if out != esperado {
		t.Errorf("layout:\n veio:     %q\n esperado: %q", out, esperado)
	}

	// cache: mesmo arquivo, mesmo *Modelo; mudou o arquivo, recompila
	m2, _ := CarregaArquivo(pag, "")
	if m2 != m {
		t.Error("cache nao reaproveitou o modelo")
	}
	escreve(t, dir, "pagina.html", "outro conteudo maior")
	m3, _ := CarregaArquivo(pag, "")
	if m3 == m {
		t.Error("cache nao viu a mudanca no arquivo")
	}
}

func TestIncluiSeguranca(t *testing.T) {
	dir := t.TempDir()
	escreve(t, dir, "segredo.txt", "SEGREDO")
	escreve(t, dir, "modelos/a.html", `{{ inclui "../segredo.txt" }}`)
	escreve(t, dir, "modelos/b.html", `{{ inclui "/etc/passwd" }}`)
	escreve(t, dir, "modelos/c.html", `{{ inclui "c.html" }}`)
	escreve(t, dir, "modelos/d.html", "a\n\n{{ inclui \"nao_tem.html\" }}")
	escreve(t, dir, "modelos/e.html", "{{ inclui \"f.html\" }}")
	escreve(t, dir, "modelos/f.html", "\n{{ pra_cada x em 1 }}{{ acabou }}")
	casos := []struct{ arq, msg string }{
		{"a.html", "sai da pasta dos modelos"},
		{"b.html", "caminho absoluto nao vale"},
		{"c.html", "em circulo"},
		{"d.html", "modelo d.html linha 3: `inclui \"nao_tem.html\"`: nao achei"},
		{"e.html", "modelo f.html linha 2: `pra_cada` quer lista"},
	}
	for _, c := range casos {
		m, err := CarregaArquivo(filepath.Join(dir, "modelos", c.arq), "")
		if err == nil {
			_, err = m.Renderiza(nil, Opcoes{})
		}
		if err == nil || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s: queria %q, veio %v", c.arq, c.msg, err)
		}
		if err != nil && strings.Contains(err.Error(), "SEGREDO") {
			t.Errorf("%s vazou o segredo", c.arq)
		}
	}
	// link simbolico pra fora da raiz tambem nao passa
	if err := os.Symlink(filepath.Join(dir, "segredo.txt"), filepath.Join(dir, "modelos", "link.html")); err == nil {
		escreve(t, dir, "modelos/g.html", `{{ inclui "link.html" }}`)
		m, _ := CarregaArquivo(filepath.Join(dir, "modelos", "g.html"), "")
		if out, err := m.Renderiza(nil, Opcoes{}); err == nil || strings.Contains(out, "SEGREDO") {
			t.Errorf("symlink escapou: %q %v", out, err)
		}
	}
	// com Pasta = dir, o ../ que fica dentro da raiz vale
	m, _ := CarregaArquivo(filepath.Join(dir, "modelos", "a.html"), "")
	if out, err := m.Renderiza(nil, Opcoes{Pasta: dir}); err != nil || out != "SEGREDO" {
		t.Errorf("Pasta maior: %q %v", out, err)
	}
}

func TestInstanciaDeTreta(t *testing.T) {
	tr, msg := object.NovaTreta("Ponto", []object.CampoTreta{{Nome: "x"}, {Nome: "y"}})
	if msg != "" {
		t.Fatal(msg)
	}
	inst, msg := object.MontaInstancia(tr, []string{"x", "y"}, []object.Object{object.NumInt(1), object.NumInt(2)}, nil)
	if msg != "" {
		t.Fatal(msg)
	}
	confere(t, "{{ p.x }},{{ p.y }}{{ p.z }}", dic("p", inst), "1,2")
	confere(t, "{{ x }}", inst, "1")
}
