//go:build !js

// TLS: configuracao compartilhada pelo escuta (HTTPS/wss), escuta_tcp,
// conecta_tcp, busca e conecta_ws, mais o gera_certificado (so pra dev).
//
// Sem build tag: crypto/tls e crypto/x509 compilam no wasm. Quem abre socket
// (rede/ws) ja tem stub proprio no navegador; o busca recusa as opcoes de TLS
// la (o fetch do navegador nao deixa trocar a validacao do certificado).

package interpreter

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"gambiarrascript/object"
)

const (
	// prazoHandshakeTLS segura cliente que conecta e nao fala TLS nenhum.
	prazoHandshakeTLS = 10 * time.Second
	// validadeCertDev e quanto vale o certificado do gera_certificado.
	validadeCertDev = 365 * 24 * time.Hour
)

// ehPEM: texto que comeca com "-----BEGIN" e o PEM em si; o resto e caminho
// de arquivo. Assim gera_certificado() encaixa direto no escuta e no "ca".
func ehPEM(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "-----BEGIN")
}

// lePEM devolve os bytes do PEM (inline ou lido do arquivo). Arquivo que nao
// abre e erro "io" com o caminho na mensagem.
func lePEM(nome, oque, valor string) ([]byte, *object.Erro) {
	if ehPEM(valor) {
		return []byte(valor), nil
	}
	if strings.TrimSpace(valor) == "" {
		return nil, erroBuiltin("%s(): %s vazio — passa o caminho do arquivo .pem (ou o PEM em si)", nome, oque)
	}
	b, err := os.ReadFile(valor)
	if err != nil {
		return nil, erroBuiltinKind(KindIO, "%s(): nao consegui ler %s %q: %v", nome, oque, valor, err)
	}
	return b, nil
}

// paresTLS le um dicionario de opcoes TLS validando as chaves permitidas.
func paresTLS(nome string, d *object.Dicionario, permitidas ...string) (map[string]object.Object, *object.Erro) {
	saida := map[string]object.Object{}
	for _, par := range d.Pares() {
		k, ok := par.Chave.(*object.Texto)
		valida := false
		for _, nomeOk := range permitidas {
			valida = valida || (ok && k.Value == nomeOk)
		}
		if !valida {
			return nil, erroBuiltin("%s(): opcao de tls %s nao existe (vale: %s)", nome, chaveComAspas(par.Chave), strings.Join(permitidas, ", "))
		}
		saida[k.Value] = par.Valor
	}
	return saida, nil
}

// chaveComAspas: chave texto aparece entre aspas na mensagem de erro.
func chaveComAspas(o object.Object) string {
	if t, ok := o.(*object.Texto); ok {
		return strconv.Quote(t.Value)
	}
	return o.Inspect()
}

func textoTLS(nome, chave string, v object.Object) (string, *object.Erro) {
	t, ok := v.(*object.Texto)
	if !ok {
		return "", erroBuiltin("%s(): \"%s\" do tls tem que ser texto, veio %s", nome, chave, v.Type())
	}
	return t.Value, nil
}

func boolTLS(nome, chave string, v object.Object) (bool, *object.Erro) {
	b, ok := v.(*object.Booleano)
	if !ok {
		return false, erroBuiltin("%s(): \"%s\" tem que ser deu_bom ou deu_ruim, veio %s", nome, chave, v.Type())
	}
	return b.Value, nil
}

