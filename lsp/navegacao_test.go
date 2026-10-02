package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- helpers ----

// pedir manda um request pro servidor e devolve o result (JSON cru) e o erro
// da resposta, se houver.
func pedir(t *testing.T, s *Servidor, metodo string, params interface{}) (json.RawMessage, *RespErro) {
	t.Helper()
	out := s.out.(*bytes.Buffer)
	out.Reset()
	raw, _ := json.Marshal(params)
	id := json.RawMessage(`7`)
	s.tratar(&Mensagem{Method: metodo, ID: &id, Params: raw})
	r := bufio.NewReader(bytes.NewReader(out.Bytes()))
	for {
		m, err := LerMensagem(r)
		if err != nil {
			t.Fatalf("%s: sem resposta (saida %q)", metodo, out.String())
		}
		if m.ID == nil {
			continue // notification (ex: showMessage)
		}
		res, _ := json.Marshal(m.Result)
		return res, m.Error
	}
}

func servidorCom(docs map[string]string) *Servidor {
	s := NovoServidor(&bytes.Buffer{})
	for uri, texto := range docs {
		s.docs[uri] = texto
	}
	return s
}

func posDoc(uri string, linha, col int) map[string]interface{} {
	return map[string]interface{}{
		"textDocument": map[string]string{"uri": uri},
		"position":     Posicao{Line: linha, Character: col},
	}
}

// acha devolve a Posicao LSP (UTF-16) da n-esima (0-based) ocorrencia de
// `trecho` no texto.
func acha(t *testing.T, texto, trecho string, n int) Posicao {
	t.Helper()
	for i, linha := range strings.Split(texto, "\n") {
		resto := linha
		base := 0
		for {
			k := strings.Index(resto, trecho)
			if k < 0 {
				break
			}
			if n == 0 {
				return Posicao{Line: i, Character: tamUTF16(linha[:base+k])}
			}
			n--
			base += k + len(trecho)
			resto = linha[base:]
		}
	}
	t.Fatalf("nao achei %q no texto", trecho)
	return Posicao{}
}

func definicaoEm(t *testing.T, s *Servidor, uri string, p Posicao) *Local {
	t.Helper()
	res, e := pedir(t, s, "textDocument/definition", posDoc(uri, p.Line, p.Character))
	if e != nil {
		t.Fatalf("definition deu erro: %v", e.Message)
	}
	if string(res) == "null" {
		return nil
	}
	var loc Local
	if err := json.Unmarshal(res, &loc); err != nil {
		t.Fatalf("definition: %v (%s)", err, res)
	}
	return &loc
}

func referenciasEm(t *testing.T, s *Servidor, uri string, p Posicao, comDecl bool) []Local {
	t.Helper()
	params := posDoc(uri, p.Line, p.Character)
	params["context"] = map[string]bool{"includeDeclaration": comDecl}
	res, e := pedir(t, s, "textDocument/references", params)
	if e != nil {
		t.Fatalf("references deu erro: %v", e.Message)
	}
	var locs []Local
	json.Unmarshal(res, &locs)
	return locs
}

const uriMain = "file:///proj/main.gs"

// ---- definicao ----

func TestDefinicaoGambiarraDeTopoEAninhada(t *testing.T) {
	src := `gambiarra fora(x)
    gambiarra dentro(y)
        funciona y * 2
    acabou_finalmente
    funciona dentro(x)
acabou_finalmente
mostra fora(3)`
	s := servidorCom(map[string]string{uriMain: src})
	loc := definicaoEm(t, s, uriMain, acha(t, src, "fora", 1))
	if loc == nil || loc.Range.Start != (Posicao{0, 10}) || loc.Range.End != (Posicao{0, 14}) {
		t.Fatalf("definicao de fora errada: %+v", loc)
	}
	loc = definicaoEm(t, s, uriMain, acha(t, src, "dentro", 1))
	if loc == nil || loc.Range.Start != (Posicao{1, 14}) {
		t.Fatalf("definicao de dentro errada: %+v", loc)
	}
	if loc.URI != uriMain {
		t.Fatalf("uri errada: %q", loc.URI)
	}
}

