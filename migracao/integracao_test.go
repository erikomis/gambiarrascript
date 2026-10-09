//go:build !js

// Testes de integracao das migracoes com postgres e mysql de verdade. So
// rodam com GS_TESTE_POSTGRES / GS_TESTE_MYSQL (url do conecta()); sem elas,
// pulam. No CI e o job `banco`. Rodar com -p 1: outros pacotes usam o mesmo
// banco e a mesma tabela gs_migracoes.
//
// Pacote externo (migracao_test) pra poder usar interpreter.AbreBanco, que
// e quem traduz a url — o interpreter importa migracao.

package migracao_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gambiarrascript/interpreter"
	"gambiarrascript/migracao"
)

type bancoIt struct {
	nome, url string
}

func bancosIt(t *testing.T) []bancoIt {
	t.Helper()
	var out []bancoIt
	if u := os.Getenv("GS_TESTE_POSTGRES"); u != "" {
		out = append(out, bancoIt{"postgres", u})
	}
	if u := os.Getenv("GS_TESTE_MYSQL"); u != "" {
		out = append(out, bancoIt{"mysql", u})
	}
	if len(out) == 0 {
		t.Skip("sem GS_TESTE_POSTGRES/GS_TESTE_MYSQL: integracao com banco pulada")
	}
	return out
}

// tabelasIt sao as tabelas que os testes criam; limpa antes e depois (ordem
// importa por causa da chave estrangeira).
var tabelasIt = []string{"gs_it_livros", "gs_it_autores", "gs_it_parcial", migracao.Tabela}

func abreIt(t *testing.T, b bancoIt) (*sql.DB, string) {
	t.Helper()
	db, driver, err := interpreter.AbreBanco(b.url)
	if err != nil {
		t.Fatalf("conectar no %s: %v", b.nome, err)
	}
	limpa := func() {
		if driver == "pgx" {
			db.Exec("DROP FUNCTION IF EXISTS gs_it_dobro(INTEGER)")
		}
		for _, tb := range tabelasIt {
			if _, err := db.Exec("DROP TABLE IF EXISTS " + tb); err != nil {
				t.Logf("limpando %s: %v", tb, err)
			}
		}
	}
	limpa()
	t.Cleanup(func() { limpa(); db.Close() })
	return db, driver
}

func escreveIt(t *testing.T, dir, nome, conteudo string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, nome), []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func existe(db *sql.DB, tabela string) bool {
	var n int64
	return db.QueryRow("SELECT COUNT(*) FROM "+tabela).Scan(&n) == nil
}

