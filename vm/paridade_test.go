package vm

import (
	"bytes"
	"strings"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// rodaTWComp roda a fonte no tree-walker e devolve (objFinal, saida, errstr).
// errstr e "" quando nao houve erro de runtime (ou quando foi capturado por
// arruma — nesse caso o resultado e a string do Erro capturado).
func rodaTWComp(t *testing.T, src string) (string, string, string) {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		// programa nao parseia: ambos engines usam o MESMO parser, entao
		// devolvemos o erro de parse (identico) em vez de rodar num AST quebrado.
		return "", "", "parse: " + strings.Join(errs, "; ")
	}
	var buf bytes.Buffer
	interp := interpreter.New(&buf)
	res := interp.Eval(prog, object.NewEnvironment())
	objSaida := ""
	errStr := ""
	if e, ok := res.(*object.Erro); ok && !e.Handled {
		errStr = e.Message
	} else {
		objSaida = res.Inspect()
	}
	return objSaida, buf.String(), errStr
}

// rodaVMComp roda a fonte na VM e devolve (objFinal, saida, errstr). errstr
// vem do Go error devolvido por Run (ja formatado "deu ruim na linha N: ...").
func rodaVMComp(t *testing.T, src string) (string, string, string) {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return "", "", "parse: " + strings.Join(errs, "; ")
	}
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		return "", "", "compile: " + err.Error()
	}
	var buf bytes.Buffer
	maq := New(comp.Bytecode(), &buf)
	err := maq.Run()
	if err != nil {
		return "", "", err.Error()
	}
	return maq.LastPoppedStackElem().Inspect(), buf.String(), ""
}

// compara engulos: roda nos dois engines e falha se divergir (resultado, saida
// ou erro). Ignora torcao de ordem de chaves de dicionario (map do Go e
// randomico) — so compara quantas linhas foram impressas quando a saida e
// multi-linha de chaves.
func comparaEngines(t *testing.T, src string) {
	t.Helper()
	objTW, saidaTW, errTW := rodaTWComp(t, src)
	objVM, saidaVM, errVM := rodaVMComp(t, src)

	// normaliza: recorta "deu ruim na linha N:" igual (TW e VM usam o mesmo
	// prefixo, mas por seguranca comparamos so o conteudo apos o prefixo).
	norm := func(s string) string {
		s = strings.TrimSpace(s)
		return s
	}
	if norm(errTW) != norm(errVM) {
		t.Errorf("divergencia de erro em %q\n  TW: %q\n  VM: %q", src, errTW, errVM)
		return
	}
	if errTW == "" && norm(objTW) != norm(objVM) {
		t.Errorf("divergencia de resultado em %q\n  TW: %q\n  VM: %q", src, objTW, objVM)
	}
	if errTW == "" && norm(saidaTW) != norm(saidaVM) {
		t.Errorf("divergencia de saida em %q\n  TW: %q\n  VM: %q", src, saidaTW, saidaVM)
	}
}

// TestParidadeCases passa por varias construcoes EM ambos engines e compara.
// O objetivo e expor divergencias no formato/mensagem dos erros — o padrao
// deve ser o mesmo ("deu ruim na linha N: ...").
func TestParidadeCases(t *testing.T) {
	casos := []string{
		// aritmetica basica
		"1 + 2",
		"10 % 3",
		"1 / 0",
		`"a" + "b"`,

		// comparacoes e tipos
		`1 == "1"`,
		`"a" < "b"`,
		`[1, 2] == [1, 2]`,

		// operacoes invalidas (devem dar erro com mesma mensagem)
		`1 + "x"`,
		`"y" - 1`,
		`-[1, 2]`,
		`1 << 2`,
		`1 >> 2`,

		// indexacao
		`bota xs = [10, 20, 30]
mostra xs[5]`,
		`bota d = {"a": 1}
mostra d["b"]`,
		`bota t = "oi"
mostra t[10]`,
		`bota n = 5
mostra n[0]`,

		// chamada de naocallable
		`bota x = 5
x()`,

		// chamada com argc errado (funcao do usuario)
		`gambiarra f(a)
    funciona a
acabou_finalmente
f(1, 2)`,

		// throw/quebra manual
		`quebra("explodiu")`,
		`arruma
    quebra("explodiu")
quebrou err
    funciona "pegou"
acabou_finalmente`,

		// divisao por zero dentro de funcao com traço
		`gambiarra g()
    funciona 1 / 0
acabou_finalmente
g()`,

		// pra_cada em dicionario — so checa contagem de linhas
		`bota d = {"a": 1, "b": 2}
bota n = 0
pra_cada k em d
    bota n = n + 1
acabou_finalmente
n`,

		// closures com freevar
		`gambiarra counter()
    bota n = 0
    gambiarra step()
        bota n = n + 1
        funciona n
    acabou_finalmente
    funciona step
acabou_finalmente
bota c = counter()
c()
c()`,

		// builtins de ordem superior
		`mapeia([1, 2, 3], gambiarra(x) funciona x * 2 acabou_finalmente)`,
		`filtra([1, 2, 3, 4], gambiarra(x) funciona x % 2 == 0 acabou_finalmente)`,
		`reduz([1, 2, 3, 4], 0, gambiarra(a, b) funciona a + b acabou_finalmente)`,

		// builtin com tipo invalido (paridade de erro)
		`tamanho(42)`,

		// bitwise com nao-inteiro
		`1.5 & 2`,

		// shift por negativo
		`1 << -1`,

		// range invalido
		`5 .. 1`,

		// modulo com texto
		`"x" % 2`,
	}
	for _, src := range casos {
		comparaEngines(t, src)
	}
}

