package main

import (
	"fmt"
	"os"

	"gambiarrascript/compiler"
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
		for _, a := range os.Args[2:] {
			if a == "--vm" {
				usarVM = true
			} else {
				arquivo = a
			}
		}
		if arquivo == "" {
			fmt.Println("uso: gs roda [--vm] <arquivo.gs>")
			os.Exit(1)
		}
		rodarArquivo(arquivo, usarVM)
	case "repl":
		fmt.Println("GambiarraScript REPL — manda ver (ctrl+d pra vazar)")
		repl.Start(os.Stdin, os.Stdout)
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
	fmt.Println("  gs roda <arquivo.gs>   # executa um arquivo")
	fmt.Println("  gs repl                # abre o modo interativo")
	fmt.Println("  gs lsp                 # inicia o language server (usado pela extensao do VSCode)")
	fmt.Println("  gs --version           # mostra a versao")
	fmt.Println("  gs --help              # mostra esta ajuda")
}

func rodarArquivo(caminho string, usarVM bool) {
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
	resultado := interp.Eval(prog, object.NewEnvironment())
	if resultado != nil && resultado.Type() == object.ERRO_OBJ {
		fmt.Println(resultado.Inspect())
		os.Exit(1)
	}
}
