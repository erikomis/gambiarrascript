package interpreter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// rodaComArgs roda a fonte no tree-walker com argumentos de linha de comando
// e devolve (stdout, stderr, resultado final).
func rodaComArgs(t *testing.T, src string, args ...string) (string, string, object.Object) {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("erros de parse: %v", errs)
	}
	var out, errOut bytes.Buffer
	i := New(&out)
	i.DefinirStderr(&errOut)
	i.DefinirArgumentos(args)
	res := i.Eval(prog, object.NewEnvironment())
	return out.String(), errOut.String(), res
}

func congelaLog(t *testing.T) {
	t.Helper()
	antes := agoraLog
	fuso := time.FixedZone("BRT", -3*3600)
	agoraLog = func() time.Time { return time.Date(2026, 10, 2, 12, 0, 0, 0, fuso) }
	t.Cleanup(func() { agoraLog = antes })
}

func TestLogTextoENivel(t *testing.T) {
	congelaLog(t)
	t.Setenv("GS_LOG_FORMATO", "")
	src := `log_debug("nada de debug")
log_info("subiu", {"porta": 8080, "modo": "dev local", "vazio": "", "ok": deu_bom})
log_aviso("cuidado")
log_erro("deu ruim\nfeio", {"tags": [1, 2]})`
	casos := []struct{ nivel, esp string }{
		{"", "2026-10-02T12:00:00-03:00 INFO subiu porta=8080 modo=\"dev local\" vazio=\"\" ok=deu_bom\n" +
			"2026-10-02T12:00:00-03:00 AVISO cuidado\n" +
			"2026-10-02T12:00:00-03:00 ERRO deu ruim\\nfeio tags=\"[1, 2]\"\n"},
		{"debug", "2026-10-02T12:00:00-03:00 DEBUG nada de debug\n"},
		{"AVISO", "2026-10-02T12:00:00-03:00 AVISO cuidado\n"},
		{"error", "2026-10-02T12:00:00-03:00 ERRO deu ruim\\nfeio tags=\"[1, 2]\"\n"},
		{"inventado", "2026-10-02T12:00:00-03:00 INFO subiu"},
	}
	for _, c := range casos {
		t.Setenv("GS_LOG_NIVEL", c.nivel)
		out, errOut, res := rodaComArgs(t, src)
		if isError(res) || out != "" {
			t.Fatalf("nivel %q: res=%s out=%q", c.nivel, res.Inspect(), out)
		}
		if !strings.HasPrefix(errOut, c.esp) && !strings.Contains(errOut, c.esp) {
			t.Errorf("nivel %q:\n veio %q\n quer %q", c.nivel, errOut, c.esp)
		}
	}
}

func TestLogJSON(t *testing.T) {
	congelaLog(t)
	t.Setenv("GS_LOG_FORMATO", "json")
	t.Setenv("GS_LOG_NIVEL", "")
	_, errOut, res := rodaComArgs(t, `log_info("oi \"tropa\"", {"n": 1.5, "lista": [1, "a"], "f": gambiarra(x) funciona x acabou_finalmente})
log_erro("so msg")`)
	if isError(res) {
		t.Fatal(res.Inspect())
	}
	linhas := strings.Split(strings.TrimSuffix(errOut, "\n"), "\n")
	if len(linhas) != 2 {
		t.Fatalf("queria 2 linhas, veio %q", errOut)
	}
	esp0 := `{"time":"2026-10-02T12:00:00.000-03:00","nivel":"info","msg":"oi \"tropa\"","campos":{"n":1.5,"lista":[1,"a"],"f":"<gambiarra>"}}`
	if !strings.HasPrefix(linhas[0], `{"time":"2026-10-02T12:00:00.000-03:00","nivel":"info","msg":"oi \"tropa\"","campos":{"n":1.5,"lista":[1,"a"],"f":`) {
		t.Errorf("linha json:\n veio %s\n quer %s", linhas[0], esp0)
	}
	for _, l := range linhas {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("linha nao e JSON valido: %v\n%s", err, l)
		}
		for _, k := range []string{"time", "nivel", "msg", "campos"} {
			if _, ok := m[k]; !ok {
				t.Errorf("faltou %q em %s", k, l)
			}
		}
	}
	if linhas[1] != `{"time":"2026-10-02T12:00:00.000-03:00","nivel":"erro","msg":"so msg","campos":{}}` {
		t.Errorf("linha 2: %s", linhas[1])
	}
}

