package lsp

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"gambiarrascript/object"
	"gambiarrascript/token"
)

// ---- recursos do editor alem de completion/hover ----
//
// definicao, referencias, renomear, formatar, signature help e simbolos do
// documento. Tudo fica nestes arquivos novos (navegacao.go, formatacao.go,
// assinatura.go, simbolos_doc.go) pra server.go so precisar despachar.

// Local e um Location do LSP.
type Local struct {
	URI   string `json:"uri"`
	Range Faixa  `json:"range"`
}

// EdicaoTexto e um TextEdit do LSP.
type EdicaoTexto struct {
	Range   Faixa  `json:"range"`
	NewText string `json:"newText"`
}

// codigos de erro do JSON-RPC/LSP usados aqui
const (
	erroMetodoNaoExiste  = -32601
	erroParamsInvalidos  = -32602
	erroRequisicaoFalhou = -32803
)

// capacidadesExtras sao anunciadas no initialize junto com as antigas.
func capacidadesExtras(caps map[string]interface{}) {
	caps["definitionProvider"] = true
	caps["referencesProvider"] = true
	caps["renameProvider"] = map[string]interface{}{"prepareProvider": true}
	caps["documentFormattingProvider"] = true
	caps["documentSymbolProvider"] = true
	caps["signatureHelpProvider"] = map[string]interface{}{
		"triggerCharacters":   []string{"(", ","},
		"retriggerCharacters": []string{","},
	}
}

