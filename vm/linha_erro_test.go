package vm

import (
	"io"
	"strings"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// rodaVMErro roda o fonte esperando erro de runtime; devolve a mensagem.
func rodaVMErro(t *testing.T, input string) string {
	t.Helper()
	return rodaVMErroObj(t, input).Error()
}

// rodaVMErroObj idem, mas devolve o Go error cru (pra inspecionar via
// ErroDoRun — traço de pilha etc).
func rodaVMErroObj(t *testing.T, input string) error {
	t.Helper()
	prog := parser.New(lexer.New(input)).ParseProgram()
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile %q: %v", input, err)
	}
	maq := New(comp.Bytecode(), io.Discard)
	err := maq.Run()
	if err == nil {
		t.Fatalf("esperava erro de runtime em %q, rodou limpo", input)
	}
	return err
}

func TestVMErroTemLinha(t *testing.T) {
	msg := rodaVMErro(t, `# comentario
bota a = 1
bota b = a / 0`)
	if !strings.Contains(msg, "deu ruim na linha 3") {
		t.Fatalf("erro sem linha 3: %q", msg)
	}
}

func TestVMErroLinhaDentroDeFuncao(t *testing.T) {
	// a linha vem da tabela DA FUNCAO (nao do call site)
	msg := rodaVMErro(t, `gambiarra f(n)
    funciona n / 0
acabou_finalmente
mostra f(1)`)
	if !strings.Contains(msg, "deu ruim na linha 2") {
		t.Fatalf("erro sem a linha do corpo da funcao: %q", msg)
	}
}

func TestVMErroLinhaViaQuebrou(t *testing.T) {
	// erro_linha(err) depois do catch le Erro.Line — paridade programatica
	src := `bota l = 0
arruma
    bota x = 1 / 0
quebrou err
    bota l = erro_linha(err)
acabou_finalmente
l`
	got, _ := rodaVM(t, src)
	if got.Inspect() != "3" {
		t.Fatalf("erro_linha => %s, esperado 3", got.Inspect())
	}
}

// TestVMErroMensagemIgualTreeWalker garante paridade byte a byte da mensagem
// de erro entre os dois engines pros casos comuns.
func TestVMErroMensagemIgualTreeWalker(t *testing.T) {
	casos := []string{
		"bota a = 1\nbota b = a / 0",
		"bota xs = [1, 2]\nmostra xs[9]",
	}
	for _, src := range casos {
		// tree-walker
		prog := parser.New(lexer.New(src)).ParseProgram()
		interp := interpreter.New(io.Discard)
		resTW := interp.Eval(prog, object.NewEnvironment())
		erroTW, ok := resTW.(*object.Erro)
		if !ok {
			t.Fatalf("TW nao deu erro em %q", src)
		}
		// vm
		msgVM := rodaVMErro(t, src)
		if erroTW.Message != msgVM {
			t.Errorf("mensagens divergem em %q:\n  TW: %q\n  VM: %q", src, erroTW.Message, msgVM)
		}
	}
}
