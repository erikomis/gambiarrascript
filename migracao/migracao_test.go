package migracao

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func escreve(t *testing.T, dir, nome, conteudo string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, nome), []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func abre(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "teste.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func tabelas(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name != 'gs_migracoes' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		out = append(out, n)
	}
	return out
}

func pastaExemplo(t *testing.T) string {
	dir := t.TempDir()
	escreve(t, dir, "001_cria_gente.sobe.sql", `-- duas tabelas num arquivo so
CREATE TABLE gente (id INTEGER PRIMARY KEY, nome TEXT NOT NULL);
INSERT INTO gente (nome) VALUES ('Jurandir; o do ponto e virgula');
CREATE TABLE bicho (nome TEXT);
`)
	escreve(t, dir, "001_cria_gente.desce.sql", "DROP TABLE bicho;\nDROP TABLE gente;\n")
	escreve(t, dir, "002_cria_log.sobe.sql", "CREATE TABLE log (msg TEXT);\nCREATE TRIGGER grava AFTER INSERT ON gente BEGIN INSERT INTO log VALUES (NEW.nome); END;\n")
	escreve(t, dir, "002_cria_log.desce.sql", "DROP TRIGGER grava;\nDROP TABLE log;")
	escreve(t, dir, "LEIAME.txt", "ignorado")
	return dir
}

func TestSobeDesceStatus(t *testing.T) {
	db := abre(t)
	m := Novo(db, "sqlite", pastaExemplo(t))

	feitas, err := m.Sobe()
	if err != nil {
		t.Fatal(err)
	}
	if len(feitas) != 2 || feitas[0].Rotulo() != "001_cria_gente" || feitas[1].Rotulo() != "002_cria_log" {
		t.Fatalf("aplicadas erradas: %+v", feitas)
	}
	if got := tabelas(t, db); !reflect.DeepEqual(got, []string{"bicho", "gente", "log"}) {
		t.Fatalf("tabelas: %v", got)
	}
	var nome string
	db.QueryRow(`SELECT nome FROM gente`).Scan(&nome)
	if nome != "Jurandir; o do ponto e virgula" {
		t.Fatalf("insert do arquivo: %q", nome)
	}
	// de novo: nada pendente
	if feitas, err = m.Sobe(); err != nil || len(feitas) != 0 {
		t.Fatalf("segunda vez deveria ser nada: %v %v", feitas, err)
	}

	est, err := m.Status()
	if err != nil || len(est) != 2 || !est[0].Aplicada || !est[1].Aplicada || est[0].AplicadaEm == "" {
		t.Fatalf("status: %+v %v", est, err)
	}

	desfeitas, err := m.Desce(1)
	if err != nil || len(desfeitas) != 1 || desfeitas[0].Versao != 2 {
		t.Fatalf("desce 1: %+v %v", desfeitas, err)
	}
	if got := tabelas(t, db); !reflect.DeepEqual(got, []string{"bicho", "gente"}) {
		t.Fatalf("tabelas depois do desce: %v", got)
	}
	est, _ = m.Status()
	if !est[0].Aplicada || est[1].Aplicada {
		t.Fatalf("status depois do desce: %+v", est)
	}
	if desfeitas, err = m.Desce(5); err != nil || len(desfeitas) != 1 {
		t.Fatalf("desce alem do que tem: %+v %v", desfeitas, err)
	}
	if got := tabelas(t, db); len(got) != 0 {
		t.Fatalf("deveria ter zerado: %v", got)
	}
}

func TestChecksumMudou(t *testing.T) {
	db := abre(t)
	dir := pastaExemplo(t)
	m := Novo(db, "sqlite", dir)
	if _, err := m.Sobe(); err != nil {
		t.Fatal(err)
	}
	escreve(t, dir, "001_cria_gente.sobe.sql", "CREATE TABLE outra (x TEXT);")
	escreve(t, dir, "003_mais.sobe.sql", "CREATE TABLE mais (x TEXT);")
	_, err := m.Sobe()
	if err == nil || !strings.Contains(err.Error(), "001_cria_gente ja foi aplicada mas o arquivo mudou") {
		t.Fatalf("esperava recusa por checksum, veio %v", err)
	}
	if got := tabelas(t, db); strings.Contains(strings.Join(got, ","), "mais") {
		t.Fatal("nao era pra ter aplicado a 003")
	}
	if _, err := m.Desce(1); err == nil {
		t.Fatal("desce tambem deveria recusar")
	}
	est, err := m.Status()
	if err != nil || !est[0].Alterada || est[2].Aplicada {
		t.Fatalf("status deveria marcar alterada: %+v %v", est, err)
	}
}

func TestFalhaDesfazTransacao(t *testing.T) {
	db := abre(t)
	dir := t.TempDir()
	escreve(t, dir, "001_ok.sobe.sql", "CREATE TABLE a (x TEXT);")
	escreve(t, dir, "002_quebra.sobe.sql", "CREATE TABLE b (x TEXT);\nISSO NAO E SQL;\n")
	m := Novo(db, "sqlite", dir)
	feitas, err := m.Sobe()
	if err == nil || !strings.Contains(err.Error(), "002_quebra falhou") {
		t.Fatalf("esperava falha na 002, veio %v", err)
	}
	if len(feitas) != 1 {
		t.Fatalf("a 001 deveria ter ficado: %+v", feitas)
	}
	if got := tabelas(t, db); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("a tabela b deveria ter sido desfeita: %v", got)
	}
	if _, err := m.Desce(1); err == nil || !strings.Contains(err.Error(), "nao tem 001_ok.desce.sql") {
		t.Fatalf("desce sem arquivo: %v", err)
	}
}

func TestCarregaErros(t *testing.T) {
	casos := map[string][]string{
		"fora do padrao": {"cria.sql"},
		"repetida":       {"001_a.sobe.sql", "001_b.sobe.sql"},
		"sem o":          {"001_a.desce.sql"},
		"nao existe":     nil,
	}
	for msg, arqs := range casos {
		dir := t.TempDir()
		if arqs == nil {
			dir = filepath.Join(dir, "nada")
		}
		for _, a := range arqs {
			escreve(t, dir, a, "SELECT 1;")
		}
		if _, err := Carrega(dir); err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("%v: esperava erro com %q, veio %v", arqs, msg, err)
		}
	}
}

func TestCriaArquivos(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "migracoes")
	sobe, desce, err := CriaArquivos(dir, "Cria Usuários!")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(sobe) != "001_cria_usuarios.sobe.sql" || filepath.Base(desce) != "001_cria_usuarios.desce.sql" {
		t.Fatalf("nomes: %s %s", sobe, desce)
	}
	sobe, _, err = CriaArquivos(dir, "add_email")
	if err != nil || filepath.Base(sobe) != "002_add_email.sobe.sql" {
		t.Fatalf("segunda: %s %v", sobe, err)
	}
	if _, _, err := CriaArquivos(dir, "  !! "); err == nil {
		t.Fatal("nome vazio deveria dar erro")
	}
}

func TestSepara(t *testing.T) {
	got := Separa(`-- comeco
CREATE TABLE a (x TEXT DEFAULT 'a;b');
/* bloco ; */ INSERT INTO a VALUES ("c;d"); # fim ;
INSERT INTO a VALUES ('it\'s');
-- so comentario no fim`)
	if len(got) != 3 {
		t.Fatalf("esperava 3 comandos, veio %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "'a;b'") || !strings.Contains(got[1], `"c;d"`) || !strings.Contains(got[2], `'it\'s'`) {
		t.Fatalf("separou errado: %q", got)
	}
}
