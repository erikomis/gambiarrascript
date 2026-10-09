//go:build !js

// Integracao do `gs migra` e de um script .gs com postgres e mysql de verdade
// (nos dois engines, processo `gs roda` de verdade). So rodam com
// GS_TESTE_POSTGRES / GS_TESTE_MYSQL (url do conecta()); sem elas, pulam. No
// CI e o job `banco`. Rodar com -p 1: outros pacotes usam o mesmo banco.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gambiarrascript/interpreter"
)

type bancoGs struct {
	nome, url, ph string // ph: placeholder com %d no lugar do numero
}

func (b bancoGs) p(i int) string {
	return strings.ReplaceAll(b.ph, "%d", string(rune('0'+i)))
}

func bancosGs(t *testing.T) []bancoGs {
	t.Helper()
	var out []bancoGs
	if u := os.Getenv("GS_TESTE_POSTGRES"); u != "" {
		out = append(out, bancoGs{"postgres", u, "$%d"})
	}
	if u := os.Getenv("GS_TESTE_MYSQL"); u != "" {
		out = append(out, bancoGs{"mysql", u, "?"})
	}
	if len(out) == 0 {
		t.Skip("sem GS_TESTE_POSTGRES/GS_TESTE_MYSQL: integracao com banco pulada")
	}
	return out
}

// limpaGs derruba as tabelas do teste e a gs_migracoes.
func limpaGs(t *testing.T, url string) {
	t.Helper()
	db, _, err := interpreter.AbreBanco(url)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	defer db.Close()
	for _, tb := range []string{"gs_it_posts", "gs_it_gente", "gs_migracoes"} {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + tb); err != nil {
			t.Fatalf("drop %s: %v", tb, err)
		}
	}
}

