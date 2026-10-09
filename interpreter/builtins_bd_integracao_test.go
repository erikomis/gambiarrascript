//go:build !js

// Testes de integracao com postgres e mysql de verdade. So rodam com as
// variaveis de ambiente (url no formato do conecta()):
//
//	GS_TESTE_POSTGRES=postgres://gs:gs@127.0.0.1:5432/gs_teste?sslmode=disable
//	GS_TESTE_MYSQL=mysql://gs:gs@127.0.0.1:3306/gs_teste
//
// Sem elas, pulam. No CI e o job `banco` (.github/workflows/ci.yml). Rodar com
// -p 1: os pacotes dividem o mesmo banco (e a tabela gs_migracoes).

package interpreter

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gambiarrascript/object"
)

type bancoIntegracao struct {
	nome   string // "postgres" ou "mysql"
	url    string
	driver string // nome no database/sql
}

// ph devolve o placeholder i (1-based) no estilo do banco.
func (b bancoIntegracao) ph(i int) string {
	if b.driver == "pgx" {
		return fmt.Sprintf("$%d", i)
	}
	return "?"
}

func bancosIntegracao(t *testing.T) []bancoIntegracao {
	t.Helper()
	var out []bancoIntegracao
	if u := os.Getenv("GS_TESTE_POSTGRES"); u != "" {
		out = append(out, bancoIntegracao{"postgres", u, "pgx"})
	}
	if u := os.Getenv("GS_TESTE_MYSQL"); u != "" {
		out = append(out, bancoIntegracao{"mysql", u, "mysql"})
	}
	if len(out) == 0 {
		t.Skip("sem GS_TESTE_POSTGRES/GS_TESTE_MYSQL: integracao com banco pulada")
	}
	return out
}

// semErro falha o teste se a builtin devolveu Erro.
func semErro(t *testing.T, o object.Object, oque string) object.Object {
	t.Helper()
	if e, ok := o.(*object.Erro); ok {
		t.Fatalf("%s: %s", oque, e.Inspect())
	}
	return o
}

func conectaIntegracao(t *testing.T, b bancoIntegracao) object.Object {
	t.Helper()
	con := semErro(t, builtinConecta([]object.Object{txt(b.url)}), "conecta")
	if got := con.Inspect(); !strings.Contains(got, "conexao "+b.driver) {
		t.Fatalf("conecta devolveu %s", got)
	}
	t.Cleanup(func() { builtinFecha([]object.Object{con}) })
	return con
}

func executaIt(t *testing.T, con object.Object, sql string, params ...object.Object) object.Object {
	t.Helper()
	args := append([]object.Object{con, txt(sql)}, params...)
	return semErro(t, builtinExecuta(args), sql)
}

func linhas(t *testing.T, con object.Object, sql string, params ...object.Object) []*object.Dicionario {
	t.Helper()
	args := append([]object.Object{con, txt(sql)}, params...)
	lst, ok := semErro(t, builtinConsulta(args), sql).(*object.Lista)
	if !ok {
		t.Fatalf("consulta devia devolver lista")
	}
	var out []*object.Dicionario
	for _, o := range lst.Visao() {
		d, ok := o.(*object.Dicionario)
		if !ok {
			t.Fatalf("linha devia ser dicionario, veio %T", o)
		}
		out = append(out, d)
	}
	return out
}

// campo confere tipo e valor de uma coluna. tipo: "int", "real", "texto",
// "bool", "nada".
func campo(t *testing.T, d *object.Dicionario, col, tipo, valor string) {
	t.Helper()
	v, ok := d.PegaTexto(col)
	if !ok {
		t.Errorf("coluna %q sumiu da linha %s", col, d.Inspect())
		return
	}
	certo := false
	switch x := v.(type) {
	case *object.Numero:
		certo = (tipo == "int" && x.EhInt) || (tipo == "real" && !x.EhInt)
	case *object.Texto:
		certo = tipo == "texto"
	case *object.Booleano:
		certo = tipo == "bool"
	case *object.Nada:
		certo = tipo == "nada"
	}
	if !certo || v.Inspect() != valor {
		t.Errorf("coluna %q: veio %T %s, esperado %s %s", col, v, v.Inspect(), tipo, valor)
	}
}

