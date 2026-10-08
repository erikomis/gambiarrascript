package formatter

import (
	"strings"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

func formataFonte(t *testing.T, src string) string {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("erros de parse: %v", errs)
	}
	return Formata(prog)
}

func TestFormataBasico(t *testing.T) {
	src := `gambiarra dobro(n)
funciona n * 2
acabou_finalmente
mostra dobro(5)`
	out := formataFonte(t, src)
	esperado := `gambiarra dobro(n)
    funciona n * 2
acabou_finalmente
mostra dobro(5)
`
	if out != esperado {
		t.Fatalf("got:\n%s\nesperado:\n%s", out, esperado)
	}
}

func TestFormataSeColarIndentado(t *testing.T) {
	src := `bota x = 10
se_colar x > 5
mostra "grande"
se_nao_colar
mostra "pequeno"
acabou_finalmente`
	out := formataFonte(t, src)
	if !strings.Contains(out, "    mostra \"grande\"") {
		t.Fatalf("esperava indentacao no se_colar, got:\n%s", out)
	}
	if !strings.Contains(out, "    mostra \"pequeno\"") {
		t.Fatalf("esperava indentacao no se_nao_colar, got:\n%s", out)
	}
}

func TestFormataRange(t *testing.T) {
	out := formataFonte(t, `mostra 0..n-1`)
	if !strings.Contains(out, "0..n - 1") {
		t.Fatalf("range mal formatado: %q", out)
	}
}

func TestFormataArrumaComFinally(t *testing.T) {
	// finally-only (sem quebrou): nao pode dar panic e nao pode sumir o bloco.
	src := `arruma
mostra "try"
finalmente
mostra "limpa"
acabou_finalmente`
	out := formataFonte(t, src)
	if !strings.Contains(out, "finalmente") || !strings.Contains(out, `mostra "limpa"`) {
		t.Fatalf("finally sumiu na formatacao:\n%s", out)
	}
	if strings.Contains(out, "quebrou") {
		t.Fatalf("nao devia ter quebrou (arruma so com finally):\n%s", out)
	}
	// e tem que reparsear
	p := parser.New(lexer.New(out))
	p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("saida formatada nao reparseou: %v\n%s", errs, out)
	}
}

func TestFormataAtribuicaoComposta(t *testing.T) {
	out := formataFonte(t, `bota x = 1
x += 2
x <<= 3`)
	if !strings.Contains(out, "x += 2") || !strings.Contains(out, "x <<= 3") {
		t.Fatalf("composto perdeu a forma original:\n%s", out)
	}
	if strings.Contains(out, "bota x = x + 2") {
		t.Fatalf("composto foi desugarado na saida:\n%s", out)
	}
}

func TestFormataDotAccess(t *testing.T) {
	out := formataFonte(t, `bota p = {"nome": "Jurandir"}
mostra p.nome
bota p.nome = "Zeh"
p.idade += 1`)
	for _, esp := range []string{"mostra p.nome", `bota p.nome = "Zeh"`, "p.idade += 1"} {
		if !strings.Contains(out, esp) {
			t.Fatalf("esperava %q na saida:\n%s", esp, out)
		}
	}
	if strings.Contains(out, `p["nome"]`) {
		t.Fatalf("dot virou colchete:\n%s", out)
	}
}

func TestFormataEscolheEDesestrutura(t *testing.T) {
	src := `escolhe x
caso 1, 2
mostra "baixo"
se_nao_colar
mostra "outro"
acabou_finalmente
bota [a, b] = [1, 2]
bota {n} = {"n": 1}
bota f = gambiarra(v) funciona v acabou_finalmente`
	out := formataFonte(t, src)
	for _, esp := range []string{"escolhe x", "caso 1, 2", "    mostra \"baixo\"", "se_nao_colar",
		"bota [a, b] = [1, 2]", `bota {n} = {"n": 1}`, "gambiarra(v) funciona v acabou_finalmente"} {
		if !strings.Contains(out, esp) {
			t.Fatalf("esperava %q na saida:\n%s", esp, out)
		}
	}
	// e reparseia
	p := parser.New(lexer.New(out))
	p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("saida formatada nao reparseou: %v\n%s", errs, out)
	}
}

func TestFormetaReparser(t *testing.T) {
	// o resultado do formatter tem que voltar a parsear sem erros
	src := `pra_cada i de 1 ate 3
se_colar i == 2
continua
acabou_finalmente
mostra i
acabou_finalmente`
	out := formataFonte(t, src)
	p := parser.New(lexer.New(out))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("saida formatada nao reparseou: %v\n%s", errs, out)
	}
	if len(prog.Statements) == 0 {
		t.Fatalf("saida formatada vazia")
	}
}

