package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gambiarrascript/compiler"
	"gambiarrascript/formatter"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/lsp"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/repl"
	"gambiarrascript/vm"
)

func main() {
	// binario gerado pelo `gs build`? roda o script embedado e pronto.
	if rodarEmbedado() {
		return
	}
	if len(os.Args) < 2 {
		uso()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "roda":
		usarVM := true // VM e o engine padrao (Tier 7); use --tree pro tree-walker
		usarCache := false
		arquivo := ""
		var scriptArgs []string
		proximoEArquivo := true
		for _, a := range os.Args[2:] {
			if a == "--vm" { // aceito por compatibilidade; a VM ja e o padrao
				usarVM = true
				continue
			}
			if a == "--tree" { // fallback pro tree-walker (interpretador)
				usarVM = false
				continue
			}
			if a == "--cache" {
				usarCache = true
				continue
			}
			if proximoEArquivo && arquivo == "" {
				arquivo = a
				proximoEArquivo = false
				continue
			}
			scriptArgs = append(scriptArgs, a)
		}
		if arquivo == "" {
			fmt.Println("uso: gs roda [--tree] [--cache] <arquivo.gs> [argumentos...]")
			os.Exit(1)
		}
		if usarCache && !usarVM {
			fmt.Println("--cache nao se aplica com --tree (bytecode e so da VM); ignorando")
			usarCache = false
		}
		rodarArquivoCache(arquivo, usarVM, usarCache, scriptArgs)
	case "formata":
		args := os.Args[2:]
		escreverFlag := false
		var arquivos []string
		for _, a := range args {
			if a == "-w" || a == "--write" {
				escreverFlag = true
				continue
			}
			if a == "-h" || a == "--help" {
				fmt.Println("uso: gs formata [-w|--write] <arquivo.gs | diretorio>...")
				fmt.Println("  sem flag: imprime no stdout ( FORMATADO).")
				fmt.Println("  -w / --write: sobrescreve cada arquivo com a versao formatada.")
				fmt.Println("  diretorio: varre recursivamente todos os .gs (ex.: gs formata -w .).")
				os.Exit(0)
			}
			arquivos = append(arquivos, a)
		}
		if len(arquivos) == 0 {
			fmt.Println("uso: gs formata [-w|--write] <arquivo.gs | diretorio>...")
			os.Exit(1)
		}
		alvos, err := coletaArquivosGs(arquivos)
		if err != nil {
			fmt.Printf("nao consegui listar os arquivos: %v\n", err)
			os.Exit(1)
		}
		if len(alvos) == 0 {
			fmt.Println("nenhum arquivo .gs encontrado")
			os.Exit(1)
		}
		for _, arq := range alvos {
			if escreverFlag {
				formatarArquivoEscrever(arq)
			} else {
				formatarArquivo(arq)
			}
		}
	case "repl":
		repl.Start(os.Stdin, os.Stdout)
	case "doc":
		comandoDoc(os.Args[2:])
	case "testa":
		rodarTestes(os.Args[2:])
	case "disasm":
		disassemblar(os.Args[2:])
	case "check":
		cmdCheck(os.Args[2:])
	case "init":
		cmdInit(os.Args[2:])
	case "bench":
		cmdBench(os.Args[2:])
	case "get":
		cmdGet(os.Args[2:])
	case "build":
		cmdBuild(os.Args[2:])
	case "--version", "-v", "version":
		fmt.Println("gs (GambiarraScript) " + Versao)
	case "--help", "-h", "ajuda":
		uso()
	case "lsp":
		if err := lsp.NovoServidor(os.Stdout).Rodar(os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, "lsp: "+err.Error())
			os.Exit(1)
		}
	default:
		uso()
		os.Exit(1)
	}
}

