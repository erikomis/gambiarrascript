package vm

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gambiarrascript/ast"
	"gambiarrascript/cobertura"
	"gambiarrascript/code"
	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Gancho de linha / cobertura: os dois engines tem que disparar o gancho pros
// MESMOS statements, o mesmo numero de vezes. Cada caso roda o programa (com
// modulos de verdade num dir temporario) no tree-walker e na VM instrumentada
// e compara as contagens por arquivo:linha.

// contagensRel troca o caminho absoluto pelo relativo ao dir do teste.
func contagensRel(dir string, c map[string]map[int]int64) map[string]map[int]int64 {
	out := map[string]map[int]int64{}
	for arq, por := range c {
		rel, err := filepath.Rel(dir, arq)
		if err != nil {
			rel = arq
		}
		out[filepath.ToSlash(rel)] = por
	}
	return out
}

func fmtContagens(c map[string]map[int]int64) string {
	var linhas []string
	for arq, por := range c {
		for l, n := range por {
			linhas = append(linhas, fmt.Sprintf("%s:%03d %d", arq, l, n))
		}
	}
	sort.Strings(linhas)
	return strings.Join(linhas, "\n")
}

// cobreTW roda o principal no tree-walker com o coletor ligado.
func cobreTW(t *testing.T, principal string) (*cobertura.Coletor, string) {
	t.Helper()
	fonte, err := os.ReadFile(principal)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	var buf bytes.Buffer
	col := cobertura.Novo()
	interp := interpreter.New(&buf)
	interp.DefinirArquivo(principal)
	interp.DefinirGancho(col.Gancho())
	if res := interp.Eval(prog, object.NewEnvironment()); object.EhErroLevantado(res) {
		buf.WriteString("ERRO: " + res.Inspect() + "\n")
	}
	return col, buf.String()
}

// compilaVM compila o principal (instrumentado ou nao).
func compilaVM(t *testing.T, principal string, instrumenta bool) *compiler.Bytecode {
	t.Helper()
	fonte, err := os.ReadFile(principal)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	comp := compiler.New()
	comp.Arquivo = principal
	comp.Instrumentar = instrumenta
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	return comp.Bytecode()
}

// rodaBCGancho roda o bytecode (com gancho opcional) e devolve a saida.
func rodaBCGancho(bc *compiler.Bytecode, principal string, g object.GanchoLinha) string {
	var buf bytes.Buffer
	interp := interpreter.New(&buf)
	interp.DefinirArquivo(principal)
	maq := NovaComInterp(bc, &buf, interp)
	maq.DefinirGancho(g)
	if err := maq.Run(); err != nil {
		buf.WriteString("ERRO: " + err.Error() + "\n")
	}
	return buf.String()
}

// cobreAmbos roda nos dois engines, confere paridade (contagens e saida) e
// que todo hit cai numa linha executavel. Devolve as contagens relativas.
func cobreAmbos(t *testing.T, arquivos map[string]string, principal string) map[string]map[int]int64 {
	t.Helper()
	dir := t.TempDir()
	montaArquivos(t, dir, arquivos)
	prin := filepath.Join(dir, principal)

	colTW, saidaTW := cobreTW(t, prin)
	colVM := cobertura.Novo()
	saidaVM := rodaBCGancho(compilaVM(t, prin, true), prin, colVM.Gancho())
	// instrumentar nao muda o que o programa faz
	if normal := rodaBCGancho(compilaVM(t, prin, false), prin, nil); normal != saidaVM {
		t.Fatalf("saida da VM instrumentada mudou:\nnormal:\n%s\ninstrumentada:\n%s", normal, saidaVM)
	}
	if saidaTW != saidaVM {
		t.Fatalf("saida diverge:\nTW:\n%s\nVM:\n%s", saidaTW, saidaVM)
	}
	tw := contagensRel(dir, colTW.Contagens())
	vmc := contagensRel(dir, colVM.Contagens())
	if a, b := fmtContagens(tw), fmtContagens(vmc); a != b {
		t.Fatalf("contagens divergem:\nTW:\n%s\nVM:\n%s", a, b)
	}
	// hit sempre em linha executavel (a regra do ast bate com quem dispara)
	for arq, por := range tw {
		fonte := arquivos[arq]
		p := parser.New(lexer.New(fonte))
		exec := map[int]bool{}
		for _, l := range ast.LinhasExecutaveis(p.ParseProgram()) {
			exec[l] = true
		}
		for l := range por {
			if !exec[l] {
				t.Errorf("%s:%d teve hit mas nao e linha executavel", arq, l)
			}
		}
	}
	return tw
}

