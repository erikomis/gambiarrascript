package vm

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gambiarrascript/interpreter"
)

// Servidor parte 3: upload (multipart e urlencoded), limite de corpo (413),
// cookies assinados, sessao em cookie, limita() (429) e comprime() (gzip).
// Mesmo fonte nos dois engines, igual a parte 2.

// multipartTeste monta um corpo multipart; arquivo = [campo, nomeArquivo, tipo, conteudo].
func multipartTeste(t *testing.T, campos [][2]string, arquivos [][4]string) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, c := range campos {
		if err := mw.WriteField(c[0], c[1]); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range arquivos {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, a[0], a[1]))
		if a[2] != "" {
			h.Set("Content-Type", a[2])
		}
		p, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(p, a[3])
	}
	mw.Close()
	return buf.String(), mw.FormDataContentType()
}

func TestUploadMultipartNosDois(t *testing.T) {
	pasta := t.TempDir()
	fonte := `rota("POST", "/eco", gambiarra(pedido)
    funciona pra_json({"campos": pedido.campos, "arquivos": pedido.arquivos})
acabou_finalmente)
rota("POST", "/salva", gambiarra(pedido)
    bota c = salva_arquivo(pedido.arquivos.doc, ` + fmt.Sprintf("%q", pasta) + `)
    funciona separa(c, "/")[-1]
acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		corpo, ct := multipartTeste(t,
			[][2]string{{"nome", "Zé"}, {"tag", "a"}, {"tag", "b"}},
			[][4]string{
				{"doc", "../../etc/passwd", "text/plain", "oi"},
				{"fotos", "um.png", "image/png", "\x89PNG"},
				{"fotos", `C:\fotos\dois.png`, "", "xy"},
				{"vazio", "", "", ""},
			})
		pede(t, "POST", c.base+"/eco", corpo, "Content-Type", ct).confere(t, "multipart", 200,
			`{"campos":{"nome":"Zé","tag":["a","b"]},"arquivos":{"doc":{"nome":"passwd","tipo":"text/plain","tamanho":2,"conteudo_base64":"b2k="},`+
				`"fotos":[{"nome":"um.png","tipo":"image/png","tamanho":4,"conteudo_base64":"iVBORw=="},{"nome":"dois.png","tipo":"application/octet-stream","tamanho":2,"conteudo_base64":"eHk="}]}}`)

		pede(t, "POST", c.base+"/eco", "a=1&b=x+y&a=2&c=%C3%A9", "Content-Type", "application/x-www-form-urlencoded").
			confere(t, "urlencoded", 200, `{"campos":{"a":["1","2"],"b":"x y","c":"é"},"arquivos":{}}`)
		pede(t, "POST", c.base+"/eco", `{"a":1}`, "Content-Type", "application/json").
			confere(t, "json nao e formulario", 200, `{"campos":{},"arquivos":{}}`)

		r := pede(t, "POST", c.base+"/eco", "--xyz\r\nlixo", "Content-Type", "multipart/form-data; boundary=xyz")
		if r.status != 400 || !strings.HasPrefix(r.corpo, "esse formulario do corpo ta quebrado, parca") {
			t.Errorf("multipart quebrado: %d %q", r.status, r.corpo)
		}

		// salva duas vezes o mesmo nome: o segundo vira passwd-1, nada sobrescrito
		corpo, ct = multipartTeste(t, nil, [][4]string{{"doc", "../passwd", "text/plain", "conteudo"}})
		r1 := pede(t, "POST", c.base+"/salva", corpo, "Content-Type", ct)
		r2 := pede(t, "POST", c.base+"/salva", corpo, "Content-Type", ct)
		if r1.status != 200 || r2.status != 200 || r1.corpo == r2.corpo {
			t.Fatalf("salva: %d %q / %d %q", r1.status, r1.corpo, r2.status, r2.corpo)
		}
		for _, nome := range []string{r1.corpo, r2.corpo} {
			b, err := os.ReadFile(filepath.Join(pasta, nome))
			if err != nil || string(b) != "conteudo" {
				t.Errorf("arquivo %s: %q %v", nome, b, err)
			}
		}
	})
	if got, _ := filepath.Glob(filepath.Join(pasta, "*")); len(got) != 4 {
		t.Errorf("esperava 4 arquivos (2 por engine) na pasta, veio %v", got)
	}
}

func TestSalvaArquivoNaoSaiDaPasta(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "uploads")
	q := fmt.Sprintf("%q", pasta)
	esperaNosDois(t, `salva_arquivo({"nome": "x.txt", "conteudo_base64": "b2k="}, `+q+`, "../fora.txt")`, "",
		`salva_arquivo(): o nome "../fora.txt" tem que ser so um nome de arquivo, sem pasta nem ".." (nada de sair da pasta, parca)`)
	esperaNosDois(t, `salva_arquivo({"nome": "x.txt", "conteudo_base64": "b2k="}, `+q+`, "..")`, "",
		`tem que ser so um nome de arquivo`)
	esperaNosDois(t, `salva_arquivo({"nome": "x"}, `+q+`)`, "",
		`salva_arquivo(): esse dicionario nao tem "conteudo_base64"`)
	esperaNosDois(t, `salva_arquivo("x", `+q+`)`, "",
		`salva_arquivo(): o 1o argumento tem que ser um arquivo do pedido.arquivos`)
	// dicionario montado na mao com nome malandro: sanitiza de novo e fica na pasta
	esperaNosDois(t, `bota c = salva_arquivo({"nome": "../../../fora.txt", "conteudo_base64": "b2k="}, `+q+`)
mostra comeca_com(separa(c, "/")[-1], "fora")`, "deu_bom\n", "")
	if _, err := os.Stat(filepath.Join(filepath.Dir(pasta), "fora.txt")); err == nil {
		t.Error("salva_arquivo escreveu fora da pasta")
	}
	if b, err := os.ReadFile(filepath.Join(pasta, "fora.txt")); err != nil || string(b) != "oi" {
		t.Errorf("fora.txt dentro da pasta: %q %v", b, err)
	}
}

func TestCorpoGrandeDa413NosDois(t *testing.T) {
	fonte := `rota("POST", "/x", gambiarra(pedido) funciona "leu " + texto(tamanho(pedido.corpo)) acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		limite := 10 << 20
		pede(t, "POST", c.base+"/x", strings.Repeat("a", limite)).confere(t, "no limite", 200, fmt.Sprintf("leu %d", limite))
		pede(t, "POST", c.base+"/x", strings.Repeat("a", limite+1)).
			confere(t, "passou", 413, "corpo grande demais, parca (o limite e 10485760 bytes)")
	})
}

