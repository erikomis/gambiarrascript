package depurador

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"gambiarrascript/object"
)

// maxValor corta a representacao de valores gigantes (lista de mil itens).
const maxValor = 400

// Mostra devolve o valor como o depurador exibe: texto entre aspas (pra
// diferenciar "1" de 1), o resto igual o `mostra`.
func Mostra(v object.Object) string {
	if v == nil {
		return "nada"
	}
	var s string
	switch x := v.(type) {
	case *object.Texto:
		s = strconv.Quote(x.Value)
	case *object.Funcao:
		nomes := make([]string, len(x.Parametros))
		for k, p := range x.Parametros {
			nomes[k] = p.Nome.Value
			if p.Variadico {
				nomes[k] = "..." + nomes[k]
			}
		}
		s = mostraGambiarra(x.Nome, nomes)
	case *object.CompiledFunction:
		var nomes []string
		if info := infoDe(x); info != nil && len(info.Locais) >= x.NumArgs {
			nomes = append(nomes, info.Locais[:x.NumArgs]...)
			if x.Variadic && len(nomes) > 0 {
				nomes[len(nomes)-1] = "..." + nomes[len(nomes)-1]
			}
		}
		s = mostraGambiarra(x.Name, nomes)
	default:
		s = v.Inspect()
	}
	if len(s) > maxValor {
		s = s[:maxValor] + "..."
	}
	return s
}

// infosVM acha os nomes de depuracao de uma gambiarra da VM pelo bytecode:
// a closure criada pelo OpClosure nao copia o Depura (o caminho quente da VM
// fica intacto), mas divide o mesmo slice de bytecode da constante.
var infosVM sync.Map // *byte -> *object.InfoDepuracao

func registraInfos(consts []object.Object) {
	for _, k := range consts {
		var cf *object.CompiledFunction
		switch c := k.(type) {
		case *object.CompiledFunction:
			cf = c
		case *object.Modulo:
			cf = c.Corpo
		}
		if cf != nil && cf.Depura != nil && len(cf.Bytecode) > 0 {
			infosVM.Store(&cf.Bytecode[0], cf.Depura)
		}
	}
}

func infoDe(cf *object.CompiledFunction) *object.InfoDepuracao {
	if cf.Depura != nil {
		return cf.Depura
	}
	if len(cf.Bytecode) == 0 {
		return nil
	}
	if v, ok := infosVM.Load(&cf.Bytecode[0]); ok {
		return v.(*object.InfoDepuracao)
	}
	return nil
}

// mostraGambiarra da o mesmo texto pras gambiarras dos dois engines (o
// Inspect delas difere: o depurador mostra nome e parametros).
func mostraGambiarra(nome string, params []string) string {
	if nome == "" {
		nome = "<anonima>"
	}
	return "gambiarra " + nome + "(" + strings.Join(params, ", ") + ")"
}

// Tipo devolve o nome do tipo como a linguagem chama (tipo()).
func Tipo(v object.Object) string {
	if i, ok := v.(*object.Instancia); ok {
		return i.Tipo.Nome
	}
	return object.NomeTipo(v)
}

// Filhos lista o conteudo de um valor composto (lista, dicionario, conjunto
// e instancia de treta) — e o que o VSCode mostra ao expandir. Valor simples
// nao tem filhos.
func Filhos(v object.Object) []object.Variavel {
	switch x := v.(type) {
	case *object.Lista:
		elems := x.Visao()
		out := make([]object.Variavel, len(elems))
		for i, e := range elems {
			out[i] = object.Variavel{Nome: fmt.Sprintf("[%d]", i), Valor: e}
		}
		return out
	case *object.Dicionario:
		pares := x.Pares()
		out := make([]object.Variavel, len(pares))
		for i, p := range pares {
			out[i] = object.Variavel{Nome: Mostra(p.Chave), Valor: p.Valor}
		}
		return out
	case *object.Conjunto:
		vals := x.Valores()
		out := make([]object.Variavel, len(vals))
		for i, e := range vals {
			out[i] = object.Variavel{Nome: fmt.Sprintf("[%d]", i), Valor: e}
		}
		return out
	case *object.Instancia:
		campos := x.Campos()
		out := make([]object.Variavel, len(campos))
		for i, c := range campos {
			out[i] = object.Variavel{Nome: x.Tipo.Campos[i].Nome, Valor: c}
		}
		return out
	}
	return nil
}

// TemFilhos diz se o valor e composto e nao esta vazio.
func TemFilhos(v object.Object) bool {
	switch x := v.(type) {
	case *object.Lista:
		return x.Tamanho() > 0
	case *object.Dicionario:
		return x.Tamanho() > 0
	case *object.Conjunto:
		return len(x.Valores()) > 0
	case *object.Instancia:
		return len(x.Tipo.Campos) > 0
	}
	return false
}