func TestLogErrosDeUso(t *testing.T) {
	_, _, res := rodaComArgs(t, `log_info("x", [1])`)
	if e, ok := res.(*object.Erro); !ok || !strings.Contains(e.Message, "campos tem que ser dicionario") {
		t.Fatalf("queria erro de campos, veio %s", res.Inspect())
	}
	_, _, res = rodaComArgs(t, `log_aviso()`)
	if e, ok := res.(*object.Erro); !ok || !strings.Contains(e.Message, "log_aviso() quer (mensagem, [campos])") {
		t.Fatalf("queria erro de aridade, veio %s", res.Inspect())
	}
}

// varias goroutines logando juntas: cada chamada sai numa linha inteira, sem
// pedaco de uma no meio da outra.
func TestLogConcorrenteNaoEmbola(t *testing.T) {
	t.Setenv("GS_LOG_FORMATO", "")
	t.Setenv("GS_LOG_NIVEL", "")
	var errOut bytes.Buffer
	i := New(&bytes.Buffer{})
	i.DefinirStderr(&errOut)
	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for n := 0; n < 50; n++ {
				campos := object.NovoDicionario()
				botaTexto(campos, "g", object.NumInt(int64(g)))
				botaTexto(campos, "n", object.NumInt(int64(n)))
				botaTexto(campos, "texto", txt(strings.Repeat("x", 200)))
				i.builtinLogInfo([]object.Object{txt("linha"), campos})
			}
		}(g)
	}
	wg.Wait()
	linhas := strings.Split(strings.TrimSuffix(errOut.String(), "\n"), "\n")
	if len(linhas) != 1000 {
		t.Fatalf("queria 1000 linhas, veio %d", len(linhas))
	}
	re := regexp.MustCompile(`^\S+ INFO linha g=\d+ n=\d+ texto=x{200}$`)
	for _, l := range linhas {
		if !re.MatchString(l) {
			t.Fatalf("linha embolada: %q", l)
		}
	}
}

func TestOpcoes(t *testing.T) {
	padroes := `{"porta": 8080, "verboso": deu_ruim, "nome": "api", "taxa": 0.5, "tag": [], "log_nivel": "info", "extra": nada}`
	casos := []struct {
		args []string
		esp  string
	}{
		{nil, `{"porta": 8080, "verboso": deu_ruim, "nome": "api", "taxa": 0.5, "tag": [], "log_nivel": "info", "extra": nada, "_": []}`},
		{[]string{"--porta", "9090", "--verboso", "x"}, `{"porta": 9090, "verboso": deu_bom, "nome": "api", "taxa": 0.5, "tag": [], "log_nivel": "info", "extra": nada, "_": ["x"]}`},
		{[]string{"--porta=9090", "--nome=", "--taxa", "-1.25", "--extra", "e"}, `{"porta": 9090, "verboso": deu_ruim, "nome": "", "taxa": -1.25, "tag": [], "log_nivel": "info", "extra": "e", "_": []}`},
		{[]string{"--verboso", "--no-verboso"}, `{"porta": 8080, "verboso": deu_ruim, "nome": "api", "taxa": 0.5, "tag": [], "log_nivel": "info", "extra": nada, "_": []}`},
		{[]string{"--verboso=sim", "--sem-verboso", "--verboso=true"}, `{"porta": 8080, "verboso": deu_bom, "nome": "api", "taxa": 0.5, "tag": [], "log_nivel": "info", "extra": nada, "_": []}`},
		{[]string{"--tag", "a", "--tag=b", "--log-nivel", "debug"}, `{"porta": 8080, "verboso": deu_ruim, "nome": "api", "taxa": 0.5, "tag": ["a", "b"], "log_nivel": "debug", "extra": nada, "_": []}`},
		{[]string{"a", "-", "-5", "--", "--porta", "x"}, `{"porta": 8080, "verboso": deu_ruim, "nome": "api", "taxa": 0.5, "tag": [], "log_nivel": "info", "extra": nada, "_": ["a", "-", "-5", "--porta", "x"]}`},
	}
	for _, c := range casos {
		out, _, res := rodaComArgs(t, "bota o = opcoes("+padroes+")\nmostra pra_json(o)\nmostra o", c.args...)
		if isError(res) {
			t.Fatalf("%v: %s", c.args, res.Inspect())
		}
		linhas := strings.Split(out, "\n")
		if linhas[1] != strings.NewReplacer(`"tag": ["a", "b"]`, `"tag": [a, b]`, `"_": ["x"]`, `"_": [x]`,
			`"_": ["a", "-", "-5", "--porta", "x"]`, `"_": [a, -, -5, --porta, x]`, `"extra": "e"`, `"extra": e`).Replace(c.esp) &&
			!jsonIgual(t, linhas[0], c.esp) {
			t.Errorf("%v:\n veio %s\n quer %s", c.args, linhas[1], c.esp)
		}
		if !jsonIgual(t, linhas[0], c.esp) {
			t.Errorf("%v (json):\n veio %s\n quer %s", c.args, linhas[0], c.esp)
		}
	}
}

