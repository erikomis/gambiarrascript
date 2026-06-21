package repl

import (
	"bufio"
	"fmt"
	"io"

	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

const prompt = "gambiarra> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()
	interp := interpreter.New(out)

	for {
		fmt.Fprint(out, prompt)
		if !scanner.Scan() {
			return
		}
		linha := scanner.Text()

		p := parser.New(lexer.New(linha))
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) != 0 {
			for _, e := range errs {
				fmt.Fprintln(out, "eita, deu ruim no parse: "+e)
			}
			continue
		}
		resultado := interp.Eval(prog, env)
		if resultado != nil && resultado.Type() == object.ERRO_OBJ {
			fmt.Fprintln(out, resultado.Inspect())
		}
	}
}
