package vm

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"gambiarrascript/interpreter"
)

// TLS: escuta com {"tls": ...} (HTTPS e wss), escuta_tcp/conecta_tcp com
// tls, busca/conecta_ws com "ca"/"inseguro" e gera_certificado. Os
// certificados de teste sao gerados aqui (autoassinados, validos pra
// localhost e 127.0.0.1) e gravados no t.TempDir().

type certTeste struct {
	cert, chave string // caminhos dos .pem
	pool        *x509.CertPool
}

func geraCertTeste(t *testing.T) certTeste {
	t.Helper()
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	modelo := &x509.Certificate{
		SerialNumber:          big.NewInt(42),
		Subject:               pkix.Name{CommonName: "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, &chave.PublicKey, chave)
	if err != nil {
		t.Fatal(err)
	}
	chaveDER, err := x509.MarshalPKCS8PrivateKey(chave)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	c := certTeste{cert: filepath.Join(dir, "cert.pem"), chave: filepath.Join(dir, "chave.pem")}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(c.cert, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.chave, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: chaveDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	c.pool = x509.NewCertPool()
	c.pool.AppendCertsFromPEM(certPEM)
	return c
}

func (c certTeste) optTLS() string {
	return `{"cert": "` + c.cert + `", "chave": "` + c.chave + `"}`
}

// HTTPS + wss: o servidor gs sobe com {"tls": ...} em cada engine; um
// cliente Go (confiando no CA) faz GET e abre o wss; e um cliente gs nos dois
// engines usa busca/conecta_ws com {"ca": ...}.
func TestEscutaHTTPSEWss(t *testing.T) {
	c := geraCertTeste(t)
	fonte := `rota("GET", "/ola", gambiarra(pedido) funciona "salve, https" acabou_finalmente)
rota_ws("/eco", gambiarra(ws)
    enquanto deu_bom
        bota m = recebe(ws)
        se_colar m == nada
            vaza
        acabou_finalmente
        envia(ws, "eco: " + m)
    acabou_finalmente
acabou_finalmente)
escuta("127.0.0.1:0", {"tls": ` + c.optTLS() + `})
mostra "escuta voltou"`
	for _, m := range motores {
		t.Run(m.nome, func(t *testing.T) {
			pr, pw := io.Pipe()
			out := &bufTravado{}
			pronto := make(chan *interpreter.Interpreter, 1)
			fim := make(chan struct{})
			go func() {
				defer close(fim)
				rodaComGancho(t, m.nome, fonte, out, pw, pronto)
			}()
			i := <-pronto
			linha, err := bufio.NewReader(pr).ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			go io.Copy(io.Discard, pr)
			sub := regexp.MustCompile(`^servidor de pe em https://127\.0\.0\.1:(\d+) \(ctrl\+c pra parar\)\n$`).FindStringSubmatch(linha)
			if sub == nil {
				t.Fatalf("linha do escuta: %q", linha)
			}
			defer func() {
				i.PararServidor()
				select {
				case <-fim:
				case <-time.After(8 * time.Second):
					t.Error("escuta nao voltou depois do desligamento")
				}
				if got := out.String(); got != "escuta voltou\n" {
					t.Errorf("saida: %q", got)
				}
			}()
			base := "https://127.0.0.1:" + sub[1]

			// cliente Go que confia no CA: TLS 1.2+ e HTTP/2 de brinde
			tr := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: c.pool}, ForceAttemptHTTP2: true}
			defer tr.CloseIdleConnections()
			cli := &http.Client{Transport: tr, Timeout: 5 * time.Second}
			resp, err := cli.Get(base + "/ola")
			if err != nil {
				t.Fatal(err)
			}
			corpo, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 || string(corpo) != "salve, https" {
				t.Errorf("GET https: %d %q", resp.StatusCode, corpo)
			}
			if resp.TLS == nil || resp.TLS.Version < tls.VersionTLS12 {
				t.Errorf("TLS esperado >= 1.2, veio %+v", resp.TLS)
			}
			if resp.ProtoMajor != 2 {
				t.Errorf("esperava HTTP/2 no https, veio %s", resp.Proto)
			}

			// TLS 1.1 e recusado
			velho := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: c.pool, MaxVersion: tls.VersionTLS11}}, Timeout: 5 * time.Second}
			if _, err := velho.Get(base + "/ola"); err == nil {
				t.Error("TLS 1.1 devia ser recusado")
			}

			// wss com cliente Go
			ctx, cancela := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancela()
			wsCli := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: c.pool}, ForceAttemptHTTP2: true}}
			conn, _, err := websocket.Dial(ctx, "wss://127.0.0.1:"+sub[1]+"/eco", &websocket.DialOptions{HTTPClient: wsCli})
			if err != nil {
				t.Fatalf("wss: %v", err)
			}
			if err := conn.Write(ctx, websocket.MessageText, []byte("oi")); err != nil {
				t.Fatal(err)
			}
			_, msg, err := conn.Read(ctx)
			if err != nil || string(msg) != "eco: oi" {
				t.Errorf("wss eco: %q %v", msg, err)
			}
			conn.Close(websocket.StatusNormalClosure, "")

			// cliente gs (nos dois engines) contra esse servidor
			cliente := `bota r = busca("` + base + `/ola", {"ca": "` + c.cert + `"})
mostra r.status + " " + r.corpo
bota ws = conecta_ws("wss://127.0.0.1:` + sub[1] + `/eco", {"ca": "` + c.cert + `", "timeout": 5})
envia(ws, "salve")
mostra recebe(ws)
fecha(ws)
bota r2 = busca("` + base + `/ola", {"inseguro": deu_bom})
mostra r2.corpo
arruma
    busca("` + base + `/ola")
quebrou erro
    mostra contem(erro_msg(erro), "o certificado nao passou")
acabou_finalmente
arruma
    conecta_ws("wss://127.0.0.1:` + sub[1] + `/eco")
quebrou erro
    mostra erro_tipo(erro) + " " + contem(erro_msg(erro), "o certificado")
acabou_finalmente
`
			esperaNosDois(t, cliente, "200 salve, https\neco: salve\nsalve, https\ndeu_bom\nrede deu_bom\n", "")
		})
	}
}