// guardaRaiz anota a pasta do workspace (rootUri/rootPath/workspaceFolders),
// usada pra achar quem importa um arquivo em referencias/renomear.
func (s *Servidor) guardaRaiz(params json.RawMessage) {
	var p struct {
		RootURI          string `json:"rootUri"`
		RootPath         string `json:"rootPath"`
		WorkspaceFolders []struct {
			URI string `json:"uri"`
		} `json:"workspaceFolders"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	switch {
	case p.RootURI != "":
		s.raiz = uriParaCaminho(p.RootURI)
	case p.RootPath != "":
		s.raiz = filepath.Clean(p.RootPath)
	case len(p.WorkspaceFolders) > 0:
		s.raiz = uriParaCaminho(p.WorkspaceFolders[0].URI)
	}
}

type paramsDoc struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position Posicao `json:"position"`
	Context  struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
	NewName string `json:"newName"`
}

// tratarRecursos despacha os metodos novos (chamado pelo default do tratar).
func (s *Servidor) tratarRecursos(msg *Mensagem) {
	var p paramsDoc
	if len(msg.Params) > 0 {
		json.Unmarshal(msg.Params, &p)
	}
	uri := p.TextDocument.URI
	switch msg.Method {
	case "textDocument/definition":
		if loc := s.definicao(uri, p.Position); loc != nil {
			s.responder(msg.ID, loc)
		} else {
			s.responder(msg.ID, nil)
		}
	case "textDocument/references":
		s.responder(msg.ID, s.referencias(uri, p.Position, p.Context.IncludeDeclaration))
	case "textDocument/prepareRename":
		faixa, nome, err := s.prepararRenomear(uri, p.Position)
		if err != "" {
			s.responderErro(msg.ID, erroRequisicaoFalhou, err)
			return
		}
		s.responder(msg.ID, map[string]interface{}{"range": faixa, "placeholder": nome})
	case "textDocument/rename":
		edicao, err := s.renomear(uri, p.Position, p.NewName)
		if err != "" {
			s.responderErro(msg.ID, erroRequisicaoFalhou, err)
			return
		}
		s.responder(msg.ID, edicao)
	case "textDocument/formatting":
		s.responder(msg.ID, s.formatar(uri))
	case "textDocument/signatureHelp":
		if sh := s.ajudaAssinatura(uri, p.Position); sh != nil {
			s.responder(msg.ID, sh)
		} else {
			s.responder(msg.ID, nil)
		}
	case "textDocument/documentSymbol":
		s.responder(msg.ID, s.simbolosDoDocumento(uri))
	default:
		// request desconhecido precisa de resposta (senao o cliente fica
		// esperando); notification desconhecida e ignorada.
		if msg.ID != nil {
			s.responderErro(msg.ID, erroMetodoNaoExiste, "metodo nao suportado: "+msg.Method)
		}
	}
}

func (s *Servidor) responderErro(id *json.RawMessage, codigo int, texto string) {
	EscreverMensagem(s.out, &Mensagem{ID: id, Error: &RespErro{Code: codigo, Message: texto}})
}

// ---- documentos: abertos no editor ou do disco ----

// carregar devolve o texto do arquivo: o do editor se estiver aberto (pode
// ter mudanca nao salva), senao o do disco.
func (s *Servidor) carregar(caminho string) (string, bool) {
	for uri, texto := range s.docs {
		if uriParaCaminho(uri) == caminho {
			return texto, true
		}
	}
	b, err := os.ReadFile(caminho)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// uriDe devolve a URI que o editor usa pro arquivo (se aberto), senao monta.
func (s *Servidor) uriDe(caminho string) string {
	for uri := range s.docs {
		if uriParaCaminho(uri) == caminho {
			return uri
		}
	}
	return caminhoParaURI(caminho)
}

// analisarDoc analisa o documento aberto `uri` num contexto novo.
func (s *Servidor) analisarDoc(uri string) (*contextoAnalise, *analise) {
	ctx := novoContexto(s.carregar)
	a := ctx.analisar(s.docs[uri], uriParaCaminho(uri))
	a.uri = uri
	return ctx, a
}

// ---- definicao ----

func (s *Servidor) definicao(uri string, pos Posicao) *Local {
	_, a := s.analisarDoc(uri)
	o := a.ocorrenciaEm(pos)
	if o == nil {
		// cursor no caminho do importa: abre o arquivo importado
		if imp := a.importacaoEm(pos); imp != nil && imp.resolvido != "" {
			if _, ok := s.carregar(imp.resolvido); ok {
				return &Local{URI: s.uriDe(imp.resolvido)}
			}
		}
		return nil
	}
	if o.simb == nil || o.simb.decl == nil {
		return nil // builtin, keyword ou indefinido
	}
	return s.localDe(o.simb.decl)
}

// localDe monta o Location de uma ocorrencia, no arquivo onde ela esta (o
// escopo sabe a analise dona; simbolo externo aponta pro arquivo do modulo).
func (s *Servidor) localDe(o *ocorrencia) *Local {
	dono := o.escopo.a
	uri := dono.uri
	if uri == "" {
		uri = s.uriDe(dono.caminho)
	}
	return &Local{URI: uri, Range: dono.linhas.faixa(o.linha, o.col, o.tam)}
}

// ---- referencias ----

// alvoRefs junta as ocorrencias de um simbolo, por arquivo.
type alvoRefs struct {
	sb      *simbolo
	porDoc  map[*analise][]*ocorrencia
	ordem   []*analise
	declDoc *analise
}

// coletar acha todas as ocorrencias do simbolo de `o`: no proprio documento
// pra simbolo local de gambiarra; pra simbolo de topo (que pode ser importado),
// tambem no arquivo que define e em todo .gs aberto ou do workspace que o
// referencia.
func (s *Servidor) coletar(ctx *contextoAnalise, a *analise, o *ocorrencia) *alvoRefs {
	r := &alvoRefs{sb: o.simb, porDoc: map[*analise][]*ocorrencia{}}
	add := func(doc *analise) {
		if doc == nil {
			return
		}
		if _, ja := r.porDoc[doc]; ja {
			return
		}
		var ocs []*ocorrencia
		for _, x := range doc.ocorrencias {
			if mesmoSimbolo(x.simb, o.simb) {
				ocs = append(ocs, x)
			}
		}
		// ordem de avaliacao != ordem do texto (lado direito do bota vem
		// antes do nome): devolve em ordem de posicao
		sort.SliceStable(ocs, func(i, j int) bool {
			if ocs[i].linha != ocs[j].linha {
				return ocs[i].linha < ocs[j].linha
			}
			return ocs[i].col < ocs[j].col
		})
		r.porDoc[doc] = ocs
		r.ordem = append(r.ordem, doc)
	}
	add(a)
	if o.simb.chave == "" {
		r.declDoc = a
		return r
	}
	defCaminho := o.simb.chave[:strings.LastIndex(o.simb.chave, "#")]
	r.declDoc = ctx.modulo(defCaminho)
	add(r.declDoc)
	for _, c := range s.candidatos(a.caminho, defCaminho) {
		texto, ok := ctx.carregar(c)
		if !ok || !strings.Contains(texto, o.simb.nome) {
			continue
		}
		add(ctx.modulo(c))
	}
	return r
}

// candidatos lista os .gs que podem referenciar um simbolo de topo: os
// abertos no editor, os das pastas do doc e da definicao e os do workspace
// (com teto, pulando pastas ocultas e node_modules).
func (s *Servidor) candidatos(pastas ...string) []string {
	vistos := map[string]bool{}
	var out []string
	add := func(c string) {
		c = filepath.Clean(c)
		if c == "" || vistos[c] || !strings.HasSuffix(c, ".gs") {
			return
		}
		vistos[c] = true
		out = append(out, c)
	}
	for uri := range s.docs {
		if c := uriParaCaminho(uri); c != "" {
			add(c)
		}
	}
	for _, arq := range pastas {
		if arq == "" {
			continue
		}
		entradas, err := os.ReadDir(filepath.Dir(arq))
		if err != nil {
			continue
		}
		for _, e := range entradas {
			if !e.IsDir() {
				add(filepath.Join(filepath.Dir(arq), e.Name()))
			}
		}
	}
	if s.raiz != "" {
		const teto = 2000
		filepath.WalkDir(s.raiz, func(p string, d fs.DirEntry, err error) error {
			if err != nil || len(out) >= teto {
				return filepath.SkipDir
			}
			if d.IsDir() {
				nome := d.Name()
				if p != s.raiz && (strings.HasPrefix(nome, ".") || nome == "node_modules") {
					return filepath.SkipDir
				}
				return nil
			}
			add(p)
			return nil
		})
	}
	sort.Strings(out)
	return out
}

func (s *Servidor) referencias(uri string, pos Posicao, comDecl bool) []Local {
	ctx, a := s.analisarDoc(uri)
	out := []Local{}
	o := a.ocorrenciaEm(pos)
	if o == nil || o.simb == nil || o.simb.decl == nil {
		return out
	}
	r := s.coletar(ctx, a, o)
	for _, doc := range r.ordem {
		for _, x := range r.porDoc[doc] {
			if !comDecl && x == r.sb.decl {
				continue
			}
			out = append(out, *s.localDe(x))
		}
	}
	return out
}

// ---- renomear ----

// podeRenomear valida a ocorrencia sob o cursor; devolve mensagem de erro.
// palavra e o texto sob o cursor (pra explicar quando nao e ocorrencia).
func podeRenomear(o *ocorrencia, palavra string) string {
	nome := palavra
	if o != nil {
		nome = o.nome
	}
	if o == nil || o.simb == nil || o.simb.decl == nil {
		switch {
		case nome == "":
			return "nao tem nome renomeavel aqui"
		case ehKeyword(nome) || token.LookupIdent(nome) != token.IDENT:
			return "`" + nome + "` e keyword, nao da pra renomear"
		case builtinsSet[nome] || docsBuiltin[nome] != "":
			return "`" + nome + "` e builtin, nao da pra renomear"
		}
		if _, ok := object.Predefinidas[nome]; ok {
			return "`" + nome + "` e builtin, nao da pra renomear"
		}
		if o == nil {
			return "nao tem nome renomeavel aqui"
		}
		return "nao achei onde `" + nome + "` e declarado"
	}
	return ""
}

// palavraEm devolve o identificador (letras, digitos, _) sob a posicao.
func palavraEm(a *analise, p Posicao) string {
	if p.Line < 0 || p.Line >= len(a.linhas) {
		return ""
	}
	rs := a.linhas[p.Line]
	col := a.linhas.utf16ParaRune(p.Line, p.Character)
	ehLetra := func(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }
	ini, fim := col, col
	for ini > 0 && ehLetra(rs[ini-1]) {
		ini--
	}
	for fim < len(rs) && ehLetra(rs[fim]) {
		fim++
	}
	return string(rs[ini:fim])
}

func (s *Servidor) prepararRenomear(uri string, pos Posicao) (Faixa, string, string) {
	_, a := s.analisarDoc(uri)
	o := a.ocorrenciaEm(pos)
	if msg := podeRenomear(o, palavraEm(a, pos)); msg != "" {
		return Faixa{}, "", msg
	}
	return a.linhas.faixa(o.linha, o.col, o.tam), o.nome, ""
}

func (s *Servidor) renomear(uri string, pos Posicao, novo string) (map[string]interface{}, string) {
	ctx, a := s.analisarDoc(uri)
	o := a.ocorrenciaEm(pos)
	if msg := podeRenomear(o, palavraEm(a, pos)); msg != "" {
		return nil, msg
	}
	if !ehNomeValido(novo) {
		return nil, "`" + novo + "` nao e um nome valido (letra ou _ no comeco, sem keyword)"
	}
	if _, ok := object.Predefinidas[novo]; ok || builtinsSet[novo] {
		return nil, "`" + novo + "` e builtin; renomear pra isso sombrearia o builtin"
	}
	if novo == o.nome {
		return map[string]interface{}{"changes": map[string][]EdicaoTexto{}}, ""
	}
	r := s.coletar(ctx, a, o)
	mudancas := map[string][]EdicaoTexto{}
	for _, doc := range r.ordem {
		ocs := r.porDoc[doc]
		if len(ocs) == 0 {
			continue
		}
		for _, x := range ocs {
			if x.lig == ligDesestruturaChave {
				return nil, "`" + o.nome + "` vem de `bota {...} = dict`: o nome e a chave do dicionario, renomear mudaria o que e lido"
			}
		}
		if msg := conflitoRenomear(doc, r, ocs, novo); msg != "" {
			return nil, msg
		}
		uriDoc := s.localDe(ocs[0]).URI
		for _, x := range ocs {
			mudancas[uriDoc] = append(mudancas[uriDoc], EdicaoTexto{
				Range:   doc.linhas.faixa(x.linha, x.col, x.tam),
				NewText: novo,
			})
		}
	}
	return map[string]interface{}{"changes": mudancas}, ""
}

// conflitoRenomear recusa o rename quando `novo` ja tem dono onde o simbolo
// vive: ligado no escopo (ou de fora, sombreando) de alguma ocorrencia, ou
// usado dentro do escopo do simbolo (o uso passaria a cair no renomeado).
func conflitoRenomear(doc *analise, r *alvoRefs, ocs []*ocorrencia, novo string) string {
	soCampos := true
	for _, x := range ocs {
		if !x.campo {
			soCampos = false
		}
	}
	if soCampos {
		// so `m.nome`: o arquivo que define e checado no proprio doc dele
		return ""
	}
	alvo := doc.raiz
	if r.sb.chave == "" && r.sb.escopo != nil {
		alvo = r.sb.escopo
	}
	for _, q := range doc.ocorrencias {
		if q.nome == novo && !q.campo && q.escopo.contem(alvo) {
			return "ja existe `" + novo + "` nesse escopo; escolhe outro nome"
		}
	}
	for _, x := range ocs {
		if !x.campo && x.escopo.ligaNaCadeia(novo) {
			return "`" + novo + "` ja esta visivel em alguma referencia; renomear mudaria o significado"
		}
	}
	return ""
}