// TestFormataPotenciaECrava: o formatter reimprime ** sem mudar o sentido
// (parentese so onde precisa) e o crava como veio.
func TestFormataPotenciaECrava(t *testing.T) {
	casos := map[string]string{
		"crava PI=3.14":      "crava PI = 3.14\n",
		"mostra 2**3**2":     "mostra 2 ** 3 ** 2\n",
		"mostra (2**3)**2":   "mostra (2 ** 3) ** 2\n",
		"mostra -2**2":       "mostra -2 ** 2\n",
		"mostra (-2)**2":     "mostra (-2) ** 2\n",
		"mostra 2**-1":       "mostra 2 ** -1\n",
		"mostra (1+2)**2":    "mostra (1 + 2) ** 2\n",
		"mostra 2*3**2":      "mostra 2 * 3 ** 2\n",
		"x **= 2":            "x **= 2\n",
		"mostra nao deu_bom": "mostra nao deu_bom\n",
	}
	for src, esp := range casos {
		out := formataFonte(t, src)
		if out != esp {
			t.Errorf("%q: got %q, esperado %q", src, out, esp)
		}
		// idempotente: formatar de novo nao muda
		if de2 := formataFonte(t, out); de2 != out {
			t.Errorf("%q nao e idempotente: %q -> %q", src, out, de2)
		}
	}
}

// spread na chamada volta com `...` (normal, bora e metodo) e o formato da
// interpolacao fica intacto.
func TestFormataEspalhaEFormato(t *testing.T) {
	src := `mostra f(1, ...xs, ...[2, 3])
bota fu = bora g(...ys)
mostra o.m(...zs)
mostra "${preco:.2f} ${n:05d}"`
	out := formataFonte(t, src)
	for _, esp := range []string{
		"mostra f(1, ...xs, ...[2, 3])",
		"bota fu = bora g(...ys)",
		"mostra o.m(...zs)",
		`mostra "${preco:.2f} ${n:05d}"`,
	} {
		if !strings.Contains(out, esp) {
			t.Fatalf("esperava %q em:\n%s", esp, out)
		}
	}
	// idempotente: formatar de novo nao muda nada
	if de_novo := formataFonte(t, out); de_novo != out {
		t.Fatalf("nao e idempotente:\n%s\n---\n%s", out, de_novo)
	}
}

// aspas dentro do ${...} ficam cruas (o lexer copia o miolo cru): o Quote
// escapava e `${junta(xs, " ")}` saia quebrado. `\${` continua escapado.
func TestFormataInterpolacaoComAspas(t *testing.T) {
	src := `mostra "a \"b\" ${junta(xs, " ")} \${x} ${d["k"]:.2f}"` + "\n"
	out := formataFonte(t, src)
	if out != src {
		t.Fatalf("got:\n%s\nesperado:\n%s", out, src)
	}
}

// `\${` escapado vira TextoLiteral com `${` dentro: tem que voltar escapado,
// senao o formatter transformava texto em interpolacao.
func TestFormataDolarEscapado(t *testing.T) {
	for _, src := range []string{`mostra "preco: \${nao interpola}"` + "\n", "mostra \"a\\\\\\${b}\"\n"} {
		out := formataFonte(t, src)
		p := parser.New(lexer.New(out))
		prog := p.ParseProgram()
		orig := parser.New(lexer.New(src)).ParseProgram()
		if len(p.Errors()) != 0 || prog.String() != orig.String() {
			t.Fatalf("%q virou %q (mudou o sentido)", src, out)
		}
	}
}

func TestFormataMultiCatch(t *testing.T) {
	src := `arruma
mostra "try"
quebrou a se erro_tipo(a)=="rede"
mostra "rede"
quebrou b se nao (erro_tipo(b) == "io")
mostra "nao io"
acabou_finalmente`
	esperado := `arruma
    mostra "try"
quebrou a se erro_tipo(a) == "rede"
    mostra "rede"
quebrou b se nao (erro_tipo(b) == "io")
    mostra "nao io"
acabou_finalmente
`
	out := formataFonte(t, src)
	if out != esperado {
		t.Fatalf("got:\n%s\n--- esperado:\n%s", out, esperado)
	}
	if de2 := formataFonte(t, out); de2 != out {
		t.Fatalf("nao e idempotente:\n%s", de2)
	}
}
