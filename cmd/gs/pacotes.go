package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gambiarrascript/lexer"
	"gambiarrascript/parser"
)

// Package manager raiz: `gs get` baixa um modulo, `gs instala` baixa tudo do
// gambiarra.json, e o gambiarra.lock guarda a URL resolvida + sha256 de cada
// dependencia pra build reprodutivel (e pra perceber se alguem trocou o
// arquivo no servidor).
//
// Fonte de uma dependencia (o valor em "dependencias" no gambiarra.json):
//
//	https://exemplo.com/libs/datas.gs           URL comum, baixada como esta
//	github.com/usuario/repo/caminho/mod.gs@ref  GitHub fixado numa tag/commit/branch
//	github.com/usuario/repo/caminho/mod.gs      GitHub sem ref (HEAD; o lock fixa o hash)
//
// O formato continua sendo nome -> texto, entao gambiarra.json antigo (so URL)
// segue valendo.

const (
	arqManifesto = "gambiarra.json"
	arqLock      = "gambiarra.lock"
	dirModulos   = "gs_modulos"
	versaoLock   = 1
	limiteModulo = 10 << 20 // 10 MB: modulo .gs maior que isso e outra coisa
)

// clienteHTTP e var pra os testes trocarem pelo cliente do httptest (TLS).
var clienteHTTP = &http.Client{Timeout: 5 * time.Minute}

// baseRawGitHub e de onde sai o conteudo cru do GitHub; var pros testes.
var baseRawGitHub = "https://raw.githubusercontent.com"

var (
	reNomeDep      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)
	reSegGitHub    = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	reCaminhoOuRef = regexp.MustCompile(`^[A-Za-z0-9_.\-/]+$`)
)

// fonteDep e uma fonte ja interpretada.
type fonteDep struct {
	Spec string // como fica no gambiarra.json
	URL  string // URL https de onde baixar
	Nome string // nome sugerido do modulo (sem .gs)
}

// resolveFonte interpreta o que o usuario passou (ou o que ta no manifesto).
func resolveFonte(spec string, inseguro bool) (fonteDep, error) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return fonteDep{}, errors.New("fonte vazia")
	}
	low := strings.ToLower(s)
	for _, pre := range []string{"https://github.com/", "github.com/"} {
		if strings.HasPrefix(low, pre) {
			return resolveGitHub(s[len(pre):])
		}
	}
	switch {
	case strings.HasPrefix(low, "https://"):
	case strings.HasPrefix(low, "http://"):
		if !inseguro {
			return fonteDep{}, fmt.Errorf("%s e http:// sem criptografia — qualquer um no caminho troca o codigo. Usa https:// (ou --inseguro se souber o que ta fazendo)", s)
		}
	default:
		return fonteDep{}, fmt.Errorf("fonte %q nao reconhecida: usa https://... ou github.com/usuario/repo/caminho.gs@ref", s)
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return fonteDep{}, fmt.Errorf("URL invalida: %q", s)
	}
	ultimo := path.Base(u.Path)
	if strings.Contains(ultimo, "@") {
		return fonteDep{}, fmt.Errorf("%q: o @ref so vale pra github.com/...; numa URL comum aponta direto pro arquivo da versao", s)
	}
	return fonteDep{Spec: s, URL: s, Nome: strings.TrimSuffix(ultimo, ".gs")}, nil
}

// resolveGitHub trata "usuario/repo/caminho/mod.gs[@ref]" e devolve a URL do
// raw.githubusercontent.com naquela ref (HEAD se nao tiver).
func resolveGitHub(resto string) (fonteDep, error) {
	formato := errors.New("formato do GitHub: github.com/usuario/repo/caminho/modulo.gs@ref")
	ref := ""
	if i := strings.LastIndex(resto, "@"); i >= 0 {
		ref = resto[i+1:]
		resto = resto[:i]
		if ref == "" {
			return fonteDep{}, formato
		}
	}
	partes := strings.SplitN(resto, "/", 3)
	if len(partes) < 3 || partes[2] == "" {
		return fonteDep{}, formato
	}
	usuario, repo, caminho := partes[0], partes[1], strings.Trim(partes[2], "/")
	if !reSegGitHub.MatchString(usuario) || !reSegGitHub.MatchString(repo) ||
		!reCaminhoOuRef.MatchString(caminho) || temPontoPonto(caminho) {
		return fonteDep{}, formato
	}
	if ref != "" && (!reCaminhoOuRef.MatchString(ref) || temPontoPonto(ref)) {
		return fonteDep{}, fmt.Errorf("ref invalida: %q", ref)
	}
	refURL := ref
	spec := "github.com/" + usuario + "/" + repo + "/" + caminho
	if ref == "" {
		refURL = "HEAD"
	} else {
		spec += "@" + ref
	}
	return fonteDep{
		Spec: spec,
		URL:  strings.TrimRight(baseRawGitHub, "/") + "/" + usuario + "/" + repo + "/" + refURL + "/" + caminho,
		Nome: strings.TrimSuffix(path.Base(caminho), ".gs"),
	}, nil
}

