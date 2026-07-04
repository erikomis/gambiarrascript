package repl

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/token"
)

const (
	prompt     = "gambiarra> "
	promptCont = "......... "
)

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()
	interp := interpreter.New(out)

	var buffer strings.Builder
	for {
		if buffer.Len() == 0 {
			fmt.Fprint(out, prompt)
		} else {
			fmt.Fprint(out, promptCont)
		}
		if !scanner.Scan() {
			return
		}
		linha := scanner.Text()
		buffer.WriteString(linha)
		buffer.WriteString("\n")

		// multiline: enquanto tiver bloco aberto (se_colar/gambiarra/...
		// sem o acabou_finalmente), continua lendo com prompt de continuacao.
		if profundidadeBlocos(buffer.String()) > 0 {
			continue
		}

		fonte := buffer.String()
		buffer.Reset()
		if strings.TrimSpace(fonte) == "" {
			continue
		}

		p := parser.New(lexer.New(fonte))
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) != 0 {
			for _, e := range errs {
				fmt.Fprintln(out, "eita, deu ruim no parse: "+e)
			}
			continue
		}
		resultado := interp.Eval(prog, env)
		if resultado != nil {
			switch resultado.Type() {
			case object.ERRO_OBJ:
				fmt.Fprintln(out, resultado.Inspect())
			case object.NADA_OBJ:
				// nada a mostrar pra bota/se_colar/etc — REPL mais limpo
			default:
				// imprime valor de expressoes (a la Python/Lua) — grande ganho
				// de DX enquando debuga interativamente.
				fmt.Fprintln(out, "=> "+resultado.Inspect())
			}
		}
	}
}

// profundidadeBlocos conta aberturas de bloco (se_colar, enquanto, pra_cada,
// gambiarra, arruma, escolhe) menos os acabou_finalmente. O `se_nao_colar
// se_colar` (elif) NAO abre bloco novo — a cadeia inteira fecha com um so
// acabou_finalmente.
func profundidadeBlocos(src string) int {
	l := lexer.New(src)
	depth := 0
	prev := token.TokenType("")
	for {
		tok := l.NextToken()
		if tok.Type == token.EOF {
			break
		}
		switch tok.Type {
		case token.SE_COLAR:
			if prev != token.SE_NAO_COLAR {
				depth++
			}
		case token.ENQUANTO, token.PRA_CADA, token.GAMBIARRA, token.ARRUMA, token.ESCOLHE:
			depth++
		case token.ACABOU:
			depth--
		}
		prev = tok.Type
	}
	return depth
}