// TestParidadePilhaErrosGarante o mesmo traço de pilha (string) entre engines
// para um erro que cruza chamadas de funcao.
func TestParidadePilhaErros(t *testing.T) {
	src := `gambiarra g()
    funciona 1 / 0
acabou_finalmente
gambiarra f()
    funciona g()
acabou_finalmente
f()`
	prog := parser.New(lexer.New(src)).ParseProgram()

	// tree-walker
	var bufTW bytes.Buffer
	interpTW := interpreter.New(&bufTW)
	resTW := interpTW.Eval(prog, object.NewEnvironment())
	eTW, ok := resTW.(*object.Erro)
	if !ok || eTW.Handled {
		t.Fatalf("TW nao devolveu erro: %s", resTW.Inspect())
	}

	// vm
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	var bufVM bytes.Buffer
	maq := New(comp.Bytecode(), &bufVM)
	err := maq.Run()
	if err == nil {
		t.Fatalf("VM nao devolveu erro")
	}
	eVM := ErroDoRun(err)
	if eVM == nil {
		t.Fatalf("ErroDoRun devolveu nil para: %v", err)
	}

	if eTW.Message != eVM.Message {
		t.Errorf("mensagens divergem:\n  TW: %q\n  VM: %q", eTW.Message, eVM.Message)
	}
	if eTW.Traco() != eVM.Traco() {
		t.Errorf("tracos divergem:\n  TW: %q\n  VM: %q", eTW.Traco(), eVM.Traco())
	}
}
// TestParidadeStats garante que soma/media/zip/enumera rodam identico no
// tree-walker e na VM (builtins puros compartilhados via BuiltinsVisiveis).
func TestParidadeStats(t *testing.T) {
	casos := []string{
		`mostra soma([1, 2, 3, 4])`,
		`mostra soma([1, 2, 0.5])`,
		`mostra soma([])`,
		`mostra media([1, 2, 3, 4])`,
		`mostra zip([1, 2, 3], ["a", "b", "c"])`,
		`mostra zip([1, 2, 3], [10, 20])`,
		`mostra enumera(["a", "b", "c"])`,
	}
	for _, src := range casos {
		comparaEngines(t, src)
	}
}

// TestParidadeSombreiaBuiltin: o usuario pode declarar bota/gambiarra com o
// mesmo nome de um builtin e o binding do usuario sombreia o builtin nos DOIS
// engines (o tree-walker checa env antes; a VM tem que casar).
func TestParidadeSombreiaBuiltin(t *testing.T) {
	casos := []string{
		`bota tamanho = 42
mostra tamanho`,
		`gambiarra soma(a, b)
    funciona a + b
acabou_finalmente
mostra soma(2, 3)`,
	}
	for _, src := range casos {
		comparaEngines(t, src)
	}
}

// TestParidadeMunging: ordena_por (puro), agrupa_por (higher-order via
// ChamaCompilada) e os aleatorios (com semente pra determinismo) rodam
// identico no tree-walker e na VM.
func TestParidadeMunging(t *testing.T) {
	casos := []string{
		`bota gente = [{"n": "Ana", "idade": 30}, {"n": "Ze", "idade": 20}, {"n": "Rita", "idade": 25}]
bota ord = ordena_por(gente, "idade")
mostra ord[0].n
mostra ord[1].n
mostra ord[2].n`,
		`bota g = agrupa_por([1, 2, 3, 4, 5, 6], gambiarra(n) funciona n % 2 acabou_finalmente)
mostra g[0]
mostra g[1]`,
		`semente(9)
mostra embaralha([1, 2, 3, 4, 5, 6, 7, 8])`,
		`semente(9)
mostra escolhe_um([10, 20, 30, 40, 50])`,
	}
	for _, src := range casos {
		comparaEngines(t, src)
	}
}

// TestParidadeIndiceNegativo: indice negativo e indexacao de texto batem nos
// dois engines.
func TestParidadeIndiceNegativo(t *testing.T) {
	casos := []string{
		`mostra [10, 20, 30][-1]`,
		`mostra [10, 20, 30][-3]`,
		`mostra "café"[3]`,
		`mostra "abc"[-1]`,
		`bota xs = [1, 2, 3]
xs[-1] += 5
mostra xs`,
		`mostra [1, 2, 3][-4]`,
		`mostra "abc"[9]`,
	}
	for _, src := range casos {
		comparaEngines(t, src)
	}
}
