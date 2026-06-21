package lsp

import (
	"bufio"
	"encoding/json"
	"io"

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
	"e", "ou", "nao",
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

// itensCompletion devolve keywords + identificadores vistos no texto.
func (s *Servidor) itensCompletion(texto string) []ItemCompletion {
	vistos := map[string]bool{}
	var itens []ItemCompletion
	for _, kw := range keywords {
		itens = append(itens, ItemCompletion{Label: kw, Kind: 14}) // 14 = Keyword
		vistos[kw] = true
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

func (s *Servidor) notificar(metodo string, params interface{}) {
	raw, _ := json.Marshal(params)
	EscreverMensagem(s.out, &Mensagem{Method: metodo, Params: raw})
}

func (s *Servidor) responder(id *json.RawMessage, result interface{}) {
	EscreverMensagem(s.out, &Mensagem{ID: id, Result: result})
}