func escreveGs(t *testing.T, caminho, conteudo string) {
	t.Helper()
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIntegracaoGsMigra(t *testing.T) {
	for _, b := range bancosGs(t) {
		t.Run(b.nome, func(t *testing.T) {
			limpaGs(t, b.url)
			t.Cleanup(func() { limpaGs(t, b.url) })
			pasta := filepath.Join(t.TempDir(), "migracoes")
			t.Setenv("GS_BANCO", b.url)

			migraOk(t, "--pasta", pasta, "novo", "cria gente")
			migraOk(t, "--pasta", pasta, "novo", "cria posts")
			escreveGs(t, filepath.Join(pasta, "001_cria_gente.sobe.sql"), `-- gente
CREATE TABLE gs_it_gente (id INTEGER PRIMARY KEY, email VARCHAR(100) UNIQUE);
CREATE INDEX gs_it_gente_email ON gs_it_gente (email);
INSERT INTO gs_it_gente (id, email) VALUES (1, 'ze@x.com');
`)
			escreveGs(t, filepath.Join(pasta, "001_cria_gente.desce.sql"), "DROP TABLE gs_it_gente;\n")
			escreveGs(t, filepath.Join(pasta, "002_cria_posts.sobe.sql"),
				"CREATE TABLE gs_it_posts (id INTEGER PRIMARY KEY, autor INTEGER, FOREIGN KEY (autor) REFERENCES gs_it_gente (id));\n"+
					"INSERT INTO gs_it_posts (id, autor) VALUES (1, 1);\n")
			escreveGs(t, filepath.Join(pasta, "002_cria_posts.desce.sql"), "DROP TABLE gs_it_posts;\n")

			if out := migraOk(t, "--pasta", pasta, "status"); !strings.Contains(out, "0 aplicada(s), 2 pendente(s)") {
				t.Fatalf("status antes: %s", out)
			}
			if out := migraOk(t, "--pasta", pasta, "sobe"); !strings.Contains(out, "2 migracoes aplicadas") {
				t.Fatalf("sobe: %s", out)
			}
			if out := migraOk(t, "--pasta", pasta); !strings.Contains(out, "banco em dia") {
				t.Fatalf("sobe de novo: %s", out)
			}
			if out := migraOk(t, "--pasta", pasta, "status"); !strings.Contains(out, "[x] 002_cria_posts") || !strings.Contains(out, "2 aplicada(s), 0 pendente(s)") {
				t.Fatalf("status depois: %s", out)
			}
			if out := migraOk(t, "--pasta", pasta, "desce", "2"); !strings.Contains(out, "desfeita  002_cria_posts\n  desfeita  001_cria_gente") {
				t.Fatalf("desce 2: %s", out)
			}

			// checksum: aplica, mexe no arquivo, sobe recusa e status marca [!]
			migraOk(t, "--pasta", pasta, "--banco", b.url)
			escreveGs(t, filepath.Join(pasta, "001_cria_gente.sobe.sql"), "CREATE TABLE gs_it_gente (id INTEGER);\n")
			var buf bytes.Buffer
			err := executaMigra([]string{"--pasta", pasta}, &buf)
			if err == nil || !strings.Contains(err.Error(), "001_cria_gente ja foi aplicada mas o arquivo mudou") {
				t.Fatalf("esperava recusa por checksum, veio %v", err)
			}
			buf.Reset()
			err = executaMigra([]string{"--pasta", pasta, "status"}, &buf)
			if err == nil || !strings.Contains(buf.String(), "[!] 001_cria_gente") {
				t.Fatalf("status com arquivo mudado: %v\n%s", err, buf.String())
			}
		})
	}
}

// Ida e volta pelo `gs roda` de verdade, nos dois engines: migra() na subida,
// executa com placeholder (soltos e em lista), consulta com filtro, tipos na
// saida, migra() de novo = [] e fecha.
func TestIntegracaoGsScriptDoisEngines(t *testing.T) {
	for _, b := range bancosGs(t) {
		t.Run(b.nome, func(t *testing.T) {
			dir := t.TempDir()
			pasta := filepath.Join(dir, "migracoes")
			if err := os.MkdirAll(pasta, 0o755); err != nil {
				t.Fatal(err)
			}
			escreveGs(t, filepath.Join(pasta, "001_gente.sobe.sql"),
				"CREATE TABLE gs_it_gente (nome VARCHAR(50) NOT NULL, idade INTEGER, altura DOUBLE PRECISION, apelido VARCHAR(20));\n"+
					"INSERT INTO gs_it_gente (nome, idade, altura) VALUES ('Bento', 12, 1.5);\n")
			ins := "INSERT INTO gs_it_gente (nome, idade, altura, apelido) VALUES (" + b.p(1) + ", " + b.p(2) + ", " + b.p(3) + ", " + b.p(4) + ")"
			script := `bota con = conecta(env("GS_IT_URL"))
mostra migra(con, env("GS_IT_PASTA"))
mostra executa(con, "` + ins + `", "Zé", 30, 1.75, "zezinho")
mostra executa(con, "` + ins + `", ["Rita", 25, 1.6, nada])
pra_cada l em consulta(con, "SELECT nome, idade, altura, apelido FROM gs_it_gente WHERE idade > ` + b.p(1) + ` ORDER BY idade", 18)
    mostra "${l.nome}|${l.idade}|${l.altura}|${l.apelido}|${tipo(l.idade)}|${tipo(l.altura)}"
acabou_finalmente
bota total = consulta(con, "SELECT COUNT(*) AS n FROM gs_it_gente")
mostra total[0].n + 1
mostra executa(con, "UPDATE gs_it_gente SET idade = idade + 1 WHERE idade < ` + b.p(1) + `", 100)
mostra migra(con, env("GS_IT_PASTA"))
fecha(con)
mostra "fim"
`
			arq := filepath.Join(dir, "banco.gs")
			escreveGs(t, arq, script)

			var saidas []string
			for _, eng := range [][]string{{"roda", "--tree", arq}, {"roda", arq}} {
				limpaGs(t, b.url)
				cmd := gsDeVerdade(t, eng...)
				cmd.Env = append(cmd.Env, "GS_IT_URL="+b.url, "GS_IT_PASTA="+pasta)
				var out, errOut bytes.Buffer
				cmd.Stdout, cmd.Stderr = &out, &errOut
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				fim := make(chan error, 1)
				go func() { fim <- cmd.Wait() }()
				select {
				case err := <-fim:
					if err != nil {
						t.Fatalf("gs %v: %v\nstdout: %s\nstderr: %s", eng, err, out.String(), errOut.String())
					}
				case <-time.After(60 * time.Second):
					cmd.Process.Kill()
					<-fim
					t.Fatalf("gs %v travou", eng)
				}
				saidas = append(saidas, out.String())
			}
			limpaGs(t, b.url)

			quer := `[1]
1
1
Rita|25|1.6|nada|numero|numero
Zé|30|1.75|zezinho|numero|numero
4
3
[]
fim
`
			for i, eng := range []string{"tree", "vm"} {
				if saidas[i] != quer {
					t.Errorf("engine %s:\n%s\nesperado:\n%s", eng, saidas[i], quer)
				}
			}
		})
	}
}
