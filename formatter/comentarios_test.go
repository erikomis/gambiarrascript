package formatter

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// formataComentarios roda o FormataFonte (o que preserva comentario), exige
// que a trava Confere passe e que formatar de novo nao mude nada.
func formataComentarios(t *testing.T, src string) string {
	t.Helper()
	out, errs := FormataFonte(src)
	if len(errs) != 0 {
		t.Fatalf("erros de parse: %v", errs)
	}
	if err := Confere(src, out); err != nil {
		t.Fatalf("Confere reprovou: %v\nsaida:\n%s", err, out)
	}
	de2, errs := FormataFonte(out)
	if len(errs) != 0 {
		t.Fatalf("saida nao reparseou: %v\n%s", errs, out)
	}
	if de2 != out {
		t.Fatalf("nao e idempotente:\n--- 1a\n%s\n--- 2a\n%s", out, de2)
	}
	return out
}

func confereIgual(t *testing.T, src, esperado string) {
	t.Helper()
	if out := formataComentarios(t, src); out != esperado {
		t.Fatalf("got:\n%s\n--- esperado:\n%s", out, esperado)
	}
}

// o repro do bug: tudo sumia, sobrava "bota x = 1\nmostra x".
func TestComentarioRepro(t *testing.T) {
	confereIgual(t,
		"# explica a conta\nbota x = 1   # inline\n\n/* bloco */\nmostra x\n",
		"# explica a conta\nbota x = 1  # inline\n\n/* bloco */\nmostra x\n")
}

func TestComentarioReindentaNoBloco(t *testing.T) {
	src := `se_colar x
# dentro
mostra 1
          # torto
mostra 2
acabou_finalmente
`
	esperado := `se_colar x
    # dentro
    mostra 1
    # torto
    mostra 2
acabou_finalmente
`
	confereIgual(t, src, esperado)
}

func TestComentarioAntesDosTerminadores(t *testing.T) {
	src := `se_colar x
mostra 1
# antes do senao
se_nao_colar
mostra 2
# antes do fim
acabou_finalmente
escolhe x
# antes do primeiro caso
caso 1
mostra 1
# antes do caso 2
caso 2
mostra 2
# antes do padrao
se_nao_colar
mostra 3
acabou_finalmente
arruma
mostra 1
# antes do quebrou
quebrou erro
mostra erro
# antes do finalmente
finalmente
mostra 2
# fim
acabou_finalmente
enquanto x
# so comentario
acabou_finalmente
`
	esperado := `se_colar x
    mostra 1
    # antes do senao
se_nao_colar
    mostra 2
    # antes do fim
acabou_finalmente
escolhe x
# antes do primeiro caso
caso 1
    mostra 1
    # antes do caso 2
caso 2
    mostra 2
    # antes do padrao
se_nao_colar
    mostra 3
acabou_finalmente
arruma
    mostra 1
    # antes do quebrou
quebrou erro
    mostra erro
    # antes do finalmente
finalmente
    mostra 2
    # fim
acabou_finalmente
enquanto x
    # so comentario
acabou_finalmente
`
	confereIgual(t, src, esperado)
}

func TestComentarioNoCabecalhoENoFim(t *testing.T) {
	src := `gambiarra f(a)   # doc da f
se_colar a > 1 # grande
mostra a
se_nao_colar se_colar a < 0 # negativo
mostra 0
se_nao_colar    # resto
mostra 1
acabou_finalmente # fecha o se
escolhe a # qual?
caso 1, 2 # pouco
mostra a
acabou_finalmente
arruma # tenta
mostra 1
quebrou erro # deu ruim
mostra erro
finalmente # sempre
mostra 2
acabou_finalmente
pra_cada i de 1 ate 3 # conta
mostra i
acabou_finalmente
acabou_finalmente # fim da f
`
	esperado := `gambiarra f(a)  # doc da f
    se_colar a > 1  # grande
        mostra a
    se_nao_colar se_colar a < 0  # negativo
        mostra 0
    se_nao_colar  # resto
        mostra 1
    acabou_finalmente  # fecha o se
    escolhe a  # qual?
    caso 1, 2  # pouco
        mostra a
    acabou_finalmente
    arruma  # tenta
        mostra 1
    quebrou erro  # deu ruim
        mostra erro
    finalmente  # sempre
        mostra 2
    acabou_finalmente
    pra_cada i de 1 ate 3  # conta
        mostra i
    acabou_finalmente
acabou_finalmente  # fim da f
`
	confereIgual(t, src, esperado)
}

