//go:build !js

package interpreter

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"gambiarrascript/object"
)

// Formularios e upload: o corpo application/x-www-form-urlencoded vira
// pedido["campos"]; o multipart/form-data vira pedido["campos"] (os campos de
// texto) e pedido["arquivos"] (os arquivos). Campo repetido (checkbox, input
// multiple) vira lista; campo unico fica no valor puro.
//
//	pedido.arquivos.foto → {"nome": "gato.png", "tipo": "image/png",
//	                        "tamanho": 1234, "conteudo_base64": "iVBOR..."}
//
// O tamanho maximo do corpo vale pra tudo (json, texto, upload): passou do
// limite → 413 sem nem chamar a rota.

// maxCorpoPadrao e o limite de corpo quando o escuta nao fala nada: 10 MB.
const maxCorpoPadrao = 10 << 20

// tamMaxNomeArquivo corta nome de arquivo absurdo (o sistema de arquivos
// costuma recusar acima de 255 bytes).
const tamMaxNomeArquivo = 200

// tipoFormulario diz qual dos dois formularios o Content-Type e ("" = nenhum).
// Devolve tambem o boundary do multipart.
func tipoFormulario(ct string) (string, string) {
	if ct == "" {
		return "", ""
	}
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return "", ""
	}
	switch mt {
	case "application/x-www-form-urlencoded":
		return "urlencoded", ""
	case "multipart/form-data":
		return "multipart", params["boundary"]
	}
	return "", ""
}

// juntaCampo bota o valor no dicionario; se a chave ja existe vira lista.
func juntaCampo(d *object.Dicionario, chave string, valor object.Object) {
	atual, ja := dicPega(d, chave)
	if !ja {
		dicBota(d, chave, valor)
		return
	}
	if l, ok := atual.(*object.Lista); ok {
		l.Adiciona(valor)
		return
	}
	dicBota(d, chave, object.NovaLista([]object.Object{atual, valor}))
}

// leFormulario preenche campos/arquivos a partir do corpo ja lido. Erro =
// corpo quebrado (vira 400).
func leFormulario(ct string, corpo []byte, campos, arquivos *object.Dicionario) error {
	tipo, boundary := tipoFormulario(ct)
	switch tipo {
	case "urlencoded":
		vals, err := url.ParseQuery(string(corpo))
		if err != nil {
			return err
		}
		for _, nome := range chavesOrdenadas(vals) {
			for _, v := range vals[nome] {
				juntaCampo(campos, nome, &object.Texto{Value: v})
			}
		}
	case "multipart":
		if boundary == "" {
			return errors.New("multipart sem boundary")
		}
		mr := multipart.NewReader(bytes.NewReader(corpo), boundary)
		for {
			parte, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			dados, err := io.ReadAll(parte)
			if err != nil {
				return err
			}
			nome := parte.FormName()
			if nome == "" {
				continue
			}
			_, disp, _ := mime.ParseMediaType(parte.Header.Get("Content-Disposition"))
			nomeArq, ehArquivo := disp["filename"]
			if !ehArquivo {
				juntaCampo(campos, nome, &object.Texto{Value: string(dados)})
				continue
			}
			if nomeArq == "" && len(dados) == 0 {
				continue // <input type=file> sem arquivo escolhido
			}
			tipoArq := parte.Header.Get("Content-Type")
			if tipoArq == "" {
				tipoArq = "application/octet-stream"
			}
			arq := object.NovoDicionario()
			dicBota(arq, "nome", &object.Texto{Value: sanitizaNomeArquivo(nomeArq)})
			dicBota(arq, "tipo", &object.Texto{Value: tipoArq})
			dicBota(arq, "tamanho", object.NumInt(int64(len(dados))))
			dicBota(arq, "conteudo_base64", &object.Texto{Value: base64.StdEncoding.EncodeToString(dados)})
			juntaCampo(arquivos, nome, arq)
		}
	}
	return nil
}

// sanitizaNomeArquivo deixa so o nome (sem pasta nenhuma, nem a do Windows),
// sem caractere de controle nem os que o Windows recusa, sem ponto/espaco nas
// pontas (nada de ".htaccess" nem ".."), com tamanho limitado. Sobrou nada →
// "arquivo".
func sanitizaNomeArquivo(nome string) string {
	if i := strings.LastIndexAny(nome, `/\`); i >= 0 {
		nome = nome[i+1:]
	}
	var b strings.Builder
	for _, r := range nome {
		if r == utf8.RuneError || unicode.IsControl(r) || strings.ContainsRune(`<>:"|?*`, r) {
			continue
		}
		b.WriteRune(r)
	}
	nome = strings.Trim(b.String(), ". ")
	if len(nome) > tamMaxNomeArquivo {
		ext := filepath.Ext(nome)
		if len(ext) > 20 {
			ext = ""
		}
		corte := tamMaxNomeArquivo - len(ext)
		for corte > 0 && !utf8.RuneStart(nome[corte]) {
			corte--
		}
		nome = nome[:corte] + ext
	}
	if nome == "" {
		return "arquivo"
	}
	return nome
}

