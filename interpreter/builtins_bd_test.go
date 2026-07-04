package interpreter

import (
	"testing"

	"gambiarrascript/object"
)

func TestUrlParaDriver(t *testing.T) {
	casos := []struct{ url, driver, dsn string }{
		{"sqlite::memory:", "sqlite", ":memory:"},
		{"sqlite://meu.db", "sqlite", "meu.db"},
		{"postgres://u:p@host/db", "pgx", "postgres://u:p@host/db"},
		{"mysql://u:p@host:3306/db", "mysql", "u:p@tcp(host:3306)/db"},
		{"mariadb://u:p@host:3306/db", "mysql", "u:p@tcp(host:3306)/db"},
	}
	for _, c := range casos {
		d, dsn, err := urlParaDriver(c.url)
		if err != nil {
			t.Fatalf("url %q: erro %v", c.url, err)
		}
		if d != c.driver || dsn != c.dsn {
			t.Fatalf("url %q => (%q, %q), esperado (%q, %q)", c.url, d, dsn, c.driver, c.dsn)
		}
	}
	if _, _, err := urlParaDriver("oracle://x"); err == nil {
		t.Fatal("banco desconhecido deveria dar erro")
	}
}

func TestConectaFechaSqlite(t *testing.T) {
	out := rodar(t, `bota c = conecta("sqlite::memory:")
mostra c
fecha(c)`)
	if out != "<nativo: conexao sqlite>\n" {
		t.Fatalf("got %q", out)
	}
}

func TestConectaUrlInvalida(t *testing.T) {
	if got := eval(t, `conecta("oracle://x")`); got.Type() != object.ERRO_OBJ {
		t.Fatalf("url desconhecida deveria dar erro, got %s", got.Type())
	}
	if got := eval(t, `fecha(42)`); got.Type() != object.ERRO_OBJ {
		t.Fatalf("fecha de nao-conexao deveria dar erro, got %s", got.Type())
	}
}

func TestConsultaExecutaSqlite(t *testing.T) {
	out := rodar(t, `bota c = conecta("sqlite::memory:")
executa(c, "CREATE TABLE gente (id INTEGER PRIMARY KEY, nome TEXT, idade INTEGER)")
bota n1 = executa(c, "INSERT INTO gente (nome, idade) VALUES (?, ?)", "Ze", 30)
bota n2 = executa(c, "INSERT INTO gente (nome, idade) VALUES (?, ?)", "Rita", 25)
mostra n1 + n2
bota r = consulta(c, "SELECT nome, idade FROM gente ORDER BY idade")
mostra tamanho(r)
mostra r[0]["nome"]
mostra r[1]["idade"]
fecha(c)`)
	esperado := "2\n2\nRita\n30\n"
	if out != esperado {
		t.Fatalf("saida: %q esperado %q", out, esperado)
	}
}

func TestConsultaSemArg(t *testing.T) {
	out := rodar(t, `bota c = conecta("sqlite::memory:")
executa(c, "CREATE TABLE x (v INTEGER)")
executa(c, "INSERT INTO x VALUES (42)")
bota r = consulta(c, "SELECT v FROM x")
mostra r[0]["v"]`)
	if out != "42\n" {
		t.Fatalf("saida %q", out)
	}
}
