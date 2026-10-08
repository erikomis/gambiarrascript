package depurador

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
	"gambiarrascript/vm"
)

// Config descreve o programa a depurar.
type Config struct {
	Arquivo string
	Args    []string
	Motor   string // "vm" (padrao) ou "tree"
	Saida   io.Writer
	Erro    io.Writer
	Entrada io.Reader
}

// Programa e o .gs ja parseado (e compilado, na VM) pronto pra rodar.
type Programa struct {
	cfg     Config
	arquivo string // absoluto
	tree    bool
	bc      *compiler.Bytecode
	prog    *ast.Program // so no tree
}

// Prepara le, parseia e (na VM) compila com instrumentacao. Erro de parse ou
// de compilacao volta aqui, antes de rodar.
func Prepara(cfg Config) (*Programa, error) {
	if cfg.Saida == nil {
		cfg.Saida = os.Stdout
	}
	if cfg.Erro == nil {
		cfg.Erro = os.Stderr
	}
	if cfg.Entrada == nil {
		cfg.Entrada = os.Stdin
	}
	abs, err := filepath.Abs(cfg.Arquivo)
	if err != nil {
		return nil, err
	}
	fonte, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("nao consegui abrir %q: %v", cfg.Arquivo, err)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return nil, fmt.Errorf("eita, teu codigo tem uns perrengue:\n  - %s", strings.Join(errs, "\n  - "))
	}
	pr := &Programa{cfg: cfg, arquivo: abs}
	switch cfg.Motor {
	case "", "vm":
		comp := compiler.New()
		comp.Arquivo = abs
		comp.Instrumentar = true
		if err := comp.Compile(prog); err != nil {
			return nil, fmt.Errorf("eita, teu codigo tem um perrengue:\n  - %s", err.Error())
		}
		pr.bc = comp.Bytecode()
		registraInfos(pr.bc.Constants)
	case "tree":
		pr.tree = true
		pr.prog = prog
	default:
		return nil, fmt.Errorf("engine %q nao existe (use vm ou tree)", cfg.Motor)
	}
	return pr, nil
}

// Arquivo devolve o caminho absoluto do programa.
func (p *Programa) Arquivo() string { return p.arquivo }

// Roda executa o programa com o depurador ligado e devolve o codigo de saida
// (sai(n) -> n; erro nao pego -> 1, com a mensagem e o traco no Erro).
func (p *Programa) Roda(d object.Depurador) int {
	cfg := p.cfg
	interp := interpreter.New(cfg.Saida)
	interp.DefinirArgumentos(cfg.Args)
	interp.DefinirStderr(cfg.Erro)
	interp.DefinirStdin(cfg.Entrada)
	if p.tree {
		interp.DefinirArquivo(p.arquivo)
		interp.DefinirDepurador(d)
		res := interp.Eval(p.prog, object.NewEnvironment())
		if s, ok := res.(*object.Sair); ok {
			return s.Codigo
		}
		if object.EhErroLevantado(res) {
			p.reportaErro(res.Inspect(), res.(*object.Erro))
			return 1
		}
		return 0
	}
	interp.DefinirDirBase(filepath.Dir(p.arquivo))
	m := vm.NovaComInterp(p.bc, cfg.Saida, interp)
	m.DefinirDepurador(d)
	if err := m.Run(); err != nil {
		if sr, ok := err.(vm.SaiRequisicao); ok {
			return sr.Codigo
		}
		p.reportaErro(err.Error(), vm.ErroDoRun(err))
		return 1
	}
	return 0
}

func (p *Programa) reportaErro(msg string, e *object.Erro) {
	fmt.Fprintln(p.cfg.Erro, msg)
	if e != nil && len(e.Stack) > 0 {
		fmt.Fprint(p.cfg.Erro, "Traço de pilha:\n"+e.Traco())
	}
}