// escuta com {"max_corpo": 100}: sobe de verdade (porta 0), o 413 vem do
// limite configurado, inclusive sem Content-Length (chunked).
func TestEscutaMaxCorpoNosDois(t *testing.T) {
	fonte := `rota("POST", "/x", gambiarra(pedido) funciona "ok " + texto(tamanho(pedido.corpo)) acabou_finalmente)
escuta("127.0.0.1:0", {"max_corpo": 100})`
	for _, m := range motores {
		t.Run(m.nome, func(t *testing.T) {
			pr, pw := io.Pipe()
			pronto := make(chan *interpreter.Interpreter, 1)
			fim := make(chan struct{})
			go func() {
				defer close(fim)
				rodaComGancho(t, m.nome, fonte, &bufTravado{}, pw, pronto)
			}()
			i := <-pronto
			linha, err := bufio.NewReader(pr).ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			go io.Copy(io.Discard, pr)
			sub := regexp.MustCompile(`http://127\.0\.0\.1:(\d+) `).FindStringSubmatch(linha)
			if sub == nil {
				t.Fatalf("linha do escuta: %q", linha)
			}
			defer func() {
				i.PararServidor()
				select {
				case <-fim:
				case <-time.After(8 * time.Second):
					t.Error("escuta nao voltou")
				}
			}()
			base := "http://127.0.0.1:" + sub[1]
			pede(t, "POST", base+"/x", strings.Repeat("a", 100)).confere(t, "100", 200, "ok 100")
			pede(t, "POST", base+"/x", strings.Repeat("a", 101)).confere(t, "101", 413, "corpo grande demais, parca (o limite e 100 bytes)")
			// sem Content-Length: o MaxBytesReader corta na leitura
			resp, err := http.Post(base+"/x", "text/plain", io.MultiReader(strings.NewReader(strings.Repeat("b", 150))))
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 413 {
				t.Errorf("chunked: %d %q", resp.StatusCode, b)
			}
		})
	}
	esperaNosDois(t, `escuta("127.0.0.1:0", {"max_corpo": 0})`, "",
		`escuta(): "max_corpo" tem que ser um numero inteiro de bytes maior que 0, veio 0`)
}