// DDL que muda de banco pra banco: REAL do postgres e float4 (FLOAT no
// mysql), DATETIME e o TIMESTAMP sem fuso do mysql.
func ddlTipos(b bancoIntegracao) string {
	peso, criado := "REAL", "TIMESTAMP"
	if b.driver == "mysql" {
		peso, criado = "FLOAT", "DATETIME"
	}
	return `CREATE TABLE gs_it_tipos (
	id INTEGER PRIMARY KEY,
	nome VARCHAR(50),
	preco DOUBLE PRECISION,
	peso ` + peso + `,
	ativo BOOLEAN,
	grande BIGINT,
	criado ` + criado + `,
	dia DATE,
	nota VARCHAR(10),
	saldo DECIMAL(10,2)
)`
}

func TestIntegracaoBDTiposEPlaceholders(t *testing.T) {
	for _, b := range bancosIntegracao(t) {
		t.Run(b.nome, func(t *testing.T) {
			con := conectaIntegracao(t, b)
			executaIt(t, con, "DROP TABLE IF EXISTS gs_it_tipos")
			t.Cleanup(func() { builtinExecuta([]object.Object{con, txt("DROP TABLE IF EXISTS gs_it_tipos")}) })
			executaIt(t, con, ddlTipos(b))

			ins := "INSERT INTO gs_it_tipos (id, nome, preco, peso, ativo, grande, criado, dia, nota, saldo) VALUES (" +
				b.ph(1) + ", " + b.ph(2) + ", " + b.ph(3) + ", " + b.ph(4) + ", " + b.ph(5) + ", " +
				b.ph(6) + ", " + b.ph(7) + ", " + b.ph(8) + ", " + b.ph(9) + ", " + b.ph(10) + ")"
			// params soltos
			n := executaIt(t, con, ins, object.NumInt(1), txt("café com pão"), &object.Numero{Value: 2.5},
				&object.Numero{Value: 1.1}, boolDoNativo(true), object.NumInt(9007199254740993),
				txt("2024-03-05 10:20:30"), txt("2024-03-05"), NADA, &object.Numero{Value: 12.34})
			if n.Inspect() != "1" {
				t.Fatalf("executa devia devolver 1 linha afetada, veio %s", n.Inspect())
			}
			// params numa lista, quase tudo nada
			lista := object.NovaLista([]object.Object{object.NumInt(2), NADA, NADA, NADA,
				boolDoNativo(false), NADA, NADA, NADA, txt("ok"), NADA})
			executaIt(t, con, ins, lista)

			// sem parametro (no mysql: protocolo de texto) e com parametro
			// (protocolo binario) tem que dar os mesmos tipos.
			sel := "SELECT id, nome, preco, peso, ativo, grande, criado, dia, nota, saldo FROM gs_it_tipos"
			for _, caso := range []struct {
				nome string
				rs   []*object.Dicionario
			}{
				{"sem params", linhas(t, con, sel+" ORDER BY id")},
				{"com params", linhas(t, con, sel+" WHERE id >= "+b.ph(1)+" ORDER BY id", object.NumInt(1))},
			} {
				t.Run(caso.nome, func(t *testing.T) {
					if len(caso.rs) != 2 {
						t.Fatalf("esperava 2 linhas, veio %d", len(caso.rs))
					}
					l1, l2 := caso.rs[0], caso.rs[1]
					campo(t, l1, "id", "int", "1")
					campo(t, l1, "nome", "texto", "café com pão")
					campo(t, l1, "preco", "real", "2.5")
					campo(t, l1, "peso", "real", "1.1")
					if b.driver == "mysql" {
						// BOOLEAN do mysql e TINYINT(1): volta 1/0
						campo(t, l1, "ativo", "int", "1")
						campo(t, l2, "ativo", "int", "0")
					} else {
						campo(t, l1, "ativo", "bool", "deu_bom")
						campo(t, l2, "ativo", "bool", "deu_ruim")
					}
					campo(t, l1, "grande", "int", "9007199254740993")
					campo(t, l1, "criado", "texto", "2024-03-05T10:20:30Z")
					campo(t, l1, "dia", "texto", "2024-03-05T00:00:00Z")
					campo(t, l1, "nota", "nada", "nada")
					// NUMERIC/DECIMAL volta texto pra nao perder casa decimal
					campo(t, l1, "saldo", "texto", "12.34")

					campo(t, l2, "id", "int", "2")
					for _, c := range []string{"nome", "preco", "peso", "grande", "criado", "dia", "saldo"} {
						campo(t, l2, c, "nada", "nada")
					}
					campo(t, l2, "nota", "texto", "ok")
				})
			}

			// agregados: COUNT e SUM
			agg := linhas(t, con, "SELECT COUNT(*) AS n FROM gs_it_tipos")
			campo(t, agg[0], "n", "int", "2")

			// UPDATE devolve linhas afetadas; DELETE tambem
			up := executaIt(t, con, "UPDATE gs_it_tipos SET nota = "+b.ph(1)+" WHERE id IN ("+b.ph(2)+", "+b.ph(3)+")",
				txt("mudou"), object.NumInt(1), object.NumInt(2))
			if up.Inspect() != "2" {
				t.Errorf("UPDATE devia afetar 2, veio %s", up.Inspect())
			}
			rs := linhas(t, con, "SELECT nota FROM gs_it_tipos WHERE id = "+b.ph(1), object.NumInt(2))
			campo(t, rs[0], "nota", "texto", "mudou")
			if del := executaIt(t, con, "DELETE FROM gs_it_tipos WHERE id = "+b.ph(1), object.NumInt(2)); del.Inspect() != "1" {
				t.Errorf("DELETE devia afetar 1, veio %s", del.Inspect())
			}
			if rs := linhas(t, con, "SELECT id FROM gs_it_tipos WHERE id = "+b.ph(1), object.NumInt(99)); len(rs) != 0 {
				t.Errorf("consulta sem resultado devia dar lista vazia, veio %d", len(rs))
			}
		})
	}
}

