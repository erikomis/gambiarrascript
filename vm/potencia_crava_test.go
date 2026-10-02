package vm

import (
	"bytes"
	"fmt"
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

// TestParidadePotencia: `**` (e `**=`) da o mesmo resultado, a mesma saida e
// o mesmo erro nos dois engines — inclusive pelo caminho do OpBinConst
// (`x ** 2`) e do constant folding (`2 ** 10`).
func TestParidadePotencia(t *testing.T) {
	casos := []string{
		"mostra 2 ** 10",
		"mostra 2 ** 3 ** 2",
		"mostra -2 ** 2",
		"mostra (-2) ** 2",
		"mostra 2 ** -1",
		"mostra 2 ** 0.5",
		"mostra 1.5 ** 2",
		"mostra 2 ** 3 * 4",
		"mostra 2 ** 64",
		"mostra 3 ** 40",
		"mostra 10 ** 30",
		"bota x = 3\nmostra x ** 2",
		"bota x = 2\nmostra 10 ** x",
		"bota x = 1.5\nmostra x ** 2",
		"bota x = 3\nx **= 3\nmostra x",
		"bota xs = [2]\nxs[0] **= 5\nmostra xs",
		"bota n = 0\nmostra n ** -1",
		"mostra 0 ** -1",
		`mostra "a" ** 2`,
		`bota x = 2` + "\n" + `mostra x ** "b"`,
		"mostra nada ** 2",
	}
	for _, src := range casos {
		comparaEngines(t, src)
	}
}

// TestParidadeMatematica: seno/cosseno/tangente/log/log10/exp e o `pi`.
func TestParidadeMatematica(t *testing.T) {
	casos := []string{
		"mostra pi",
		"bota r = 2\nbota area = pi * r ** 2\nmostra area",
		"mostra seno(pi / 2)",
		"mostra cosseno(0)",
		"mostra tangente(0)",
		"mostra log(exp(2))",
		"mostra log10(1000)",
		"mostra exp(0)",
		"mostra log(0)",
		"mostra log10(-1)",
		`mostra seno("x")`,
		"mostra cosseno()",
		// pi sombreavel, igual builtin
		"bota pi = 3\nmostra pi",
		"gambiarra f()\n    funciona pi * 2\nacabou_finalmente\nmostra f()",
		"bota g = gambiarra(r) funciona pi * r ** 2 acabou_finalmente\nmostra g(1)",
	}
	for _, src := range casos {
		comparaEngines(t, src)
	}
}

// TestParidadeCravaRoda: o que crava permite roda igual nos dois engines.
func TestParidadeCravaRoda(t *testing.T) {
	casos := []string{
		"crava PI = 3.14\nmostra PI",
		// conteudo de lista/dict cravado muda por dentro
		"crava XS = [1, 2]\nbota XS[0] = 9\nXS[1] += 1\nadiciona(XS, 3)\nmostra XS",
		"crava D = {\"a\": 1}\nbota D.b = 2\nmostra D.b",
		// gambiarra de dentro sombreia com bota (escopo de funcao)
		"crava N = 1\ngambiarra f()\n    bota N = 2\n    funciona N\nacabou_finalmente\nmostra f()\nmostra N",
		"crava N = 1\ngambiarra f()\n    N += 10\n    funciona N\nacabou_finalmente\nmostra f()\nmostra N",
		// crava local de gambiarra, chamada varias vezes
		"gambiarra f(x)\n    crava DOBRO = x * 2\n    funciona DOBRO\nacabou_finalmente\nmostra f(1)\nmostra f(2)",
		// crava dentro de laco: o texto e um so
		// (o `K` no fim so iguala o ultimo valor: o pra_cada em si devolve coisas
		// diferentes nos dois engines, e nao e isso que esta em teste aqui)
		"pra_cada i de 1 ate 3\n    crava K = i * 10\n    mostra K\nacabou_finalmente\nK",
		// closure le o cravado de fora
		"crava BASE = 100\nbota soma = gambiarra(x) funciona BASE + x acabou_finalmente\nmostra soma(5)",
		// bota antes, crava depois
		"bota Y = 1\ncrava Y = 2\nmostra Y",
	}
	for _, src := range casos {
		comparaEngines(t, src)
	}
}

// rodaCravaErro roda nos dois engines e devolve os erros. A VM barra na
// compilacao (`linha N: ...`), o tree-walker antes de rodar (`deu ruim na
// linha N: ...`) — o miolo da mensagem tem que ser o mesmo.
func rodaCravaErro(t *testing.T, src string) (string, string, string) {
	t.Helper()
	_, saidaTW, errTW := rodaTWComp(t, src)
	_, _, errVM := rodaVMComp(t, src)
	return errTW, errVM, saidaTW
}

func TestParidadeCravaErro(t *testing.T) {
	casos := []struct {
		src   string
		linha int
		msg   string
	}{
		{"crava PI = 3\nmostra PI\nbota PI = 4", 3, "`PI` foi cravada, nao da pra mudar"},
		{"crava PI = 3\nPI += 1", 2, "`PI` foi cravada, nao da pra mudar"},
		{"crava PI = 3\nPI **= 2", 2, "`PI` foi cravada, nao da pra mudar"},
		{"crava PI = 3\ncrava PI = 4", 2, "`PI` ja foi cravada, nao da pra cravar de novo"},
		{"crava A = 1\nbota [A, b] = [5, 6]", 2, "`A` foi cravada"},
		{"crava I = 0\npra_cada I de 1 ate 3\nacabou_finalmente", 2, "`I` foi cravada"},
		{"crava F = 0\ngambiarra F()\n    funciona 1\nacabou_finalmente", 2, "`F` foi cravada"},
		{"crava N = 1\nse_colar deu_ruim\n    bota N = 2\nacabou_finalmente", 3, "`N` foi cravada"},
		{"gambiarra f()\n    crava L = 1\n    L -= 1\n    funciona L\nacabou_finalmente\nmostra f()", 3, "`L` foi cravada"},
	}
	for _, c := range casos {
		errTW, errVM, saidaTW := rodaCravaErro(t, c.src)
		esperado := fmt.Sprintf("linha %d: %s", c.linha, c.msg)
		if !strings.Contains(errTW, esperado) {
			t.Errorf("TW %q: erro %q, queria %q", c.src, errTW, esperado)
		}
		if !strings.Contains(errVM, esperado) {
			t.Errorf("VM %q: erro %q, queria %q", c.src, errVM, esperado)
		}
		// barra antes de rodar: nada foi impresso
		if saidaTW != "" {
			t.Errorf("TW %q rodou antes de reclamar: %q", c.src, saidaTW)
		}
	}
}

// TestBinConstPotencia: `x ** K` vira OpBinConst — tem que bater com o tree.
func TestBinConstPotencia(t *testing.T) {
	casos := []string{
		"bota i = 7\nmostra i ** 2",
		"bota i = 7\nmostra i ** 0.5",
		"bota i = 7\nmostra i ** -1",
		"bota s = 0\nbota i = 0\nenquanto i < 5\n    bota s = s + i ** 2\n    bota i = i + 1\nacabou_finalmente\nmostra s",
	}
	for _, src := range casos {
		naVM := strings.TrimSpace(rodaFonte(t, src))
		tree := strings.TrimSpace(rodaNoTree(t, src))
		if naVM != tree {
			t.Errorf("divergiu em %q:\n  VM:   %q\n  tree: %q", src, naVM, tree)
		}
	}
}

// TestCravaImporta: o modulo nao mexe no que o importador cravou, e o que o
// modulo crava vale pro importador — nos dois engines.
func TestCravaImporta(t *testing.T) {
	dir := t.TempDir()
	escreve := func(nome, fonte string) {
		if err := os.WriteFile(filepath.Join(dir, nome), []byte(fonte), 0644); err != nil {
			t.Fatal(err)
		}
	}
	escreve("muda.gs", "bota LIMITE = 99")
	escreve("crava.gs", "crava TAXA = 0.1")
	escreve("crava_de_novo.gs", "bota TAXA = 0.2")

	roda := func(src string) (string, string) {
		prog := parser.New(lexer.New(src)).ParseProgram()
		var bufTW bytes.Buffer
		interp := interpreter.New(&bufTW)
		interp.DefinirDirBase(dir)
		errTW := ""
		if e, ok := interp.Eval(prog, object.NewEnvironment()).(*object.Erro); ok {
			errTW = e.Message
		}
		comp := compiler.New()
		comp.DirBase = dir
		errVM := ""
		if err := comp.Compile(prog); err != nil {
			errVM = err.Error()
		}
		return errTW, errVM
	}

	// importador cravou, modulo tenta mudar
	errTW, errVM := roda("crava LIMITE = 10\nimporta \"muda.gs\"")
	for _, e := range []string{errTW, errVM} {
		if !strings.Contains(e, "muda.gs") || !strings.Contains(e, "`LIMITE` foi cravada") {
			t.Errorf("modulo mudando cravada: TW %q / VM %q", errTW, errVM)
			break
		}
	}
	// modulo cravou: o proximo modulo nao muda mais
	errTW, errVM = roda("importa \"crava.gs\"\nimporta \"crava_de_novo.gs\"")
	for _, e := range []string{errTW, errVM} {
		if !strings.Contains(e, "`TAXA` foi cravada") {
			t.Errorf("cravada vinda de modulo: TW %q / VM %q", errTW, errVM)
			break
		}
	}
	// sem conflito: roda limpo
	if errTW, errVM = roda("importa \"crava.gs\"\nmostra TAXA"); errTW != "" || errVM != "" {
		t.Errorf("importa simples deu erro: TW %q / VM %q", errTW, errVM)
	}
}
