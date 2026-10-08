//go:build !js

package vm

import (
	"os"
	"path/filepath"
	"testing"
)

// migra(conexao, [pasta]) nos dois engines, com sqlite num arquivo
// temporario: aplica as pendentes, devolve as versoes e na segunda vez nada.
func TestMigraParidade(t *testing.T) {
	for _, engine := range []string{"tree", "vm"} {
		t.Run(engine, func(t *testing.T) {
			dir := t.TempDir()
			pasta := filepath.Join(dir, "migracoes")
			os.MkdirAll(pasta, 0o755)
			arqs := map[string]string{
				"001_cria_gente.sobe.sql":  "CREATE TABLE gente (id INTEGER PRIMARY KEY, nome TEXT);\nINSERT INTO gente (nome) VALUES ('Jurandir');\n",
				"001_cria_gente.desce.sql": "DROP TABLE gente;",
				"002_poe_idade.sobe.sql":   "ALTER TABLE gente ADD COLUMN idade INTEGER;\nUPDATE gente SET idade = 30;",
			}
			for n, c := range arqs {
				if err := os.WriteFile(filepath.Join(pasta, n), []byte(c), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			banco := filepath.Join(dir, "app.db")
			src := `bota c = conecta("sqlite:` + banco + `")
mostra migra(c, "` + pasta + `")
mostra migra(c, "` + pasta + `")
bota r = consulta(c, "SELECT nome, idade FROM gente")
mostra "${r[0].nome} ${r[0].idade}"
mostra tamanho(consulta(c, "SELECT * FROM gs_migracoes"))
fecha(c)`
			roda := rodaTWComp
			if engine == "vm" {
				roda = rodaVMComp
			}
			_, saida, errStr := roda(t, src)
			if errStr != "" {
				t.Fatalf("erro: %s", errStr)
			}
			if saida != "[1, 2]\n[]\nJurandir 30\n2\n" {
				t.Fatalf("saida: %q", saida)
			}
		})
	}
}

func TestMigraErros(t *testing.T) {
	dir := t.TempDir()
	banco := filepath.Join(dir, "app.db")
	esperaNosDois(t, `bota c = conecta("sqlite:`+banco+`")
migra(c, "`+filepath.Join(dir, "nao_tem")+`")`, "", "pasta de migracoes")
	esperaNosDois(t, `migra(42)`, "", "esperava uma conexao de banco")
	pasta := filepath.Join(dir, "m")
	os.MkdirAll(pasta, 0o755)
	os.WriteFile(filepath.Join(pasta, "001_quebra.sobe.sql"), []byte("ISSO NAO E SQL;"), 0o644)
	esperaNosDois(t, `bota c = conecta("sqlite:`+banco+`")
migra(c, "`+pasta+`")`, "", "migracao 001_quebra falhou")
}