func TestDefineCookieELeCookieNosDois(t *testing.T) {
	fonte := `bota SEGREDO = "0123456789abcdef0123456789abcdef"
rota("GET", "/define", gambiarra(pedido)
    bota r = responde_html("<p>oi</p>")
    define_cookie(r, "tema", "escuro")
    define_cookie(r, "lembra", "1", {"expira_em": 3600, "seguro": deu_bom, "mesmo_site": "Strict", "http_only": deu_ruim})
    define_cookie(r, "usuario", "42", {"segredo": SEGREDO})
    funciona r
acabou_finalmente)
rota("GET", "/apaga", gambiarra()
    funciona define_cookie({}, "tema", "", {"expira_em": 0})
acabou_finalmente)
rota("GET", "/le", gambiarra(pedido)
    funciona pra_json([le_cookie(pedido, "tema"), le_cookie(pedido, "usuario", SEGREDO), le_cookie(pedido, "nao_tem")])
acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		r := pede(t, "GET", c.base+"/define", "")
		r.confere(t, "define", 200, "<p>oi</p>")
		cs := r.cab.Values("Set-Cookie")
		if len(cs) != 3 {
			t.Fatalf("Set-Cookie: %q", cs)
		}
		if cs[0] != "tema=escuro; Path=/; HttpOnly; SameSite=Lax" {
			t.Errorf("padrao: %q", cs[0])
		}
		if cs[1] != "lembra=1; Path=/; Max-Age=3600; Secure; SameSite=Strict" {
			t.Errorf("opcoes: %q", cs[1])
		}
		assinado := strings.TrimSuffix(strings.TrimPrefix(cs[2], "usuario="), "; Path=/; HttpOnly; SameSite=Lax")
		if !strings.HasPrefix(assinado, "42.") || len(assinado) != 3+43 {
			t.Errorf("assinado: %q", cs[2])
		}
		pede(t, "GET", c.base+"/apaga", "").cabecalho(t, "apaga", "Set-Cookie", "tema=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax")

		pede(t, "GET", c.base+"/le", "", "Cookie", "tema=claro; usuario="+assinado).confere(t, "le ok", 200, `["claro","42",null]`)
		adulterado := "43" + assinado[2:]
		pede(t, "GET", c.base+"/le", "", "Cookie", "usuario="+adulterado).confere(t, "adulterado", 200, `[null,null,null]`)
		pede(t, "GET", c.base+"/le", "", "Cookie", "usuario=42").confere(t, "sem assinatura", 200, `[null,null,null]`)
	})
	esperaNosDois(t, `define_cookie({}, "a", "tem;ponto")`, "", "define_cookie(): cookie invalido")
	esperaNosDois(t, `define_cookie({"x": 1}, "a", "b")`, "", "define_cookie(): o 1o argumento tem que ser um dicionario de resposta")
	esperaNosDois(t, `define_cookie({}, "a", "b", {"segredo": "curto"})`, "", `"segredo" tem que ser texto de pelo menos 32 caracteres`)
	esperaNosDois(t, `define_cookie({}, "a", "b", {"mesmo_site": "None"})`, "", `"mesmo_site": "None" so vale com "seguro": deu_bom`)
	esperaNosDois(t, `define_cookie({}, "a", "b", {"validade": 1})`, "", `define_cookie(): opcao "validade" nao existe`)
	esperaNosDois(t, `mostra define_cookie({}, "a", "b")`,
		`{"cabecalhos": {"Set-Cookie": "a=b; Path=/; HttpOnly; SameSite=Lax"}}`+"\n", "")
}

// clienteComJar segue o cookie entre pedidos, igual navegador.
func clienteComJar(t *testing.T) *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func getCom(t *testing.T, cl *http.Client, url string, cab ...string) respTeste {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	for i := 0; i+1 < len(cab); i += 2 {
		req.Header.Add(cab[i], cab[i+1])
	}
	resp, err := cl.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return respTeste{resp.StatusCode, string(b), resp.Header}
}

func TestSessaoNosDois(t *testing.T) {
	for _, cripto := range []string{"deu_ruim", "deu_bom"} {
		fonte := `usa_sessao({"segredo": "0123456789abcdef0123456789abcdef", "encripta": ` + cripto + `})
rota("GET", "/conta", gambiarra(pedido)
    bota pedido.sessao["visitas"] = (pedido.sessao.visitas ?? 0) + 1
    funciona "visitas: " + texto(pedido.sessao.visitas)
acabou_finalmente)
rota("GET", "/olha", gambiarra(pedido)
    funciona pra_json(pedido.sessao)
acabou_finalmente)
rota("GET", "/sai", gambiarra(pedido)
    bota pedido["sessao"] = {}
    funciona "tchau"
acabou_finalmente)
rota("GET", "/ruim", gambiarra(pedido)
    bota pedido.sessao["f"] = gambiarra() funciona 1 acabou_finalmente
    funciona "nao chega"
acabou_finalmente)`
		t.Run(cripto, func(t *testing.T) {
			servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
				cl := clienteComJar(t)
				for n := 1; n <= 3; n++ {
					getCom(t, cl, c.base+"/conta").confere(t, "conta", 200, fmt.Sprintf("visitas: %d", n))
				}
				// so ler nao reemite o cookie
				r := getCom(t, cl, c.base+"/olha")
				r.confere(t, "olha", 200, `{"visitas":3}`)
				if v := r.cab.Values("Set-Cookie"); len(v) != 0 {
					t.Errorf("leitura reemitiu cookie: %q", v)
				}
				// o cookie: HttpOnly + Lax; cifrado nao mostra o conteudo
				r = pede(t, "GET", c.base+"/conta", "")
				sc := r.cab.Get("Set-Cookie")
				if !strings.HasPrefix(sc, "gs_sessao=") || !strings.HasSuffix(sc, "; Path=/; Max-Age=604800; HttpOnly; SameSite=Lax") {
					t.Errorf("Set-Cookie: %q", sc)
				}
				valor := strings.TrimPrefix(strings.SplitN(sc, ";", 2)[0], "gs_sessao=")
				carga := strings.SplitN(valor, ".", 2)[0]
				if cripto == "deu_bom" && strings.Contains(carga, "dmlzaXRhc") { // "visitas" em base64
					t.Errorf("sessao cifrada mostra o conteudo: %q", valor)
				}
				pede(t, "GET", c.base+"/olha", "", "Cookie", "gs_sessao="+valor).confere(t, "cookie certo", 200, `{"visitas":1}`)
				// adulterado (um caractere da carga trocado), sem assinatura, lixo: sessao vazia
				troca := byte('A')
				if valor[5] == 'A' {
					troca = 'B'
				}
				adulterado := valor[:5] + string(troca) + valor[6:]
				for _, ruim := range []string{adulterado, carga, "lixo", carga + ".", ""} {
					pede(t, "GET", c.base+"/olha", "", "Cookie", "gs_sessao="+ruim).confere(t, "adulterado "+ruim, 200, `{}`)
				}
				pede(t, "GET", c.base+"/conta", "", "Cookie", "gs_sessao="+adulterado).confere(t, "adulterado recomeca", 200, "visitas: 1")

				r = getCom(t, cl, c.base+"/sai")
				r.confere(t, "sai", 200, "tchau")
				getCom(t, cl, c.base+"/olha").confere(t, "depois de sair", 200, `{}`)
				// sem sessao nenhuma, /sai nao manda cookie
				if v := pede(t, "GET", c.base+"/sai", "").cab.Values("Set-Cookie"); len(v) != 0 {
					t.Errorf("sai sem sessao: %q", v)
				}
				pede(t, "GET", c.base+"/ruim", "").confere(t, "nao serializa", 500, "deu ruim no servidor (erro interno), parca")
				if !strings.Contains(c.err.String(), "pedido.sessao nao virou json") {
					t.Errorf("stderr: %q", c.err.String())
				}
			})
		})
	}
	esperaNosDois(t, `usa_sessao({})`, "", `usa_sessao(): falta o "segredo"`)
	esperaNosDois(t, `usa_sessao({"segredo": "curto"})`, "", `"segredo" tem que ser texto de pelo menos 32 caracteres`)
	esperaNosDois(t, `usa_sessao({"segredo": "0123456789abcdef0123456789abcdef", "cor": 1})`, "", `usa_sessao(): opcao "cor" nao existe`)
}

func TestLimitaNosDois(t *testing.T) {
	fonte := `limita({"por_minuto": 2})
limita({"por_minuto": 60, "rajada": 1, "prefixo": "/login", "chave": gambiarra(pedido)
    funciona pedido.cabecalhos["X-Usuario"]
acabou_finalmente})
rota("GET", "/", gambiarra() funciona "oi" acabou_finalmente)
rota("POST", "/login", gambiarra() funciona "entrou" acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		pede(t, "GET", c.base+"/", "").confere(t, "1", 200, "oi")
		pede(t, "GET", c.base+"/", "").confere(t, "2", 200, "oi")
		r := pede(t, "GET", c.base+"/", "")
		r.confere(t, "3", 429, "calma la, parca: pedido demais (tenta de novo em 30s)")
		r.cabecalho(t, "429", "Retry-After", "30")
		pede(t, "POST", c.base+"/login", "", "X-Usuario", "ana").confere(t, "login ip estourado", 429, "calma la, parca: pedido demais (tenta de novo em 30s)")
	})
	fonte = `limita({"por_minuto": 60, "rajada": 1, "prefixo": "/login", "chave": gambiarra(pedido)
    funciona pedido.cabecalhos["X-Usuario"]
acabou_finalmente})
rota("GET", "/", gambiarra() funciona "oi" acabou_finalmente)
rota("POST", "/login", gambiarra() funciona "entrou" acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		pede(t, "POST", c.base+"/login", "", "X-Usuario", "ana").confere(t, "ana 1", 200, "entrou")
		r := pede(t, "POST", c.base+"/login", "", "X-Usuario", "ana")
		r.confere(t, "ana 2", 429, "calma la, parca: pedido demais (tenta de novo em 1s)")
		r.cabecalho(t, "ana 2", "Retry-After", "1")
		pede(t, "POST", c.base+"/login", "", "X-Usuario", "bia").confere(t, "bia", 200, "entrou")
		// chave nada = fora do limite
		pede(t, "POST", c.base+"/login", "").confere(t, "sem chave 1", 200, "entrou")
		pede(t, "POST", c.base+"/login", "").confere(t, "sem chave 2", 200, "entrou")
		for n := 0; n < 5; n++ {
			pede(t, "GET", c.base+"/", "").confere(t, "fora do prefixo", 200, "oi")
		}
	})
	// concorrencia: 40 pedidos em paralelo contra um balde de 10 → exatos 10 passam
	servidorNosDois(t, `limita({"por_minuto": 10})
