package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gambiarrascript/cobertura"
	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/vm"
)

// opcoesTesta sao as flags do `gs testa`.
type opcoesTesta struct {
	dir    string // default "."
	usarVM bool   // a VM e o padrao; --tree volta pro tree-walker
	filtro string // -so <nome>: so arquivos cujo nome casa

	// cobertura de linhas: --cobertura liga o resumo; --cobertura-perfil e
	// --cobertura-html gravam o perfil (arquivo:linha contagem) e o HTML, e
	// ligam a cobertura sozinhos.
	cobertura bool
	perfil    string
	html      string
}

// parseArgsTesta le as flags do `gs testa`: `--tree` (volta pro tree-walker),
// `-so <nome>` (só arquivos cujo nome casa), `--cobertura`,
// `--cobertura-perfil <arq>`, `--cobertura-html <arq>` (tambem com `=arq`) e
// o diretório posicional (default "."). A VM e o padrao — a suite tem que
// validar o engine que roda em producao, nao o fallback.
func parseArgsTesta(args []string) (opcoesTesta, error) {
	op := opcoesTesta{dir: ".", usarVM: true}
	valor := func(i *int, flag string) (string, error) {
		if pre := flag + "="; strings.HasPrefix(args[*i], pre) {
			return strings.TrimPrefix(args[*i], pre), nil
		}
		if *i+1 >= len(args) {
			return "", fmt.Errorf("%s quer o nome do arquivo", flag)
		}
		*i++
		return args[*i], nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--vm":
			op.usarVM = true
		case a == "--tree":
			op.usarVM = false
		case a == "-so" || a == "--somente":
			if i+1 < len(args) {
				op.filtro = args[i+1]
				i++
			}
		case a == "--cobertura":
			op.cobertura = true
		case a == "--cobertura-perfil" || strings.HasPrefix(a, "--cobertura-perfil="):
			v, err := valor(&i, "--cobertura-perfil")
			if err != nil {
				return op, err
			}
			op.perfil, op.cobertura = v, true
		case a == "--cobertura-html" || strings.HasPrefix(a, "--cobertura-html="):
			v, err := valor(&i, "--cobertura-html")
			if err != nil {
				return op, err
			}
			op.html, op.cobertura = v, true
		case strings.HasPrefix(a, "--"):
			return op, fmt.Errorf("flag desconhecida: %s", a)
		default:
			op.dir = a
		}
	}
	return op, nil
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

// rodarTestes procura arquivos *_test.gs (no dir informado, default ".") e
// roda cada um num interpretador fresco. Contabiliza total/ok de asserts
// (espera()/afirma()) somando os contadores do Interpreter, e conta arquivos
// cuja execucao retornou Erro como "com perrengue". Exit 0 sse todos os
// asserts passarem e nenhum arquivo deu Erro (a cobertura nunca muda o exit).
func rodarTestes(args []string) {
	op, err := parseArgsTesta(args)
	if err != nil {
		fmt.Println("gs testa: " + err.Error())
		os.Exit(2)
	}
	if !executaTestes(op, os.Stdout) {
		os.Exit(1)
	}
}

// executaTestes roda a suite e escreve o relatorio em w. Devolve false se
// algum assert falhou ou algum arquivo deu perrengue.
func executaTestes(op opcoesTesta, w io.Writer) bool {
	arquivos, err := filepath.Glob(filepath.Join(op.dir, "*_test.gs"))
	if err != nil {
		fmt.Fprintf(w, "nao conseguir achar testes: %v\n", err)
		return false
	}
	arquivos = filtraTestes(arquivos, op.filtro)
	if len(arquivos) == 0 {
		if op.filtro != "" {
			fmt.Fprintf(w, "nenhum *_test.gs casa com %q em %s\n", op.filtro, op.dir)
		} else {
			fmt.Fprintln(w, "nada de *_test.gs aqui, parca. cria um arquivo tipo `meu_test.gs` com `espera(1, 1)`.")
		}
		return true
	}

	engine := "tree-walker"
	if op.usarVM {
		engine = "VM"
	}
	fmt.Fprintf(w, "rodando %d arquivo(s) no %s:\n", len(arquivos), engine)

	var col *cobertura.Coletor
	var gancho object.GanchoLinha
	if op.cobertura {
		col = cobertura.Novo()
		gancho = col.Gancho()
	}

	totalAsserts, totalOk, falhas := 0, 0, 0
	for _, arq := range arquivos {
		total, ok, nota := rodaUmTeste(arq, op.usarVM, w, gancho)
		totalAsserts += total
		totalOk += ok
		if nota != "OK" {
			falhas++
		}
		fmt.Fprintf(w, "  %s  %s  (%d/%d asserts)\n", nota, filepath.Base(arq), ok, total)
	}

	fmt.Fprintf(w, "\nResumo (%s): %d arquivos, %d/%d asserts passaram, %d com perrengue\n",
		engine, len(arquivos), totalOk, totalAsserts, falhas)
	if col != nil {
		fmt.Fprintln(w)
		if err := relataCobertura(col, op, engine, w); err != nil {
			fmt.Fprintln(w, err.Error())
		}
	}
	return falhas == 0 && totalAsserts == totalOk
}

