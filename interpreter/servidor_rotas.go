//go:build !js

package interpreter

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"gambiarrascript/object"
)

// Roteador do servidor HTTP. Tres tipos de pedaco de caminho:
//
//	/usuarios        fixo: casa so o texto igual
//	/:id             parametro: casa UM pedaco nao-vazio → pedido["params"]["id"]
//	/*caminho        curinga: so no fim, casa o resto todo (pode ser vazio)
//
// Rota sem parametro nem curinga vai pro mapa de exatas (lookup O(1), igual
// era antes); as com padrao ficam numa lista e a mais especifica ganha.

const (
	pedacoFixo = iota
	pedacoParam
	pedacoCuringa
)

type pedacoRota struct {
	tipo  int
	texto string // fixo: o texto; param/curinga: o nome
}

type rotaHTTP struct {
	metodo  string
	padrao  string
	pedacos []pedacoRota // nil = rota exata
	handler object.Object
	ws      bool // rota_ws: o handler recebe (ws, pedido)
	// origens liberadas no handshake do rota_ws (nil = so a mesma origem;
	// "*" = qualquer uma, so quando o script pede com todas as letras)
	wsOrigens []string
	ordem     int // ordem de registro, desempata especificidade
}

// rotulo e o nome que aparece no log de erro e no traço de pilha.
func (r *rotaHTTP) rotulo() string {
	if r.ws {
		return "<rota_ws " + r.padrao + ">"
	}
	return "<rota " + r.metodo + " " + r.padrao + ">"
}

// compilaPadrao quebra o caminho em pedacos. Devolve nil (sem erro) quando e
// rota exata. Erro quando o padrao nao faz sentido (curinga no meio, nome
// vazio, nome repetido).
func compilaPadrao(caminho string) ([]pedacoRota, error) {
	if !strings.Contains(caminho, "/:") && !strings.Contains(caminho, "/*") {
		return nil, nil
	}
	partes := strings.Split(strings.TrimPrefix(caminho, "/"), "/")
	pedacos := make([]pedacoRota, 0, len(partes))
	vistos := map[string]bool{}
	for idx, p := range partes {
		switch {
		case strings.HasPrefix(p, ":") || strings.HasPrefix(p, "*"):
			nome := p[1:]
			if nome == "" {
				return nil, fmt.Errorf("o parametro %q ficou sem nome (use tipo /:id ou /*resto)", p)
			}
			if vistos[nome] {
				return nil, fmt.Errorf("o parametro %q apareceu duas vezes no caminho", nome)
			}
			vistos[nome] = true
			tipo := pedacoParam
			if p[0] == '*' {
				tipo = pedacoCuringa
				if idx != len(partes)-1 {
					return nil, fmt.Errorf("o curinga %q so pode ser o ultimo pedaco do caminho", p)
				}
			}
			pedacos = append(pedacos, pedacoRota{tipo: tipo, texto: nome})
		default:
			pedacos = append(pedacos, pedacoRota{tipo: pedacoFixo, texto: p})
		}
	}
	return pedacos, nil
}

// pedacosDoCaminho quebra o path ESCAPADO do request e desescapa cada pedaco
// separado — assim um %2F dentro de um :id nao vira barra que racha a rota.
func pedacosDoCaminho(escapado string) []string {
	partes := strings.Split(strings.TrimPrefix(escapado, "/"), "/")
	for i, p := range partes {
		if d, err := url.PathUnescape(p); err == nil {
			partes[i] = d
		}
	}
	return partes
}