func esperaHits(t *testing.T, c map[string]map[int]int64, arq string, linhas map[int]int64) {
	t.Helper()
	for l, n := range linhas {
		if got := c[arq][l]; got != n {
			t.Errorf("%s:%d: esperava %d hits, veio %d\n%s", arq, l, n, got, fmtContagens(c))
		}
	}
}

func TestCoberturaFluxo(t *testing.T) {
	src := `gambiarra classifica(n)
    se_colar n < 0
        funciona "negativo"
    se_nao_colar se_colar n == 0
        funciona "zero"
    acabou_finalmente
    funciona "positivo"
acabou_finalmente

gambiarra nunca()
    mostra "nunca"
acabou_finalmente

bota total = 0
pra_cada i de 1 ate 5
    se_colar i == 2
        continua
    acabou_finalmente
    se_colar i == 4
        vaza
    acabou_finalmente
    total += i
acabou_finalmente
mostra total
mostra classifica(3)
mostra classifica(0)

bota k = 0
enquanto k < 3
    k += 1
acabou_finalmente

escolhe k
caso 1, 2
    mostra "pouco"
caso 3
    mostra "tres"
se_nao_colar
    mostra "muito"
acabou_finalmente

bota dobro = gambiarra(x)
    funciona x * 2
acabou_finalmente
mostra mapeia([1, 2, 3], dobro)
mostra se_colar k > 1 entao "sim" se_nao_colar "nao"
pra_cada v em [10, 20]
    mostra v
acabou_finalmente
`
	c := cobreAmbos(t, map[string]string{"main.gs": src}, "main.gs")
	esperaHits(t, c, "main.gs", map[int]int64{
		1: 1, 2: 2, 3: 0, 4: 0, 5: 1, 7: 1, // classifica(3) e classifica(0)
		10: 1, 11: 0, // gambiarra nunca chamada: so a declaracao roda
		15: 1, 16: 4, 17: 1, 19: 3, 20: 1, 22: 2, // pra_cada com continua/vaza
		29: 1, 30: 3, // enquanto
		33: 1, 35: 0, 37: 1, 39: 0, // escolhe: so o caso 3
		42: 1, 43: 3, // lambda chamada 3x pelo mapeia
		46: 1, 47: 1, 48: 2,
	})
}

func TestCoberturaArrumaFinalmente(t *testing.T) {
	src := `gambiarra tenta(x)
    arruma
        se_colar x == 0
            quebra("zero")
        acabou_finalmente
        funciona 10 / x
    quebrou err
        mostra erro_msg(err)
        funciona -1
    finalmente
        mostra "fim"
    acabou_finalmente
acabou_finalmente

mostra tenta(2)
mostra tenta(0)
pra_cada i de 1 ate 3
    arruma
        se_colar i == 2
            vaza
        acabou_finalmente
    finalmente
        mostra "sai " + texto(i)
    acabou_finalmente
acabou_finalmente
quebra("estoura aqui")
mostra "nunca chega"
`
	c := cobreAmbos(t, map[string]string{"main.gs": src}, "main.gs")
	esperaHits(t, c, "main.gs", map[int]int64{
		2: 2, 3: 2, 4: 1, 6: 1, 8: 1, 9: 1, 11: 2,
		18: 2, 19: 2, 20: 1, 23: 2,
		26: 1, 27: 0, // erro nao capturado para o programa
	})
}

func TestCoberturaMultiCatch(t *testing.T) {
	src := `gambiarra trata(msg)
    arruma
        quebra(msg)
    quebrou err se termina_com(erro_msg(err), "alfa")
        funciona "pegou a"
    quebrou err se termina_com(erro_msg(err), "beta")
        funciona "pegou b"
    quebrou err
        funciona "resto"
    acabou_finalmente
acabou_finalmente
mostra trata("alfa")
mostra trata("alfa")
mostra trata("gama")
`
	c := cobreAmbos(t, map[string]string{"main.gs": src}, "main.gs")
	esperaHits(t, c, "main.gs", map[int]int64{2: 3, 3: 3, 5: 2, 7: 0, 9: 1})
}

