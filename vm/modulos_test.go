package vm

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Testes do sistema de modulos (importa) com arquivos de verdade num dir
// temporario, rodando o MESMO programa nos dois engines.

// montaArquivos grava os arquivos (caminho relativo -> fonte) no dir.
func montaArquivos(t *testing.T, dir string, arquivos map[string]string) {
	t.Helper()
	for nome, fonte := range arquivos {
		caminho := filepath.Join(dir, nome)
		if err := os.MkdirAll(filepath.Dir(caminho), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, []byte(fonte), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// rodaArquivoTW roda o arquivo principal no tree-walker: (saida, erro).
func rodaArquivoTW(t *testing.T, principal string) (string, string) {
	t.Helper()
	fonte, err := os.ReadFile(principal)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return "", "parse: " + strings.Join(errs, "; ")
	}
	var buf bytes.Buffer
	interp := interpreter.New(&buf)
	interp.DefinirArquivo(principal)
	res := interp.Eval(prog, object.NewEnvironment())
	if e, ok := res.(*object.Erro); ok && !e.Handled {
		return buf.String(), e.Message
	}
	return buf.String(), ""
}

// rodaArquivoVM roda o arquivo principal na VM: (saida, erro).
func rodaArquivoVM(t *testing.T, principal string) (string, string) {
	t.Helper()
	fonte, err := os.ReadFile(principal)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return "", "parse: " + strings.Join(errs, "; ")
	}
	comp := compiler.New()
	comp.Arquivo = principal
	if err := comp.Compile(prog); err != nil {
		return "", "compile: " + err.Error()
	}
	var buf bytes.Buffer
	maq := New(comp.Bytecode(), &buf)
	if err := maq.Run(); err != nil {
		return buf.String(), err.Error()
	}
	return buf.String(), ""
}

// esperaModulos monta os arquivos, roda `principal` nos dois engines e confere
// a saida exata e (se pedido) um pedaco do erro.
func esperaModulos(t *testing.T, arquivos map[string]string, principal, saidaEsp, erroEsp string) {
	t.Helper()
	dir := t.TempDir()
	montaArquivos(t, dir, arquivos)
	caminho := filepath.Join(dir, principal)
	engines := []struct {
		nome string
		roda func(*testing.T, string) (string, string)
	}{{"tree", rodaArquivoTW}, {"vm", rodaArquivoVM}}
	for _, e := range engines {
		saida, errStr := e.roda(t, caminho)
		if saida != saidaEsp {
			t.Errorf("[%s] saida errada\n  veio:     %q\n  esperado: %q (erro: %q)", e.nome, saida, saidaEsp, errStr)
		}
		if erroEsp == "" && errStr != "" {
			t.Errorf("[%s] erro inesperado: %s", e.nome, errStr)
		}
		if erroEsp != "" && !strings.Contains(errStr, erroEsp) {
			t.Errorf("[%s] erro esperado %q, veio: %q", e.nome, erroEsp, errStr)
		}
	}
}

const utilGS = `mostra "carregou util"
bota versao = "1.0"
gambiarra dobra(n)
    funciona n * 2
acabou_finalmente
`

// Modulo roda UMA vez por processo, nao importa quantas vezes nem de onde
// (com ou sem `como`, do principal ou de outro modulo).
func TestModuloRodaUmaVez(t *testing.T) {
	esperaModulos(t, map[string]string{
		"util.gs": utilGS,
		"a.gs":    "importa \"util.gs\"\nmostra \"a: \" + texto(dobra(3))\n",
		"main.gs": `importa "util.gs" como u
importa "a.gs"
importa "util.gs"
importa "util.gs" como u2
mostra u.dobra(5)
mostra dobra(1)
mostra u2.versao
`,
	}, "main.gs", "carregou util\na: 6\n10\n2\n1.0\n", "")
}

// `como` amarra SO o alias: nada do modulo vaza pro escopo de quem importa.
func TestModuloComoNaoVaza(t *testing.T) {
	esperaModulos(t, map[string]string{
		"util.gs": utilGS,
		"main.gs": `bota versao = "principal"
importa "util.gs" como u
mostra versao
mostra u.versao
mostra tipo(u)
`,
	}, "main.gs", "carregou util\nprincipal\n1.0\ndicionario\n", "")

	// usar o nome sem o alias e erro nos dois (VM acusa na compilacao)
	esperaModulos(t, map[string]string{
		"util.gs": "gambiarra dobra(n)\n    funciona n * 2\nacabou_finalmente\n",
		"main.gs": "importa \"util.gs\" como u\nmostra dobra(2)\n",
	}, "main.gs", "", "`dobra`")
}

// O modulo tem namespace proprio: nao enxerga as globais de quem importa, e as
// gambiarras dele mexem no estado DELE (o alias e uma foto do fim do modulo).
func TestModuloNamespaceProprio(t *testing.T) {
	esperaModulos(t, map[string]string{
		"contador.gs": `bota estado = {"n": 0}
bota passo = 1
gambiarra inc()
    estado["n"] += passo
    funciona estado["n"]
acabou_finalmente
`,
		"main.gs": `bota estado = {"n": 100}
bota passo = 50
importa "contador.gs" como c
c.inc()
mostra c.inc()
mostra c.estado["n"]
mostra estado["n"]
`,
	}, "main.gs", "2\n2\n100\n", "")
}

// Import circular vira erro claro com a cadeia, nos dois engines (antes o
// tree-walker travava e a VM ignorava calada).
func TestModuloCircular(t *testing.T) {
	esperaModulos(t, map[string]string{
		"a.gs": "mostra \"a\"\nimporta \"b.gs\"\n",
		"b.gs": "mostra \"b\"\nimporta \"a.gs\"\n",
	}, "a.gs", "a\nb\n", "importa circular: a.gs -> b.gs -> a.gs")

	esperaModulos(t, map[string]string{
		"main.gs":  "importa \"x.gs\" como x\nmostra \"nunca\"\n",
		"x.gs":     "importa \"lib/y.gs\"\n",
		"lib/y.gs": "gambiarra f()\n    funciona 1\nacabou_finalmente\nimporta \"../x.gs\"\n",
	}, "main.gs", "", "importa circular: x.gs -> lib/y.gs -> x.gs")

	// o erro de ciclo da pra pegar com arruma, com tipo "parse"
	esperaModulos(t, map[string]string{
		"a.gs": `arruma
    importa "b.gs"
quebrou erro
    mostra erro_tipo(erro)
    mostra contem(erro_msg(erro), "importa circular")
acabou_finalmente
`,
		"b.gs": "importa \"a.gs\"\n",
	}, "a.gs", "parse\ndeu_bom\n", "")
}

// Import "preguicoso" (dentro de gambiarra) do modulo que ja terminou nao e
// ciclo: pega do cache.
func TestModuloImportPreguicoso(t *testing.T) {
	esperaModulos(t, map[string]string{
		"a.gs": `bota nome = "a"
gambiarra usa_b()
    importa "b.gs" como b
    funciona b.ola()
acabou_finalmente
`,
		"b.gs": `gambiarra ola()
    importa "a.gs" como a
    funciona "ola de b pra " + a.nome
acabou_finalmente
`,
		"main.gs": "importa \"a.gs\" como a\nmostra a.usa_b()\n",
	}, "main.gs", "ola de b pra a\n", "")
}

// Caminho relativo e relativo ao ARQUIVO que importa — inclusive num importa
// dentro de gambiarra chamada de outro lugar.
func TestModuloCaminhoRelativoAoImportador(t *testing.T) {
	esperaModulos(t, map[string]string{
		"lib/a.gs": `importa "b.gs"
gambiarra pega_c()
    importa "c.gs" como c
    funciona c.valor
acabou_finalmente
`,
		"lib/b.gs": "bota de_b = \"b de lib\"\n",
		"lib/c.gs": "bota valor = \"c de lib\"\n",
		"c.gs":     "bota valor = \"c da raiz (errado)\"\n",
		"main.gs":  "importa \"lib/a.gs\"\nmostra de_b\nmostra pega_c()\n",
	}, "main.gs", "b de lib\nc de lib\n", "")
}

// Sem achar do lado de quem importa, procura em gs_modulos/ subindo pelos
// diretorios (onde o `gs get` baixa).
func TestModuloGsModulos(t *testing.T) {
	esperaModulos(t, map[string]string{
		"gs_modulos/datas.gs": "bota hoje = \"segunda\"\n",
		"sub/usa.gs":          "importa \"datas.gs\" como d\nbota dia = d.hoje\n",
		"main.gs":             "importa \"datas.gs\"\nimporta \"sub/usa.gs\" como u\nimporta \"gs_modulos/datas.gs\" como d2\nmostra hoje\nmostra u.dia\nmostra d2.hoje\n",
	}, "main.gs", "segunda\nsegunda\nsegunda\n", "")
}

// Modulo que nao existe / que nao parseia: o erro diz qual modulo e a linha.
func TestModuloErros(t *testing.T) {
	esperaModulos(t, map[string]string{
		"main.gs": "importa \"sumido.gs\"\n",
	}, "main.gs", "", `deu ruim na linha 1: nao achei o modulo "sumido.gs"`)

	esperaModulos(t, map[string]string{
		"ruim.gs": "bota x = 1\nbota = \n",
		"main.gs": "importa \"ruim.gs\" como r\n",
	}, "main.gs", "", `o modulo "ruim.gs" ta com perrengue: linha 2:`)

	// erro dentro de um modulo importado por outro modulo
	esperaModulos(t, map[string]string{
		"a.gs":    "bota x = 1\n\nimporta \"sumido.gs\"\n",
		"main.gs": "importa \"a.gs\"\n",
	}, "main.gs", "", `deu ruim na linha 3: nao achei o modulo "sumido.gs"`)

	// erro de runtime no topo do modulo: mensagem diz de qual modulo veio
	esperaModulos(t, map[string]string{
		"quebra.gs": "mostra \"antes\"\nquebra(\"explodiu\")\n",
		"main.gs":   "importa \"quebra.gs\" como q\n",
	}, "main.gs", "antes\n", `explodiu (no modulo "quebra.gs")`)
}

// Varias goroutines importando o mesmo modulo ao mesmo tempo: roda uma vez so
// e todo mundo recebe o mesmo modulo.
func TestModuloConcorrente(t *testing.T) {
	esperaModulos(t, map[string]string{
		"lento.gs": "mostra \"rodou\"\nespera_ms(30)\nbota valor = 42\n",
		"main.gs": `gambiarra carrega()
    importa "lento.gs" como m
    funciona m.valor
acabou_finalmente
bota fs = []
pra_cada i em 1..16
    adiciona(fs, bora carrega())
acabou_finalmente
bota total = 0
pra_cada f em fs
    total += espera(f)
acabou_finalmente
mostra total
`,
	}, "main.gs", "rodou\n672\n", "")
}
