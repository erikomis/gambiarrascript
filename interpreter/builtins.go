package interpreter

import (
	"strconv"
	"strings"

	"gambiarrascript/object"
)

var builtins = map[string]*object.Builtin{
	"tamanho":  {Nome: "tamanho", Fn: builtinTamanho},
	"chaves":   {Nome: "chaves", Fn: builtinChaves},
	"tem":      {Nome: "tem", Fn: builtinTem},
	"texto":    {Nome: "texto", Fn: builtinTexto},
	"numero":   {Nome: "numero", Fn: builtinNumero},
	"busca":    {Nome: "busca", Fn: builtinBusca},
	"de_json":  {Nome: "de_json", Fn: builtinDeJson},
	"pra_json": {Nome: "pra_json", Fn: builtinPraJson},

	// texto
	"formata":     {Nome: "formata", Fn: builtinFormata},
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
	"remove":   {Nome: "remove", Fn: builtinRemove},
	"ordena":   {Nome: "ordena", Fn: builtinOrdena},
	"inverte":  {Nome: "inverte", Fn: builtinInverte},
	// (reduz/acha/acha_indice viraram metodos do Interpreter — precisam do
	// applyFunction pra aceitar gambiarra do usuario; registrados no New.)
	"unicos":   {Nome: "unicos", Fn: builtinUnicos},
	"achatada": {Nome: "achatada", Fn: builtinAchatada},

	// estatistica / munging de dados
	"soma":    {Nome: "soma", Fn: builtinSoma},
	"media":   {Nome: "media", Fn: builtinMedia},
	"zip":     {Nome: "zip", Fn: builtinZip},
	"enumera": {Nome: "enumera", Fn: builtinEnumera},

	// conjunto (Set)
	"conjunto":          {Nome: "conjunto", Fn: builtinConjunto},
	"contem_conjunto":   {Nome: "contem_conjunto", Fn: builtinContemConjunto},
	"adiciona_conjunto": {Nome: "adiciona_conjunto", Fn: builtinAdicionaConjunto},
	"remove_conjunto":   {Nome: "remove_conjunto", Fn: builtinRemoveConjunto},
	"uniao":             {Nome: "uniao", Fn: builtinUniao},
	"intersecao":        {Nome: "intersecao", Fn: builtinIntersecao},
	"diferenca":         {Nome: "diferenca", Fn: builtinDiferenca},

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
	"le_arquivo":      {Nome: "le_arquivo", Fn: builtinLeArquivo},
	"escreve_arquivo": {Nome: "escreve_arquivo", Fn: builtinEscreveArquivo},
	"anexa_arquivo":   {Nome: "anexa_arquivo", Fn: builtinAnexaArquivo},
	// fs (sistema de arquivos)
	"existe":        {Nome: "existe", Fn: builtinExiste},
	"eh_dir":        {Nome: "eh_dir", Fn: builtinEhDir},
	"deleta":        {Nome: "deleta", Fn: builtinDeleta},
	"cria_dir":      {Nome: "cria_dir", Fn: builtinCriaDir},
	"le_dir":        {Nome: "le_dir", Fn: builtinLeDir},
	"caminho_junta": {Nome: "caminho_junta", Fn: builtinCaminhoJunta},
	"caminho_base":  {Nome: "caminho_base", Fn: builtinCaminhoBase},
	"caminho_dir":   {Nome: "caminho_dir", Fn: builtinCaminhoDir},
	"caminho_ext":   {Nome: "caminho_ext", Fn: builtinCaminhoExt},
	"caminho_abs":   {Nome: "caminho_abs", Fn: builtinCaminhoAbs},

	// banco de dados
	"conecta":  {Nome: "conecta", Fn: builtinConecta},
	"fecha":    {Nome: "fecha", Fn: builtinFecha},
	"consulta": {Nome: "consulta", Fn: builtinConsulta},
	"executa":  {Nome: "executa", Fn: builtinExecuta},

	// erros
	"quebra":       {Nome: "quebra", Fn: builtinQuebra},
	"erro_msg":     {Nome: "erro_msg", Fn: builtinErroMsg},
	"erro_linha":   {Nome: "erro_linha", Fn: builtinErroLinha},
	"erro_tipo":    {Nome: "erro_tipo", Fn: builtinErroTipo},
	"erro_pilha":   {Nome: "erro_pilha", Fn: builtinErroPilha},
	"erro_causa":   {Nome: "erro_causa", Fn: builtinErroCausa},
	"envolve_erro": {Nome: "envolve_erro", Fn: builtinEnvolveErro},

	// regex
	"busca_regex":     {Nome: "busca_regex", Fn: builtinBuscaRegex},
	"acha_regex":      {Nome: "acha_regex", Fn: builtinAchaRegex},
	"combina_regex":   {Nome: "combina_regex", Fn: builtinCombinaRegex},
	"substitui_regex": {Nome: "substitui_regex", Fn: builtinSubstituiRegex},
	"separa_regex":    {Nome: "separa_regex", Fn: builtinSeparaRegex},

	// tempo/datetime
	"agora":         {Nome: "agora", Fn: builtinAgora},
	"agora_num":     {Nome: "agora_num", Fn: builtinAgoraNum},
	"agora_ns":      {Nome: "agora_ns", Fn: builtinAgoraNs},
	"formata_tempo": {Nome: "formata_tempo", Fn: builtinFormataTempo},
	"parse_tempo":   {Nome: "parse_tempo", Fn: builtinParseTempo},
	"duracao":       {Nome: "duracao", Fn: builtinDuracao},
	"espera_ms":     {Nome: "espera_ms", Fn: builtinEsperaMs},

	// crypto / codificacao
	"md5":               {Nome: "md5", Fn: builtinMd5},
	"sha1":              {Nome: "sha1", Fn: builtinSha1},
	"sha256":            {Nome: "sha256", Fn: builtinSha256},
	"sha512":            {Nome: "sha512", Fn: builtinSha512},
	"hmac_sha256":       {Nome: "hmac_sha256", Fn: builtinHmacSha256},
	"base64_codifica":   {Nome: "base64_codifica", Fn: builtinBase64Codifica},
	"base64_decodifica": {Nome: "base64_decodifica", Fn: builtinBase64Decodifica},
	"base32_codifica":   {Nome: "base32_codifica", Fn: builtinBase32Codifica},
	"base32_decodifica": {Nome: "base32_decodifica", Fn: builtinBase32Decodifica},
	"hex_codifica":      {Nome: "hex_codifica", Fn: builtinHexCodifica},
	"hex_decodifica":    {Nome: "hex_decodifica", Fn: builtinHexDecodifica},
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