func versoes(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	rows, err := db.Query("SELECT versao FROM " + migracao.Tabela + " ORDER BY versao")
	if err != nil {
		t.Fatalf("lendo %s: %v", migracao.Tabela, err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

// Arquivo com varios comandos, comentarios e `;` dentro de texto e de
// comentario: no postgres vai num Exec so (dentro da transacao); no mysql
// passa pelo Separa e roda comando por comando.
const sobeAutores = `-- autores; com indice
CREATE TABLE gs_it_autores (
    id INTEGER PRIMARY KEY,
    nome VARCHAR(100) NOT NULL
);
CREATE INDEX gs_it_autores_nome ON gs_it_autores (nome);

/* comentario de bloco; com ponto e virgula */
INSERT INTO gs_it_autores (id, nome) VALUES (1, 'Ana; a primeira');
INSERT INTO gs_it_autores (id, nome) VALUES (2, 'Bia -- nao e comentario');
INSERT INTO gs_it_autores (id, nome) VALUES (3, 'D''Avila; com aspas');
-- sobra so comentario depois do ultimo ;
`

const sobeLivros = `CREATE TABLE gs_it_livros (
    id INTEGER PRIMARY KEY,
    autor INTEGER NOT NULL,
    titulo VARCHAR(100),
    FOREIGN KEY (autor) REFERENCES gs_it_autores (id)
);
INSERT INTO gs_it_livros (id, autor, titulo) VALUES (10, 1, 'Primeiro');
INSERT INTO gs_it_livros (id, autor, titulo) VALUES (11, 3, 'Segundo');
`

func pastaIt(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	escreveIt(t, dir, "001_cria_autores.sobe.sql", sobeAutores)
	escreveIt(t, dir, "001_cria_autores.desce.sql", "DROP TABLE gs_it_autores;\n")
	escreveIt(t, dir, "002_cria_livros.sobe.sql", sobeLivros)
	escreveIt(t, dir, "002_cria_livros.desce.sql", "DROP TABLE gs_it_livros;\n")
	return dir
}

func TestIntegracaoMigraSobeStatusDesce(t *testing.T) {
	for _, b := range bancosIt(t) {
		t.Run(b.nome, func(t *testing.T) {
			db, driver := abreIt(t, b)
			dir := pastaIt(t)
			m := migracao.Novo(db, driver, dir)

			st, err := m.Status()
			if err != nil || len(st) != 2 || st[0].Aplicada || st[1].Aplicada {
				t.Fatalf("status antes: %+v, %v", st, err)
			}
			feitas, err := m.Sobe()
			if err != nil || len(feitas) != 2 {
				t.Fatalf("sobe: %d aplicadas, %v", len(feitas), err)
			}
			if v := versoes(t, db); len(v) != 2 || v[0] != 1 || v[1] != 2 {
				t.Fatalf("gs_migracoes depois do sobe: %v", v)
			}

			// o texto com ; -- e '' chegou inteiro
			rows, err := db.Query("SELECT nome FROM gs_it_autores ORDER BY id")
			if err != nil {
				t.Fatal(err)
			}
			var nomes []string
			for rows.Next() {
				var s string
				rows.Scan(&s)
				nomes = append(nomes, s)
			}
			rows.Close()
			quer := "Ana; a primeira|Bia -- nao e comentario|D'Avila; com aspas"
			if got := strings.Join(nomes, "|"); got != quer {
				t.Fatalf("autores: %q, esperado %q", got, quer)
			}
			var livros int64
			if err := db.QueryRow("SELECT COUNT(*) FROM gs_it_livros").Scan(&livros); err != nil || livros != 2 {
				t.Fatalf("livros: %d, %v", livros, err)
			}

			st, err = m.Status()
			if err != nil || len(st) != 2 || !st[0].Aplicada || !st[1].Aplicada || st[0].AplicadaEm == "" {
				t.Fatalf("status depois: %+v, %v", st, err)
			}
			if feitas, err := m.Sobe(); err != nil || len(feitas) != 0 {
				t.Fatalf("sobe de novo devia ser no-op: %d, %v", len(feitas), err)
			}

			desfeitas, err := m.Desce(1)
			if err != nil || len(desfeitas) != 1 || desfeitas[0].Versao != 2 {
				t.Fatalf("desce 1: %+v, %v", desfeitas, err)
			}
			if existe(db, "gs_it_livros") || !existe(db, "gs_it_autores") {
				t.Fatal("desce 1 devia tirar so gs_it_livros")
			}
			if v := versoes(t, db); len(v) != 1 || v[0] != 1 {
				t.Fatalf("gs_migracoes depois do desce: %v", v)
			}
			// sobe de novo reaplica a 2; desce 5 tira tudo que tem
			if feitas, err := m.Sobe(); err != nil || len(feitas) != 1 {
				t.Fatalf("resobe: %d, %v", len(feitas), err)
			}
			desfeitas, err = m.Desce(5)
			if err != nil || len(desfeitas) != 2 {
				t.Fatalf("desce 5: %d, %v", len(desfeitas), err)
			}
			if existe(db, "gs_it_autores") || len(versoes(t, db)) != 0 {
				t.Fatal("desce tudo devia deixar o banco vazio")
			}
		})
	}
}

func TestIntegracaoMigraChecksumRecusa(t *testing.T) {
	for _, b := range bancosIt(t) {
		t.Run(b.nome, func(t *testing.T) {
			db, driver := abreIt(t, b)
			dir := pastaIt(t)
			m := migracao.Novo(db, driver, dir)
			if _, err := m.Sobe(); err != nil {
				t.Fatal(err)
			}
			escreveIt(t, dir, "001_cria_autores.sobe.sql", sobeAutores+"-- mexeram depois\n")
			escreveIt(t, dir, "003_mais.sobe.sql", "CREATE TABLE gs_it_parcial (id INTEGER);")

			if _, err := m.Sobe(); err == nil || !strings.Contains(err.Error(), "001_cria_autores ja foi aplicada mas o arquivo mudou") {
				t.Fatalf("sobe devia recusar por checksum, veio %v", err)
			}
			if existe(db, "gs_it_parcial") {
				t.Fatal("com checksum errado nada devia rodar (nem a 003)")
			}
			if _, err := m.Desce(1); err == nil || !strings.Contains(err.Error(), "arquivo mudou") {
				t.Fatalf("desce devia recusar por checksum, veio %v", err)
			}
			st, err := m.Status()
			if err != nil || len(st) != 3 || !st[0].Alterada || st[1].Alterada || st[2].Aplicada {
				t.Fatalf("status devia marcar a 001 alterada: %+v, %v", st, err)
			}
		})
	}
}

// Migracao que quebra no meio: no postgres a transacao desfaz ate o CREATE
// TABLE; no mysql o DDL ja fez commit e fica. Nos dois a 003 nao e anotada
// e as anteriores ficam.
func TestIntegracaoMigraFalhaNoMeio(t *testing.T) {
	for _, b := range bancosIt(t) {
		t.Run(b.nome, func(t *testing.T) {
			db, driver := abreIt(t, b)
			dir := pastaIt(t)
			escreveIt(t, dir, "003_quebra.sobe.sql", `CREATE TABLE gs_it_parcial (id INTEGER);
INSERT INTO gs_it_parcial (id) VALUES (1);
INSERT INTO gs_it_tabela_que_nao_existe (id) VALUES (1);
`)
			m := migracao.Novo(db, driver, dir)
			feitas, err := m.Sobe()
			if err == nil || !strings.Contains(err.Error(), "migracao 003_quebra falhou") {
				t.Fatalf("esperava falha na 003, veio %v", err)
			}
			if len(feitas) != 2 {
				t.Fatalf("as 2 antes da falha deviam ter sido aplicadas, veio %d", len(feitas))
			}
			if v := versoes(t, db); len(v) != 2 {
				t.Fatalf("a 003 nao devia estar anotada: %v", v)
			}
			ficou := existe(db, "gs_it_parcial")
			if driver == "mysql" {
				if !ficou {
					t.Fatal("no mysql o CREATE TABLE antes da falha faz commit e devia ficar")
				}
				var n int64
				if err := db.QueryRow("SELECT COUNT(*) FROM gs_it_parcial").Scan(&n); err != nil || n != 1 {
					t.Fatalf("no mysql o INSERT antes da falha tambem fica (autocommit): %d, %v", n, err)
				}
			} else if ficou {
				t.Fatal("no postgres a transacao devia desfazer o CREATE TABLE")
			}

			// conserta o arquivo (nunca foi aplicado, entao pode) e sobe
			conserto := "CREATE TABLE gs_it_parcial (id INTEGER);\n"
			if driver == "mysql" {
				conserto = "CREATE TABLE IF NOT EXISTS gs_it_parcial (id INTEGER);\n"
			}
			escreveIt(t, dir, "003_quebra.sobe.sql", conserto)
			if feitas, err := m.Sobe(); err != nil || len(feitas) != 1 {
				t.Fatalf("depois do conserto: %d, %v", len(feitas), err)
			}
		})
	}
}

// Postgres manda o arquivo inteiro num Exec: corpo de funcao com ; dentro
// de $$ ... $$ passa sem separador nenhum.
func TestIntegracaoMigraPostgresFuncao(t *testing.T) {
	for _, b := range bancosIt(t) {
		if b.nome != "postgres" {
			continue
		}
		t.Run(b.nome, func(t *testing.T) {
			db, driver := abreIt(t, b)
			dir := t.TempDir()
			escreveIt(t, dir, "001_funcao.sobe.sql", `CREATE FUNCTION gs_it_dobro(x INTEGER) RETURNS INTEGER AS $$
BEGIN
    RETURN x * 2;
END;
$$ LANGUAGE plpgsql;
`)
			escreveIt(t, dir, "001_funcao.desce.sql", "DROP FUNCTION gs_it_dobro(INTEGER);")
			m := migracao.Novo(db, driver, dir)
			if _, err := m.Sobe(); err != nil {
				t.Fatal(err)
			}
			var n int64
			if err := db.QueryRow("SELECT gs_it_dobro(21)").Scan(&n); err != nil || n != 42 {
				t.Fatalf("gs_it_dobro(21) = %d, %v", n, err)
			}
			if _, err := m.Desce(1); err != nil {
				t.Fatal(err)
			}
		})
	}
}