func TestComentarioLinhasEmBranco(t *testing.T) {
	src := "\n\n# topo\n\n\n\nbota x = 1\n\n\n\nse_colar x\n\n    mostra 1\n\n\n    mostra 2\n\n" +
		"acabou_finalmente\n\n\n"
	esperado := "# topo\n\nbota x = 1\n\nse_colar x\n    mostra 1\n\n    mostra 2\nacabou_finalmente\n"
	confereIgual(t, src, esperado)
}

func TestComentarioBloco(t *testing.T) {
	src := `/*
  cabecalho
     com recuo proprio
*/
se_colar x
/* um */ /* dois */
mostra 1 /* depois */
  /* de varias
     linhas */
acabou_finalmente
bota y = 1 /* meio */ + 2
`
	esperado := `/*
  cabecalho
     com recuo proprio
*/
se_colar x
    /* um */
    /* dois */
    mostra 1  /* depois */
    /* de varias
     linhas */
acabou_finalmente
bota y = 1 + 2  /* meio */
`
	confereIgual(t, src, esperado)
}

func TestComentarioVariosNaMesmaLinha(t *testing.T) {
	confereIgual(t, "bota x = 1 /* c */ mostra x # d\nx += 1 # soma\n",
		"bota x = 1  /* c */\nmostra x  # d\nx += 1  # soma\n")
}

func TestComentarioDentroDeStringNaoE(t *testing.T) {
	src := "mostra \"#fff ${x} /* nao */\" # esse e\nbota y = `#raw /* tb nao */`\nmostra \"${junta([\"#\"], \"\")}\"\n"
	out := formataComentarios(t, src)
	for _, esp := range []string{`mostra "#fff ${x} /* nao */"  # esse e`, `bota y = "#raw /* tb nao */"`,
		`mostra "${junta(["#"], "")}"`} {
		if !strings.Contains(out, esp) {
			t.Fatalf("esperava %q em:\n%s", esp, out)
		}
	}
}

func TestComentarioEntreDeclaracoes(t *testing.T) {
	src := `# utilitarios de conta

# soma dois
gambiarra soma(a, b)
    funciona a + b
acabou_finalmente

# dobra
gambiarra dobro(n)
    funciona n * 2
acabou_finalmente
`
	confereIgual(t, src, src)
}

func TestComentarioSozinhoNoArquivo(t *testing.T) {
	confereIgual(t, "# so isso", "# so isso\n")
	confereIgual(t, "/* so\n   isso */\n\n\n", "/* so\n   isso */\n")
	confereIgual(t, "#\n", "#\n")
	confereIgual(t, "", "")
}

// lambda escrita em varias linhas fica em bloco (com os comentarios dentro).
func TestComentarioLambdaEmBloco(t *testing.T) {
	src := `gambiarra contador()
bota estado = {"n": 0}
funciona gambiarra()   # fecha o estado
estado.n += 1          # muda o dicionario
# devolve
funciona estado.n
acabou_finalmente
acabou_finalmente
bota f = gambiarra(x) funciona x acabou_finalmente # inline fica inline
mostra mapeia(xs, gambiarra(x)
funciona x * 2 # dobra
acabou_finalmente)
`
	esperado := `gambiarra contador()
    bota estado = {"n": 0}
    funciona gambiarra()  # fecha o estado
        estado.n += 1  # muda o dicionario
        # devolve
        funciona estado.n
    acabou_finalmente
acabou_finalmente
bota f = gambiarra(x) funciona x acabou_finalmente  # inline fica inline
mostra mapeia(xs, gambiarra(x)
    funciona x * 2  # dobra
acabou_finalmente)
`
	confereIgual(t, src, esperado)
}

