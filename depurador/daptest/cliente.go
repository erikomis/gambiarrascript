// Package daptest e um cliente DAP minimo pros testes do gs debug --dap (no
// processo, por pipes, ou num `gs` de verdade). Nao entra no binario.
package daptest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Msg e uma mensagem do adapter (resposta ou evento), decodificada.
type Msg map[string]any

// Corpo devolve o body como mapa.
func (m Msg) Corpo() map[string]any {
	b, _ := m["body"].(map[string]any)
	return b
}

// Cliente fala DAP com um adapter.
type Cliente struct {
	w       io.Writer
	seq     int
	msgs    chan Msg
	mu      sync.Mutex
	eventos []Msg // eventos lidos e ainda nao consumidos
	Timeout time.Duration
	Log     []string // tudo o que chegou, na ordem (pra diagnostico)
}

// Novo liga o cliente: escreve requests em w e le as mensagens de r.
func Novo(w io.Writer, r io.Reader) *Cliente {
	c := &Cliente{w: w, msgs: make(chan Msg, 1024), Timeout: 10 * time.Second}
	go func() {
		br := bufio.NewReader(r)
		for {
			n := -1
			for {
				linha, err := br.ReadString('\n')
				if err != nil {
					close(c.msgs)
					return
				}
				linha = strings.TrimSpace(linha)
				if linha == "" && n >= 0 {
					break
				}
				if v, ok := strings.CutPrefix(linha, "Content-Length:"); ok {
					n, _ = strconv.Atoi(strings.TrimSpace(v))
				}
			}
			corpo := make([]byte, n)
			if _, err := io.ReadFull(br, corpo); err != nil {
				close(c.msgs)
				return
			}
			var m Msg
			if json.Unmarshal(corpo, &m) == nil {
				c.msgs <- m
			}
		}
	}()
	return c
}

// Envia manda um request e devolve o seq dele.
func (c *Cliente) Envia(cmd string, args any) (int, error) {
	c.seq++
	m := map[string]any{"seq": c.seq, "type": "request", "command": cmd}
	if args != nil {
		m["arguments"] = args
	}
	corpo, _ := json.Marshal(m)
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n%s", len(corpo), corpo); err != nil {
		return 0, err
	}
	return c.seq, nil
}

// proxima le a proxima mensagem (com timeout).
func (c *Cliente) proxima() (Msg, error) {
	select {
	case m, ok := <-c.msgs:
		if !ok {
			return nil, io.EOF
		}
		c.Log = append(c.Log, fmt.Sprintf("%v", m))
		return m, nil
	case <-time.After(c.Timeout):
		return nil, fmt.Errorf("timeout esperando mensagem do adapter")
	}
}

// Pede manda o request e espera a resposta (os eventos no meio ficam
// guardados pro Espera).
func (c *Cliente) Pede(cmd string, args any) (Msg, error) {
	seq, err := c.Envia(cmd, args)
	if err != nil {
		return nil, err
	}
	for {
		m, err := c.proxima()
		if err != nil {
			return nil, fmt.Errorf("%s: %v", cmd, err)
		}
		if m["type"] == "response" && int(m["request_seq"].(float64)) == seq {
			return m, nil
		}
		if m["type"] == "event" {
			c.mu.Lock()
			c.eventos = append(c.eventos, m)
			c.mu.Unlock()
		}
	}
}

// Espera devolve o primeiro evento `nome` que satisfaz ok (nil = qualquer),
// consumindo ele (e os eventos anteriores a ele de mesmo nome que nao
// serviram continuam guardados).
func (c *Cliente) Espera(nome string, ok func(Msg) bool) (Msg, error) {
	confere := func(m Msg) bool {
		return m["event"] == nome && (ok == nil || ok(m))
	}
	c.mu.Lock()
	for k, m := range c.eventos {
		if confere(m) {
			c.eventos = append(c.eventos[:k:k], c.eventos[k+1:]...)
			c.mu.Unlock()
			return m, nil
		}
	}
	c.mu.Unlock()
	for {
		m, err := c.proxima()
		if err != nil {
			return nil, fmt.Errorf("esperando %s: %v", nome, err)
		}
		if m["type"] != "event" {
			continue
		}
		if confere(m) {
			return m, nil
		}
		c.mu.Lock()
		c.eventos = append(c.eventos, m)
		c.mu.Unlock()
	}
}

// Saida junta o texto dos eventos output (de uma categoria) ja recebidos.
func (c *Cliente) Saida(cat string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var b strings.Builder
	for _, m := range c.eventos {
		if m["event"] == "output" && m.Corpo()["category"] == cat {
			b.WriteString(m.Corpo()["output"].(string))
		}
	}
	return b.String()
}