func TestDefinicaoEscopoDeFuncaoBlocosNaoAbremEscopo(t *testing.T) {
	src := `gambiarra f(xs)
    bota total = 0
    pra_cada x em xs
        se_colar x > 0
            bota ultimo = x
            bota total = total + x
        acabou_finalmente
    acabou_finalmente
    funciona [total, ultimo, x]
acabou_finalmente`
	s := servidorCom(map[string]string{uriMain: src})
	// `ultimo` lido fora do se_colar/pra_cada -> bota la de dentro
	loc := definicaoEm(t, s, uriMain, acha(t, src, "ultimo", 1))
	if loc == nil || loc.Range.Start.Line != 4 {
		t.Fatalf("ultimo deveria ir pra linha 5 (bota dentro do bloco): %+v", loc)
	}
	// reatribuicao `bota total = ...` e a leitura final -> primeira ligacao
	for _, n := range []int{1, 2, 3} {
		loc = definicaoEm(t, s, uriMain, acha(t, src, "total", n))
		if loc == nil || loc.Range.Start.Line != 1 {
			t.Fatalf("total #%d deveria ir pra linha 2: %+v", n, loc)
		}
	}
	// var do pra_cada continua valendo depois do laco
	loc = definicaoEm(t, s, uriMain, acha(t, src, "x]", 0))
	if loc == nil || loc.Range.Start != (Posicao{2, 13}) {
		t.Fatalf("x deveria ir pro pra_cada: %+v", loc)
	}
	// parametro
	loc = definicaoEm(t, s, uriMain, acha(t, src, "xs", 1))
	if loc == nil || loc.Range.Start != (Posicao{0, 12}) {
		t.Fatalf("xs deveria ir pro parametro: %+v", loc)
	}
}

func TestDefinicaoLocalSombreiaGlobalNaOrdemDeAvaliacao(t *testing.T) {
	src := `bota n = 1
gambiarra f()
    mostra n
    bota n = n + 1
    funciona n
acabou_finalmente`
	s := servidorCom(map[string]string{uriMain: src})
	// antes do bota local: le o global
	if loc := definicaoEm(t, s, uriMain, acha(t, src, "n", 1)); loc == nil || loc.Range.Start.Line != 0 {
		t.Fatalf("mostra n deveria ler o global: %+v", loc)
	}
	// lado direito do bota roda antes da ligacao: ainda e o global
	if loc := definicaoEm(t, s, uriMain, acha(t, src, "n + 1", 0)); loc == nil || loc.Range.Start.Line != 0 {
		t.Fatalf("n do lado direito deveria ler o global: %+v", loc)
	}
	// depois: o local
	if loc := definicaoEm(t, s, uriMain, acha(t, src, "funciona n", 0)); loc != nil {
		t.Fatalf("cursor no 'funciona' (keyword) nao deveria ter definicao: %+v", loc)
	}
	p := acha(t, src, "funciona n", 0)
	p.Character += len("funciona ")
	if loc := definicaoEm(t, s, uriMain, p); loc == nil || loc.Range.Start != (Posicao{3, 9}) {
		t.Fatalf("funciona n deveria ler o local: %+v", loc)
	}
}