// lista/dicionario que abre e quebra linha fica um item por linha.
func TestComentarioListaEDictEmLinhas(t *testing.T) {
	src := `bota cfg = {   # config
    "porta": 8080,  # padrao
    # o host
    "host": "localhost",
    "rotas": [
        "/a", # a
        "/b"
    ],
}
bota xs = [1, 2,
  3]
`
	esperado := `bota cfg = {  # config
    "porta": 8080,  # padrao
    # o host
    "host": "localhost",
    "rotas": [
        "/a",  # a
        "/b"
    ]
}
bota xs = [1, 2, 3]
`
	confereIgual(t, src, esperado)
}

// comentario no meio de uma expressao que vira uma linha so sobe pra antes
// do statement (nao pode sumir nem comentar o resto da linha).
func TestComentarioNoMeioDeExpressaoSobe(t *testing.T) {
	src := `mostra soma(1, # um
  2, /* dois */ 3)
se_colar a e # cond
  b
mostra 1
acabou_finalmente
`
	esperado := `# um
mostra soma(1, 2, 3)  /* dois */
# cond
se_colar a e b
    mostra 1
acabou_finalmente
`
	confereIgual(t, src, esperado)
}

func TestConfereReprovaComentarioPerdido(t *testing.T) {
	src := "# a\nbota x = 1\n"
	if err := Confere(src, "bota x = 1\n"); err == nil {
		t.Fatal("Confere devia reprovar saida sem o comentario")
	}
	if err := Confere(src, "# a\nbota x = 2\n"); err == nil {
		t.Fatal("Confere devia reprovar saida com AST diferente")
	}
	if err := Confere(src, "# a\nbota x = \n"); err == nil {
		t.Fatal("Confere devia reprovar saida que nao parseia")
	}
	if err := Confere(src, "# a\n\nbota   x=1"); err != nil {
		t.Fatalf("Confere reprovou saida equivalente: %v", err)
	}
}

// ---- corpus: todo exemplo e todo bloco da doc ----

func confereCorpus(t *testing.T, nome, src string) {
	t.Helper()
	out, errs := FormataFonte(src)
	if len(errs) != 0 {
		return // bloco de doc que nem parseia (trecho/erro de proposito)
	}
	if err := Confere(src, out); err != nil {
		t.Errorf("%s: %v\n--- saida:\n%s", nome, err, out)
		return
	}
	if de2, _ := FormataFonte(out); de2 != out {
		t.Errorf("%s: nao e idempotente:\n--- 1a\n%s\n--- 2a\n%s", nome, out, de2)
	}
}

func TestComentarioCorpusExemplos(t *testing.T) {
	arqs, _ := filepath.Glob("../examples/*.gs")
	mods, _ := filepath.Glob("../examples/*/*.gs")
	arqs = append(arqs, mods...)
	if len(arqs) == 0 {
		t.Fatal("nao achei os exemplos")
	}
	for _, a := range arqs {
		b, err := os.ReadFile(a)
		if err != nil {
			t.Fatal(err)
		}
		confereCorpus(t, a, string(b))
	}
}

var blocoGs = regexp.MustCompile("(?s)```gambiarrascript[^\n]*\n(.*?)```")

func TestComentarioCorpusDocs(t *testing.T) {
	n := 0
	filepath.WalkDir("../web/content/docs", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".mdx") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for i, m := range blocoGs.FindAllStringSubmatch(string(b), -1) {
			n++
			confereCorpus(t, p+"#"+strconv.Itoa(i), m[1])
		}
		return nil
	})
	if n == 0 {
		t.Skip("sem docs (web/content/docs) nesse checkout")
	}
}

