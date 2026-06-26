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
	driver, dsn, err := urlParaDriver(urlObj.Value)
	if err != nil {
		return erroBuiltin("%v", err)
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return erroBuiltin("nao consegui abrir o banco: %v", err)
	}
	if driver == "sqlite" {
		db.SetMaxOpenConns(1)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return erroBuiltin("nao consegui conectar no banco: %v", err)
	}
	return &object.Nativo{Rotulo: "conexao " + driver, Valor: &conexaoBD{db: db, driver: driver}}
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