// jsonIgual compara o pra_json com o Inspect esperado trocando ": " por ":"
// e ", " por "," (o Inspect e o JSON so diferem no espaco e no nada/null).
func jsonIgual(t *testing.T, js, insp string) bool {
	t.Helper()
	conv := strings.NewReplacer(": ", ":", ", ", ",", "deu_bom", "true", "deu_ruim", "false", "nada", "null").Replace(insp)
	return js == conv
}

func TestOpcoesErros(t *testing.T) {
	padroes := `{"porta": 8080, "verboso": deu_ruim}`
	casos := []struct {
		args   []string
		trecho string
	}{
		{[]string{"--porta", "abc"}, "--porta espera numero, veio abc"},
		{[]string{"--porta"}, "--porta precisa de um valor (numero)"},
		{[]string{"--porta", "--verboso"}, "--porta precisa de um valor"},
		{[]string{"--xpto"}, "flag desconhecida --xpto (as que rolam: --porta, --verboso, --ajuda)"},
		{[]string{"-v"}, "flag curta -v nao rola"},
		{[]string{"--verboso=talvez"}, "--verboso espera booleano"},
		{[]string{"--no-porta"}, "flag desconhecida --no-porta"},
	}
	for _, c := range casos {
		_, _, res := rodaComArgs(t, "opcoes("+padroes+")", c.args...)
		e, ok := res.(*object.Erro)
		if !ok || !strings.Contains(e.Message, c.trecho) {
			t.Errorf("%v: queria erro %q, veio %s", c.args, c.trecho, res.Inspect())
		}
	}
	for _, src := range []string{`opcoes([1])`, `opcoes({"x": {"a": 1}})`, `opcoes({"_": 1})`, `opcoes({"x": 1}, [1])`} {
		if _, _, res := rodaComArgs(t, src); !isError(res) {
			t.Errorf("%s devia dar erro, veio %s", src, res.Inspect())
		}
	}
}

func TestOpcoesAjuda(t *testing.T) {
	for _, flag := range []string{"--ajuda", "--help", "-h"} {
		out, _, res := rodaComArgs(t, `opcoes({"porta": 8080, "verboso": deu_ruim, "nome": "api", "tag": []}, {"porta": "porta HTTP"})
mostra "nao chega aqui"`, "x", flag, "--porta", "lixo")
		s, ok := res.(*object.Sair)
		if !ok || s.Codigo != 0 {
			t.Fatalf("%s: queria sai(0), veio %s", flag, res.Inspect())
		}
		esp := `uso: [opcoes] [--] [argumentos...]

opcoes:
  --porta <numero>         porta HTTP (padrao: 8080)
  --verboso, --no-verboso  (padrao: deu_ruim)
  --nome <texto>           (padrao: "api")
  --tag <texto>...
  --ajuda, -h              mostra essa ajuda e sai
`
		if out != esp {
			t.Fatalf("%s: ajuda:\n%s\nquer:\n%s", flag, out, esp)
		}
	}
	// se "ajuda" e opcao do usuario, --ajuda e dele
	out, _, res := rodaComArgs(t, `mostra opcoes({"ajuda": deu_ruim}).ajuda`, "--ajuda")
	if isError(res) || out != "deu_bom\n" {
		t.Fatalf("ajuda do usuario: %q %s", out, res.Inspect())
	}
}

func TestEnvComPadrao(t *testing.T) {
	t.Setenv("GS_TESTE_ENV_VAZIA", "")
	os.Unsetenv("GS_TESTE_ENV_NAO_TEM")
	out, _, res := rodaComArgs(t, `mostra env("GS_TESTE_ENV_NAO_TEM")
mostra env("GS_TESTE_ENV_NAO_TEM", 42)
mostra env("GS_TESTE_ENV_NAO_TEM", "padrao")
mostra "[" + env("GS_TESTE_ENV_VAZIA", "padrao") + "]"`)
	if isError(res) || out != "nada\n42\npadrao\n[]\n" {
		t.Fatalf("env com padrao: %q %s", out, res.Inspect())
	}
}

