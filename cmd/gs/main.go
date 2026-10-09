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
			// depois do arquivo, TUDO e do script: `gs roda app.gs --cache`
			// manda --cache pro opcoes()/argumentos(), nao pro gs.
			if arquivo != "" {
				scriptArgs = append(scriptArgs, a)
				continue
			}
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
		pare := iniciaPprof()
		rodarArquivoCache(arquivo, usarVM, usarCache, scriptArgs)
		pare()
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
				fmt.Println("  comentarios e linhas em branco ficam; se a saida nao conferir (AST ou")
				fmt.Println("  comentarios diferentes), nao escreve nada e sai com erro.")
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
		falhou := false
		for _, arq := range alvos {
			if escreverFlag {
				falhou = !formatarArquivoEscrever(arq) || falhou
			} else {
				formatarArquivo(arq)
			}
		}
		if falhou {
			os.Exit(1)
		}
	case "repl":
		repl.Start(os.Stdin, os.Stdout)
	case "doc":
		comandoDoc(os.Args[2:])
	case "testa":
		rodarTestes(os.Args[2:])
	case "disasm":
		disassemblar(os.Args[2:])
	case "debug":
		cmdDebug(os.Args[2:])
	case "check":
		cmdCheck(os.Args[2:])
	case "init":
		cmdInit(os.Args[2:])
	case "bench":
		cmdBench(os.Args[2:])
	case "get":
		cmdGet(os.Args[2:])
	case "instala":
		cmdInstala(os.Args[2:])
	case "build":
		cmdBuild(os.Args[2:])
	case "migra":
		cmdMigra(os.Args[2:])
	case "--version", "-v", "version", "versao":
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
	fmt.Println("  gs bench [--tree] <arquivo.gs> [n]     # mede o tempo de execucao (n rodadas)")
	fmt.Println("  gs get <url | github.com/u/r/x.gs@ref> # baixa um modulo pra gs_modulos/ (+ lock)")
	fmt.Println("  gs instala [--atualiza]                # instala as dependencias, conferindo o gambiarra.lock")
	fmt.Println("  gs build <arquivo.gs> [-o saida]       # gera binario standalone com o script")
	fmt.Println("  gs build <arq.gs> --alvo linux/amd64   # standalone pra outra plataforma (ou --gs-base gs-do-alvo)")
	fmt.Println("  gs migra [sobe|desce [n]|status|novo nome] # migracoes SQL (--banco URL ou $GS_BANCO, --pasta)")
	fmt.Println("  gs repl                                # abre o modo interativo (multiline)")
	fmt.Println("  gs testa [--tree] [-so nome] [<dir>]   # roda os testes (*_test.gs) e soma os asserts")
	fmt.Println("  gs testa --cobertura [<dir>]           # + % de linhas rodadas (--cobertura-perfil/--cobertura-html arq)")
	fmt.Println("  gs doc <arquivo.gs|dir>                # gera markdown com as gambiarras e cravas documentadas")
	fmt.Println("  gs debug [--tree] <arquivo.gs> [args]  # depurador: breakpoints, passo a passo, variaveis")
	fmt.Println("  gs debug --dap                         # depurador no protocolo DAP (usado pela extensao do VSCode)")
	fmt.Println("  gs disasm <arquivo.gs>                 # disassembla o bytecode (VM)")
	fmt.Println("  gs lsp                                 # inicia o language server (usado pela extensao do VSCode)")
	fmt.Println("  gs --version  (ou gs versao)           # mostra a versao")
	fmt.Println("  gs --help                              # mostra esta ajuda")
}

// rodaNaVM executa o bytecode na VM com o interpretador JA configurado
// (argumentos do script + diretorio base). Antes cada call site chamava
// vm.New, que monta um interpreter proprio e vazio — por isso `argumentos()`
// voltava lista vazia na VM, que e o engine padrao.
func rodaNaVM(bc *compiler.Bytecode, dirBase string, scriptArgs []string) {
	interp := interpreter.New(os.Stdout)
	interp.DefinirArgumentos(scriptArgs)
	interp.DefinirDirBase(dirBase)
	maquina := vm.NovaComInterp(bc, os.Stdout, interp)
	if err := maquina.Run(); err != nil {
		trataSaiVM(err)
		reportaErroVM(err)
		os.Exit(1)
	}
	esperaAgendamentos(interp)
}

// esperaAgendamentos segura o processo enquanto o script tiver a_cada,
// depois_de ou agenda de pe (ctrl+c para com calma). Se uma tarefa chamou
// sai(codigo), sai com esse codigo.
func esperaAgendamentos(interp *interpreter.Interpreter) {
	if s := interp.EsperaAgendamentos(); s != nil {
		os.Exit(s.Codigo)
	}
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
			rodaNaVM(bc, filepath.Dir(caminho), scriptArgs)
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
		comp.Arquivo = caminho
		if err := comp.Compile(prog); err != nil {
			fmt.Println("eita, teu codigo tem um perrengue:")
			fmt.Println("  - " + err.Error())
			os.Exit(1)
		}
		if usarCache {
			gravaCache(strings.TrimSuffix(caminho, ".gs")+".gsc", fonte, comp.Bytecode())
		}
		rodaNaVM(comp.Bytecode(), filepath.Dir(caminho), scriptArgs)
		return
	}

	interp := interpreter.New(os.Stdout)
	interp.DefinirArgumentos(scriptArgs)
	interp.DefinirArquivo(caminho)
	resultado := interp.Eval(prog, object.NewEnvironment())
	if s, ok := resultado.(*object.Sair); ok {
		os.Exit(s.Codigo) // sai(codigo) no tree-walker
	}
	if object.EhErroLevantado(resultado) {
		fmt.Println(resultado.Inspect())
		if err, ok := resultado.(*object.Erro); ok && len(err.Stack) > 0 {
			fmt.Fprint(os.Stderr, "Traço de pilha:\n"+err.Traco())
		}
		os.Exit(1)
	}
	esperaAgendamentos(interp)
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
	saida, err := formataConferido(string(fonte), formatter.FormataFonte)
	if err != nil {
		mostraErroFormata(caminho, err)
		os.Exit(1)
	}
	fmt.Print(saida)
}

// formatarArquivoEscrever formata o arquivo e sobrescreve no disco se (e
// somente se) algo mudou e a saida passou na trava. Devolve false se falhou.
func formatarArquivoEscrever(caminho string) bool {
	mudou, err := escreveFormatado(caminho, formatter.FormataFonte)
	switch {
	case err != nil:
		mostraErroFormata(caminho, err)
		return false
	case mudou:
		fmt.Printf("  %s  (formatado)\n", filepath.Base(caminho))
	default:
		fmt.Printf("  %s  (sem mudanca)\n", filepath.Base(caminho))
	}
	return true
}

func mostraErroFormata(caminho string, err error) {
	if errs, ok := err.(errParse); ok {
		fmt.Printf("eita, %s tem uns perrengue:\n", filepath.Base(caminho))
		for _, e := range errs {
			fmt.Println("  - " + e)
		}
		return
	}
	fmt.Printf("  %s  NAO formatado, o arquivo ficou como estava: %v (bug do formatter, avisa a gente)\n",
		filepath.Base(caminho), err)
}