// TestComentarioEnxerto enfia comentario em toda linha de todo exemplo (no
// fim, no comeco, em linha propria) e exige trava ok + idempotencia. Se cair
// dentro de string, muda a string, mas tudo bem: o que importa e que a saida
// confere com a fonte enxertada.
func TestComentarioEnxerto(t *testing.T) {
	arqs, _ := filepath.Glob("../examples/*.gs")
	var fontes []string
	for _, a := range arqs {
		b, _ := os.ReadFile(a)
		fontes = append(fontes, string(b))
	}
	filepath.WalkDir("../web/content/docs", func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".mdx") {
			b, _ := os.ReadFile(p)
			for _, m := range blocoGs.FindAllStringSubmatch(string(b), -1) {
				fontes = append(fontes, m[1])
			}
		}
		return nil
	})
	fontes = append(fontes, `bota cfg = {
    "a": [
        1,
        gambiarra(x)
            funciona x
        acabou_finalmente
    ],
    "b": 2
}
se_colar algum(xs, gambiarra(x)
    funciona x > 1
acabou_finalmente)
    mostra 1
acabou_finalmente
escolhe x

caso 1
    mostra 1
acabou_finalmente
`)
	enxertos := []func(linha string) string{
		func(l string) string { return l + "  # zz" },
		func(l string) string { return l + " /* zz */" },
		func(l string) string { return "/* zz */ " + l },
		func(l string) string { return "# zz\n" + l },
		func(l string) string { return "\n\n" + l },
	}
	conferidos := 0
	defer func() { t.Logf("%d fontes, %d variantes conferidas", len(fontes), conferidos) }()
	for fi, src := range fontes {
		linhas := strings.Split(src, "\n")
		for i := range linhas {
			for ei, enx := range enxertos {
				mut := make([]string, len(linhas))
				copy(mut, linhas)
				mut[i] = enx(mut[i])
				nova := strings.Join(mut, "\n")
				out, errs := FormataFonte(nova)
				if len(errs) != 0 {
					continue
				}
				conferidos++
				if err := Confere(nova, out); err != nil {
					t.Fatalf("fonte %d linha %d enxerto %d: %v\n--- fonte:\n%s\n--- saida:\n%s", fi, i+1, ei, err, nova, out)
				}
				if de2, _ := FormataFonte(out); de2 != out {
					t.Fatalf("fonte %d linha %d enxerto %d: nao idempotente\n--- 1a\n%s\n--- 2a\n%s", fi, i+1, ei, out, de2)
				}
			}
		}
	}
}

// `/*` que nunca fecha vai ate o fim do arquivo: o \n do fim faz parte dele.
func TestComentarioBlocoSemFechar(t *testing.T) {
	confereIgual(t, "mostra 1 /* nunca fecha\nmostra 2\n\n", "mostra 1  /* nunca fecha\nmostra 2\n")
	confereIgual(t, "mostra 1\n/* nunca", "mostra 1\n/* nunca\n")
}

// multi-catch: cada `quebrou NOME se COND` numa linha, comentario no fim da
// linha do filtro fica nela, comentario antes de um quebrou fica no nivel do
// bloco que fecha.
func TestComentarioMultiCatch(t *testing.T) {
	src := `arruma
  le_arquivo("x")   # tenta
quebrou erro   se   erro_tipo(erro)=="rede"   # so rede
  mostra "rede"
  # antes do proximo
quebrou erro se erro_tipo(erro) == "jwt" ou contem(erro_msg(erro), "token") /* jwt */
  mostra "jwt"
# pega o resto
quebrou erro
  mostra "resto"  # qualquer um
finalmente
  mostra "fim"
acabou_finalmente
`
	esperado := `arruma
    le_arquivo("x")  # tenta
quebrou erro se erro_tipo(erro) == "rede"  # so rede
    mostra "rede"
    # antes do proximo
quebrou erro se erro_tipo(erro) == "jwt" ou contem(erro_msg(erro), "token")  /* jwt */
    mostra "jwt"
    # pega o resto
quebrou erro
    mostra "resto"  # qualquer um
finalmente
    mostra "fim"
acabou_finalmente
`
	confereIgual(t, src, esperado)
}

// comentario no meio de um filtro quebrado em linhas nao some
func TestComentarioMultiCatchFiltroEmLinhas(t *testing.T) {
	src := `arruma
    quebra("x")
quebrou erro se contem(
    erro_msg(erro),  # a mensagem
    "x")
    mostra 1
acabou_finalmente
`
	out := formataComentarios(t, src)
	if !strings.Contains(out, "# a mensagem") || !strings.Contains(out, `quebrou erro se contem(erro_msg(erro), "x")`) {
		t.Fatalf("filtro em linhas mal formatado:\n%s", out)
	}
}
