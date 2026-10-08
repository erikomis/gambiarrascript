package depurador

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// Framing do Debug Adapter Protocol: cada mensagem e um JSON precedido de
// cabecalhos HTTP-like ("Content-Length: N\r\n\r\n"). Feito na mao (igual o
// jsonrpc do LSP) pra nao puxar dependencia.

// msgDAP e o envelope de entrada (request) — so o que o adapter le.
type msgDAP struct {
	Seq       int             `json:"seq"`
	Type      string          `json:"type"`
	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments"`
}

// leitorDAP le mensagens do cliente.
type leitorDAP struct {
	r *bufio.Reader
}

func novoLeitorDAP(r io.Reader) *leitorDAP {
	return &leitorDAP{r: bufio.NewReader(r)}
}

// le devolve o corpo da proxima mensagem (io.EOF quando o cliente fecha).
func (l *leitorDAP) le() ([]byte, error) {
	tamanho := -1
	for {
		linha, err := l.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		linha = strings.TrimRight(linha, "\r\n")
		if linha == "" {
			if tamanho < 0 {
				continue // linha em branco solta antes do cabecalho
			}
			break
		}
		nome, valor, ok := strings.Cut(linha, ":")
		if ok && strings.EqualFold(strings.TrimSpace(nome), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(valor))
			if err != nil || n < 0 {
				return nil, fmt.Errorf("dap: Content-Length invalido: %q", valor)
			}
			tamanho = n
		}
	}
	corpo := make([]byte, tamanho)
	if _, err := io.ReadFull(l.r, corpo); err != nil {
		return nil, err
	}
	return corpo, nil
}

// escritorDAP manda mensagens pro cliente; seguro pra varias goroutines
// (eventos saem das goroutines dos fluxos).
type escritorDAP struct {
	mu  sync.Mutex
	w   io.Writer
	seq int
}

func (e *escritorDAP) manda(m map[string]any) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seq++
	m["seq"] = e.seq
	corpo, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(e.w, "Content-Length: %d\r\n\r\n", len(corpo)); err != nil {
		return err
	}
	_, err = e.w.Write(corpo)
	return err
}

func (e *escritorDAP) resposta(req *msgDAP, body any) {
	m := map[string]any{"type": "response", "request_seq": req.Seq, "success": true, "command": req.Command}
	if body != nil {
		m["body"] = body
	}
	e.manda(m)
}

func (e *escritorDAP) falha(req *msgDAP, msg string) {
	e.manda(map[string]any{"type": "response", "request_seq": req.Seq, "success": false,
		"command": req.Command, "message": msg})
}

func (e *escritorDAP) evento(nome string, body any) {
	m := map[string]any{"type": "event", "event": nome}
	if body != nil {
		m["body"] = body
	}
	e.manda(m)
}