func temPontoPonto(s string) bool {
	for _, seg := range strings.Split(s, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// erroStatus e resposta HTTP diferente de 200 (o build usa pra dizer "nao
// tem essa release").
type erroStatus struct {
	URL    string
	Codigo int
}

func (e erroStatus) Error() string {
	return fmt.Sprintf("%s respondeu %d", e.URL, e.Codigo)
}

// baixa faz GET so em https (http so com inseguro), inclusive nos
// redirecionamentos, e corta em `limite` bytes.
func baixa(u string, inseguro bool, limite int64) ([]byte, error) {
	lu := strings.ToLower(u)
	if !strings.HasPrefix(lu, "https://") && !(inseguro && strings.HasPrefix(lu, "http://")) {
		return nil, fmt.Errorf("recusado: %s nao e https", u)
	}
	c := *clienteHTTP
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("redirecionamento demais")
		}
		if req.URL.Scheme != "https" && !inseguro {
			return fmt.Errorf("redirecionou pra %s sem https — recusado", req.URL)
		}
		return nil
	}
	resp, err := c.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, erroStatus{URL: u, Codigo: resp.StatusCode}
	}
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, limite+1))
	if err != nil {
		return nil, fmt.Errorf("erro lendo %s: %v", u, err)
	}
	if int64(len(corpo)) > limite {
		return nil, fmt.Errorf("%s passou de %d bytes — recusado", u, limite)
	}
	return corpo, nil
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// validaGs confere que o conteudo parseia como GambiarraScript.
func validaGs(corpo []byte) error {
	p := parser.New(lexer.New(string(corpo)))
	p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return fmt.Errorf("nem parseia como .gs: %s", errs[0])
	}
	return nil
}

// ---------- manifesto e lock ----------

// jsonBonito serializa com indentacao e sem escapar &<> (URL com query fica
// legivel). Mapas saem com chave ordenada: arquivo deterministico.
func jsonBonito(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// lerManifesto devolve o gambiarra.json cru (mantem os outros campos).
func lerManifesto(dir string) (map[string]interface{}, error) {
	blob, err := os.ReadFile(filepath.Join(dir, arqManifesto))
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(blob, &m); err != nil {
		return nil, fmt.Errorf("%s invalido: %v", arqManifesto, err)
	}
	if m == nil {
		m = map[string]interface{}{}
	}
	return m, nil
}

func gravarManifesto(dir string, m map[string]interface{}) error {
	blob, err := jsonBonito(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, arqManifesto), blob, 0644)
}

// depsDoManifesto extrai "dependencias" como nome -> fonte.
func depsDoManifesto(m map[string]interface{}) (map[string]string, error) {
	deps := map[string]string{}
	bruto, ok := m["dependencias"]
	if !ok || bruto == nil {
		return deps, nil
	}
	mapa, ok := bruto.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("\"dependencias\" no %s tem que ser um objeto nome -> fonte", arqManifesto)
	}
	for nome, v := range mapa {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("dependencia %q: a fonte tem que ser texto (URL ou github.com/...@ref)", nome)
		}
		if !reNomeDep.MatchString(nome) {
			return nil, fmt.Errorf("nome de dependencia invalido: %q", nome)
		}
		deps[nome] = s
	}
	return deps, nil
}

