//go:build !js

// Acesso a banco de dados (conecta/fecha). Fica de fora do build wasm (js)
// porque os drivers (modernc.org/sqlite via libc) nao compilam para
// GOOS=js/GOARCH=wasm. No navegador o stub em builtins_bd_js.go assume o lugar.

package interpreter

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gambiarrascript/migracao"
	"gambiarrascript/object"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type conexaoBD struct {
	db     *sql.DB
	driver string
}

// urlParaDriver mapeia o esquema da url para (driverName, dsn).
func urlParaDriver(bruta string) (string, string, error) {
	if strings.HasPrefix(bruta, "sqlite:") {
		dsn := strings.TrimPrefix(bruta, "sqlite:")
		dsn = strings.TrimPrefix(dsn, "//")
		return "sqlite", dsn, nil
	}
	u, err := url.Parse(bruta)
	if err != nil {
		return "", "", fmt.Errorf("url de banco invalida: %v", err)
	}
	switch u.Scheme {
	case "mysql", "mariadb":
		return "mysql", dsnMySQL(u), nil
	case "postgres", "postgresql":
		return "pgx", bruta, nil
	default:
		return "", "", fmt.Errorf("banco desconhecido: %q (use mysql, mariadb, postgres ou sqlite)", u.Scheme)
	}
}

// dsnMySQL converte mysql://user:pass@host:port/db no DSN do go-sql-driver.
func dsnMySQL(u *url.URL) string {
	cred := u.User.Username()
	if senha, ok := u.User.Password(); ok && senha != "" {
		cred += ":" + senha
	}
	host := u.Host
	if host == "" {
		host = "127.0.0.1:3306"
	}
	banco := strings.TrimPrefix(u.Path, "/")
	dsn := fmt.Sprintf("%s@tcp(%s)/%s", cred, host, banco)
	if u.RawQuery != "" {
		dsn += "?" + u.RawQuery
	}
	return dsn
}

func builtinConecta(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("conecta() quer 1 argumento (url), veio %d", len(args))
	}
	urlObj, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("conecta() espera a url como texto, veio %s", args[0].Type())
	}
	db, driver, err := AbreBanco(urlObj.Value)
	if err != nil {
		return erroBuiltin("%v", err)
	}
	return &object.Nativo{Rotulo: "conexao " + driver, Valor: &conexaoBD{db: db, driver: driver}}
}

// AbreBanco abre e testa (ping) a conexao a partir da url do conecta()
// ("sqlite:arquivo.db", "postgres://...", "mysql://..."). Devolve o nome do
// driver database/sql junto. Usado tambem pelo `gs migra`.
func AbreBanco(bruta string) (*sql.DB, string, error) {
	driver, dsn, err := urlParaDriver(bruta)
	if err != nil {
		return nil, "", err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, "", fmt.Errorf("nao consegui abrir o banco: %v", err)
	}
	if driver == "sqlite" {
		db.SetMaxOpenConns(1)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, "", fmt.Errorf("nao consegui conectar no banco: %v", err)
	}
	return db, driver, nil
}

func builtinFecha(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("fecha() quer 1 argumento (conexao), veio %d", len(args))
	}
	con, erro := pegaConexao(args[0])
	if erro != nil {
		return erro
	}
	con.db.Close()
	return NADA
}

// pegaConexao extrai o *conexaoBD de um argumento Nativo, ou devolve Erro.
func pegaConexao(o object.Object) (*conexaoBD, *object.Erro) {
	nat, ok := o.(*object.Nativo)
	if !ok {
		return nil, erroBuiltin("esperava uma conexao de banco, veio %s", o.Type())
	}
	con, ok := nat.Valor.(*conexaoBD)
	if !ok {
		return nil, erroBuiltin("esse nativo nao e uma conexao de banco")
	}
	return con, nil
}

