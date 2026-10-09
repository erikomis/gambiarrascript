package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gambiarrascript/formatter"
	"gambiarrascript/lexer"
	"gambiarrascript/lsp"
	"gambiarrascript/parser"
)

// arquivos que cada esqueleto tem que gerar (amostra do essencial)
var arquivosEsperados = map[string][]string{
	"api": {
		"principal.gs", "rotas/usuarios.gs", "rotas/tarefas.gs", "lib/http.gs",
		"migracoes/001_cria_usuarios.sobe.sql", "migracoes/001_cria_usuarios.desce.sql",
		"migracoes/002_cria_tarefas.sobe.sql", "migracoes/002_cria_tarefas.desce.sql",
		"http_test.gs", "usuarios_test.gs", "tarefas_test.gs",
		".env.exemplo", ".gitignore", ".dockerignore", "Dockerfile", "README.md",
		"gambiarra.json", "dados/.gitkeep",
	},
	"site": {
		"principal.gs", "conteudo.gs", "conteudo_test.gs",
		"modelos/base.html", "modelos/parciais/cabecalho.html", "modelos/parciais/rodape.html",
		"modelos/inicio.html", "estatico/estilo.css",
		".gitignore", "Dockerfile", "README.md", "gambiarra.json",
	},
	"script": {"principal.gs", "gambiarra.json", "README.md", ".gitignore"},
}

func TestNovoCadaTipoGeraProjetoValido(t *testing.T) {
	if len(arquivosEsperados) != len(tiposProjeto) {
		t.Fatalf("tem tipo sem teste: %d tipos, %d no teste", len(tiposProjeto), len(arquivosEsperados))
	}
	for _, tipo := range tiposProjeto {
		t.Run(tipo.nome, func(t *testing.T) {
			destino := filepath.Join(t.TempDir(), "meu-"+tipo.nome)
			criados, err := criaProjeto(tipo.nome, "meu-"+tipo.nome, destino)
			if err != nil {
				t.Fatalf("criaProjeto: %v", err)
			}
			tem := map[string]bool{}
			for _, c := range criados {
				tem[c] = true
			}
			for _, arq := range arquivosEsperados[tipo.nome] {
				if !tem[arq] {
					t.Errorf("faltou %s (criados: %v)", arq, criados)
				}
			}
			for _, c := range criados {
				conteudo, err := os.ReadFile(filepath.Join(destino, filepath.FromSlash(c)))
				if err != nil {
					t.Fatal(err)
				}
				fonte := string(conteudo)
				if strings.Contains(fonte, marcaNome) || strings.Contains(fonte, marcaVersaoGs) {
					t.Errorf("%s ficou com marcador sem trocar", c)
				}
				if strings.HasSuffix(c, ".gs") {
					confereFonteGs(t, c, fonte)
				}
			}
			manifesto, _ := os.ReadFile(filepath.Join(destino, "gambiarra.json"))
			if !strings.Contains(string(manifesto), `"nome": "meu-`+tipo.nome+`"`) {
				t.Errorf("gambiarra.json sem o nome: %s", manifesto)
			}
		})
	}
}

// confereFonteGs: parseia, passa no lint sem aviso e ja esta formatado
// (gs check e gs formata nao reclamam do que o gs novo gera).
func confereFonteGs(t *testing.T, nome, fonte string) {
	t.Helper()
	p := parser.New(lexer.New(fonte))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Errorf("%s nao parseia: %v", nome, errs)
		return
	}
	for _, d := range lsp.Typecheck(prog) {
		t.Errorf("%s:%d: gs check reclamou: %s", nome, d.Range.Start.Line+1, d.Message)
	}
	formatado, err := formataConferido(fonte, formatter.FormataFonte)
	if err != nil {
		t.Errorf("%s nao formata: %v", nome, err)
	} else if formatado != fonte {
		t.Errorf("%s nao ta formatado (gs formata -w mudaria o arquivo)", nome)
	}
}

// os *_test.gs do projeto gerado passam (o que o `gs testa` roda la dentro)
func TestNovoTestesDoProjetoPassam(t *testing.T) {
	for _, tipo := range []string{"api", "site"} {
		t.Run(tipo, func(t *testing.T) {
			destino := filepath.Join(t.TempDir(), "proj")
			if _, err := criaProjeto(tipo, "proj", destino); err != nil {
				t.Fatal(err)
			}
			// os testes do site leem modelos/ relativo a pasta atual
			antes, _ := os.Getwd()
			if err := os.Chdir(destino); err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(antes)
			var saida bytes.Buffer
			if !executaTestes(opcoesTesta{dir: ".", usarVM: true}, &saida) {
				t.Fatalf("gs testa falhou no projeto %s:\n%s", tipo, saida.String())
			}
			if !strings.Contains(saida.String(), "0 com perrengue") {
				t.Fatalf("saida inesperada:\n%s", saida.String())
			}
		})
	}
}