func TestCoberturaPOO(t *testing.T) {
	src := `treta Animal
    nome
acabou_finalmente
gambiarra (a Animal) fala()
    funciona a.nome + " faz barulho"
acabou_finalmente
gambiarra (a Animal) dorme()
    funciona "zzz"
acabou_finalmente

treta Cachorro
    Animal
    truques = []
acabou_finalmente
gambiarra (c Cachorro) aprende(t)
    adiciona(c.truques, t)
acabou_finalmente

combinado Falante
    fala()
acabou_finalmente

bota d = Cachorro{Animal: Animal{nome: "rex"}}
d.aprende("senta")
d.aprende("rola")
mostra d.fala()
mostra satisfaz(d, Falante)
mostra tamanho(d.truques)
`
	c := cobreAmbos(t, map[string]string{"main.gs": src}, "main.gs")
	esperaHits(t, c, "main.gs", map[int]int64{
		1: 1, 4: 1, 5: 1, 7: 1, 8: 0, // dorme nunca chamado
		11: 1, 15: 1, 16: 2, 19: 1, 23: 1, 24: 1,
	})
	// linha de campo e de assinatura nao e executavel
	for _, l := range []int{2, 12, 13, 20} {
		if _, ok := c["main.gs"][l]; ok {
			t.Errorf("linha %d (campo/assinatura) nao devia contar", l)
		}
	}
}

func TestCoberturaModulos(t *testing.T) {
	arquivos := map[string]string{
		"main.gs": `importa "lib/mat.gs" como m
importa "lib/mat.gs"
mostra m.dobra(2)
mostra triplica(3)
`,
		"lib/mat.gs": `importa "base.gs" como b
gambiarra dobra(x)
    funciona b.vezes(x, 2)
acabou_finalmente
gambiarra triplica(x)
    funciona b.vezes(x, 3)
acabou_finalmente
gambiarra sem_uso(x)
    funciona x
acabou_finalmente
mostra "carregou mat"
`,
		"lib/base.gs": `gambiarra vezes(a, b)
    funciona a * b
acabou_finalmente
`,
	}
	c := cobreAmbos(t, arquivos, "main.gs")
	// o modulo roda uma vez so, mesmo importado duas vezes
	esperaHits(t, c, "lib/mat.gs", map[int]int64{1: 1, 2: 1, 3: 1, 5: 1, 6: 1, 8: 1, 9: 0, 11: 1})
	esperaHits(t, c, "lib/base.gs", map[int]int64{1: 1, 2: 2})
	esperaHits(t, c, "main.gs", map[int]int64{1: 1, 2: 1, 3: 1, 4: 1})
}

func TestCoberturaConcorrencia(t *testing.T) {
	src := `gambiarra quadrado(x)
    bota r = x * x
    funciona r
acabou_finalmente
bota fs = []
pra_cada i de 1 ate 4
    adiciona(fs, bora quadrado(i))
acabou_finalmente
bota soma_q = 0
pra_cada f em fs
    soma_q += espera(f)
acabou_finalmente
mostra soma_q
mostra paralelo([1, 2, 3], quadrado)
gambiarra fat(n, acc = 1)
    se_colar n <= 1
        funciona acc
    acabou_finalmente
    funciona fat(n - 1, acc * n)
acabou_finalmente
mostra fat(5)
`
	c := cobreAmbos(t, map[string]string{"main.gs": src}, "main.gs")
	esperaHits(t, c, "main.gs", map[int]int64{2: 7, 3: 7, 7: 4, 11: 4, 16: 5, 17: 1, 19: 4})
}

