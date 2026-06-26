package main

import (
	"fmt"
	"os"
	"path/filepath"

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
	if len(os.Args) < 2 {
		uso()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "roda":
		usarVM := false
		arquivo := ""
		var scriptArgs []string
		proximoEArquivo := true
		for _, a := range os.Args[2:] {
			if a == "--vm" {
				usarVM = true
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
			fmt.Println("uso: gs roda [--vm] <arquivo.gs> [argumentos...]")
			os.Exit(1)
		}
		rodarArquivo(arquivo, usarVM, scriptArgs)
	case "formata":
		if len(os.Args) < 3 {
			fmt.Println("uso: gs formata <arquivo.gs>")
			os.Exit(1)
		}
		formatarArquivo(os.Args[2])
	case "repl":
		fmt.Println("GambiarraScript REPL — manda ver (ctrl+d pra vazar)")
		repl.Start(os.Stdin, os.Stdout)
	case "testa":
		rodarTestes(os.Args[2:])
	case "disasm":
		disassemblar(os.Args[2:])
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
	fmt.Println("  gs roda <arquivo.gs> [argumentos...]   # executa um arquivo")
	fmt.Println("  gs roda --vm <arquivo.gs>              # executa na VM experimental")
	fmt.Println("  gs formata <arquivo.gs>                # formata o arquivo e imprime")
	fmt.Println("  gs repl                                # abre o modo interativo")
	fmt.Println("  gs testa [<dir>]                       # roda os testes (*_test.gs) e soma os asserts")
	fmt.Println("  gs disasm <arquivo.gs>                 # disassembla o bytecode (VM)")
	fmt.Println("  gs lsp                                 # inicia o language server (usado pela extensao do VSCode)")
	fmt.Println("  gs --version                           # mostra a versao")
	fmt.Println("  gs --help                              # mostra esta ajuda")
}

func rodarArquivo(caminho string, usarVM bool, scriptArgs []string) {
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

	if usarVM {
		comp := compiler.New()
		if err := comp.Compile(prog); err != nil {
			fmt.Println("eita, a VM nao compilou: " + err.Error())
			os.Exit(1)
		}
		maquina := vm.New(comp.Bytecode(), os.Stdout)
		if err := maquina.Run(); err != nil {
			fmt.Println("deu ruim na VM: " + err.Error())
			os.Exit(1)
		}
		return
	}

	interp := interpreter.New(os.Stdout)
	interp.DefinirArgumentos(scriptArgs)
	interp.DefinirDirBase(filepath.Dir(caminho))
	resultado := interp.Eval(prog, object.NewEnvironment())
	if resultado != nil && resultado.Type() == object.ERRO_OBJ {
		fmt.Println(resultado.Inspect())
		if err, ok := resultado.(*object.Erro); ok && len(err.Stack) > 0 {
			fmt.Fprint(os.Stderr, "Traço de pilha:\n"+err.Traco())
		}
		os.Exit(1)
	}
}

// rodarTestes procura arquivos *_test.gs (no dir informado, default ".") e
// roda cada um num interpretador fresco. Contabiliza total/ok de asserts
// (espera()/afirma()) somando os contadores do Interpreter, e conta arquivos
// cuja execucao retornou Erro como "com perrengue". Exit 0 sse todos os
// asserts passarem e nenhum arquivo deu Erro.
func rodarTestes(args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	arquivos, err := filepath.Glob(filepath.Join(dir, "*_test.gs"))
	if err != nil {
		fmt.Printf("nao conseguir achar testes: %v\n", err)
		os.Exit(1)
	}
	if len(arquivos) == 0 {
		fmt.Println("nada de *_test.gs aqui, parca. cria um arquivo tipo `meu_test.gs` com `espera(1, 1)`.")
		os.Exit(0)
	}

	totalArqs := len(arquivos)
	totalAsserts, totalOk, falhas := 0, 0, 0
	for _, arq := range arquivos {
		fonte, err := os.ReadFile(arq)
		if err != nil {
			fmt.Printf("%s: nao deu pra ler: %v\n", arq, err)
			falhas++
			continue
		}
		p := parser.New(lexer.New(string(fonte)))
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) != 0 {
			fmt.Printf("%s: perrengue de parse:\n", arq)
			for _, e := range errs {
				fmt.Println("  - " + e)
			}
			falhas++
			continue
		}

		interp := interpreter.New(os.Stdout)
		interp.DefinirDirBase(filepath.Dir(arq))
		interp.ResetTeste()
		res := interp.Eval(prog, object.NewEnvironment())
		esperaTotal, esperaOk := interp.TotaisTeste()
		totalAsserts += esperaTotal
		totalOk += esperaOk

		nota := "OK"
		if res != nil && res.Type() == object.ERRO_OBJ {
			nota = "DEU RUIM: " + res.Inspect()
			falhas++
		} else if esperaTotal > 0 && esperaOk != esperaTotal {
			nota = "FALHA"
			falhas++
		}
		fmt.Printf("  %s  %s  (%d/%d asserts)\n", nota, filepath.Base(arq), esperaOk, esperaTotal)
	}

	fmt.Printf("\nResumo: %d arquivos, %d/%d asserts passaram, %d com perrengue\n",
		totalArqs, totalOk, totalAsserts, falhas)
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
