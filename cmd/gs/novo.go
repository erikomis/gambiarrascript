package main

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ---------- gs novo ----------

// esqueletos guarda os projetos-modelo do `gs novo`, um por pasta
// (esqueletos/api, esqueletos/site, esqueletos/script). O `all:` traz junto
// os arquivos com ponto na frente (.gitignore, .env.exemplo, dados/.gitkeep).
//
//go:embed all:esqueletos
var esqueletos embed.FS

// Marcadores trocados em todo arquivo do esqueleto. Ficam dentro de texto ou
// comentario nos .gs, entao o esqueleto em si tambem parseia e passa no
// gs check/formata (o teste confere).
const (
	marcaNome      = "__NOME__"
	marcaVersaoGs  = "__VERSAO_GS__"
	dirEsqueletos  = "esqueletos"
	maxNomeProjeto = 64
)

type tipoProjeto struct {
	nome     string
	resumo   string
	proximos []string // comandos sugeridos depois do `cd nome`
}

var tiposProjeto = []tipoProjeto{
	{"api", "API REST: sqlite + migracoes, cadastro/login com JWT, CRUD de tarefas, testes e Dockerfile",
		[]string{"cp .env.exemplo .env", "gs testa", "gs roda principal.gs"}},
	{"site", "site com modelos HTML: layout + parciais + laco, arquivos estaticos e testes",
		[]string{"gs testa", "gs roda principal.gs"}},
	{"script", "script de linha de comando com opcoes() e --ajuda",
		[]string{"gs roda principal.gs --ajuda", "gs roda principal.gs --nome tropa --vezes 2"}},
}

// nome de projeto vira pasta, nome do gambiarra.json e tag da imagem Docker
// (que so aceita minuscula): letra minuscula na frente, depois letra, numero,
// - ou _.
var reNomeProjeto = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func acharTipoProjeto(nome string) (tipoProjeto, bool) {
	for _, t := range tiposProjeto {
		if t.nome == nome {
			return t, true
		}
	}
	return tipoProjeto{}, false
}

func validaNomeProjeto(nome string) error {
	if len(nome) > maxNomeProjeto {
		return fmt.Errorf("nome %q comprido demais (maximo %d caracteres)", nome, maxNomeProjeto)
	}
	if !reNomeProjeto.MatchString(nome) {
		return fmt.Errorf("nome %q nao rola: use letra minuscula, numero, - ou _, comecando com letra (ex.: minha-api)", nome)
	}
	return nil
}

// tagImagemGs e a tag da imagem oficial que o Dockerfile gerado usa: a versao
// deste gs quando e release (x.y.z ou x.y.z-rc1, que a release tambem
// publica como tag), senao latest.
func tagImagemGs() string {
	if reVersaoRelease.MatchString(Versao) {
		return Versao
	}
	return "latest"
}

func listaTiposProjeto(w io.Writer) {
	fmt.Fprintln(w, "uso: gs novo <tipo> <nome>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "tipos:")
	for _, t := range tiposProjeto {
		fmt.Fprintf(w, "  %-7s %s\n", t.nome, t.resumo)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "ex.: gs novo api minha-api   (cria a pasta minha-api/ com tudo pronto pra rodar)")
}

// pastaLivre diz se da pra criar o projeto em `destino`: nao existe ou e uma
// pasta vazia. Nunca passa por cima de nada.
func pastaLivre(destino string) error {
	info, err := os.Stat(destino)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s ja existe e nao e pasta — nao vou passar por cima", destino)
	}
	itens, err := os.ReadDir(destino)
	if err != nil {
		return err
	}
	if len(itens) > 0 {
		return fmt.Errorf("a pasta %s ja existe e nao ta vazia — nao vou passar por cima (escolhe outro nome ou apaga ela)", destino)
	}
	return nil
}

// criaProjeto copia o esqueleto `tipo` pra pasta `destino` trocando os
// marcadores, e devolve os caminhos criados (relativos ao destino, com /).
func criaProjeto(tipo, nome, destino string) ([]string, error) {
	if _, ok := acharTipoProjeto(tipo); !ok {
		return nil, fmt.Errorf("tipo %q nao existe (tem: %s)", tipo, nomesTiposProjeto())
	}
	if err := validaNomeProjeto(nome); err != nil {
		return nil, err
	}
	if err := pastaLivre(destino); err != nil {
		return nil, err
	}
	raiz := path.Join(dirEsqueletos, tipo)
	troca := strings.NewReplacer(marcaNome, nome, marcaVersaoGs, tagImagemGs())

	var criados []string
	err := fs.WalkDir(esqueletos, raiz, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, raiz+"/")
		conteudo, err := esqueletos.ReadFile(p)
		if err != nil {
			return err
		}
		alvo := filepath.Join(destino, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(alvo), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(alvo, []byte(troca.Replace(string(conteudo))), 0o644); err != nil {
			return err
		}
		criados = append(criados, rel)
		return nil
	})
	if err != nil {
		return criados, fmt.Errorf("nao consegui criar o projeto: %w", err)
	}
	sort.Strings(criados)
	return criados, nil
}

func nomesTiposProjeto() string {
	nomes := make([]string, len(tiposProjeto))
	for i, t := range tiposProjeto {
		nomes[i] = t.nome
	}
	return strings.Join(nomes, ", ")
}

// novoProjeto e o `gs novo` sem os.Exit (testavel). Sem argumento (ou com
// --ajuda) lista os tipos.
func novoProjeto(args []string, w io.Writer) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "--ajuda" {
		listaTiposProjeto(w)
		return nil
	}
	tipo, ok := acharTipoProjeto(args[0])
	if !ok {
		listaTiposProjeto(w)
		return fmt.Errorf("tipo %q nao existe (tem: %s)", args[0], nomesTiposProjeto())
	}
	if len(args) != 2 {
		return fmt.Errorf("uso: gs novo %s <nome>", tipo.nome)
	}
	nome := args[1]
	criados, err := criaProjeto(tipo.nome, nome, nome)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "projeto %s criado (%s):\n", nome, tipo.nome)
	for _, c := range criados {
		fmt.Fprintf(w, "  %s/%s\n", nome, c)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "bora:")
	fmt.Fprintf(w, "  cd %s\n", nome)
	for _, p := range tipo.proximos {
		fmt.Fprintf(w, "  %s\n", p)
	}
	return nil
}

func cmdNovo(args []string) {
	if err := novoProjeto(args, os.Stdout); err != nil {
		fmt.Println("gs novo: " + err.Error())
		os.Exit(1)
	}
}