// Sem Instrumentar o bytecode nao tem OpLinha nem sitio: o caminho normal
// fica byte a byte igual ao de antes.
func TestSemInstrumentarNaoTemOpLinha(t *testing.T) {
	dir := t.TempDir()
	montaArquivos(t, dir, map[string]string{
		"main.gs": "importa \"m.gs\"\ngambiarra f(x)\n    funciona x + 1\nacabou_finalmente\nmostra f(1)\n",
		"m.gs":    "bota y = 2\n",
	})
	prin := filepath.Join(dir, "main.gs")
	temOpLinha := func(bc *compiler.Bytecode) bool {
		achou := func(ins code.Instructions) bool {
			return strings.Contains(ins.String(), "OpLinha")
		}
		if achou(bc.Instructions) {
			return true
		}
		for _, k := range bc.Constants {
			switch v := k.(type) {
			case *object.CompiledFunction:
				if achou(v.Bytecode) {
					return true
				}
			case *object.Modulo:
				if v.Corpo != nil && achou(v.Corpo.Bytecode) {
					return true
				}
			}
		}
		return false
	}
	normal := compilaVM(t, prin, false)
	if temOpLinha(normal) || normal.Sitios != nil {
		t.Fatalf("bytecode normal nao pode ter OpLinha/sitios")
	}
	inst := compilaVM(t, prin, true)
	if !temOpLinha(inst) || len(inst.Sitios) != 5 {
		t.Fatalf("instrumentado devia ter OpLinha e 5 sitios, veio %d", len(inst.Sitios))
	}
	// gancho ligado em bytecode normal: nao dispara nada
	n := 0
	rodaBCGancho(normal, prin, func(*object.SitioLinha) { n++ })
	if n != 0 {
		t.Fatalf("gancho disparou %d vezes em bytecode sem instrumentar", n)
	}
}

// benchCobertura mede o custo da VM instrumentada com o coletor ligado (o
// preco do `gs testa --cobertura`); compara com o Benchmark<Nome> normal.
func benchCobertura(b *testing.B, src string) {
	prog := parser.New(lexer.New(src)).ParseProgram()
	comp := compiler.New()
	comp.Instrumentar = true
	if err := comp.Compile(prog); err != nil {
		b.Fatalf("compile: %v", err)
	}
	bc := comp.Bytecode()
	col := cobertura.Novo()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		maq := New(bc, io.Discard)
		maq.DefinirGancho(col.Gancho())
		if err := maq.Run(); err != nil {
			b.Fatalf("run: %v", err)
		}
	}
}

func BenchmarkFibCobertura(b *testing.B)       { benchCobertura(b, fonteFib) }
func BenchmarkLoopCobertura(b *testing.B)      { benchCobertura(b, fonteLoop) }
func BenchmarkLoopLocalCobertura(b *testing.B) { benchCobertura(b, fonteLoopLocal) }
func BenchmarkMapeiaCobertura(b *testing.B)    { benchCobertura(b, fonteMapeia) }

// Exemplos puros (sem rede, stdin, relogio ou sorteio): mesma cobertura nos
// dois engines e a VM instrumentada imprime o mesmo que a normal.
func TestCoberturaExemplos(t *testing.T) {
	puros := []string{
		"lista.gs", "dicionario.gs", "texto.gs", "gambiarra.gs",
		"matematica2.gs", "poo.gs", "constantes.gs", "conta.gs",
		"indice.gs", "comentarios.gs", "salve.gs", "stats.gs", "json.gs",
		"tier3.gs", "tipo_spread.gs", "tropa.gs", "modulos/principal.gs",
	}
	for _, nome := range puros {
		t.Run(nome, func(t *testing.T) {
			prin, err := filepath.Abs(filepath.Join("..", "examples", nome))
			if err != nil {
				t.Fatal(err)
			}
			colTW, saidaTW := cobreTW(t, prin)
			colVM := cobertura.Novo()
			saidaVM := rodaBCGancho(compilaVM(t, prin, true), prin, colVM.Gancho())
			if normal := rodaBCGancho(compilaVM(t, prin, false), prin, nil); normal != saidaVM {
				t.Fatalf("saida da VM instrumentada mudou:\nnormal:\n%s\ninstrumentada:\n%s", normal, saidaVM)
			}
			if saidaTW != saidaVM {
				t.Fatalf("saida diverge:\nTW:\n%s\nVM:\n%s", saidaTW, saidaVM)
			}
			if a, b := fmtContagens(colTW.Contagens()), fmtContagens(colVM.Contagens()); a != b || a == "" {
				t.Fatalf("contagens divergem (ou vazias):\nTW:\n%s\nVM:\n%s", a, b)
			}
		})
	}
}