// builtinConsulta executa um SELECT e devolve uma Lista de Dicionarios (cada
// par: nomeColuna -> valor). Params: podem ser passados como lista (3o arg)
// ou como argumentos soltos apos o sql. O driver decide o placeholder (?, $1).
func builtinConsulta(args []object.Object) object.Object {
	if len(args) < 2 {
		return erroBuiltin("consulta() quer conexao + sql (+params), veio %d", len(args))
	}
	con, e := pegaConexao(args[0])
	if e != nil {
		return e
	}
	sqlText, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("consulta() espera o sql como texto, veio %s", args[1].Type())
	}
	goArgs, e := argParaGo(args[2:])
	if e != nil {
		return e
	}
	rows, err := con.db.Query(sqlText.Value, goArgs...)
	if err != nil {
		return erroBuiltin("consulta falhou: %v", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return erroBuiltin("nao peguei colunas: %v", err)
	}
	linhas := []object.Object{}
	for rows.Next() {
		valores := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range valores {
			ptrs[i] = &valores[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return erroBuiltin("escanear linha: %v", err)
		}
		dic := object.NovoDicionario()
		for i, c := range cols {
			chave := &object.Texto{Value: c}
			dic.Bota(chave.ChaveHash(), object.ParDic{Chave: chave, Valor: goParaObj(valores[i])})
		}
		linhas = append(linhas, dic)
	}
	if err := rows.Err(); err != nil {
		return erroBuiltin("iteracao: %v", err)
	}
	return object.NovaLista(linhas)
}

// builtinExecuta roda INSERT/UPDATE/DELETE e devolve o numero de linhas
// afetadas (ou nada se o driver nao suportar). Mesma regra de params.
func builtinExecuta(args []object.Object) object.Object {
	if len(args) < 2 {
		return erroBuiltin("executa() quer conexao + sql (+params), veio %d", len(args))
	}
	con, e := pegaConexao(args[0])
	if e != nil {
		return e
	}
	sqlText, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("executa() espera o sql como texto, veio %s", args[1].Type())
	}
	goArgs, e := argParaGo(args[2:])
	if e != nil {
		return e
	}
	res, err := con.db.Exec(sqlText.Value, goArgs...)
	if err != nil {
		return erroBuiltin("executa falhou: %v", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return NADA
	}
	return object.NumInt(n)
}

// builtinMigra aplica as migracoes pendentes da pasta (padrao "migracoes")
// e devolve a lista de versoes aplicadas agora (vazia = banco ja estava em
// dia). Feito pro servidor migrar na subida: migra(con) antes do escuta().
func builtinMigra(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("migra() quer conexao e, opcional, a pasta; veio %d argumento(s)", len(args))
	}
	con, e := pegaConexao(args[0])
	if e != nil {
		return e
	}
	pasta := migracao.PastaPadrao
	if len(args) == 2 {
		t, ok := args[1].(*object.Texto)
		if !ok {
			return erroBuiltin("migra() espera a pasta como texto, veio %s", object.NomeTipo(args[1]))
		}
		pasta = t.Value
	}
	feitas, err := migracao.Novo(con.db, con.driver, pasta).Sobe()
	if err != nil {
		return erroBuiltin("migra(): %v", err)
	}
	out := make([]object.Object, len(feitas))
	for i, m := range feitas {
		out[i] = object.NumInt(m.Versao)
	}
	return object.NovaLista(out)
}

// argParaGo converte argumentos variaveis da chamada. Aceita dois formatos:
//  1. lista unica专场 params = [valor1, valor2]  ->  v1, v2
//  2. argumentos soltos 后面 params  ->  v1, v2
func argParaGo(args []object.Object) ([]interface{}, *object.Erro) {
	if len(args) == 0 {
		return nil, nil
	}
	if len(args) == 1 {
		if lst, ok := args[0].(*object.Lista); ok {
			elems := lst.Visao()
			out := make([]interface{}, 0, len(elems))
			for _, a := range elems {
				out = append(out, objParaGo(a))
			}
			return out, nil
		}
	}
	out := make([]interface{}, 0, len(args))
	for _, a := range args {
		out = append(out, objParaGo(a))
	}
	return out, nil
}

func objParaGo(o object.Object) interface{} {
	switch v := o.(type) {
	case *object.Numero:
		if v.EhInt {
			return v.Int
		}
		return v.Value
	case *object.Texto:
		return v.Value
	case *object.Booleano:
		return v.Value
	case *object.Nada:
		return nil
	}
	return o.Inspect()
}

func goParaObj(v interface{}) object.Object {
	if v == nil {
		return NADA
	}
	switch x := v.(type) {
	case int64:
		return object.NumInt(x)
	case int:
		return object.NumInt(int64(x))
	case float64:
		return &object.Numero{Value: x}
	case bool:
		return boolDoNativo(x)
	case string:
		return &object.Texto{Value: x}
	case []byte:
		return &object.Texto{Value: string(x)}
	case time.Time:
		return &object.Texto{Value: x.Format(time.RFC3339)}
	}
	return &object.Texto{Value: fmt.Sprintf("%v", v)}
}
