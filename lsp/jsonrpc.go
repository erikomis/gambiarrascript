package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Mensagem representa uma mensagem JSON-RPC 2.0 (request, response ou notification).
type Mensagem struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *RespErro        `json:"error,omitempty"`
}

// RespErro e o objeto de erro do JSON-RPC.
type RespErro struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// LerMensagem le uma mensagem com framing "Content-Length: N\r\n\r\n<corpo>".
func LerMensagem(r *bufio.Reader) (*Mensagem, error) {
	contentLength := -1
	for {
		linha, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		linha = strings.TrimRight(linha, "\r\n")
		if linha == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(linha), "content-length:") {
			valor := strings.TrimSpace(linha[len("content-length:"):])
			contentLength, err = strconv.Atoi(valor)
			if err != nil {
				return nil, fmt.Errorf("content-length invalido: %v", err)
			}
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("mensagem sem content-length")
	}
	corpo := make([]byte, contentLength)
	if _, err := io.ReadFull(r, corpo); err != nil {
		return nil, err
	}
	var m Mensagem
	if err := json.Unmarshal(corpo, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// EscreverMensagem serializa m em JSON e escreve com framing Content-Length.
func EscreverMensagem(w io.Writer, m *Mensagem) error {
	m.JSONRPC = "2.0"
	corpo, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(corpo)); err != nil {
		return err
	}
	_, err = w.Write(corpo)
	return err
}
