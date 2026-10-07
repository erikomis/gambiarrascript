package lsp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

const msgSombra = "aqui dentro e local"

func TestLintSombraCompostaNaGlobal(t *testing.T) {
	// o bug classico: `total += n` querendo mexer na global
	diags := diagsDeTypecheck(t, `bota total = 0
gambiarra soma(n)
    total += n
acabou_finalmente
soma(1)
mostra total`)
	if !contemMsg(diags, msgSombra) || !contemMsg(diags, "a global `total` nao muda") {
		t.Fatalf("nao avisou `total += n` dentro da funcao: %v", diags)
	}
}

func TestLintSombraLeituraAntesDoBota(t *testing.T) {
	// `bota x = x + 1` le antes de botar: le a global, escreve no local
	diags := diagsDeTypecheck(t, `bota contador = 0
gambiarra incrementa()
    bota contador = contador + 1
    funciona contador
acabou_finalmente
mostra incrementa()`)
	if !contemMsg(diags, msgSombra) {
		t.Fatalf("nao avisou leitura da global antes do bota local: %v", diags)
	}
}

func TestLintSombraGlobalDeclaradaDepois(t *testing.T) {
	diags := diagsDeTypecheck(t, `gambiarra conta()
    vezes += 1
acabou_finalmente
bota vezes = 0
conta()`)
	if !contemMsg(diags, msgSombra) {
		t.Fatalf("nao avisou global declarada depois da funcao: %v", diags)
	}
}

func TestLintSombraClosureNaVarDeFora(t *testing.T) {
	// closure que faz `cont += 1` cria um `cont` dela: o de fora nao muda
	diags := diagsDeTypecheck(t, `gambiarra f()
    bota cont = 0
    bota inc = gambiarra()
        cont += 1
    acabou_finalmente
    inc()
    funciona cont
acabou_finalmente
mostra f()`)
	if !contemMsg(diags, "o `cont` da funcao de fora nao muda") {
		t.Fatalf("nao avisou closure atribuindo na var de fora: %v", diags)
	}
}

func TestLintSombraLocalDeclaradoNaoAvisa(t *testing.T) {
	// `bota` antes de ler deixa claro que e um local: nada de aviso
	diags := diagsDeTypecheck(t, `bota total = 0
bota nome = "fulano"
gambiarra f(xs)
    bota total = 0
    pra_cada x em xs
        total += x
    acabou_finalmente
    bota nome = "outro"
    funciona total + tamanho(nome)
acabou_finalmente
gambiarra g(total)
    total += 1
    funciona total
acabou_finalmente
gambiarra h()
    funciona total + 1
acabou_finalmente
gambiarra estado()
    bota d = {"total": total}
    d["total"] += 1
    funciona d
acabou_finalmente
mostra f([1]) + g(1) + h()
mostra estado()`)
	if contemMsg(diags, msgSombra) {
		t.Fatalf("falso positivo de sombreamento: %v", diags)
	}
}

func TestLintSombraNomeSoDeDentroNaoAvisa(t *testing.T) {
	// nome que nao existe fora: o aviso e o de indefinido, nao o de sombra
	diags := diagsDeTypecheck(t, `gambiarra f()
    soma += 1
    funciona soma
acabou_finalmente`)
	if contemMsg(diags, msgSombra) {
		t.Fatalf("avisou sombra de nome que so existe dentro: %v", diags)
	}
}

func TestLintSombraUmAvisoPorNome(t *testing.T) {
	diags := diagsDeTypecheck(t, `bota total = 0
gambiarra f()
    total += 1
    total += 2
acabou_finalmente
f()`)
	n := 0
	for _, d := range diags {
		if strings.Contains(d.Message, msgSombra) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("esperava 1 aviso de sombra, veio %d: %v", n, diags)
	}
}

// sombrasEm roda o typecheck e devolve so os avisos de sombreamento (codigo
// que nao parseia e ignorado: trecho de doc pode ser pedaco de exemplo).
func sombrasEm(src string) []string {
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil
	}
	var out []string
	for _, d := range typecheck(prog) {
		if strings.Contains(d.Message, msgSombra) {
			out = append(out, d.Message)
		}
	}
	return out
}

func TestLintSombraNenhumNosExemplos(t *testing.T) {
	arquivos, _ := filepath.Glob("../examples/*.gs")
	sub, _ := filepath.Glob("../examples/*/*.gs")
	arquivos = append(arquivos, sub...)
	if len(arquivos) == 0 {
		t.Skip("sem examples/")
	}
	for _, arq := range arquivos {
		src, err := os.ReadFile(arq)
		if err != nil {
			t.Fatal(err)
		}
		if s := sombrasEm(string(src)); len(s) > 0 {
			t.Errorf("%s: %v", arq, s)
		}
	}
}

var blocoGS = regexp.MustCompile("(?s)```gambiarrascript\\n(.*?)```")

func TestLintSombraNenhumNasDocs(t *testing.T) {
	var docs []string
	filepath.Walk("../web/content/docs", func(p string, info os.FileInfo, err error) error {
		if err == nil && strings.HasSuffix(p, ".mdx") {
			docs = append(docs, p)
		}
		return nil
	})
	if len(docs) == 0 {
		t.Skip("sem web/content/docs")
	}
	for _, doc := range docs {
		src, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range blocoGS.FindAllStringSubmatch(string(src), -1) {
			// trecho que mostra a pegadinha de proposito marca no comentario
			marcado := strings.Contains(m[1], "gs check avisa") || strings.Contains(m[1], "gs check warns")
			s := sombrasEm(m[1])
			if marcado && len(s) == 0 {
				t.Errorf("%s: trecho diz que o gs check avisa, mas nao avisou:\n%s", doc, m[1])
			}
			if !marcado && len(s) > 0 {
				t.Errorf("%s: %v\n%s", doc, s, m[1])
			}
		}
	}
}