func TestDefinicaoQuebrouDesestruturaLambda(t *testing.T) {
	src := `arruma
    quebra("x")
quebrou erro
    mostra erro_msg(erro)
acabou_finalmente
bota [a, b] = [1, 2]
bota dobra = gambiarra(v) funciona v * 2 acabou_finalmente
mostra dobra(a + b)`
	s := servidorCom(map[string]string{uriMain: src})
	if loc := definicaoEm(t, s, uriMain, acha(t, src, "erro)", 0)); loc == nil || loc.Range.Start != (Posicao{2, 8}) {
		t.Fatalf("erro deveria ir pro quebrou: %+v", loc)
	}
	if loc := definicaoEm(t, s, uriMain, acha(t, src, "b)", 0)); loc == nil || loc.Range.Start != (Posicao{5, 9}) {
		t.Fatalf("b deveria ir pra desestruturacao: %+v", loc)
	}
	if loc := definicaoEm(t, s, uriMain, acha(t, src, "v * 2", 0)); loc == nil || loc.Range.Start != (Posicao{6, 23}) {
		t.Fatalf("v deveria ir pro param da lambda: %+v", loc)
	}
	if loc := definicaoEm(t, s, uriMain, acha(t, src, "dobra(", 0)); loc == nil || loc.Range.Start.Line != 6 {
		t.Fatalf("dobra deveria ir pro bota: %+v", loc)
	}
}

func TestDefinicaoBuiltinEKeywordSemResultado(t *testing.T) {
	src := `bota xs = [1]
mostra tamanho(xs)`
	s := servidorCom(map[string]string{uriMain: src})
	if loc := definicaoEm(t, s, uriMain, acha(t, src, "tamanho", 0)); loc != nil {
		t.Fatalf("builtin nao tem definicao: %+v", loc)
	}
	if loc := definicaoEm(t, s, uriMain, acha(t, src, "mostra", 0)); loc != nil {
		t.Fatalf("keyword nao tem definicao: %+v", loc)
	}
}

func TestDefinicaoComAcentoEEmojiNaLinha(t *testing.T) {
	// 😀 ocupa 2 unidades UTF-16 e 1 rune: a coluna do LSP tem que pular 2
	src := "bota café = \"☕\"\nmostra \"😀\" + café"
	s := servidorCom(map[string]string{uriMain: src})
	p := acha(t, src, "café", 1)
	if p.Character != 14 {
		t.Fatalf("helper: esperava coluna UTF-16 14, veio %d", p.Character)
	}
	loc := definicaoEm(t, s, uriMain, p)
	if loc == nil || loc.Range.Start != (Posicao{0, 5}) || loc.Range.End != (Posicao{0, 9}) {
		t.Fatalf("definicao de café errada: %+v", loc)
	}
	refs := referenciasEm(t, s, uriMain, p, true)
	if len(refs) != 2 || refs[1].Range.Start != (Posicao{1, 14}) || refs[1].Range.End != (Posicao{1, 18}) {
		t.Fatalf("referencias de café erradas: %+v", refs)
	}
}

func TestDefinicaoDentroDeInterpolacao(t *testing.T) {
	src := "bota nome = \"x\"\nmostra \"olá ${nome}, \\\"${nome + \"!\"}\""
	s := servidorCom(map[string]string{uriMain: src})
	refs := referenciasEm(t, s, uriMain, Posicao{0, 6}, true)
	if len(refs) != 3 {
		t.Fatalf("esperava 3 referencias (decl + 2 na interpolacao), veio %+v", refs)
	}
	linha := strings.Split(src, "\n")[1]
	for _, r := range refs[1:] {
		ini := r.Range.Start.Character
		if r.Range.Start.Line != 1 || string([]rune(linha)[ini:ini+4]) != "nome" {
			t.Fatalf("referencia na interpolacao fora do lugar: %+v", r)
		}
	}
	// e o cursor la dentro tambem acha a definicao
	if loc := definicaoEm(t, s, uriMain, refs[2].Range.Start); loc == nil || loc.Range.Start.Line != 0 {
		t.Fatalf("definicao a partir da interpolacao: %+v", loc)
	}
}

// ---- entre arquivos ----

