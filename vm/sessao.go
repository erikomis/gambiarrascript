package vm

import (
	"io"

	"gambiarrascript/ast"
	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/object"
)

// Sessao roda um programa em PEDACOS, mantendo o estado entre eles — e o que o
// REPL precisa pra rodar na VM em vez do tree-walker. O compilador guarda a
// tabela de simbolos e o pool de constantes; a sessao guarda os valores das
// globais. Cada entrada compila so o codigo novo e roda numa VM limpa em cima
// das mesmas globais.
type Sessao struct {
	comp    *compiler.Compiler
	globals []object.Object
	interp  *interpreter.Interpreter
	out     io.Writer
}

// NovaSessao cria a sessao com o interpretador que hospeda os builtins.
func NovaSessao(out io.Writer) *Sessao {
	return NovaSessaoComInterp(out, interpreter.New(out))
}

// NovaSessaoComInterp deixa quem chama passar o interpretador (pra ler estado
// depois, ou pra compartilhar builtins configurados).
func NovaSessaoComInterp(out io.Writer, interp *interpreter.Interpreter) *Sessao {
	// O array de globais e alocado no TAMANHO MAXIMO de uma vez, ao contrario
	// do `gs roda`, que aloca o tanto que o programa precisa. Aqui nao da pra
	// saber o tamanho final (cada entrada pode declarar mais), e crescer
	// depois seria furada: o slice e compartilhado com os clones do `bora`,
	// que ficariam com o array velho. 1 MB numa sessao interativa nao pesa.
	globals := make([]object.Object, MaxGlobals)
	// nada em vez de nil: uma global declarada numa entrada que quebrou no meio
	// existe na tabela de simbolos mas nunca foi escrita. Lida como nil ela
	// derrubaria a VM; como `nada` ela se comporta igual ao tree-walker.
	for i := range globals {
		globals[i] = NADA
	}
	return &Sessao{
		comp:    compiler.New(),
		globals: globals,
		interp:  interp,
		out:     out,
	}
}

// Interp devolve o interpretador da sessao (builtins, autocomplete, etc).
func (s *Sessao) Interp() *interpreter.Interpreter { return s.interp }

// NomesGlobais lista o que o usuario ja definiu na sessao (pro autocomplete).
func (s *Sessao) NomesGlobais() []string { return s.comp.NomesGlobais() }

// Avalia compila e roda mais um pedaco de programa.
//
// Devolve o valor da ULTIMA expressao quando a entrada termina numa (pra o
// REPL imprimir `=> valor`, estilo Python) e nil quando termina num comando
// (`bota`, `mostra`, `se_colar`...), que nao tem valor pra mostrar.
//
// Erro de compilacao ou de runtime NAO derruba a sessao: as globais ja
// definidas continuam valendo e a proxima entrada roda normalmente.
func (s *Sessao) Avalia(prog *ast.Program) (object.Object, error) {
	s.comp.NovaEntrada()
	if err := s.comp.Compile(prog); err != nil {
		return nil, err
	}
	bc := s.comp.Bytecode()

	maq := NovaComInterp(bc, s.out, s.interp)
	maq.globals = s.globals // as globais da sessao, nao as da VM nova
	if err := maq.Run(); err != nil {
		return nil, err
	}
	if !terminaEmExpressao(prog) {
		return nil, nil
	}
	return maq.LastPoppedStackElem(), nil
}

// terminaEmExpressao diz se o ultimo comando do programa e uma expressao solta
// — o unico caso em que sobra valor pra mostrar. O compilador emite OpPop
// depois dela, entao o valor fica onde LastPoppedStackElem le.
func terminaEmExpressao(prog *ast.Program) bool {
	if len(prog.Statements) == 0 {
		return false
	}
	_, ok := prog.Statements[len(prog.Statements)-1].(*ast.ExpressionStatement)
	return ok
}