type entradaLock struct {
	Fonte  string `json:"fonte"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type arquivoLock struct {
	Versao       int                    `json:"versao"`
	Dependencias map[string]entradaLock `json:"dependencias"`
}

// lerLock devolve o lock (vazio se nao existir) e se ele existia.
func lerLock(dir string) (*arquivoLock, bool, error) {
	l := &arquivoLock{Versao: versaoLock, Dependencias: map[string]entradaLock{}}
	blob, err := os.ReadFile(filepath.Join(dir, arqLock))
	if errors.Is(err, os.ErrNotExist) {
		return l, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(blob, l); err != nil {
		return nil, false, fmt.Errorf("%s invalido: %v", arqLock, err)
	}
	if l.Versao != versaoLock {
		return nil, false, fmt.Errorf("%s com versao %d; este gs entende a %d", arqLock, l.Versao, versaoLock)
	}
	if l.Dependencias == nil {
		l.Dependencias = map[string]entradaLock{}
	}
	return l, true, nil
}

// gravarLock so escreve se o conteudo mudou (nao mexe no mtime a toa).
func gravarLock(dir string, l *arquivoLock) (bool, error) {
	l.Versao = versaoLock
	blob, err := jsonBonito(l)
	if err != nil {
		return false, err
	}
	caminho := filepath.Join(dir, arqLock)
	if velho, err := os.ReadFile(caminho); err == nil && bytes.Equal(velho, blob) {
		return false, nil
	}
	return true, os.WriteFile(caminho, blob, 0644)
}

func gravarModulo(dir, nome string, corpo []byte) (string, error) {
	if err := os.MkdirAll(filepath.Join(dir, dirModulos), 0755); err != nil {
		return "", fmt.Errorf("nao consegui criar %s/: %v", dirModulos, err)
	}
	rel := filepath.Join(dirModulos, nome+".gs")
	if err := os.WriteFile(filepath.Join(dir, rel), corpo, 0644); err != nil {
		return "", fmt.Errorf("nao consegui salvar %s: %v", rel, err)
	}
	return rel, nil
}

// ---------- gs get ----------

func cmdGet(args []string) {
	spec, nomeArq, inseguro := "", "", false
	for _, a := range args {
		switch {
		case a == "--inseguro":
			inseguro = true
		case strings.HasPrefix(a, "-"):
			fmt.Printf("flag desconhecida: %s\n", a)
			os.Exit(1)
		case spec == "":
			spec = a
		case nomeArq == "":
			nomeArq = a
		}
	}
	if spec == "" {
		fmt.Println("uso: gs get [--inseguro] <url | github.com/usuario/repo/caminho.gs@ref> [nome.gs]")
		os.Exit(1)
	}
	if err := pegaDependencia(".", spec, nomeArq, inseguro, os.Stdout); err != nil {
		fmt.Println("gs get: " + err.Error())
		os.Exit(1)
	}
}

// pegaDependencia baixa um modulo pra gs_modulos/ e, se tiver gambiarra.json,
// registra a fonte nele e a URL + sha256 no gambiarra.lock.
func pegaDependencia(dir, spec, nomeArq string, inseguro bool, w io.Writer) error {
	f, err := resolveFonte(spec, inseguro)
	if err != nil {
		return err
	}
	nome := f.Nome
	if nomeArq != "" {
		nome = strings.TrimSuffix(filepath.Base(nomeArq), ".gs")
	}
	if !reNomeDep.MatchString(nome) {
		return fmt.Errorf("nome de modulo invalido: %q (passa um nome: gs get <fonte> nome.gs)", nome)
	}
	corpo, err := baixa(f.URL, inseguro, limiteModulo)
	if err != nil {
		return fmt.Errorf("nao consegui baixar: %v", err)
	}
	if err := validaGs(corpo); err != nil {
		return fmt.Errorf("o arquivo baixado %v — abortando", err)
	}
	rel, err := gravarModulo(dir, nome, corpo)
	if err != nil {
		return err
	}
	hash := sha256Hex(corpo)
	fmt.Fprintf(w, "  %s  (baixado, %d bytes, sha256 %s)\n", rel, len(corpo), hash[:12])

	m, err := lerManifesto(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		fmt.Fprintf(w, "  (sem %s: nada registrado — roda gs init pra ter manifesto e lock)\n", arqManifesto)
	case err != nil:
		return err
	default:
		deps, err := depsDoManifesto(m)
		if err != nil {
			return err
		}
		deps[nome] = f.Spec
		m["dependencias"] = deps
		if err := gravarManifesto(dir, m); err != nil {
			return err
		}
		fmt.Fprintf(w, "  %s  (dependencia %q = %q)\n", arqManifesto, nome, f.Spec)

		lock, _, err := lerLock(dir)
		if err != nil {
			return err
		}
		if ant, ok := lock.Dependencias[nome]; ok && ant.Fonte == f.Spec && ant.SHA256 != hash {
			fmt.Fprintf(w, "  aviso: o conteudo de %q mudou desde o lock (%s -> %s)\n", nome, ant.SHA256[:12], hash[:12])
		}
		lock.Dependencias[nome] = entradaLock{Fonte: f.Spec, URL: f.URL, SHA256: hash}
		if _, err := gravarLock(dir, lock); err != nil {
			return err
		}
		fmt.Fprintf(w, "  %s  (fixado)\n", arqLock)
	}
	fmt.Fprintf(w, "usa com: importa \"%s/%s.gs\"\n", dirModulos, nome)
	return nil
}

// ---------- gs instala ----------

func cmdInstala(args []string) {
	atualiza, inseguro := false, false
	for _, a := range args {
		switch a {
		case "--atualiza":
			atualiza = true
		case "--inseguro":
			inseguro = true
		default:
			fmt.Println("uso: gs instala [--atualiza] [--inseguro]")
			os.Exit(1)
		}
	}
	if err := instalaDependencias(".", atualiza, inseguro, os.Stdout); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

// erroRecusa: o conteudo baixado nao bate com o lock.
type erroRecusa struct{ detalhes []string }

func (e erroRecusa) Error() string {
	return "RECUSADO: conteudo diferente do que o " + arqLock + " fixou\n" +
		strings.Join(e.detalhes, "\n") + "\n" +
		"Isso pode ser ataque na cadeia de suprimentos (alguem trocou o arquivo no\n" +
		"servidor) ou o autor reescreveu a tag/branch. Nada foi instalado.\n" +
		"Se voce confere a mudanca e confia nela: gs instala --atualiza"
}

// instalaDependencias baixa tudo do gambiarra.json pra gs_modulos/. Com lock,
// cada arquivo tem que bater com o sha256 fixado — se um so nao bater, nada e
// gravado. --atualiza re-resolve tudo e reescreve o lock.
func instalaDependencias(dir string, atualiza, inseguro bool, w io.Writer) error {
	m, err := lerManifesto(dir)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("nao achei %s aqui (roda gs init)", arqManifesto)
	}
	if err != nil {
		return err
	}
	deps, err := depsDoManifesto(m)
	if err != nil {
		return err
	}
	lock, temLock, err := lerLock(dir)
	if err != nil {
		return err
	}

	nomes := make([]string, 0, len(deps))
	for n := range deps {
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)

	type passo struct {
		nome    string
		corpo   []byte // nil = arquivo local ja confere
		entrada entradaLock
		nota    string
	}
	var passos []passo
	var recusas []string
	for _, nome := range nomes {
		spec := deps[nome]
		f, err := resolveFonte(spec, inseguro)
		if err != nil {
			return fmt.Errorf("dependencia %q: %v", nome, err)
		}
		ant, temAnt := lock.Dependencias[nome]
		if temLock && temAnt && !atualiza && ant.Fonte == f.Spec {
			// atalho offline: o arquivo local ja e o que o lock fixou
			if b, err := os.ReadFile(filepath.Join(dir, dirModulos, nome+".gs")); err == nil && sha256Hex(b) == ant.SHA256 {
				passos = append(passos, passo{nome: nome, entrada: ant, nota: "ja confere com o lock"})
				continue
			}
			corpo, err := baixa(ant.URL, inseguro, limiteModulo)
			if err != nil {
				return fmt.Errorf("dependencia %q: %v", nome, err)
			}
			if h := sha256Hex(corpo); h != ant.SHA256 {
				recusas = append(recusas, fmt.Sprintf("  %s\n    url:      %s\n    esperado: sha256 %s\n    veio:     sha256 %s", nome, ant.URL, ant.SHA256, h))
				continue
			}
			if err := validaGs(corpo); err != nil {
				return fmt.Errorf("dependencia %q: %v", nome, err)
			}
			passos = append(passos, passo{nome: nome, corpo: corpo, entrada: ant, nota: "conferido com o lock"})
			continue
		}
		corpo, err := baixa(f.URL, inseguro, limiteModulo)
		if err != nil {
			return fmt.Errorf("dependencia %q: %v", nome, err)
		}
		if err := validaGs(corpo); err != nil {
			return fmt.Errorf("dependencia %q: %v", nome, err)
		}
		nota := "novo no lock"
		switch {
		case temAnt && ant.Fonte != f.Spec:
			nota = "fonte mudou, lock atualizado"
		case temAnt && atualiza && ant.SHA256 != sha256Hex(corpo):
			nota = "atualizado"
		case temAnt && atualiza:
			nota = "sem mudanca"
		}
		passos = append(passos, passo{nome: nome, corpo: corpo, entrada: entradaLock{Fonte: f.Spec, URL: f.URL, SHA256: sha256Hex(corpo)}, nota: nota})
	}
	if len(recusas) > 0 {
		return erroRecusa{detalhes: recusas}
	}

	novo := &arquivoLock{Versao: versaoLock, Dependencias: map[string]entradaLock{}}
	for _, p := range passos {
		novo.Dependencias[p.nome] = p.entrada
		rel := filepath.Join(dirModulos, p.nome+".gs")
		if p.corpo != nil {
			if rel, err = gravarModulo(dir, p.nome, p.corpo); err != nil {
				return err
			}
		}
		fmt.Fprintf(w, "  %s  (%s)\n", rel, p.nota)
	}
	mudou, err := gravarLock(dir, novo)
	if err != nil {
		return err
	}
	if mudou {
		fmt.Fprintf(w, "  %s  (gravado)\n", arqLock)
	}
	if len(passos) == 0 {
		fmt.Fprintln(w, "nenhuma dependencia no "+arqManifesto+" — nada pra instalar")
		return nil
	}
	fmt.Fprintf(w, "pronto: %d dependencia(s) em %s/\n", len(passos), dirModulos)
	return nil
}