func projetoComUtil(t *testing.T) (dir, uriPrincipal, src string, s *Servidor) {
	t.Helper()
	dir = t.TempDir()
	util := `# dobra o numero
gambiarra dobro(n)
    funciona n * 2
acabou_finalmente
crava LIMITE = 10`
	if err := os.WriteFile(filepath.Join(dir, "util.gs"), []byte(util), 0644); err != nil {
		t.Fatal(err)
	}
	src = `importa "util.gs"
importa "util.gs" como u
mostra dobro(LIMITE)
mostra u.dobro(3)`
	uriPrincipal = caminhoParaURI(filepath.Join(dir, "main.gs"))
	s = servidorCom(map[string]string{uriPrincipal: src})
	return
}

func TestDefinicaoEntreArquivos(t *testing.T) {
	dir, uri, src, s := projetoComUtil(t)
	uriUtil := caminhoParaURI(filepath.Join(dir, "util.gs"))
	loc := definicaoEm(t, s, uri, acha(t, src, "dobro", 0))
	if loc == nil || loc.URI != uriUtil || loc.Range.Start != (Posicao{1, 10}) {
		t.Fatalf("dobro deveria ir pro util.gs:2: %+v", loc)
	}
	loc = definicaoEm(t, s, uri, acha(t, src, "dobro", 1)) // u.dobro
	if loc == nil || loc.URI != uriUtil || loc.Range.Start != (Posicao{1, 10}) {
		t.Fatalf("u.dobro deveria ir pro util.gs:2: %+v", loc)
	}
	loc = definicaoEm(t, s, uri, acha(t, src, "LIMITE", 0))
	if loc == nil || loc.URI != uriUtil || loc.Range.Start != (Posicao{4, 6}) {
		t.Fatalf("LIMITE deveria ir pro crava do util.gs: %+v", loc)
	}
	loc = definicaoEm(t, s, uri, acha(t, src, "u.", 0)) // o alias
	if loc == nil || loc.URI != uri || loc.Range.Start != (Posicao{1, 23}) {
		t.Fatalf("u deveria ir pro `como u`: %+v", loc)
	}
	p := acha(t, src, "util.gs", 0)
	loc = definicaoEm(t, s, uri, p)
	if loc == nil || loc.URI != uriUtil {
		t.Fatalf("cursor no caminho do importa deveria abrir o arquivo: %+v", loc)
	}
}

func TestDefinicaoEntreArquivosPrefereDocAberto(t *testing.T) {
	dir, uri, src, s := projetoComUtil(t)
	// util.gs aberto com mudanca nao salva: a definicao muda de linha
	uriUtil := caminhoParaURI(filepath.Join(dir, "util.gs"))
	s.docs[uriUtil] = "\n\ngambiarra dobro(n)\n    funciona n\nacabou_finalmente"
	loc := definicaoEm(t, s, uri, acha(t, src, "dobro", 0))
	if loc == nil || loc.URI != uriUtil || loc.Range.Start.Line != 2 {
		t.Fatalf("deveria usar o texto aberto do util.gs: %+v", loc)
	}
}

func TestReferenciasEntreArquivos(t *testing.T) {
	dir, uri, src, s := projetoComUtil(t)
	uriUtil := caminhoParaURI(filepath.Join(dir, "util.gs"))
	refs := referenciasEm(t, s, uri, acha(t, src, "dobro", 0), true)
	// main: dobro(LIMITE) e u.dobro(3); util: a declaracao
	porURI := map[string]int{}
	for _, r := range refs {
		porURI[r.URI]++
	}
	if porURI[uri] != 2 || porURI[uriUtil] != 1 {
		t.Fatalf("referencias entre arquivos erradas: %+v", refs)
	}
	semDecl := referenciasEm(t, s, uri, acha(t, src, "dobro", 0), false)
	if len(semDecl) != 2 {
		t.Fatalf("sem a declaracao deveria sobrar 2: %+v", semDecl)
	}
}

// ---- referencias ----

