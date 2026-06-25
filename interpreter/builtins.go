package interpreter

import (
	"strconv"
	"strings"

	"gambiarrascript/object"
)

var builtins = map[string]*object.Builtin{
	"tamanho":   {Nome: "tamanho", Fn: builtinTamanho},
	"chaves":    {Nome: "chaves", Fn: builtinChaves},
	"tem":       {Nome: "tem", Fn: builtinTem},
	"texto":     {Nome: "texto", Fn: builtinTexto},
	"numero":    {Nome: "numero", Fn: builtinNumero},
	"busca":     {Nome: "busca", Fn: builtinBusca},
	"de_json":   {Nome: "de_json", Fn: builtinDeJson},
	"pra_json":  {Nome: "pra_json", Fn: builtinPraJson},

	// texto
	"separa":      {Nome: "separa", Fn: builtinSepara},
	"junta":       {Nome: "junta", Fn: builtinJunta},
	"maiusculo":   {Nome: "maiusculo", Fn: builtinMaiusculo},
	"minusculo":   {Nome: "minusculo", Fn: builtinMinusculo},
	"substitui":   {Nome: "substitui", Fn: builtinSubstitui},
	"fatia":       {Nome: "fatia", Fn: builtinFatia},
	"contem":      {Nome: "contem", Fn: builtinContem},
	"comeca_com":  {Nome: "comeca_com", Fn: builtinComecaCom},
	"termina_com": {Nome: "termina_com", Fn: builtinTerminaCom},
	"tira_espaco": {Nome: "tira_espaco", Fn: builtinTiraEspaco},

	// lista
	"adiciona": {Nome: "adiciona", Fn: builtinAdiciona},
	"remove":    {Nome: "remove", Fn: builtinRemove},
	"ordena":    {Nome: "ordena", Fn: builtinOrdena},
	"inverte":   {Nome: "inverte", Fn: builtinInverte},

	// matematica
	"raiz":      {Nome: "raiz", Fn: builtinRaiz},
	"aleatorio": {Nome: "aleatorio", Fn: builtinAleatorio},
	"arredonda": {Nome: "arredonda", Fn: builtinArredonda},
	"teto":      {Nome: "teto", Fn: builtinTeto},
	"chao":      {Nome: "chao", Fn: builtinChao},
	"abs":       {Nome: "abs", Fn: builtinAbs},
	"min":       {Nome: "min", Fn: builtinMin},
	"max":       {Nome: "max", Fn: builtinMax},

	// arquivo
	"le_arquivo":     {Nome: "le_arquivo", Fn: builtinLeArquivo},
	"escreve_arquivo": {Nome: "escreve_arquivo", Fn: builtinEscreveArquivo},
	"anexa_arquivo":   {Nome: "anexa_arquivo", Fn: builtinAnexaArquivo},

	// banco de dados
	"conecta": {Nome: "conecta", Fn: builtinConecta},
	"fecha":   {Nome: "fecha", Fn: builtinFecha},

	// erros
	"quebra":       {Nome: "quebra", Fn: builtinQuebra},
	"erro_msg":     {Nome: "erro_msg", Fn: builtinErroMsg},
	"erro_linha":   {Nome: "erro_linha", Fn: builtinErroLinha},
	"erro_tipo":    {Nome: "erro_tipo", Fn: builtinErroTipo},
	"erro_pilha":   {Nome: "erro_pilha", Fn: builtinErroPilha},
	"erro_causa":   {Nome: "erro_causa", Fn: builtinErroCausa},
	"envolve_erro": {Nome: "envolve_erro", Fn: builtinEnvolveErro},
}

func builtinTamanho(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("tamanho() quer 1 argumento, veio %d", len(args))
	}
	switch arg := args[0].(type) {
	case *object.Lista:
		return object.NumInt(int64(len(arg.Elements)))
	case *object.Dicionario:
		return object.NumInt(int64(len(arg.Pares)))
	case *object.Texto:
		return object.NumInt(int64(len([]rune(arg.Value))))
	default:
		return erroBuiltin("tamanho() nao funciona com %s", args[0].Type())
	}
}

func builtinChaves(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("chaves() quer 1 argumento, veio %d", len(args))
	}
	d, ok := args[0].(*object.Dicionario)
	if !ok {
		return erroBuiltin("chaves() so funciona com dicionario, veio %s", args[0].Type())
	}
	elems := make([]object.Object, 0, len(d.Pares))
	for _, par := range d.Pares {
		elems = append(elems, par.Chave)
	}
	return &object.Lista{Elements: elems}
}

func builtinTem(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("tem() quer 2 argumentos (dicionario, chave), veio %d", len(args))
	}
	d, ok := args[0].(*object.Dicionario)
	if !ok {
		return erroBuiltin("tem() espera um dicionario no primeiro argumento, veio %s", args[0].Type())
	}
	chave, ok := args[1].(object.Chaveavel)
	if !ok {
		return erroBuiltin("tem() nao consegue usar %s como chave", args[1].Type())
	}
	_, existe := d.Pares[chave.ChaveHash()]
	return boolDoNativo(existe)
}

func builtinTexto(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("texto() quer 1 argumento, veio %d", len(args))
	}
	return &object.Texto{Value: args[0].Inspect()}
}

func builtinNumero(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("numero() quer 1 argumento, veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("numero() so converte texto, veio %s", args[0].Type())
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(t.Value), 64)
	if err != nil {
		return erroBuiltin("isso ai nao e numero, parca: %q", t.Value)
	}
	return &object.Numero{Value: v}
}
