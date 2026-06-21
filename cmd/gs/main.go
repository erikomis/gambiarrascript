package main

import (
	"fmt"
	"os"

	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/repl"
)

func main() {
	if len(os.Args) < 2 {
		uso()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "roda":
		if len(os.Args) < 3 {
			fmt.Println("uso: gs roda <arquivo.gs>")
			os.Exit(1)
		}
		rodarArquivo(os.Args[2])
	case "repl":
		fmt.Println("GambiarraScript REPL — manda ver (ctrl+d pra vazar)")
		repl.Start(os.Stdin, os.Stdout)
	case "--version", "-v", "version":
		fmt.Println("gs (GambiarraScript) " + Versao)
	case "--help", "-h", "ajuda":
		uso()
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
}

func rodarArquivo(caminho string) {
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

	interp := interpreter.New(os.Stdout)
	resultado := interp.Eval(prog, object.NewEnvironment())
	if resultado != nil && resultado.Type() == object.ERRO_OBJ {
		fmt.Println(resultado.Inspect())
		os.Exit(1)
	}
}