// busca com "ca" contra o httptest.NewTLSServer (o CA dele gravado em PEM),
// e com o PEM inline no lugar do caminho.
func TestBuscaComCA(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "seguro")
	}))
	defer srv.Close()
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, caPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	src := `mostra busca("` + srv.URL + `", {"ca": "` + ca + `"}).corpo
bota pem = le_arquivo("` + ca + `")
mostra busca("` + srv.URL + `", {"ca": pem}).corpo
`
	esperaNosDois(t, src, "seguro\nseguro\n", "")

	// erros de uso / arquivo
	esperaNosDois(t, `arruma
    busca("`+srv.URL+`", {"ca": "/nao/existe/ca.pem"})
quebrou erro
    mostra erro_tipo(erro)
    mostra contem(erro_msg(erro), "nao consegui ler o ca")
acabou_finalmente
arruma
    busca("`+srv.URL+`", {"inseguro": "sim"})
quebrou erro
    mostra erro_msg(erro)
acabou_finalmente
`, "io\ndeu_bom\ndeu ruim: busca(): \"inseguro\" tem que ser deu_bom ou deu_ruim, veio TEXTO\n", "")
}

// TCP com TLS: eco por linha com o CA certo (SNI pelo IP e por "servidor"),
// recusa sem CA, "inseguro" passa, e cliente sem TLS nao chega no handler.
func TestTcpTLS(t *testing.T) {
	c := geraCertTeste(t)
	src := handlerEco + sobeServidor(`, "tls": `+c.optTLS()) + `bota cli = conecta_tcp(end, {"tls": {"ca": "` + c.cert + `"}, "timeout": 5})
mostra tipo(cli)
envia(cli, "salve")
mostra recebe(cli)
fecha(cli)
bota porta = separa(end, ":")[1]
bota c2 = conecta_tcp("localhost:" + porta, {"tls": {"ca": "` + c.cert + `", "servidor": "localhost"}, "modo": "bruto"})
envia(c2, "bruto\n")
mostra tira_espaco(recebe(c2))
fecha(c2)
arruma
    conecta_tcp(end, {"tls": deu_bom, "timeout": 5})
quebrou erro
    mostra erro_tipo(erro) + " " + contem(erro_msg(erro), "o certificado de")
acabou_finalmente
arruma
    conecta_tcp(end, {"tls": {"ca": "` + c.cert + `", "servidor": "outro.com"}, "timeout": 5})
quebrou erro
    mostra "nome errado: " + erro_tipo(erro)
acabou_finalmente
bota c3 = conecta_tcp(end, {"tls": {"inseguro": deu_bom}, "timeout": 5})
envia(c3, "sem validar")
mostra recebe(c3)
fecha(c3)
bota cru = conecta_tcp(end, {"timeout": 5})
envia(cru, "texto puro")
mostra recebe(cru)
fecha(cru)
` + derrubaServidor
	rodaRedeNosDois(t, src, "nativo\neco: salve\neco: bruto\nrede deu_bom\nnome errado: rede\neco: sem validar\nnada\nnada\n",
		"escuta_tcp: handshake tls com 127.0.0.1:")
}

