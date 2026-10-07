package main

import (
	"path/filepath"
	"strings"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/vm"

	"os"
)

// parseArgsTesta le as flags do `gs testa`: `--tree` (volta pro tree-walker),
// `-so <nome>` (só arquivos cujo nome casa) e o diretório posicional
// (default "."). A VM e o padrao — a suite tem que validar o engine que roda
// em producao, nao o fallback.
func parseArgsTesta(args []string) (dir string, usarVM bool, filtro string) {
	dir = "."
	usarVM = true
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--vm":
			usarVM = true
		case "--tree":
			usarVM = false
		case "-so", "--somente":
			if i+1 < len(args) {
				filtro = args[i+1]
				i++
			}
		default:
			dir = args[i]
		}
	}
	return
}

// filtraTestes mantém só os arquivos cujo basename contém `filtro` (vazio =
// todos).
func filtraTestes(arqs []string, filtro string) []string {
	if filtro == "" {
		return arqs
	}
	var out []string
	for _, a := range arqs {
		if strings.Contains(filepath.Base(a), filtro) {
			out = append(out, a)
		}
	}
	return out
}

// rodaUmTeste roda um arquivo de teste no engine escolhido e devolve
// (asserts totais, asserts ok, nota). A nota é "OK", "FALHA" ou uma mensagem
// de erro. Os contadores vêm de espera()/afirma() via interp.TotaisTeste() —
// na VM reusamos o mesmo interp (NovaComInterp) pra ler esses contadores.
func rodaUmTeste(arq string, usarVM bool) (total, ok int, nota string) {
	fonte, err := os.ReadFile(arq)
	if err != nil {
		return 0, 0, "nao deu pra ler: " + err.Error()
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return 0, 0, "perrengue de parse: " + strings.Join(errs, "; ")
	}

	interp := interpreter.New(os.Stdout)
	interp.DefinirArquivo(arq)
	interp.ResetTeste()

	var runErro object.Object
	if usarVM {
		comp := compiler.New()
		comp.Arquivo = arq
		if err := comp.Compile(prog); err != nil {
			return 0, 0, "nao compilou pra VM: " + err.Error()
		}
		maq := vm.NovaComInterp(comp.Bytecode(), os.Stdout, interp)
		if err := maq.Run(); err != nil {
			runErro = &object.Erro{Message: err.Error(), Kind: "runtime"}
		}
	} else {
		res := interp.Eval(prog, object.NewEnvironment())
		if object.EhErroLevantado(res) {
			runErro = res
		}
	}

	total, ok = interp.TotaisTeste()
	if runErro != nil {
		return total, ok, "DEU RUIM: " + runErro.Inspect()
	}
	if total > 0 && ok != total {
		return total, ok, "FALHA"
	}
	return total, ok, "OK"
}