func TestIntegracaoBDErros(t *testing.T) {
	for _, b := range bancosIntegracao(t) {
		t.Run(b.nome, func(t *testing.T) {
			con := conectaIntegracao(t, b)
			if e, ok := builtinExecuta([]object.Object{con, txt("CREATE TABEL nada_disso (x INT)")}).(*object.Erro); !ok ||
				!strings.Contains(e.Inspect(), "executa falhou") {
				t.Errorf("sql invalido no executa devia dar erro, veio %v", e)
			}
			if e, ok := builtinConsulta([]object.Object{con, txt("SELECT * FROM gs_it_nao_existe")}).(*object.Erro); !ok ||
				!strings.Contains(e.Inspect(), "consulta falhou") {
				t.Errorf("tabela inexistente devia dar erro, veio %v", e)
			}
			// a conexao segue boa depois do erro
			rs := linhas(t, con, "SELECT 1 AS um")
			campo(t, rs[0], "um", "int", "1")

			// senha errada: conecta falha no ping, nao na primeira consulta
			ruim := strings.Replace(b.url, "://gs:gs@", "://gs:errada@", 1)
			if ruim != b.url {
				if e, ok := builtinConecta([]object.Object{txt(ruim)}).(*object.Erro); !ok ||
					!strings.Contains(e.Inspect(), "nao consegui conectar") {
					t.Errorf("senha errada devia falhar no conecta, veio %v", e)
				}
			}

			builtinFecha([]object.Object{con})
			if _, ok := builtinConsulta([]object.Object{con, txt("SELECT 1")}).(*object.Erro); !ok {
				t.Error("consulta depois do fecha devia dar erro")
			}
		})
	}
}
