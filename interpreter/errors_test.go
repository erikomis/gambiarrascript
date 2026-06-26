package interpreter

import (
	"bytes"
	"strings"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// evalErro roda o input e devolve o resultado (usado pra validar erros
// propagados — sem mata no Erro, ao contrario do `rodar`).
func evalErro(t *testing.T, input string) object.Object {
	t.Helper()
	p := parser.New(lexer.New(input))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse: %v", errs)
	}
	i := New(&bytes.Buffer{})
	return i.Eval(prog, object.NewEnvironment())
}

func TestQuebrouAmarraErroNaoTexto(t *testing.T) {
	res := evalErro(t, `arruma
    bota x = 10 / 0
quebrou erro
    mostra erro
acabou_finalmente`)
	if res == nil {
		t.Fatal("esperava resultado")
	}
	// arruma retorna valor da catch block. Como mostra retorna val e val e
	// o Erro com Handled=true, evalArruma repassa. Top-level deve ver Tipo ERRO
	// mas Handled — nao deve propagar como erro duro. Aqui so conferimos que o
	// programa rodou sem falhar (evalErro so devolve o que o Eval devolve).
	if res.Type() == object.ERRO_OBJ {
		if !res.(*object.Erro).Handled {
			t.Fatalf("erro capturado devia estar Handled, veio fresco: %s", res.Inspect())
		}
	}
}

func TestErroBuiltinCarregaKind(t *testing.T) {
	res := builtinTamanho([]object.Object{NADA})
	e, ok := res.(*object.Erro)
	if !ok {
		t.Fatalf("esperava Erro, got %s", res.Type())
	}
	if e.Kind != KindBuiltin {
		t.Fatalf("Kind esperado 'builtin', got %q", e.Kind)
	}
}

func TestQuebraCriaErroUsuario(t *testing.T) {
	res := evalErro(t, `quebra("quebrei a unha")`)
	e, ok := res.(*object.Erro)
	if !ok {
		t.Fatalf("esperava Erro, got %s", res.Type())
	}
	if e.Kind != KindUsuario {
		t.Fatalf("Kind esperado 'usuario', got %q", e.Kind)
	}
	if !strings.Contains(e.Message, "quebrei a unha") {
		t.Fatalf("mensagem errada: %q", e.Message)
	}
}

func TestQuebraECapturadoPorArruma(t *testing.T) {
	out := rodar(t, `arruma
    bota x = quebra("explodi")
quebrou erro
    mostra "peguei: " + erro
acabou_finalmente`)
	if !strings.HasPrefix(out, "peguei: ") {
		t.Fatalf("saida: %q", out)
	}
	if !strings.Contains(out, "explodi") {
		t.Fatalf("mensagem nao chegou: %q", out)
	}
}

func TestErroLinhaETipo(t *testing.T) {
	res := evalErro(t, `bota x = 1 / 0`)
	e, ok := res.(*object.Erro)
	if !ok {
		t.Fatalf("esperava Erro, got %s", res.Type())
	}
	if e.Kind != KindRuntime {
		t.Fatalf("Kind esperado 'runtime', got %q", e.Kind)
	}
	if e.Line != 1 {
		t.Fatalf("Line esperado 1, got %d", e.Line)
	}
}

func TestErroPilhaAtravessaChamadas(t *testing.T) {
	res := evalErro(t, `gambiarra externa()
    funciona interna()
acabou_finalmente
gambiarra interna()
    funciona 1 / 0
acabou_finalmente
mostra externa()`)
	e, ok := res.(*object.Erro)
	if !ok {
		t.Fatalf("esperava Erro, got %s", res.Type())
	}
	// Devem ter pelo menos 2 frames (chamada de externa() e interna()).
	if len(e.Stack) < 2 {
		t.Fatalf("Stack curta (%d frames): %v", len(e.Stack), e.Stack)
	}
	// outer-first: o primeiro frame deve ser 'externa' ou 'mostra-args',
	// o segundo 'interna'.
	if e.Stack[0].Funcao != "externa" {
		t.Fatalf("frame externo esperado 'externa', got %q", e.Stack[0].Funcao)
	}
	if e.Stack[1].Funcao != "interna" {
		t.Fatalf("frame interno esperado 'interna', got %q", e.Stack[1].Funcao)
	}
}

func TestBuiltinErroXXCampos(t *testing.T) {
	e := evalErro(t, `bota x = 1 / 0`).(*object.Erro)
	msg := builtinErroMsg([]object.Object{e}).Inspect()
	if !strings.Contains(msg, "dividir por zero") {
		t.Fatalf("erro_msg: %q", msg)
	}
	lin := builtinErroLinha([]object.Object{e}).(*object.Numero).Value
	if lin != 1 {
		t.Fatalf("erro_linha: %v", lin)
	}
	tipo := builtinErroTipo([]object.Object{e}).Inspect()
	if tipo != KindRuntime {
		t.Fatalf("erro_tipo: %q", tipo)
	}
}

func TestEnvolveErro(t *testing.T) {
	causa := evalErro(t, `quebra("raiz do problema")`).(*object.Erro)
	wrapped, ok := builtinEnvolveErro([]object.Object{
		&object.Texto{Value: "io"},
		&object.Texto{Value: "disco cheio"},
		causa,
	}).(*object.Erro)
	if !ok {
		t.Fatal("envolve_erro devia devolver Erro")
	}
	if wrapped.Kind != "io" {
		t.Fatalf("kind: %q", wrapped.Kind)
	}
	if wrapped.Cause == nil || !strings.Contains(wrapped.Cause.Message, "raiz") {
		t.Fatalf("causa errada: %v", wrapped.Cause)
	}
}