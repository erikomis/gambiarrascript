package interpreter

import (
	"encoding/csv"
	"os"
	"strconv"

	"gambiarrascript/object"
)

// Lib padrão — CSV.
//
//	le_csv(caminho)                       → lista de dicionarios (1 dict por linha, chaves = cabecalho)
//	escreve_csv(caminho, lista, [cabecalhos]) → nada (escreve arquivo CSV)
//
// le_csv devolve lista de dicts. A primeira linha do arquivo e usada como
// cabecalho (nomes das colunas). Cada linha subsequente vira um dicionario
// {coluna: valor}. Valores sempre texto (CSV nao tem tipo).
//
// escreve_csv recebe uma lista de dicionarios e escreve no arquivo. O cabecalho
// e derivado das chaves do primeiro dicionario (a menos que o 3o arg seja
// passado — uma lista de textos com a ordem/colunas desejadas).

// builtinLeCsv le um arquivo CSV e devolve uma lista de dicionarios.
func builtinLeCsv(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("le_csv() quer 1 arg (caminho), veio %d", len(args))
	}
	caminho, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("le_csv: caminho tem que ser texto, veio %s", args[0].Type())
	}
	f, err := os.Open(caminho.Value)
	if err != nil {
		return erroBuiltinKind(KindIO, "le_csv %q: %v", caminho.Value, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return erroBuiltinKind(KindIO, "le_csv %q: %v", caminho.Value, err)
	}
	if len(records) == 0 {
		return &object.Lista{}
	}
	cabecalho := records[0]
	linhas := make([]object.Object, 0, len(records)-1)
	for _, rec := range records[1:] {
		d := &object.Dicionario{Pares: map[object.HashKey]object.ParDic{}}
		for i, val := range rec {
			if i >= len(cabecalho) {
				break
			}
			chave := &object.Texto{Value: cabecalho[i]}
			d.Pares[chave.ChaveHash()] = object.ParDic{
				Chave: chave,
				Valor: &object.Texto{Value: val},
			}
		}
		linhas = append(linhas, d)
	}
	return &object.Lista{Elements: linhas}
}

// builtinEscreveCsv escreve uma lista de dicionarios num arquivo CSV. O
// cabecalho vem das chaves do primeiro dicionario (ordem nao deterministica)
// ou do 3o argumento — uma lista de textos com a ordem das colunas.
func builtinEscreveCsv(args []object.Object) object.Object {
	if len(args) != 2 && len(args) != 3 {
		return erroBuiltin("escreve_csv() quer 2-3 args (caminho, lista, [cabecalhos]), veio %d", len(args))
	}
	caminho, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("escreve_csv: 1o arg (caminho) tem que ser texto, veio %s", args[0].Type())
	}
	lista, ok := args[1].(*object.Lista)
	if !ok {
		return erroBuiltin("escreve_csv: 2o arg (lista) tem que ser lista, veio %s", args[1].Type())
	}

	var cabecalhos []string
	if len(args) == 3 {
		cab, ok := args[2].(*object.Lista)
		if !ok {
			return erroBuiltin("escreve_csv: 3o arg (cabecalhos) tem que ser lista de textos, veio %s", args[2].Type())
		}
		for _, c := range cab.Elements {
			t, ok := c.(*object.Texto)
			if !ok {
				return erroBuiltin("escreve_csv: cabecalhos tem que ser lista de textos, veio %s", c.Type())
			}
			cabecalhos = append(cabecalhos, t.Value)
		}
	}
	if cabecalhos == nil && len(lista.Elements) > 0 {
		if primeiro, ok := lista.Elements[0].(*object.Dicionario); ok {
			for _, par := range primeiro.Pares {
				if t, ok := par.Chave.(*object.Texto); ok {
					cabecalhos = append(cabecalhos, t.Value)
				}
			}
		}
	}

	f, err := os.Create(caminho.Value)
	if err != nil {
		return erroBuiltinKind(KindIO, "escreve_csv %q: %v", caminho.Value, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if cabecalhos != nil {
		w.Write(cabecalhos)
	}
	for _, elem := range lista.Elements {
		d, ok := elem.(*object.Dicionario)
		if !ok {
			w.Flush()
			return erroBuiltin("escreve_csv: cada item da lista tem que ser dicionario, veio %s", elem.Type())
		}
		linha := make([]string, len(cabecalhos))
		for i, col := range cabecalhos {
			chave := &object.Texto{Value: col}
			if par, existe := d.Pares[chave.ChaveHash()]; existe {
				linha[i] = par.Valor.Inspect()
			}
		}
		w.Write(linha)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return erroBuiltinKind(KindIO, "escreve_csv %q: %v", caminho.Value, err)
	}
	return NADA
}

// usado pra evitar import strconv sem uso se o build for incremental.
var _ = strconv.Itoa