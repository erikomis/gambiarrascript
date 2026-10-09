//go:build !js

package interpreter

import (
	"bytes"
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gambiarrascript/object"
)

// Compressao: comprime([{"minimo": 1024, "nivel": 6}]) manda as respostas das
// rotas em gzip quando o cliente aceita (Accept-Encoding: gzip), o corpo tem
// pelo menos "minimo" bytes e o tipo e texto (html, css, js, json, xml,
// svg...). Imagem/zip/video ja vem comprimido: nao mexe. Resposta que ja tem
// Content-Encoding tambem nao (nada de comprimir duas vezes). Toda resposta
// que PODERIA ir comprimida ganha Vary: Accept-Encoding, pro cache nao
// entregar gzip pra quem nao pediu.

const minimoCompressaoPadrao = 1024

type configCompressao struct {
	minimo int
	nivel  int
}

func (s *servidorEstado) builtinComprime(args []object.Object) object.Object {
	if len(args) > 1 {
		return erroBuiltin("comprime() quer 0 ou 1 argumento (opcoes), veio %d", len(args))
	}
	c := &configCompressao{minimo: minimoCompressaoPadrao, nivel: gzip.DefaultCompression}
	if len(args) == 1 {
		d, ok := args[0].(*object.Dicionario)
		if !ok {
			return erroBuiltin("comprime(): as opcoes tem que ser dicionario, tipo {\"minimo\": 1024}, veio %s", object.NomeTipo(args[0]))
		}
		for _, par := range d.Pares() {
			k, _ := par.Chave.(*object.Texto)
			n, _ := par.Valor.(*object.Numero)
			switch {
			case k != nil && k.Value == "minimo":
				if n == nil || !ehInteiro(n) || n.Value < 0 {
					return erroBuiltin("comprime(): \"minimo\" tem que ser numero inteiro de bytes (0 ou mais), veio %s", par.Valor.Inspect())
				}
				c.minimo = int(n.Value)
			case k != nil && k.Value == "nivel":
				if n == nil || !ehInteiro(n) || n.Value < 1 || n.Value > 9 {
					return erroBuiltin("comprime(): \"nivel\" vai de 1 (rapido) a 9 (menor), veio %s", par.Valor.Inspect())
				}
				c.nivel = int(n.Value)
			default:
				return erroBuiltin("comprime(): opcao %s nao existe (as que existem: minimo, nivel)", chaveComAspas(par.Chave))
			}
		}
	}
	s.mu.Lock()
	s.compressao = c
	s.mu.Unlock()
	return NADA
}

// tipoComprimivel: texto de verdade. O resto (png, zip, mp4...) ja vem
// comprimido e gzip so gasta CPU.
func tipoComprimivel(ct string) bool {
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	switch {
	case strings.HasPrefix(mt, "text/"),
		strings.HasSuffix(mt, "+json"), strings.HasSuffix(mt, "+xml"):
		return true
	}
	switch mt {
	case "application/json", "application/javascript", "application/x-javascript",
		"application/xml", "image/svg+xml", "application/wasm", "application/x-ndjson":
		return true
	}
	return false
}

// aceitaGzip le o Accept-Encoding (gzip ou *, com q diferente de 0).
func aceitaGzip(r *http.Request) bool {
	for _, item := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		nome, params, _ := strings.Cut(strings.TrimSpace(item), ";")
		nome = strings.ToLower(strings.TrimSpace(nome))
		if nome != "gzip" && nome != "*" {
			continue
		}
		q := 1.0
		if k, v, ok := strings.Cut(strings.TrimSpace(params), "="); ok && strings.TrimSpace(k) == "q" {
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				q = f
			}
		}
		return q > 0
	}
	return false
}

// comprimeResposta troca o corpo pelo gzip dele quando vale a pena.
func (c *configCompressao) comprimeResposta(r *http.Request, resp *respostaHTTP) {
	if c == nil || resp.cabecalhos.Get("Content-Encoding") != "" || len(resp.corpo) < c.minimo ||
		resp.status == http.StatusNoContent || resp.status == http.StatusNotModified || resp.status < 200 {
		return
	}
	ct := resp.cabecalhos.Get("Content-Type")
	if ct == "" {
		// sem tipo o net/http adivinharia pelo corpo — e depois do gzip ia
		// adivinhar errado. Decide agora, com o corpo cru.
		ct = http.DetectContentType(resp.corpo)
		resp.cabecalhos.Set("Content-Type", ct)
	}
	if !tipoComprimivel(ct) {
		return
	}
	resp.cabecalhos.Add("Vary", "Accept-Encoding")
	if !aceitaGzip(r) {
		return
	}
	z, err := c.gzipa(resp.corpo)
	if err != nil {
		return
	}
	resp.corpo = z
	resp.cabecalhos.Set("Content-Encoding", "gzip")
	resp.cabecalhos.Del("Content-Length")
	// o ETag do corpo cru nao vale pro comprimido
	if et := resp.cabecalhos.Get("ETag"); et != "" && !strings.HasPrefix(et, "W/") {
		resp.cabecalhos.Set("ETag", "W/"+et)
	}
}

func (c *configCompressao) gzipa(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, c.nivel)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(b); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// maxGzipEstatico: arquivo estatico maior que isso vai cru (comprimir na
// hora um arquivo gigante a cada pedido nao compensa).
const maxGzipEstatico = 10 << 20

// serveGzipEstatico manda o arquivo do serve_pasta em gzip quando da (sem
// Range: faixa de bytes de arquivo comprimido na hora nao faz sentido).
// Devolve true se ja respondeu.
func (c *configCompressao) serveGzipEstatico(w http.ResponseWriter, r *http.Request, f *os.File, info os.FileInfo) bool {
	if c == nil || r.Header.Get("Range") != "" || info.Size() < int64(c.minimo) || info.Size() > maxGzipEstatico {
		return false
	}
	ct := mime.TypeByExtension(filepath.Ext(info.Name()))
	if ct == "" || !tipoComprimivel(ct) {
		return false
	}
	w.Header().Add("Vary", "Accept-Encoding")
	if !aceitaGzip(r) {
		return false
	}
	cru, err := io.ReadAll(f)
	if err == nil {
		var z []byte
		if z, err = c.gzipa(cru); err == nil {
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Content-Encoding", "gzip")
			http.ServeContent(w, r, info.Name(), info.ModTime(), bytes.NewReader(z))
			return true
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		escreveTexto(w, http.StatusInternalServerError, corpoErroInterno)
		return true
	}
	return false
}