// casa tenta casar os pedacos do request com o padrao. Devolve os parametros
// (na ordem do padrao) e se casou.
func (r *rotaHTTP) casa(partes []string) ([][2]string, bool) {
	var params [][2]string
	for idx, p := range r.pedacos {
		if p.tipo == pedacoCuringa {
			resto := ""
			if idx < len(partes) {
				resto = strings.Join(partes[idx:], "/")
			}
			return append(params, [2]string{p.texto, resto}), true
		}
		if idx >= len(partes) {
			return nil, false
		}
		switch p.tipo {
		case pedacoFixo:
			if partes[idx] != p.texto {
				return nil, false
			}
		case pedacoParam:
			if partes[idx] == "" {
				return nil, false
			}
			params = append(params, [2]string{p.texto, partes[idx]})
		}
	}
	if len(partes) != len(r.pedacos) {
		return nil, false
	}
	return params, true
}

// maisEspecifica diz se a rota a ganha da b: pedaco a pedaco, fixo ganha de
// parametro, que ganha de curinga. Empate total = quem registrou primeiro.
func maisEspecifica(a, b *rotaHTTP) bool {
	for idx := 0; idx < len(a.pedacos) && idx < len(b.pedacos); idx++ {
		if a.pedacos[idx].tipo != b.pedacos[idx].tipo {
			return a.pedacos[idx].tipo < b.pedacos[idx].tipo
		}
	}
	if len(a.pedacos) != len(b.pedacos) {
		return len(a.pedacos) > len(b.pedacos)
	}
	return a.ordem < b.ordem
}

// registraRota guarda a rota. Mesmo metodo + mesmo padrao substitui (como era
// com o mapa antigo).
func (s *servidorEstado) registraRota(metodo, caminho string, handler object.Object, ws bool, wsOrigens []string) error {
	pedacos, err := compilaPadrao(caminho)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contaRotas++
	nova := &rotaHTTP{metodo: metodo, padrao: caminho, pedacos: pedacos, handler: handler, ws: ws, wsOrigens: wsOrigens, ordem: s.contaRotas}
	if pedacos == nil {
		s.exatas[metodo+" "+caminho] = nova
		return nil
	}
	for idx, r := range s.padroes {
		if r.metodo == metodo && r.padrao == caminho {
			nova.ordem = r.ordem
			s.padroes[idx] = nova
			return nil
		}
	}
	s.padroes = append(s.padroes, nova)
	return nil
}

// acha procura a rota do request. Sem rota pro metodo, devolve a lista de
// metodos que existem pro caminho (pra responder 405 com Allow).
func (s *servidorEstado) acha(metodo, caminho, escapado string) (*rotaHTTP, [][2]string, []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	partes := pedacosDoCaminho(escapado)

	tenta := func(m string) (*rotaHTTP, [][2]string) {
		if r, ok := s.exatas[m+" "+caminho]; ok {
			return r, nil
		}
		var melhor *rotaHTTP
		var melhorParams [][2]string
		for _, r := range s.padroes {
			if r.metodo != m {
				continue
			}
			if params, ok := r.casa(partes); ok && (melhor == nil || maisEspecifica(r, melhor)) {
				melhor, melhorParams = r, params
			}
		}
		return melhor, melhorParams
	}

	if r, params := tenta(metodo); r != nil {
		return r, params, nil
	}
	// HEAD sem rota propria usa a de GET (o net/http joga o corpo fora).
	if metodo == "HEAD" {
		if r, params := tenta("GET"); r != nil {
			return r, params, nil
		}
	}

	// nada pro metodo: o caminho existe com outro metodo?
	permitidos := map[string]bool{}
	for chave, r := range s.exatas {
		if strings.HasSuffix(chave, " "+caminho) && r.padrao == caminho {
			permitidos[r.metodo] = true
		}
	}
	for _, r := range s.padroes {
		if _, ok := r.casa(partes); ok {
			permitidos[r.metodo] = true
		}
	}
	if len(permitidos) == 0 {
		return nil, nil, nil
	}
	if permitidos["GET"] {
		permitidos["HEAD"] = true
	}
	lista := make([]string, 0, len(permitidos))
	for m := range permitidos {
		lista = append(lista, m)
	}
	sort.Strings(lista)
	return nil, nil, lista
}