func TestReferenciasLocalNaoMisturaEscopos(t *testing.T) {
	src := `bota x = 1
gambiarra f(x)
    funciona x + 1
acabou_finalmente
gambiarra g()
    funciona x
acabou_finalmente
mostra f(x) + g()`
	s := servidorCom(map[string]string{uriMain: src})
	global := referenciasEm(t, s, uriMain, Posicao{0, 5}, true)
	// decl, `funciona x` do g (closure le o global) e f(x)
	if len(global) != 3 {
		t.Fatalf("x global: esperava 3 referencias, veio %+v", global)
	}
	param := referenciasEm(t, s, uriMain, acha(t, src, "x + 1", 0), true)
	if len(param) != 2 || param[0].Range.Start != (Posicao{1, 12}) {
		t.Fatalf("x param: esperava 2 referencias, veio %+v", param)
	}
	semDecl := referenciasEm(t, s, uriMain, acha(t, src, "x + 1", 0), false)
	if len(semDecl) != 1 {
		t.Fatalf("includeDeclaration=false deveria tirar a declaracao: %+v", semDecl)
	}
}

func TestReferenciasAtribuicaoComposta(t *testing.T) {
	src := `bota soma = 0
soma += 2
mostra soma`
	s := servidorCom(map[string]string{uriMain: src})
	refs := referenciasEm(t, s, uriMain, Posicao{2, 8}, true)
	if len(refs) != 3 {
		t.Fatalf("esperava 3 referencias (sem duplicar o += ), veio %+v", refs)
	}
}

// ---- renomear ----

type edicaoWS struct {
	Changes map[string][]EdicaoTexto `json:"changes"`
}

func renomearEm(t *testing.T, s *Servidor, uri string, p Posicao, novo string) (*edicaoWS, *RespErro) {
	t.Helper()
	params := posDoc(uri, p.Line, p.Character)
	params["newName"] = novo
	res, e := pedir(t, s, "textDocument/rename", params)
	if e != nil {
		return nil, e
	}
	var w edicaoWS
	json.Unmarshal(res, &w)
	return &w, nil
}

// aplica aplica as edicoes (todas numa linha cada) no texto.
func aplica(texto string, eds []EdicaoTexto) string {
	linhas := strings.Split(texto, "\n")
	ls := novasLinhas(texto)
	// de tras pra frente pra nao bagunçar as colunas
	for i := len(eds) - 1; i >= 0; i-- {
		e := eds[i]
		l := e.Range.Start.Line
		rs := []rune(linhas[l])
		ini := ls.utf16ParaRune(l, e.Range.Start.Character)
		fim := ls.utf16ParaRune(l, e.Range.End.Character)
		linhas[l] = string(rs[:ini]) + e.NewText + string(rs[fim:])
	}
	return strings.Join(linhas, "\n")
}

