package compiler

import "gambiarrascript/ast"

// NomesLocais devolve os nomes que viram locais numa funcao (tudo que o corpo
// bota, fora os params) — a mesma varredura que o compilador usa. O linter do
// LSP usa pra achar leitura de local antes do primeiro `bota`.
func NomesLocais(params []*ast.Parametro, corpo *ast.BlockStatement) []string {
	return varreFuncao(params, corpo).declaradas
}