func uso() {
	fmt.Println("GambiarraScript")
	fmt.Println("uso:")
	fmt.Println("  gs roda [--cache] <arquivo.gs> [args]   # executa na VM (padrao; --cache grava/usa .gsc)")
	fmt.Println("  gs roda --tree <arquivo.gs>            # executa no tree-walker (fallback)")
	fmt.Println("  gs formata <arquivo.gs>                # formata o arquivo e imprime")
	fmt.Println("  gs formata -w <arquivo.gs>...         # formata e sobrescreve no disco")
	fmt.Println("  gs check <arquivo.gs>...               # parse + lint (erros e avisos)")
	fmt.Println("  gs init [nome]                         # cria gambiarra.json + principal.gs")
	fmt.Println("  gs bench [--vm] <arquivo.gs> [n]       # mede o tempo de execucao (n rodadas)")
	fmt.Println("  gs get <url> [nome.gs]                 # baixa um modulo pra gs_modulos/")
	fmt.Println("  gs build <arquivo.gs> [-o saida]       # gera binario standalone com o script")
	fmt.Println("  gs repl                                # abre o modo interativo (multiline)")
	fmt.Println("  gs testa [<dir>]                       # roda os testes (*_test.gs) e soma os asserts")
	fmt.Println("  gs disasm <arquivo.gs>                 # disassembla o bytecode (VM)")
	fmt.Println("  gs lsp                                 # inicia o language server (usado pela extensao do VSCode)")
	fmt.Println("  gs --version                           # mostra a versao")
	fmt.Println("  gs --help                              # mostra esta ajuda")
}

func rodarArquivoCache(caminho string, usarVM, usarCache bool, scriptArgs []string) {
	fonte, err := os.ReadFile(caminho)
	if err != nil {
		fmt.Printf("nao consegui abrir %q: %v\n", caminho, err)
		os.Exit(1)
	}

	// caminho rapido: cache de bytecode valido dispensa parse+compile
	if usarVM && usarCache {
		caminhoGSC := strings.TrimSuffix(caminho, ".gs") + ".gsc"
		if bc := carregaCache(caminhoGSC, fonte); bc != nil {
			maquina := vm.New(bc, os.Stdout)
			if err := maquina.Run(); err != nil {
				trataSaiVM(err)
				reportaErroVM(err)
				os.Exit(1)
			}
			return
		}
	}

	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		fmt.Println("eita, teu codigo tem uns perrengue:")
		for _, e := range errs {
			fmt.Println("  - " + e)
		}
		os.Exit(1)
	}

	if usarVM {
		comp := compiler.New()
		comp.DirBase = filepath.Dir(caminho)
		if err := comp.Compile(prog); err != nil {
			fmt.Println("eita, a VM nao compilou: " + err.Error())
			os.Exit(1)
		}
		if usarCache {
			gravaCache(strings.TrimSuffix(caminho, ".gs")+".gsc", fonte, comp.Bytecode())
		}
		maquina := vm.New(comp.Bytecode(), os.Stdout)
		if err := maquina.Run(); err != nil {
			trataSaiVM(err)
			reportaErroVM(err)
			os.Exit(1)
		}
		return
	}

	interp := interpreter.New(os.Stdout)
	interp.DefinirArgumentos(scriptArgs)
	interp.DefinirDirBase(filepath.Dir(caminho))
	resultado := interp.Eval(prog, object.NewEnvironment())
	if s, ok := resultado.(*object.Sair); ok {
		os.Exit(s.Codigo) // sai(codigo) no tree-walker
	}
	if resultado != nil && resultado.Type() == object.ERRO_OBJ {
		fmt.Println(resultado.Inspect())
		if err, ok := resultado.(*object.Erro); ok && len(err.Stack) > 0 {
			fmt.Fprint(os.Stderr, "Traço de pilha:\n"+err.Traco())
		}
		os.Exit(1)
	}
}

// trataSaiVM: quando o Run da VM devolve um sai(codigo), encerra o processo
// com esse codigo (nao e erro). Se nao for, nao faz nada.
func trataSaiVM(err error) {
	if sr, ok := err.(vm.SaiRequisicao); ok {
		os.Exit(sr.Codigo)
	}
}

// reportaErroVM imprime o erro de runtime da VM no mesmo formato do
// tree-walker: mensagem (ja com "deu ruim na linha N") no stdout + traço de
// pilha no stderr quando houver.
func reportaErroVM(err error) {
	fmt.Println(err.Error())
	if eo := vm.ErroDoRun(err); eo != nil && len(eo.Stack) > 0 {
		fmt.Fprint(os.Stderr, "Traço de pilha:\n"+eo.Traco())
	}
}