rota("GET", "/", gambiarra() funciona "oi" acabou_finalmente)`, func(t *testing.T, c ctxServidor) {
		var ok, barrado atomic.Int32
		var wg sync.WaitGroup
		for n := 0; n < 40; n++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := http.Get(c.base + "/")
				if err != nil {
					t.Error(err)
					return
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				switch resp.StatusCode {
				case 200:
					ok.Add(1)
				case 429:
					barrado.Add(1)
				}
			}()
		}
		wg.Wait()
		if ok.Load() != 10 || barrado.Load() != 30 {
			t.Errorf("paralelo: %d ok, %d barrados", ok.Load(), barrado.Load())
		}
	})
	esperaNosDois(t, `limita({})`, "", `limita(): falta o "por_minuto"`)
	esperaNosDois(t, `limita({"por_minuto": 0})`, "", `limita(): "por_minuto" tem que ser numero inteiro maior que 0`)
	esperaNosDois(t, `limita({"por_minuto": 5, "chave": "cookie"})`, "", `limita(): "chave" tem que ser "ip" ou uma gambiarra(pedido)`)
}

func gunzipTeste(t *testing.T, s string) string {
	t.Helper()
	zr, err := gzip.NewReader(strings.NewReader(s))
	if err != nil {
		t.Fatalf("nao e gzip: %v", err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestComprimeNosDois(t *testing.T) {
	grande := strings.Repeat("gambiarra ", 200) // 2000 bytes
	pasta := t.TempDir()
	os.WriteFile(filepath.Join(pasta, "site.css"), []byte(grande), 0o644)
	os.WriteFile(filepath.Join(pasta, "foto.png"), []byte(grande), 0o644)
	fonte := `comprime()
