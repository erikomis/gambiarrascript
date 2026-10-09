//go:build !js

package interpreter

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gambiarrascript/object"
)

// pastaEstatica e um serve_pasta(prefixo, pasta). raiz ja vem absoluta e com
// os symlinks resolvidos, pra checar que nenhum arquivo servido escapa dela.
type pastaEstatica struct {
	prefixo string
	raiz    string
}

// builtinServePasta: serve_pasta("/publico", "./public") serve os arquivos da
// pasta debaixo do prefixo (GET/HEAD). Rota registrada com rota() ganha.
func (s *servidorEstado) builtinServePasta(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("serve_pasta() quer 2 argumentos (prefixo, pasta), veio %d", len(args))
	}
	prefixo, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("serve_pasta(): o prefixo tem que ser texto (tipo \"/publico\"), veio %s", args[0].Type())
	}
	pasta, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("serve_pasta(): a pasta tem que ser texto, veio %s", args[1].Type())
	}
	p := "/" + strings.Trim(prefixo.Value, "/")
	abs, err := filepath.Abs(pasta.Value)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		return erroBuiltinKind(KindIO, "serve_pasta(): a pasta %q nao existe, parca", pasta.Value)
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return erroBuiltinKind(KindIO, "serve_pasta(): %q nao e uma pasta", pasta.Value)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for idx, pe := range s.pastas {
		if pe.prefixo == p {
			s.pastas[idx] = &pastaEstatica{prefixo: p, raiz: abs}
			return NADA
		}
	}
	s.pastas = append(s.pastas, &pastaEstatica{prefixo: p, raiz: abs})
	return NADA
}

// achaPasta devolve a pasta de prefixo mais comprido que cobre o caminho.
func (s *servidorEstado) achaPasta(caminho string) *pastaEstatica {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var melhor *pastaEstatica
	for _, p := range s.pastas {
		cobre := p.prefixo == "/" || caminho == p.prefixo || strings.HasPrefix(caminho, p.prefixo+"/")
		if cobre && (melhor == nil || len(p.prefixo) > len(melhor.prefixo)) {
			melhor = p
		}
	}
	return melhor
}

// dentro diz se o caminho (ja sem symlink) fica dentro da raiz.
func dentro(raiz, alvo string) bool {
	return alvo == raiz || strings.HasPrefix(alvo, raiz+string(filepath.Separator))
}

func (p *pastaEstatica) serve(w http.ResponseWriter, r *http.Request, comp *configCompressao) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		escreveTexto(w, http.StatusMethodNotAllowed, "pasta estatica so atende GET e HEAD, parca")
		return
	}
	naoAchei := func() { escreveTexto(w, http.StatusNotFound, "arquivo nao encontrado, parca") }

	rel := r.URL.Path
	if p.prefixo != "/" {
		rel = strings.TrimPrefix(rel, p.prefixo)
	}
	if rel == "" {
		// "/publico" → "/publico/": link relativo do index.html funciona
		redireciona(w, r, r.URL.Path+"/")
		return
	}
	limpo := path.Clean("/" + rel)
	// Clean ja some com o ".."; pedaco com ponto na frente (.env, .git) nao sai.
	for _, pedaco := range strings.Split(limpo, "/") {
		if strings.HasPrefix(pedaco, ".") {
			naoAchei()
			return
		}
	}
	alvo, err := filepath.EvalSymlinks(filepath.Join(p.raiz, filepath.FromSlash(limpo)))
	if err != nil || !dentro(p.raiz, alvo) {
		naoAchei()
		return
	}
	info, err := os.Stat(alvo)
	if err != nil {
		naoAchei()
		return
	}
	if info.IsDir() {
		if !strings.HasSuffix(r.URL.Path, "/") {
			redireciona(w, r, r.URL.Path+"/")
			return
		}
		// pasta sem index.html da 404: nada de listar o conteudo
		alvo, err = filepath.EvalSymlinks(filepath.Join(alvo, "index.html"))
		if err != nil || !dentro(p.raiz, alvo) {
			naoAchei()
			return
		}
		if info, err = os.Stat(alvo); err != nil || info.IsDir() {
			naoAchei()
			return
		}
	}
	f, err := os.Open(alvo)
	if err != nil {
		naoAchei()
		return
	}
	defer f.Close()
	if comp.serveGzipEstatico(w, r, f, info) {
		return
	}
	// ServeContent: Content-Type pela extensao (ou farejando), Range,
	// If-Modified-Since e HEAD sem corpo.
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func redireciona(w http.ResponseWriter, r *http.Request, destino string) {
	if r.URL.RawQuery != "" {
		destino += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, destino, http.StatusMovedPermanently)
}
