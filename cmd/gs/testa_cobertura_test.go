package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// projeto de exemplo: modulo testado (com ramo e funcao que nunca rodam),
// modulo dependente numa subpasta, um .gs que nenhum teste importa e uma
// dependencia em gs_modulos/ (fica de fora do relatorio).
var projetoCobertura = map[string]string{
	"calc.gs": `importa "util/fmt.gs" como f
importa "dep.gs" como d
gambiarra divide(a, b)
    se_colar b == 0
        quebra("divisao por zero")
    acabou_finalmente
    funciona a / b
acabou_finalmente
gambiarra nunca()
    funciona 0
acabou_finalmente
gambiarra mostra_div(a, b)
    funciona f.rotulo(divide(a, b)) + d.marca()
acabou_finalmente
`,
	"util/fmt.gs": `gambiarra rotulo(x)
    funciona "=" + texto(x)
acabou_finalmente
`,
	"gs_modulos/dep.gs": `gambiarra marca()
    funciona "!"
acabou_finalmente
`,
	"solto.gs": `mostra "ninguem me importa"
`,
	"calc_test.gs": `importa "calc.gs"
espera(divide(10, 2), 5)
espera(mostra_div(9, 3), "=3!")
`,
}

func montaProjeto(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for nome, fonte := range projetoCobertura {
		caminho := filepath.Join(dir, nome)
		if err := os.MkdirAll(filepath.Dir(caminho), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(fonte), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// linhaCobertura acha a linha do resumo de um arquivo: "  <nome>  c/e linhas  p%".
func linhaCobertura(t *testing.T, saida, sufixo string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^  (\S*` + regexp.QuoteMeta(sufixo) + `)\s+(\d+)/(\d+)\s+linhas\s+([\d.]+)%$`)
	m := re.FindStringSubmatch(saida)
	if m == nil {
		t.Fatalf("sem linha de cobertura pra %q na saida:\n%s", sufixo, saida)
	}
	return m[2] + "/" + m[3] + " " + m[4]
}

func TestTestaCoberturaCLI(t *testing.T) {
	dir := montaProjeto(t)
	perfis := map[string]string{}
	for _, engine := range []string{"VM", "tree-walker"} {
		perfil := filepath.Join(t.TempDir(), "cobertura.out")
		html := filepath.Join(t.TempDir(), "cobertura.html")
		op := opcoesTesta{dir: dir, usarVM: engine == "VM", cobertura: true, perfil: perfil, html: html}
		var buf bytes.Buffer
		if !executaTestes(op, &buf) {
			t.Fatalf("[%s] suite devia passar:\n%s", engine, buf.String())
		}
		saida := buf.String()
		for _, want := range []string{
			"OK  calc_test.gs  (2/2 asserts)",
			"cobertura (" + engine + "):",
			"perfil gravado em " + perfil,
			"relatorio HTML gravado em " + html,
		} {
			if !strings.Contains(saida, want) {
				t.Fatalf("[%s] faltou %q na saida:\n%s", engine, want, saida)
			}
		}
		// calc.gs: 10 linhas executaveis; quebra (5) e o corpo de nunca (10)
		// nao rodam
		if got := linhaCobertura(t, saida, "/calc.gs"); got != "8/10 80.0" {
			t.Errorf("[%s] calc.gs: %s", engine, got)
		}
		if got := linhaCobertura(t, saida, "util/fmt.gs"); got != "2/2 100.0" {
			t.Errorf("[%s] util/fmt.gs: %s", engine, got)
		}
		if got := linhaCobertura(t, saida, "/solto.gs"); got != "0/1 0.0" {
			t.Errorf("[%s] solto.gs: %s", engine, got)
		}
		if got := linhaCobertura(t, saida, "total"); got != "10/13 76.9" {
			t.Errorf("[%s] total: %s", engine, got)
		}
		relatorio := saida[strings.Index(saida, "cobertura ("):]
		if strings.Contains(relatorio, "dep.gs") || strings.Contains(relatorio, "_test.gs") {
			t.Errorf("[%s] gs_modulos/ e *_test.gs ficam de fora:\n%s", engine, saida)
		}
		dados, err := os.ReadFile(perfil)
		if err != nil {
			t.Fatal(err)
		}
		perfis[engine] = string(dados)
		if !strings.HasPrefix(perfis[engine], "modo: contagem\n") || !strings.Contains(perfis[engine], "calc.gs:5 0\n") ||
			!strings.Contains(perfis[engine], "calc.gs:4 2\n") {
			t.Errorf("[%s] perfil inesperado:\n%s", engine, perfis[engine])
		}
		pagina, err := os.ReadFile(html)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(pagina), `class="l faltou"`) || !strings.Contains(string(pagina), `class="l rodou"`) {
			t.Errorf("[%s] html sem linhas coloridas", engine)
		}
	}
	if perfis["VM"] != perfis["tree-walker"] {
		t.Fatalf("perfil diverge entre engines:\nVM:\n%s\nTW:\n%s", perfis["VM"], perfis["tree-walker"])
	}
}

// sem --cobertura a saida e a de sempre (nada de relatorio).
func TestTestaSemCobertura(t *testing.T) {
	dir := montaProjeto(t)
	var buf bytes.Buffer
	if !executaTestes(opcoesTesta{dir: dir, usarVM: true}, &buf) {
		t.Fatalf("suite devia passar:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "cobertura") {
		t.Fatalf("sem --cobertura nao imprime relatorio:\n%s", buf.String())
	}
}