func TestRenomearVariavelComAcentoEInterpolacao(t *testing.T) {
	src := "bota preço = 10\nbota preço = preço * 2\nmostra \"😀 custa ${preço}\""
	s := servidorCom(map[string]string{uriMain: src})
	w, e := renomearEm(t, s, uriMain, acha(t, src, "preço", 2), "valor_ção")
	if e != nil {
		t.Fatalf("rename falhou: %s", e.Message)
	}
	got := aplica(src, w.Changes[uriMain])
	want := "bota valor_ção = 10\nbota valor_ção = valor_ção * 2\nmostra \"😀 custa ${valor_ção}\""
	if got != want {
		t.Fatalf("rename errado:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenomearSoOEscopoCerto(t *testing.T) {
	src := `bota x = 1
gambiarra f(x)
    funciona x + 1
acabou_finalmente
mostra f(x)`
	s := servidorCom(map[string]string{uriMain: src})
	w, e := renomearEm(t, s, uriMain, acha(t, src, "x + 1", 0), "y")
	if e != nil {
		t.Fatalf("rename falhou: %s", e.Message)
	}
	got := aplica(src, w.Changes[uriMain])
	if !strings.Contains(got, "gambiarra f(y)") || !strings.Contains(got, "funciona y + 1") || !strings.Contains(got, "bota x = 1") || !strings.Contains(got, "mostra f(x)") {
		t.Fatalf("rename do param vazou de escopo:\n%s", got)
	}
}

func TestRenomearRecusa(t *testing.T) {
	src := `bota x = 1
bota y = 2
bota {nome} = {"nome": "a"}
mostra tamanho([x, y, nome])`
	s := servidorCom(map[string]string{uriMain: src})
	casos := []struct {
		pos  Posicao
		novo string
		msg  string
	}{
		{acha(t, src, "tamanho", 0), "t", "builtin"},
		{acha(t, src, "mostra", 0), "m", "keyword"},
		{Posicao{0, 5}, "1abc", "nome valido"},
		{Posicao{0, 5}, "bota", "nome valido"},
		{Posicao{0, 5}, "y", "ja existe"},
		{Posicao{0, 5}, "tamanho", "builtin"},
		{acha(t, src, "nome}", 0), "outro", "chave"},
	}
	for _, c := range casos {
		_, e := renomearEm(t, s, uriMain, c.pos, c.novo)
		if e == nil || !strings.Contains(e.Message, c.msg) {
			t.Errorf("rename %v -> %q deveria recusar com %q, veio %+v", c.pos, c.novo, c.msg, e)
		}
	}
}

func TestRenomearRecusaCaptura(t *testing.T) {
	// renomear o global x pra `y` faria o `x` dentro de f cair no y local
	src := `bota x = 1
gambiarra f()
    bota y = 2
    funciona x + y
acabou_finalmente`
	s := servidorCom(map[string]string{uriMain: src})
	if _, e := renomearEm(t, s, uriMain, Posicao{0, 5}, "y"); e == nil {
		t.Fatal("deveria recusar: o y local capturaria a referencia")
	}
}

func TestPrepararRenomear(t *testing.T) {
	src := "bota ação = 1\nmostra ação"
	s := servidorCom(map[string]string{uriMain: src})
	res, e := pedir(t, s, "textDocument/prepareRename", posDoc(uriMain, 1, 9))
	if e != nil {
		t.Fatalf("prepareRename falhou: %s", e.Message)
	}
	var r struct {
		Range       Faixa  `json:"range"`
		Placeholder string `json:"placeholder"`
	}
	json.Unmarshal(res, &r)
	if r.Placeholder != "ação" || r.Range.Start != (Posicao{1, 7}) || r.Range.End != (Posicao{1, 11}) {
		t.Fatalf("prepareRename errado: %+v", r)
	}
	if _, e := pedir(t, s, "textDocument/prepareRename", posDoc(uriMain, 1, 2)); e == nil {
		t.Fatal("prepareRename na keyword deveria dar erro")
	}
}

func TestRenomearEntreArquivos(t *testing.T) {
	dir, uri, src, s := projetoComUtil(t)
	uriUtil := caminhoParaURI(filepath.Join(dir, "util.gs"))
	w, e := renomearEm(t, s, uri, acha(t, src, "dobro", 1), "duplica")
	if e != nil {
		t.Fatalf("rename falhou: %s", e.Message)
	}
	got := aplica(src, w.Changes[uri])
	if !strings.Contains(got, "mostra duplica(LIMITE)") || !strings.Contains(got, "u.duplica(3)") {
		t.Fatalf("main.gs nao renomeou:\n%s", got)
	}
	if len(w.Changes[uriUtil]) != 1 {
		t.Fatalf("util.gs deveria ter 1 edicao (a declaracao): %+v", w.Changes)
	}
}

// ---- protocolo ----

func TestInitializeAnunciaCapacidades(t *testing.T) {
	s := servidorCom(nil)
	res, _ := pedir(t, s, "initialize", map[string]string{"rootUri": "file:///proj"})
	var r struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	json.Unmarshal(res, &r)
	for _, c := range []string{"definitionProvider", "referencesProvider", "renameProvider",
		"documentFormattingProvider", "signatureHelpProvider", "documentSymbolProvider",
		"completionProvider", "hoverProvider"} {
		if _, ok := r.Capabilities[c]; !ok {
			t.Errorf("faltou %s no initialize: %s", c, res)
		}
	}
	if !strings.Contains(string(r.Capabilities["signatureHelpProvider"]), `"("`) {
		t.Errorf("signatureHelp deveria disparar no '(': %s", r.Capabilities["signatureHelpProvider"])
	}
	if s.raiz != filepath.Clean("/proj") {
		t.Errorf("raiz do workspace nao guardada: %q", s.raiz)
	}
}

func TestMetodoDesconhecidoRespondeErro(t *testing.T) {
	s := servidorCom(nil)
	_, e := pedir(t, s, "textDocument/codeLens", map[string]string{})
	if e == nil || e.Code != erroMetodoNaoExiste {
		t.Fatalf("request desconhecido deveria dar MethodNotFound, veio %+v", e)
	}
}

func TestInterpolacaoCrasEAninhada(t *testing.T) {
	// crase tambem interpola; string dentro de ${} com outra ${} dentro
	src := "bota açúcar = 1\nmostra `cru ${açúcar}` + \"a ${texto(\"${açúcar}\")} b\""
	s := servidorCom(map[string]string{uriMain: src})
	refs := referenciasEm(t, s, uriMain, Posicao{0, 6}, true)
	if len(refs) != 3 {
		t.Fatalf("esperava 3 referencias, veio %+v", refs)
	}
	linha := []rune(strings.Split(src, "\n")[1])
	for _, r := range refs[1:] {
		ini := r.Range.Start.Character
		if string(linha[ini:ini+6]) != "açúcar" {
			t.Fatalf("referencia fora do lugar: %+v (%q)", r, string(linha[ini:ini+6]))
		}
	}
}

func TestImportaCiclicoNaoTrava(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.gs"), []byte("importa \"b.gs\"\ngambiarra fa() funciona fb() acabou_finalmente"), 0644)
	os.WriteFile(filepath.Join(dir, "b.gs"), []byte("importa \"a.gs\"\ngambiarra fb() funciona fa() acabou_finalmente"), 0644)
	uri := caminhoParaURI(filepath.Join(dir, "a.gs"))
	src := "importa \"b.gs\"\ngambiarra fa() funciona fb() acabou_finalmente\nmostra nada_aqui()"
	s := servidorCom(map[string]string{uri: src})
	loc := definicaoEm(t, s, uri, acha(t, src, "fb", 0))
	if loc == nil || !strings.HasSuffix(loc.URI, "/b.gs") {
		t.Fatalf("fb deveria ir pro b.gs: %+v", loc)
	}
	if loc := definicaoEm(t, s, uri, acha(t, src, "nada_aqui", 0)); loc != nil {
		t.Fatalf("nome inexistente nao tem definicao: %+v", loc)
	}
	refs := referenciasEm(t, s, uri, acha(t, src, "fa", 0), true)
	if len(refs) != 2 {
		t.Fatalf("fa: decl no a.gs + uso no b.gs, veio %+v", refs)
	}
}

func TestConversaoUTF16(t *testing.T) {
	ls := novasLinhas("a😀ção")
	if got := ls.runeParaUTF16(0, 2); got != 3 {
		t.Errorf("runeParaUTF16 = %d, want 3", got)
	}
	if got := ls.utf16ParaRune(0, 3); got != 2 {
		t.Errorf("utf16ParaRune = %d, want 2", got)
	}
	if got := ls.utf16ParaRune(0, 2); got != 2 {
		t.Errorf("meio do surrogate deveria ir pra rune seguinte, veio %d", got)
	}
	if got := uriParaCaminho(caminhoParaURI("/a b/ção.gs")); got != filepath.Clean("/a b/ção.gs") {
		t.Errorf("ida e volta da URI: %q", got)
	}
}