// configTLSServidor monta o tls.Config do lado servidor a partir de
// {"cert": "cert.pem", "chave": "chave.pem"} (caminho ou PEM inline).
// TLS 1.2 no minimo; suites e curvas ficam no padrao do Go (ja seguro).
func configTLSServidor(nome string, o object.Object) (*tls.Config, *object.Erro) {
	d, ok := o.(*object.Dicionario)
	if !ok {
		return nil, erroBuiltin("%s(): \"tls\" tem que ser um dicionario {\"cert\": \"cert.pem\", \"chave\": \"chave.pem\"}, veio %s", nome, o.Type())
	}
	ops, e := paresTLS(nome, d, "cert", "chave")
	if e != nil {
		return nil, e
	}
	if ops["cert"] == nil || ops["chave"] == nil {
		return nil, erroBuiltin("%s(): \"tls\" precisa de \"cert\" e \"chave\" (caminho do .pem ou o PEM em si)", nome)
	}
	certTxt, e := textoTLS(nome, "cert", ops["cert"])
	if e != nil {
		return nil, e
	}
	chaveTxt, e := textoTLS(nome, "chave", ops["chave"])
	if e != nil {
		return nil, e
	}
	certPEM, e := lePEM(nome, "o certificado", certTxt)
	if e != nil {
		return nil, e
	}
	chavePEM, e := lePEM(nome, "a chave", chaveTxt)
	if e != nil {
		return nil, e
	}
	par, err := tls.X509KeyPair(certPEM, chavePEM)
	if err != nil {
		return nil, erroBuiltinKind(KindIO, "%s(): certificado/chave nao prestam (confere se sao PEM e se a chave e desse certificado): %v", nome, err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{par},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// poolDaCA le um ou mais certificados PEM (caminho ou inline) num pool. Com
// "ca" SO esse pool vale: os CAs do sistema saem (igual curl --cacert).
func poolDaCA(nome, valor string) (*x509.CertPool, *object.Erro) {
	pemCA, e := lePEM(nome, "o ca", valor)
	if e != nil {
		return nil, e
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemCA) {
		return nil, erroBuiltinKind(KindIO, "%s(): o ca %s nao tem nenhum certificado PEM valido", nome, rotuloPEM(valor))
	}
	return pool, nil
}

func rotuloPEM(valor string) string {
	if ehPEM(valor) {
		return "(PEM inline)"
	}
	return "\"" + valor + "\""
}

// configTLSCliente le a opcao "tls" do conecta_tcp: deu_bom (valida contra os
// CAs do sistema, SNI = host do endereco), deu_ruim (sem TLS = nil) ou
// {"servidor": nome, "inseguro": bool, "ca": "ca.pem"}.
func configTLSCliente(nome string, o object.Object) (*tls.Config, *object.Erro) {
	switch v := o.(type) {
	case *object.Booleano:
		if !v.Value {
			return nil, nil
		}
		return &tls.Config{MinVersion: tls.VersionTLS12}, nil
	case *object.Dicionario:
		ops, e := paresTLS(nome, v, "servidor", "inseguro", "ca")
		if e != nil {
			return nil, e
		}
		cfg := &tls.Config{MinVersion: tls.VersionTLS12}
		if s, ok := ops["servidor"]; ok {
			if cfg.ServerName, e = textoTLS(nome, "servidor", s); e != nil {
				return nil, e
			}
		}
		if s, ok := ops["inseguro"]; ok {
			if cfg.InsecureSkipVerify, e = boolTLS(nome, "inseguro", s); e != nil {
				return nil, e
			}
		}
		if s, ok := ops["ca"]; ok {
			ca, e := textoTLS(nome, "ca", s)
			if e != nil {
				return nil, e
			}
			if cfg.RootCAs, e = poolDaCA(nome, ca); e != nil {
				return nil, e
			}
		}
		return cfg, nil
	}
	return nil, erroBuiltin("%s(): \"tls\" tem que ser deu_bom ou dicionario {\"servidor\", \"inseguro\", \"ca\"}, veio %s", nome, o.Type())
}

// tlsDoClienteHTTP le "ca" e "inseguro" direto das opcoes do busca/conecta_ws.
// nil = nenhuma das duas veio (fica o cliente padrao do Go).
func tlsDoClienteHTTP(nome string, d *object.Dicionario) (*tls.Config, *object.Erro) {
	ca, temCA := dicPega(d, "ca")
	ins, temIns := dicPega(d, "inseguro")
	if !temCA && !temIns {
		return nil, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if temIns {
		b, ok := ins.(*object.Booleano)
		if !ok {
			return nil, erroBuiltin("%s(): \"inseguro\" tem que ser deu_bom ou deu_ruim, veio %s", nome, ins.Type())
		}
		cfg.InsecureSkipVerify = b.Value
	}
	if temCA {
		t, ok := ca.(*object.Texto)
		if !ok {
			return nil, erroBuiltin("%s(): \"ca\" tem que ser texto (caminho do .pem ou o PEM em si), veio %s", nome, ca.Type())
		}
		pool, e := poolDaCA(nome, t.Value)
		if e != nil {
			return nil, e
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

// builtinGeraCertificado: gera_certificado([hosts]) -> {"cert": pem, "chave":
// pem}. Autoassinado, ECDSA P-256, 1 ano, SO PRA DESENVOLVIMENTO: nenhum
// navegador confia nele sem voce mandar. Ele e o proprio CA, entao o cliente
// confia passando {"ca": par.cert}. Hosts padrao: localhost, 127.0.0.1, ::1.
func builtinGeraCertificado(args []object.Object) object.Object {
	if len(args) > 1 {
		return erroBuiltin("gera_certificado() quer 0 ou 1 argumento ([hosts]), veio %d", len(args))
	}
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	if len(args) == 1 {
		l, e := listaDeTextos("hosts", args[0])
		if e != nil {
			return erroBuiltin("gera_certificado(): os hosts tem que ser lista de textos, tipo [\"localhost\", \"127.0.0.1\"]")
		}
		if len(l) == 0 {
			return erroBuiltin("gera_certificado(): lista de hosts vazia — passa pelo menos um (\"localhost\")")
		}
		for _, h := range l {
			if strings.TrimSpace(h) == "" {
				return erroBuiltin("gera_certificado(): host vazio na lista")
			}
		}
		hosts = l
	}
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return erroBuiltin("gera_certificado(): nao rolou gerar a chave: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return erroBuiltin("gera_certificado(): nao rolou gerar o serial: %v", err)
	}
	agora := time.Now()
	modelo := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{"GambiarraScript (dev)"}, CommonName: hosts[0]},
		NotBefore:             agora.Add(-time.Hour),
		NotAfter:              agora.Add(validadeCertDev),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			modelo.IPAddresses = append(modelo.IPAddresses, ip)
		} else {
			modelo.DNSNames = append(modelo.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, &chave.PublicKey, chave)
	if err != nil {
		return erroBuiltin("gera_certificado(): nao rolou assinar: %v", err)
	}
	chaveDER, err := x509.MarshalPKCS8PrivateKey(chave)
	if err != nil {
		return erroBuiltin("gera_certificado(): nao rolou exportar a chave: %v", err)
	}
	d := object.NovoDicionario()
	dicBota(d, "cert", &object.Texto{Value: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))})
	dicBota(d, "chave", &object.Texto{Value: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: chaveDER}))})
	return d
}

// ehErroCertificado: a falha foi na validacao do certificado do servidor (CA
// desconhecido, nome errado, vencido) — da pra dar a dica do "ca".
func ehErroCertificado(err error) bool {
	var (
		verif    *tls.CertificateVerificationError
		ca       x509.UnknownAuthorityError
		host     x509.HostnameError
		invalido x509.CertificateInvalidError
	)
	return errors.As(err, &verif) || errors.As(err, &ca) || errors.As(err, &host) || errors.As(err, &invalido)
}