func TestNovoRecusaPastaComConteudo(t *testing.T) {
	destino := filepath.Join(t.TempDir(), "ja-tem")
	if err := os.MkdirAll(destino, 0o755); err != nil {
		t.Fatal(err)
	}
	meu := filepath.Join(destino, "principal.gs")
	if err := os.WriteFile(meu, []byte("mostra 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := criaProjeto("api", "ja-tem", destino)
	if err == nil || !strings.Contains(err.Error(), "nao vou passar por cima") {
		t.Fatalf("devia recusar pasta com conteudo, veio %v", err)
	}
	if b, _ := os.ReadFile(meu); string(b) != "mostra 1\n" {
		t.Fatalf("mexeu no arquivo que ja existia: %q", b)
	}
	if _, err := os.Stat(filepath.Join(destino, "rotas")); err == nil {
		t.Fatalf("criou coisa dentro da pasta recusada")
	}

	// arquivo (nao pasta) com o nome tambem e recusado
	arq := filepath.Join(t.TempDir(), "arquivo")
	os.WriteFile(arq, []byte("x"), 0o644)
	if _, err := criaProjeto("script", "arquivo", arq); err == nil {
		t.Fatalf("devia recusar quando o destino e um arquivo")
	}
}

func TestNovoAceitaPastaVazia(t *testing.T) {
	destino := filepath.Join(t.TempDir(), "vazia")
	os.MkdirAll(destino, 0o755)
	if _, err := criaProjeto("script", "vazia", destino); err != nil {
		t.Fatalf("pasta vazia devia ser aceita: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destino, "principal.gs")); err != nil {
		t.Fatalf("nao criou o principal.gs: %v", err)
	}
}

func TestNovoValidaNome(t *testing.T) {
	bons := []string{"api", "minha-api", "meu_site2", "a"}
	for _, n := range bons {
		if err := validaNomeProjeto(n); err != nil {
			t.Errorf("%q devia valer: %v", n, err)
		}
	}
	ruins := []string{"", "Maiuscula", "2fast", "-traco", "com espaco", "../fora", "a/b", ".escondido", "acentuação", strings.Repeat("a", 65)}
	for _, n := range ruins {
		if err := validaNomeProjeto(n); err == nil {
			t.Errorf("%q devia ser recusado", n)
		}
	}
}

func TestNovoSemArgumentosListaTipos(t *testing.T) {
	var saida bytes.Buffer
	if err := novoProjeto(nil, &saida); err != nil {
		t.Fatal(err)
	}
	for _, tipo := range tiposProjeto {
		if !strings.Contains(saida.String(), tipo.nome) {
			t.Errorf("a lista nao mostra o tipo %s:\n%s", tipo.nome, saida.String())
		}
	}
}

func TestNovoArgumentosErrados(t *testing.T) {
	var saida bytes.Buffer
	if err := novoProjeto([]string{"blog", "x"}, &saida); err == nil || !strings.Contains(err.Error(), "api, site, script") {
		t.Fatalf("tipo desconhecido devia listar os tipos, veio %v", err)
	}
	if err := novoProjeto([]string{"api"}, &saida); err == nil || !strings.Contains(err.Error(), "gs novo api <nome>") {
		t.Fatalf("faltar o nome devia mostrar o uso, veio %v", err)
	}
}

// gs novo cria a pasta relativa ao diretorio atual e mostra os proximos passos
func TestNovoCriaNaPastaAtual(t *testing.T) {
	dir := t.TempDir()
	antes, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(antes)
	var saida bytes.Buffer
	if err := novoProjeto([]string{"api", "loja"}, &saida); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "loja", "rotas", "tarefas.gs")); err != nil {
		t.Fatalf("nao criou loja/rotas/tarefas.gs: %v", err)
	}
	if !strings.Contains(saida.String(), "cd loja") {
		t.Fatalf("faltou o proximo passo:\n%s", saida.String())
	}
	// segunda vez: recusa
	if err := novoProjeto([]string{"api", "loja"}, &saida); err == nil {
		t.Fatalf("segunda vez devia recusar")
	}
}

func TestTagImagemGs(t *testing.T) {
	orig := Versao
	defer func() { Versao = orig }()
	Versao = "1.2.3"
	if tagImagemGs() != "1.2.3" {
		t.Fatalf("release devia fixar a versao, veio %s", tagImagemGs())
	}
	Versao = "dev"
	if tagImagemGs() != "latest" {
		t.Fatalf("build de dev devia usar latest, veio %s", tagImagemGs())
	}
}
