package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/token"
	"gambiarrascript/vm"
)

const (
	prompt     = "gambiarra> "
	promptCont = "......... "
)

// Start sobe o REPL. Se a entrada e um terminal de verdade, usa o modo rico
// (historico com setas, autocomplete no TAB, :ajuda/:limpa) via x/term; senao
// (pipe, teste) cai no modo simples linha-a-linha.
func Start(in io.Reader, out io.Writer) {
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if startRico(f) {
			return
		}
	}
	startSimples(in, out)
}

// avalia parseia e roda uma fonte completa na sessao da VM, imprimindo erros
// ou `=> valor`. Roda na VM (nao no tree-walker) pra o REPL se comportar igual
// ao `gs roda` — antes dava pra uma construcao funcionar aqui e falhar la.
func avalia(s *vm.Sessao, fonte string, out io.Writer) {
	p := parser.New(lexer.New(fonte))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		for _, e := range errs {
			fmt.Fprintln(out, "eita, deu ruim no parse: "+e)
		}
		return
	}
	valor, err := s.Avalia(prog)
	if err != nil {
		if e := vm.ErroDoRun(err); e != nil {
			fmt.Fprintln(out, e.Inspect())
		} else {
			fmt.Fprintln(out, "eita, deu ruim: "+err.Error())
		}
		return
	}
	// so expressao tem valor pra mostrar, e `nada` nao vira `=> nada`:
	// comando como `ordena(xs)` nao suja a tela.
	if valor != nil && valor.Type() != object.NADA_OBJ {
		fmt.Fprintln(out, "=> "+valor.Inspect())
	}
}

// startSimples e o REPL linha-a-linha (fallback pra pipes/testes, sem readline).
func startSimples(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	sessao := vm.NovaSessao(out)

	fmt.Fprintln(out, "GambiarraScript REPL — manda ver (:ajuda pra comandos, ctrl+d pra vazar)")
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
		if buffer.Len() == 0 && strings.HasPrefix(strings.TrimSpace(linha), ":") {
			trataComando(linha, out)
			continue
		}
		buffer.WriteString(linha)
		buffer.WriteString("\n")

		if profundidadeBlocos(buffer.String()) > 0 {
			continue
		}
		fonte := buffer.String()
		buffer.Reset()
		if strings.TrimSpace(fonte) == "" {
			continue
		}
		avalia(sessao, fonte, out)
	}
}

// crlfWriter traduz \n -> \r\n: no modo raw do terminal a saida do programa
// (mostra/=>) escadearia sem o \r.
type crlfWriter struct{ w io.Writer }

func (c *crlfWriter) Write(p []byte) (int, error) {
	if _, err := c.w.Write([]byte(strings.ReplaceAll(string(p), "\n", "\r\n"))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// startRico usa x/term pra dar readline (historico, edicao, setas) + TAB
// autocomplete. Devolve false se nao conseguir por o terminal em modo raw
// (ai o Start cai no modo simples).
func startRico(stdin *os.File) bool {
	fd := int(stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return false
	}
	defer term.Restore(fd, old)

	saida := &crlfWriter{w: os.Stdout}
	rw := struct {
		io.Reader
		io.Writer
	}{stdin, os.Stdout}
	t := term.NewTerminal(rw, prompt)

	sessao := vm.NovaSessao(saida)

	t.AutoCompleteCallback = func(linha string, pos int, key rune) (string, int, bool) {
		if key != '\t' {
			return "", 0, false
		}
		return autocompleta(linha, pos, nomesCompletaveis(sessao))
	}

	fmt.Fprint(saida, "GambiarraScript REPL — ↑/↓ historico, TAB completa, :ajuda, ctrl+d pra vazar\n")
	var buffer strings.Builder
	for {
		if buffer.Len() == 0 {
			t.SetPrompt(prompt)
		} else {
			t.SetPrompt(promptCont)
		}
		linha, err := t.ReadLine()
		if err != nil { // io.EOF (ctrl+d) ou erro de leitura
			break
		}
		if buffer.Len() == 0 && strings.HasPrefix(strings.TrimSpace(linha), ":") {
			trataComando(linha, saida)
			continue
		}
		buffer.WriteString(linha)
		buffer.WriteString("\n")
		if profundidadeBlocos(buffer.String()) > 0 {
			continue
		}
		fonte := buffer.String()
		buffer.Reset()
		if strings.TrimSpace(fonte) == "" {
			continue
		}
		avalia(sessao, fonte, saida)
	}
	return true
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
