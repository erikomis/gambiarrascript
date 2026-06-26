package lsp

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"gambiarrascript/lexer"
	"gambiarrascript/parser"
	"gambiarrascript/token"
)

// ---- tipos do protocolo (subconjunto) ----

type Posicao struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Faixa struct {
	Start Posicao `json:"start"`
	End   Posicao `json:"end"`
}

type Diagnostico struct {
	Range    Faixa  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

type ItemCompletion struct {
	Label string `json:"label"`
	Kind  int    `json:"kind"`
}

// keywords da GambiarraScript, sem acento, para o autocomplete.
var keywords = []string{
	"bota", "mostra", "se_colar", "se_nao_colar", "enquanto", "pra_cada",
	"de", "ate", "em", "gambiarra", "funciona", "arruma", "quebrou",
	"vaza", "continua", "deu_bom", "deu_ruim", "nada", "acabou_finalmente",
	"e", "ou", "nao", "importa",
	"bora", // bora fn(args) -> Futuro (concorrencia)
}

// builtinsCompletion são as funções nativas da linguagem expostas no autocomplete.
var builtinsCompletion = []string{
	"tamanho", "chaves", "tem", "texto", "numero", "busca", "rota", "escuta",
	"de_json", "pra_json",
	"separa", "junta", "maiusculo", "minusculo", "substitui", "fatia",
	"contem", "comeca_com", "termina_com", "tira_espaco",
	"adiciona", "remove", "ordena", "inverte", "mapeia", "filtra",
	"raiz", "aleatorio", "arredonda", "teto", "chao", "abs", "min", "max",
	"le_arquivo", "escreve_arquivo",
	"pergunta", "argumentos",
	// concorrencia
	"cano", "envia", "recebe", "fecha", "espera", "afirma",
}

// docsBuiltin descreve cada builtin pro hover do LSP.
var docsBuiltin = map[string]string{
	"tamanho":   "tamanho(x) -> numero: devolve o tamanho de lista, dicionario ou texto.",
	"chaves":    "chaves(dicionario) -> lista: devolve as chaves do dicionario.",
	"tem":       "tem(dicionario, chave) -> booleano: checa se a chave existe.",
	"texto":     "texto(valor) -> texto: converte qualquer valor em texto.",
	"numero":    "numero(texto) -> numero: converte texto em numero.",
	"busca":     "busca(url, [opcoes]) -> dicionario: faz uma requisicao HTTP.",
	"rota":      "rota(metodo, caminho, handler): registra uma rota no servidor HTTP.",
	"escuta":    "escuta(porta): sobe o servidor HTTP e bloqueia.",
	"de_json":   "de_json(texto) -> valor: converte JSON em valor GambiarraScript.",
	"pra_json":  "pra_json(valor) -> texto: serializa um valor pra JSON.",
	"separa":    "separa(texto, separador) -> lista: quebra o texto em partes.",
	"junta":     "junta(lista, separador) -> texto: junta os itens da lista num texto.",
	"maiusculo": "maiusculo(texto) -> texto: converte pra maiusculas.",
	"minusculo": "minusculo(texto) -> texto: converte pra minusculas.",
	"substitui": "substitui(texto, antigo, novo) -> texto: troca todas as ocorrencias.",
	"fatia":     "fatia(texto, inicio, [fim]) -> texto: devolve um pedaco do texto.",
	"contem":    "contem(texto, pedaco) -> booleano: checa se o texto contem o pedaco.",
	"comeca_com":  "comeca_com(texto, prefixo) -> booleano.",
	"termina_com": "termina_com(texto, sufixo) -> booleano.",
	"tira_espaco": "tira_espaco(texto) -> texto: remove espacos nas pontas (trim).",
	"adiciona":  "adiciona(lista, item): adiciona item ao final da lista (muda a lista).",
	"remove":    "remove(lista, item): remove a primeira ocorrencia de item.",
	"ordena":    "ordena(lista): ordena a lista in-place (numeros ou textos).",
	"inverte":   "inverte(lista): inverte a lista in-place.",
	"mapeia":    "mapeia(lista, gambiarra) -> lista: aplica a gambiarra em cada item.",
	"filtra":    "filtra(lista, gambiarra) -> lista: keep itens em que a gambiarra devolve verdadeiro.",
	"raiz":      "raiz(numero) -> numero: raiz quadrada.",
	"aleatorio": "aleatorio([max]) -> numero: numero aleatorio em [0, max).",
	"arredonda": "arredonda(numero) -> numero: arredonda pro inteiro mais proximo.",
	"teto":      "teto(numero) -> numero: arredonda pra cima.",
	"chao":      "chao(numero) -> numero: arredonda pra baixo.",
	"abs":       "abs(numero) -> numero: valor absoluto.",
	"min":       "min(n1, n2, ...) -> numero: o menor dos numeros.",
	"max":       "max(n1, n2, ...) -> numero: o maior dos numeros.",
	"le_arquivo":     "le_arquivo(caminho) -> texto: le todo o conteudo de um arquivo.",
	"escreve_arquivo": "escreve_arquivo(caminho, texto): escreve texto num arquivo.",
	"pergunta":  "pergunta([prompt]) -> texto: le uma linha do stdin.",
	"argumentos": "argumentos() -> lista: argumentos de linha de comando passados ao script.",
	// concorrencia
	"cano":   "cano([capacidade]) -> cano: cria um canal (channel). Sem args = sincrono.",
	"envia":  "envia(cano, valor): manda um valor pro cano. Bloqueia se o cano estiver cheio ou se nao houver receptor.",
	"recebe": "recebe(cano) -> valor: pega o proximo valor do cano. Bloqueia ate ter algo (ou cano ser fechado -> nada).",
	"fecha":  "fecha(cano_ou_conexao): fecha um cano (channel) ou uma conexao de banco. Idempotente.",
	"espera": "espera(futuro|lista_de_futuros) -> valor|lista: aguarda o(s) futuro(s) e devolve o(s) valor(es). Tambem: espera(a, b) = assert de teste.",
	"afirma": "afirma(cond, [msg]): assert de teste pra gs testa.",
}

// docsKeyword descreve cada keyword pro hover do LSP.
var docsKeyword = map[string]string{
	"bota":              "bota nome = valor: declara (ou reatribui) uma variavel.",
	"mostra":            "mostra valor: imprime no stdout.",
	"se_colar":          "se_colar condicao ... se_nao_colar ... acabou_finalmente: condicional.",
	"se_nao_colar":      "se_nao_colar: ramo alternativo (else / else-if).",
	"enquanto":          "enquanto condicao ... acabou_finalmente: laco while.",
	"pra_cada":          "pra_cada var de A ate B / pra_cada var em lista ... acabou_finalmente: laco for.",
	"gambiarra":         "gambiarra nome(params) ... acabou_finalmente: declara uma funcao.",
	"funciona":          "funciona valor: return de uma gambiarra.",
	"arruma":            "arruma ... quebrou erro ... acabou_finalmente: try/catch.",
	"quebrou":           "quebrou nome: captura o erro do arruma.",
	"vaza":              "vaza: break de um loop.",
	"continua":          "continua: continue de um loop.",
	"deu_bom":           "deu_bom: booleano verdadeiro.",
	"deu_ruim":          "deu_ruim: booleano falso.",
	"nada":              "nada: valor nulo.",
	"acabou_finalmente": "acabou_finalmente: fecha um bloco.",
	"e":                 "e: operador logico AND (curto-circuito).",
	"ou":                "ou: operador logico OR (curto-circuito).",
	"nao":               "nao: operador logico NOT.",
	"importa":           "importa \"caminho.gs\": carrega outro script e traz suas definicoes.",
	"bora":              "bora fn(args) -> Futuro: dispara a gambiarra numa goroutine e devolve um Futuro imediatamente. Use espera(futuro) pra aguardar o valor.",
}

// ---- servidor ----

type Servidor struct {
	docs map[string]string
	out  io.Writer
}

func NovoServidor(out io.Writer) *Servidor {
	return &Servidor{docs: map[string]string{}, out: out}
}

// Rodar processa mensagens do stdin ate o EOF ou um 'exit'.
func (s *Servidor) Rodar(in io.Reader) error {
	r := bufio.NewReader(in)
	for {
		msg, err := LerMensagem(r)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if s.tratar(msg) {
			return nil // exit
		}
	}
}

// tratar despacha uma mensagem; devolve true se for 'exit'.
func (s *Servidor) tratar(msg *Mensagem) bool {
	switch msg.Method {
	case "initialize":
		s.responder(msg.ID, map[string]interface{}{
			"capabilities": map[string]interface{}{
				"textDocumentSync":   1, // Full
				"completionProvider": map[string]interface{}{},
				"hoverProvider":      true,
			},
			"serverInfo": map[string]interface{}{"name": "gambiarrascript-lsp"},
		})
	case "shutdown":
		s.responder(msg.ID, nil)
	case "exit":
		return true
	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"textDocument"`
		}
		json.Unmarshal(msg.Params, &p)
		s.docs[p.TextDocument.URI] = p.TextDocument.Text
		s.PublicarDiagnosticos(p.TextDocument.URI)
	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		json.Unmarshal(msg.Params, &p)
		if len(p.ContentChanges) > 0 {
			s.docs[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
			s.PublicarDiagnosticos(p.TextDocument.URI)
		}
	case "textDocument/didClose":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		json.Unmarshal(msg.Params, &p)
		delete(s.docs, p.TextDocument.URI)
	case "textDocument/completion":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		json.Unmarshal(msg.Params, &p)
		s.responder(msg.ID, map[string]interface{}{
			"isIncomplete": false,
			"items":        s.itensCompletion(s.docs[p.TextDocument.URI]),
		})
	case "textDocument/hover":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		json.Unmarshal(msg.Params, &p)
		doc := s.docs[p.TextDocument.URI]
		conteudo := hoverConteudo(doc, p.Position.Line, p.Position.Character)
		if conteudo == "" {
			s.responder(msg.ID, nil)
		} else {
			s.responder(msg.ID, map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": conteudo,
				},
			})
		}
	}
	// 'initialized' e outras notifications sem id sao ignoradas.
	return false
}

// PublicarDiagnosticos reparseia o documento e envia publishDiagnostics.
func (s *Servidor) PublicarDiagnosticos(uri string) {
	texto := s.docs[uri]
	p := parser.New(lexer.New(texto))
	p.ParseProgram()

	diags := []Diagnostico{}
	for _, e := range p.ErrosDetalhados() {
		linha := e.Linha - 1
		if linha < 0 {
			linha = 0
		}
		col := e.Coluna - 1
		if col < 0 {
			col = 0
		}
		diags = append(diags, Diagnostico{
			Range: Faixa{
				Start: Posicao{Line: linha, Character: col},
				End:   Posicao{Line: linha, Character: col + 1},
			},
			Severity: 1, // Error
			Source:   "gambiarrascript",
			Message:  e.Msg,
		})
	}
	s.notificar("textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         uri,
		"diagnostics": diags,
	})
}

// itensCompletion devolve keywords + builtins + identificadores vistos no texto.
func (s *Servidor) itensCompletion(texto string) []ItemCompletion {
	vistos := map[string]bool{}
	var itens []ItemCompletion
	for _, kw := range keywords {
		itens = append(itens, ItemCompletion{Label: kw, Kind: 14}) // 14 = Keyword
		vistos[kw] = true
	}
	for _, b := range builtinsCompletion {
		if !vistos[b] {
			itens = append(itens, ItemCompletion{Label: b, Kind: 3}) // 3 = Function
			vistos[b] = true
		}
	}
	l := lexer.New(texto)
	for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
		if tok.Type == token.IDENT && !vistos[tok.Literal] {
			vistos[tok.Literal] = true
			itens = append(itens, ItemCompletion{Label: tok.Literal, Kind: 6}) // 6 = Variable
		}
	}
	return itens
}

// hoverConteudo devolve o texto markdown de hover pro word na posicao dada,
// ou "" se nao houver documentacao. linha e coluna sao 0-indexadas (LSP).
func hoverConteudo(doc string, linha, coluna int) string {
	linhas := strings.Split(doc, "\n")
	if linha < 0 || linha >= len(linhas) {
		return ""
	}
	linhaTexto := linhas[linha]
	if coluna < 0 || coluna > len([]rune(linhaTexto)) {
		return ""
	}
	runes := []rune(linhaTexto)
	inicio := coluna
	for inicio > 0 && isWordChar(runes[inicio-1]) {
		inicio--
	}
	fim := coluna
	for fim < len(runes) && isWordChar(runes[fim]) {
		fim++
	}
	if inicio >= fim {
		return ""
	}
	palavra := string(runes[inicio:fim])
	if d, ok := docsBuiltin[palavra]; ok {
		return "`" + palavra + "`\n\n" + d
	}
	if d, ok := docsKeyword[palavra]; ok {
		return "`" + palavra + "` — keyword\n\n" + d
	}
	return ""
}

func isWordChar(r rune) bool {
	return r == '_' ||
		('a' <= r && r <= 'z') ||
		('A' <= r && r <= 'Z') ||
		('0' <= r && r <= '9')
}

func (s *Servidor) notificar(metodo string, params interface{}) {
	raw, _ := json.Marshal(params)
	EscreverMensagem(s.out, &Mensagem{Method: metodo, Params: raw})
}

func (s *Servidor) responder(id *json.RawMessage, result interface{}) {
	if result == nil {
		result = json.RawMessage("null")
	}
	EscreverMensagem(s.out, &Mensagem{ID: id, Result: result})
}