// rodarTestes procura arquivos *_test.gs (no dir informado, default ".") e
// roda cada um num interpretador fresco. Contabiliza total/ok de asserts
// (espera()/afirma()) somando os contadores do Interpreter, e conta arquivos
// cuja execucao retornou Erro como "com perrengue". Exit 0 sse todos os
// asserts passarem e nenhum arquivo deu Erro.
func rodarTestes(args []string) {
	dir, usarVM, filtro := parseArgsTesta(args)
	arquivos, err := filepath.Glob(filepath.Join(dir, "*_test.gs"))
	if err != nil {
		fmt.Printf("nao conseguir achar testes: %v\n", err)
		os.Exit(1)
	}
	arquivos = filtraTestes(arquivos, filtro)
	if len(arquivos) == 0 {
		if filtro != "" {
			fmt.Printf("nenhum *_test.gs casa com %q em %s\n", filtro, dir)
		} else {
			fmt.Println("nada de *_test.gs aqui, parca. cria um arquivo tipo `meu_test.gs` com `espera(1, 1)`.")
		}
		os.Exit(0)
	}

	engine := "tree-walker"
	if usarVM {
		engine = "VM"
	}
	fmt.Printf("rodando %d arquivo(s) no %s:\n", len(arquivos), engine)

	totalAsserts, totalOk, falhas := 0, 0, 0
	for _, arq := range arquivos {
		total, ok, nota := rodaUmTeste(arq, usarVM)
		totalAsserts += total
		totalOk += ok
		if nota != "OK" {
			falhas++
		}
		fmt.Printf("  %s  %s  (%d/%d asserts)\n", nota, filepath.Base(arq), ok, total)
	}

	fmt.Printf("\nResumo (%s): %d arquivos, %d/%d asserts passaram, %d com perrengue\n",
		engine, len(arquivos), totalOk, totalAsserts, falhas)
	if falhas > 0 || totalAsserts != totalOk {
		os.Exit(1)
	}
}

// disassemblar monta o bytecode do arquivo e imprime o disassembly. Usa o
// compilador da VM. Util pra depurar o que a VM esta realmente enxergando.
func disassemblar(args []string) {
	if len(args) < 1 {
		fmt.Println("uso: gs disasm <arquivo.gs>")
		os.Exit(1)
	}
	fonte, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("nao consegui abrir %q: %v\n", args[0], err)
		os.Exit(1)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		fmt.Println("eita, perrengue de parse:")
		for _, e := range errs {
			fmt.Println("  - " + e)
		}
		os.Exit(1)
	}
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		fmt.Println("a VM nao consegue compilar isso: " + err.Error())
		os.Exit(1)
	}
	fmt.Print(comp.Bytecode().Instructions.String())
}

func formatarArquivo(caminho string) {
	fonte, err := os.ReadFile(caminho)
	if err != nil {
		fmt.Printf("nao consegui abrir %q: %v\n", caminho, err)
		os.Exit(1)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		fmt.Println("eita, teu codigo tem uns perrengue:")
		for _, e := range errs {
			fmt.Println("  - " + e)
		}
		os.Exit(1)
	}
	fmt.Print(formatter.Formata(prog))
}

// formatarArquivoEscrever formata o arquivo e sobrescreve no disco se (e
// somente se) algo mudou. Informa no stdout a acao tomada.
func formatarArquivoEscrever(caminho string) {
	fonte, err := os.ReadFile(caminho)
	if err != nil {
		fmt.Printf("nao consegui abrir %q: %v\n", caminho, err)
		os.Exit(1)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		fmt.Println("eita, teu codigo tem uns perrengue:")
		for _, e := range errs {
			fmt.Println("  - " + e)
		}
		os.Exit(1)
	}
	formatado := formatter.Formata(prog)
	// adiciona quebra de linha final se o original tinha (preserva)
	if len(fonte) > 0 && fonte[len(fonte)-1] == '\n' && !strings.HasSuffix(formatado, "\n") {
		formatado += "\n"
	}
	if formatado == string(fonte) {
		fmt.Printf("  %s  (sem mudanca)\n", filepath.Base(caminho))
		return
	}
	if err := os.WriteFile(caminho, []byte(formatado), 0644); err != nil {
		fmt.Printf("nao consegui sobrescrever %q: %v\n", caminho, err)
		os.Exit(1)
	}
	fmt.Printf("  %s  (formatado)\n", filepath.Base(caminho))
}