serve_pasta("/estatico", ` + fmt.Sprintf("%q", pasta) + `)
bota GRANDE = "` + grande + `"
rota("GET", "/texto", gambiarra() funciona GRANDE acabou_finalmente)
rota("GET", "/json", gambiarra() funciona responde_json({"x": GRANDE}) acabou_finalmente)
rota("GET", "/pequeno", gambiarra() funciona "oi" acabou_finalmente)
rota("GET", "/png", gambiarra() funciona {"corpo": GRANDE, "cabecalhos": {"Content-Type": "image/png"}} acabou_finalmente)
rota("GET", "/ja", gambiarra() funciona {"corpo": GRANDE, "cabecalhos": {"Content-Type": "text/plain", "Content-Encoding": "br"}} acabou_finalmente)`
	servidorNosDois(t, fonte, func(t *testing.T, c ctxServidor) {
		r := pede(t, "GET", c.base+"/texto", "", "Accept-Encoding", "gzip, deflate, br")
		r.cabecalho(t, "texto", "Content-Encoding", "gzip")
		r.cabecalho(t, "texto", "Vary", "Accept-Encoding")
		r.cabecalho(t, "texto", "Content-Type", "text/plain; charset=utf-8")
		if r.status != 200 || len(r.corpo) >= len(grande) || gunzipTeste(t, r.corpo) != grande {
			t.Errorf("texto comprimido: %d, %d bytes", r.status, len(r.corpo))
		}
		r = pede(t, "GET", c.base+"/json", "", "Accept-Encoding", "gzip")
		r.cabecalho(t, "json", "Content-Encoding", "gzip")
		if gunzipTeste(t, r.corpo) != `{"x":"`+grande+`"}` {
			t.Error("json comprimido nao bate")
		}
		// cliente que nao pede gzip: cru, mas com Vary
		r = pede(t, "GET", c.base+"/texto", "", "Accept-Encoding", "identity")
		r.confere(t, "sem gzip", 200, grande)
		r.cabecalho(t, "sem gzip", "Content-Encoding", "")
		r.cabecalho(t, "sem gzip", "Vary", "Accept-Encoding")
		pede(t, "GET", c.base+"/texto", "", "Accept-Encoding", "gzip;q=0").cabecalho(t, "q=0", "Content-Encoding", "")
		pede(t, "GET", c.base+"/texto", "", "Accept-Encoding", "*").cabecalho(t, "*", "Content-Encoding", "gzip")

		r = pede(t, "GET", c.base+"/pequeno", "", "Accept-Encoding", "gzip")
		r.confere(t, "pequeno", 200, "oi")
		r.cabecalho(t, "pequeno", "Content-Encoding", "")
		r = pede(t, "GET", c.base+"/png", "", "Accept-Encoding", "gzip")
		r.confere(t, "png", 200, grande)
		r.cabecalho(t, "png", "Content-Encoding", "")
		r.cabecalho(t, "png", "Vary", "")
		r = pede(t, "GET", c.base+"/ja", "", "Accept-Encoding", "gzip")
		r.confere(t, "ja comprimido", 200, grande)
		r.cabecalho(t, "ja comprimido", "Content-Encoding", "br")

		// serve_pasta: css vai em gzip, png nao, Range vai cru
		r = pede(t, "GET", c.base+"/estatico/site.css", "", "Accept-Encoding", "gzip")
		r.cabecalho(t, "css", "Content-Encoding", "gzip")
		r.cabecalho(t, "css", "Content-Type", "text/css; charset=utf-8")
		r.cabecalho(t, "css", "Vary", "Accept-Encoding")
		if r.status != 200 || gunzipTeste(t, r.corpo) != grande {
			t.Errorf("css: %d", r.status)
		}
		r = pede(t, "GET", c.base+"/estatico/foto.png", "", "Accept-Encoding", "gzip")
		r.confere(t, "png estatico", 200, grande)
		r.cabecalho(t, "png estatico", "Content-Encoding", "")
		r = pede(t, "GET", c.base+"/estatico/site.css", "", "Accept-Encoding", "gzip", "Range", "bytes=0-8")
		r.confere(t, "range", 206, "gambiarra")
		r.cabecalho(t, "range", "Content-Encoding", "")
	})
	esperaNosDois(t, `comprime({"nivel": 10})`, "", `comprime(): "nivel" vai de 1 (rapido) a 9 (menor), veio 10`)
	esperaNosDois(t, `comprime({"tamanho": 10})`, "", `comprime(): opcao "tamanho" nao existe`)
}