// gera_certificado: o par serve inline no tls do servidor e o cert vira o ca
// do cliente. Hosts customizados e erros de uso.
func TestGeraCertificado(t *testing.T) {
	src := `bota par = gera_certificado()
mostra chaves(par)
mostra comeca_com(par.cert, "-----BEGIN CERTIFICATE-----")
mostra comeca_com(par.chave, "-----BEGIN PRIVATE KEY-----")
gambiarra atende(c)
    envia(c, "ola " + recebe(c))
acabou_finalmente
` + sobeServidor(`, "tls": par`) + `bota cli = conecta_tcp(end, {"tls": {"ca": par.cert}, "timeout": 5})
envia(cli, "dev")
mostra recebe(cli)
fecha(cli)
` + derrubaServidor + `bota outro = gera_certificado(["meu.dev"])
mostra tamanho(outro.cert) > 100
arruma
    gera_certificado([])
quebrou erro
    mostra erro_msg(erro)
acabou_finalmente
arruma
    gera_certificado([1])
quebrou erro
    mostra erro_msg(erro)
acabou_finalmente
`
	rodaRedeNosDois(t, src, "[cert, chave]\ndeu_bom\ndeu_bom\nola dev\nnada\ndeu_bom\n"+
		"deu ruim: gera_certificado(): lista de hosts vazia — passa pelo menos um (\"localhost\")\n"+
		"deu ruim: gera_certificado(): os hosts tem que ser lista de textos, tipo [\"localhost\", \"127.0.0.1\"]\n")
}

// Erros de configuracao: arquivo faltando / PEM invalido sao "io" e saem
// ANTES de abrir a porta; opcao errada e "builtin".
func TestTLSErrosDeConfig(t *testing.T) {
	c := geraCertTeste(t)
	lixo := filepath.Join(t.TempDir(), "lixo.pem")
	if err := os.WriteFile(lixo, []byte("nao sou pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	casos := []struct{ codigo, saida string }{
		{`escuta("127.0.0.1:0", {"tls": {"cert": "/nao/existe.pem", "chave": "` + c.chave + `"}})`,
			"io | deu ruim: escuta(): nao consegui ler o certificado \"/nao/existe.pem\": open /nao/existe.pem: no such file or directory"},
		{`escuta("127.0.0.1:0", {"tls": {"cert": "` + lixo + `", "chave": "` + c.chave + `"}})`,
			"io | deu ruim: escuta(): certificado/chave nao prestam (confere se sao PEM e se a chave e desse certificado): tls: failed to find any PEM data in certificate input"},
		{`escuta("127.0.0.1:0", {"tls": {"cert": "` + c.cert + `"}})`,
			"builtin | deu ruim: escuta(): \"tls\" precisa de \"cert\" e \"chave\" (caminho do .pem ou o PEM em si)"},
		{`escuta("127.0.0.1:0", {"tsl": {}})`,
			"builtin | deu ruim: escuta(): opcao \"tsl\" nao existe (a que existe: tls)"},
		{`escuta("127.0.0.1:0", {"tls": deu_bom})`,
			"builtin | deu ruim: escuta(): \"tls\" tem que ser um dicionario {\"cert\": \"cert.pem\", \"chave\": \"chave.pem\"}, veio BOOLEANO"},
		{`escuta_tcp("127.0.0.1:0", gambiarra(c) acabou_finalmente, {"tls": {"cert": "` + c.cert + `", "chave": "/nao/existe.pem"}})`,
			"io | deu ruim: escuta_tcp(): nao consegui ler a chave \"/nao/existe.pem\": open /nao/existe.pem: no such file or directory"},
		{`conecta_tcp("127.0.0.1:1", {"tls": {"ca": "` + lixo + `"}})`,
			"io | deu ruim: conecta_tcp(): o ca \"" + lixo + "\" nao tem nenhum certificado PEM valido"},
		{`conecta_tcp("127.0.0.1:1", {"tls": {"sni": "x"}})`,
			"builtin | deu ruim: conecta_tcp(): opcao de tls \"sni\" nao existe (vale: servidor, inseguro, ca)"},
		{`conecta_tcp("127.0.0.1:1", {"tls": "sim"})`,
			"builtin | deu ruim: conecta_tcp(): \"tls\" tem que ser deu_bom ou dicionario {\"servidor\", \"inseguro\", \"ca\"}, veio TEXTO"},
		{`conecta_ws("wss://127.0.0.1:1/x", {"ca": 1})`,
			"builtin | deu ruim: conecta_ws(): \"ca\" tem que ser texto (caminho do .pem ou o PEM em si), veio NUMERO"},
	}
	for _, cs := range casos {
		src := "arruma\n    " + cs.codigo + "\nquebrou erro\n    mostra erro_tipo(erro) + \" | \" + erro_msg(erro)\nacabou_finalmente\n"
		esperaNosDois(t, src, cs.saida+"\n", "")
	}
	// conecta_tcp com tls: deu_ruim e TCP puro (nada de TLS)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				l, _ := bufio.NewReader(conn).ReadString('\n')
				io.WriteString(conn, strings.ToUpper(l))
			}()
		}
	}()
	esperaNosDois(t, `bota c = conecta_tcp("`+ln.Addr().String()+`", {"tls": deu_ruim, "timeout": 5})
envia(c, "puro")
mostra recebe(c)
fecha(c)
`, "PURO\n", "")
}