// relataCobertura monta o relatorio (arquivos que rodaram + os .gs do
// diretorio testado que nenhum teste tocou, tudo menos *_test.gs e o que mora
// em gs_modulos/ — dependencia nao e codigo teu) e escreve o resumo, o perfil
// e o HTML pedidos.
func relataCobertura(col *cobertura.Coletor, op opcoesTesta, engine string, w io.Writer) error {
	todos, _ := filepath.Glob(filepath.Join(op.dir, "*.gs"))
	inclui := func(caminho string) bool {
		if strings.HasSuffix(caminho, "_test.gs") {
			return false
		}
		for _, parte := range strings.Split(filepath.ToSlash(caminho), "/") {
			if parte == "gs_modulos" {
				return false
			}
		}
		return true
	}
	rel, err := col.Relatorio(todos, inclui)
	if err != nil {
		return err
	}
	base, _ := os.Getwd()
	rel.EscreveResumo(w, engine, base)
	if op.perfil != "" {
		if err := gravaArquivo(op.perfil, func(f io.Writer) error { return rel.EscrevePerfil(f, base) }); err != nil {
			return err
		}
		fmt.Fprintf(w, "perfil gravado em %s\n", op.perfil)
	}
	if op.html != "" {
		if err := gravaArquivo(op.html, func(f io.Writer) error { return rel.EscreveHTML(f, engine, base) }); err != nil {
			return err
		}
		fmt.Fprintf(w, "relatorio HTML gravado em %s\n", op.html)
	}
	return nil
}

func gravaArquivo(caminho string, escreve func(io.Writer) error) error {
	f, err := os.Create(caminho)
	if err != nil {
		return fmt.Errorf("cobertura: nao consegui criar %s: %v", caminho, err)
	}
	if err := escreve(f); err != nil {
		f.Close()
		return fmt.Errorf("cobertura: nao consegui gravar %s: %v", caminho, err)
	}
	return f.Close()
}

// rodaUmTeste roda um arquivo de teste no engine escolhido e devolve
// (asserts totais, asserts ok, nota). A nota é "OK", "FALHA" ou uma mensagem
// de erro. Os contadores vêm de espera()/afirma() via interp.TotaisTeste() —
// na VM reusamos o mesmo interp (NovaComInterp) pra ler esses contadores.
// Com gancho != nil roda instrumentado (cobertura): na VM o bytecode sai com
// OpLinha (e nunca vai pro cache .gsc).
func rodaUmTeste(arq string, usarVM bool, out io.Writer, gancho object.GanchoLinha) (total, ok int, nota string) {
	fonte, err := os.ReadFile(arq)
	if err != nil {
		return 0, 0, "nao deu pra ler: " + err.Error()
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return 0, 0, "perrengue de parse: " + strings.Join(errs, "; ")
	}

	interp := interpreter.New(out)
	interp.DefinirArquivo(arq)
	interp.ResetTeste()

	var runErro object.Object
	if usarVM {
		comp := compiler.New()
		comp.Arquivo = arq
		comp.Instrumentar = gancho != nil
		if err := comp.Compile(prog); err != nil {
			return 0, 0, "nao compilou pra VM: " + err.Error()
		}
		maq := vm.NovaComInterp(comp.Bytecode(), out, interp)
		maq.DefinirGancho(gancho)
		if err := maq.Run(); err != nil {
			runErro = &object.Erro{Message: err.Error(), Kind: "runtime"}
		}
	} else {
		interp.DefinirGancho(gancho)
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