// builtinSalvaArquivo: salva_arquivo(arquivo, pasta, [nome]) grava o upload
// DENTRO da pasta (cria se faltar) e devolve o caminho final. O nome vem do
// proprio arquivo (ja sanitizado) ou do 3o argumento, que tem que ser so um
// nome — com pasta no meio ou ".." e erro, pra ninguem escrever fora da pasta.
// Nunca sobrescreve: se o nome ja existe vira "foto-1.png", "foto-2.png"...
func builtinSalvaArquivo(args []object.Object) object.Object {
	if len(args) < 2 || len(args) > 3 {
		return erroBuiltin("salva_arquivo() quer 2 ou 3 argumentos (arquivo, pasta, [nome]), veio %d", len(args))
	}
	arq, ok := args[0].(*object.Dicionario)
	if !ok {
		return erroBuiltin("salva_arquivo(): o 1o argumento tem que ser um arquivo do pedido.arquivos, veio %s", object.NomeTipo(args[0]))
	}
	b64, ok := dicPega(arq, "conteudo_base64")
	t64, ehTexto := b64.(*object.Texto)
	if !ok || !ehTexto {
		return erroBuiltin("salva_arquivo(): esse dicionario nao tem \"conteudo_base64\" — passa um item do pedido.arquivos")
	}
	dados, err := base64.StdEncoding.DecodeString(t64.Value)
	if err != nil {
		return erroBuiltin("salva_arquivo(): o conteudo_base64 ta quebrado: %v", err)
	}
	pasta, ok := args[1].(*object.Texto)
	if !ok || strings.TrimSpace(pasta.Value) == "" {
		return erroBuiltin("salva_arquivo(): a pasta tem que ser texto nao vazio, veio %s", args[1].Inspect())
	}
	var nome string
	if len(args) == 3 {
		n, ok := args[2].(*object.Texto)
		if !ok {
			return erroBuiltin("salva_arquivo(): o nome tem que ser texto, veio %s", object.NomeTipo(args[2]))
		}
		if n.Value == "" || n.Value == "." || n.Value == ".." || strings.ContainsAny(n.Value, `/\`) || strings.ContainsRune(n.Value, 0) {
			return erroBuiltin("salva_arquivo(): o nome %q tem que ser so um nome de arquivo, sem pasta nem \"..\" (nada de sair da pasta, parca)", n.Value)
		}
		nome = n.Value
	} else {
		nomeObj, _ := dicPega(arq, "nome")
		texto, _ := nomeObj.(*object.Texto)
		if texto == nil {
			texto = &object.Texto{}
		}
		// sanitiza de novo: o dicionario pode ter sido montado na mao
		nome = sanitizaNomeArquivo(texto.Value)
	}
	if err := os.MkdirAll(pasta.Value, 0o755); err != nil {
		return erroBuiltinKind(KindIO, "salva_arquivo(): nao consegui criar a pasta %q: %v", pasta.Value, err)
	}
	ext := filepath.Ext(nome)
	base := strings.TrimSuffix(nome, ext)
	for n := 0; n < 1000; n++ {
		final := nome
		if n > 0 {
			final = base + "-" + strconv.Itoa(n) + ext
		}
		caminho := filepath.Join(pasta.Value, final)
		f, err := os.OpenFile(caminho, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return erroBuiltinKind(KindIO, "salva_arquivo(): nao consegui criar %q: %v", caminho, err)
		}
		_, errEsc := f.Write(dados)
		errFecha := f.Close()
		if errEsc == nil {
			errEsc = errFecha
		}
		if errEsc != nil {
			os.Remove(caminho)
			return erroBuiltinKind(KindIO, "salva_arquivo(): nao consegui gravar %q: %v", caminho, errEsc)
		}
		return &object.Texto{Value: caminho}
	}
	return erroBuiltinKind(KindIO, "salva_arquivo(): ja tem arquivo demais chamado %q nessa pasta", nome)
}