func TestParseDotenv(t *testing.T) {
	src := "\xef\xbb\xbf# comentario\n" +
		"SIMPLES=valor\r\n" +
		"  ESPACO = com espaco no meio  \n" +
		"export EXPORTADA=sim\n" +
		"exportar=nao e export\n" +
		"COMENT=abc # isso some\n" +
		"HASH=abc#isso fica\n" +
		"DUPLA=\"linha1\\nlinha2 \\\"aspas\\\" \\\\ \\$HOME\" # comentario\n" +
		"SIMPLES_ASPA='literal \\n ${NADA}'\n" +
		"VAZIA=\n" +
		"MULTI=\"primeira\nsegunda\"\n" +
		"DEPOIS=ok\n" +
		"COM.PONTO-E-TRACO=1\n" +
		"IGUAL=a=b=c"
	pares, err := parseDotenv(src)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range pares {
		got = append(got, fmt.Sprintf("%s=%q", p.chave, p.valor))
	}
	esp := []string{
		`SIMPLES="valor"`,
		`ESPACO="com espaco no meio"`,
		`EXPORTADA="sim"`,
		`exportar="nao e export"`,
		`COMENT="abc"`,
		`HASH="abc#isso fica"`,
		`DUPLA="linha1\nlinha2 \"aspas\" \\ $HOME"`,
		`SIMPLES_ASPA="literal \\n ${NADA}"`,
		`VAZIA=""`,
		`MULTI="primeira\nsegunda"`,
		`DEPOIS="ok"`,
		`COM.PONTO-E-TRACO="1"`,
		`IGUAL="a=b=c"`,
	}
	if strings.Join(got, "\n") != strings.Join(esp, "\n") {
		t.Fatalf("parse:\n%s\nquer:\n%s", strings.Join(got, "\n"), strings.Join(esp, "\n"))
	}

	ruins := map[string]string{
		"SEM_IGUAL":         "linha 1 sem '='",
		"OK=1\n1COMECA=x":   "linha 2: nome de variavel invalido",
		"A=\"nunca fecha\n": "linha 1: aspa \" abriu e nunca fechou",
		"A=\"x\" lixo":      "lixo depois das aspas",
		"A=\"x\ny\"\nB":     "linha 3 sem '='",
		"TEM ESPACO=1":      "nome de variavel invalido",
	}
	for s, trecho := range ruins {
		if _, err := parseDotenv(s); err == nil || !strings.Contains(err.Error(), trecho) {
			t.Errorf("%q: queria erro %q, veio %v", s, trecho, err)
		}
	}
}

func TestCarregaEnv(t *testing.T) {
	dir := t.TempDir()
	arq := filepath.Join(dir, "app.env")
	os.WriteFile(arq, []byte("GS_TESTE_A=do_arquivo\nGS_TESTE_B=\"b b\"\n"), 0o600)
	t.Setenv("GS_TESTE_A", "do_ambiente") // ja existe: nao sobrescreve
	os.Unsetenv("GS_TESTE_B")
	t.Cleanup(func() { os.Unsetenv("GS_TESTE_B") })

	src := fmt.Sprintf(`bota r = carrega_env(%q)
mostra r
mostra env("GS_TESTE_A") + "|" + env("GS_TESTE_B")
bota r2 = carrega_env(%q, {"sobrescreve": deu_bom})
mostra env("GS_TESTE_A")`, arq, arq)
	out, _, res := rodaComArgs(t, src)
	if isError(res) {
		t.Fatal(res.Inspect())
	}
	esp := `{"GS_TESTE_A": "do_ambiente", "GS_TESTE_B": "b b"}
do_ambiente|b b
do_arquivo
`
	if out != esp {
		t.Fatalf("carrega_env:\n%s\nquer:\n%s", out, esp)
	}

	// sem arquivo: erro "io", pegavel
	_, _, res = rodaComArgs(t, fmt.Sprintf(`carrega_env(%q)`, filepath.Join(dir, "nao_tem.env")))
	if e, ok := res.(*object.Erro); !ok || e.Kind != KindIO || !strings.Contains(e.Message, "nao consegui ler") {
		t.Fatalf("queria erro io, veio %s", res.Inspect())
	}
	// arquivo com erro nao aplica nada pela metade
	ruim := filepath.Join(dir, "ruim.env")
	os.WriteFile(ruim, []byte("GS_TESTE_C=1\nquebrado\n"), 0o600)
	os.Unsetenv("GS_TESTE_C")
	_, _, res = rodaComArgs(t, fmt.Sprintf(`carrega_env(%q)`, ruim))
	if e, ok := res.(*object.Erro); !ok || !strings.Contains(e.Message, "linha 2 sem '='") {
		t.Fatalf("queria erro de parse, veio %s", res.Inspect())
	}
	if _, tem := os.LookupEnv("GS_TESTE_C"); tem {
		t.Fatal("arquivo quebrado aplicou variavel pela metade")
	}
	// opcoes so como 1o arg (caminho padrao .env) e opcao invalida
	_, _, res = rodaComArgs(t, `carrega_env({"sobrescreve": 1})`)
	if !isError(res) || !strings.Contains(res.Inspect(), "sobrescreve quer booleano") {
		t.Fatalf("queria erro de opcao, veio %s", res.Inspect())
	}
}
