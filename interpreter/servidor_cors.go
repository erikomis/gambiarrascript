//go:build !js

package interpreter

import (
	"net/http"
	"strconv"
	"strings"

	"gambiarrascript/object"
)

// configCors e o que o cors() configurou. Vale pro servidor todo (rotas,
// pasta estatica, 404/405), porque o navegador precisa dos cabecalhos ate
// pra ler a resposta de erro.
type configCors struct {
	todas       bool            // "*" na lista de origens
	origens     map[string]bool // origens liberadas (sem barra no fim)
	listaOrig   []string        // na ordem, pro OriginPatterns do websocket
	metodos     string
	cabecalhos  string // "" = devolve o que o navegador pediu
	credenciais bool
	maxIdade    int
}

const metodosCorsPadrao = "GET, POST, PUT, PATCH, DELETE, OPTIONS"

// builtinCors: cors() libera geral; cors({"origens": [...], "metodos": [...],
// "cabecalhos": [...], "credenciais": deu_bom, "max_idade": 600}) restringe.
func (s *servidorEstado) builtinCors(args []object.Object) object.Object {
	if len(args) > 1 {
		return erroBuiltin("cors() quer 0 ou 1 argumento (opcoes), veio %d", len(args))
	}
	c := &configCors{todas: true, metodos: metodosCorsPadrao, maxIdade: 600, origens: map[string]bool{}}
	if len(args) == 1 {
		opcoes, ok := args[0].(*object.Dicionario)
		if !ok {
			return erroBuiltin("cors() espera um dicionario de opcoes, veio %s", args[0].Type())
		}
		var erro *object.Erro
		opcoes.Itera(func(par object.ParDic) {
			if erro != nil {
				return
			}
			erro = c.leOpcao(par)
		})
		if erro != nil {
			return erro
		}
	}
	if c.todas && c.credenciais {
		// ecoar qualquer origem com credenciais deixa qualquer site fazer
		// pedido logado em nome do usuario (o cookie vai junto)
		return erroBuiltin("cors(): \"credenciais\" com qualquer origem deixa qualquer site usar o login do usuario — lista as origens: cors({\"origens\": [\"https://app.com\"], \"credenciais\": deu_bom})")
	}
	s.mu.Lock()
	s.cors = c
	s.mu.Unlock()
	return NADA
}

func (c *configCors) leOpcao(par object.ParDic) *object.Erro {
	chave, _ := par.Chave.(*object.Texto)
	if chave == nil {
		return erroBuiltin("cors(): as opcoes tem que ter chave texto, veio %s", par.Chave.Type())
	}
	switch chave.Value {
	case "origens":
		lista, erro := listaDeTextos("origens", par.Valor)
		if erro != nil {
			return erro
		}
		c.todas = false
		for _, o := range lista {
			o = strings.TrimSuffix(o, "/")
			if o == "*" {
				c.todas = true
				continue
			}
			c.origens[o] = true
			c.listaOrig = append(c.listaOrig, o)
		}
	case "metodos":
		lista, erro := listaDeTextos("metodos", par.Valor)
		if erro != nil {
			return erro
		}
		for i := range lista {
			lista[i] = strings.ToUpper(lista[i])
		}
		c.metodos = strings.Join(lista, ", ")
	case "cabecalhos":
		lista, erro := listaDeTextos("cabecalhos", par.Valor)
		if erro != nil {
			return erro
		}
		c.cabecalhos = strings.Join(lista, ", ")
	case "credenciais":
		b, ok := par.Valor.(*object.Booleano)
		if !ok {
			return erroBuiltin("cors(): \"credenciais\" e deu_bom ou deu_ruim, veio %s", par.Valor.Type())
		}
		c.credenciais = b.Value
	case "max_idade":
		n, ok := par.Valor.(*object.Numero)
		if !ok || n.Value < 0 {
			return erroBuiltin("cors(): \"max_idade\" e numero de segundos (>= 0), veio %s", par.Valor.Inspect())
		}
		c.maxIdade = int(n.Value)
	default:
		return erroBuiltin("cors(): opcao desconhecida %q (as que existem: origens, metodos, cabecalhos, credenciais, max_idade)", chave.Value)
	}
	return nil
}

func listaDeTextos(nome string, v object.Object) ([]string, *object.Erro) {
	l, ok := v.(*object.Lista)
	if !ok {
		return nil, erroBuiltin("cors(): %q tem que ser lista de texto, veio %s", nome, v.Type())
	}
	elems := l.Copia()
	out := make([]string, 0, len(elems))
	for _, e := range elems {
		t, ok := e.(*object.Texto)
		if !ok {
			return nil, erroBuiltin("cors(): %q tem que ser lista de texto, achei um %s la dentro", nome, e.Type())
		}
		out = append(out, t.Value)
	}
	return out, nil
}

// aplica poe os cabecalhos de CORS. Devolve true quando ja respondeu
// (preflight OPTIONS) e a rota nao deve rodar.
func (c *configCors) aplica(w http.ResponseWriter, r *http.Request) bool {
	origem := r.Header.Get("Origin")
	if origem == "" {
		return false // nao e navegador cross-origin: nada a fazer
	}
	h := w.Header()
	h.Add("Vary", "Origin")
	preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
	if !c.todas && !c.origens[strings.TrimSuffix(origem, "/")] {
		if preflight {
			escreveTexto(w, http.StatusForbidden, "origem "+origem+" nao ta liberada no cors(), parca")
			return true
		}
		return false // sem cabecalho: o navegador barra a leitura
	}
	if c.todas && !c.credenciais {
		h.Set("Access-Control-Allow-Origin", "*")
	} else {
		// com credenciais o navegador nao aceita "*": ecoa a origem
		h.Set("Access-Control-Allow-Origin", origem)
	}
	if c.credenciais {
		h.Set("Access-Control-Allow-Credentials", "true")
	}
	if !preflight {
		return false
	}
	h.Add("Vary", "Access-Control-Request-Method")
	h.Add("Vary", "Access-Control-Request-Headers")
	h.Set("Access-Control-Allow-Methods", c.metodos)
	cab := c.cabecalhos
	if cab == "" {
		cab = r.Header.Get("Access-Control-Request-Headers")
	}
	if cab != "" {
		h.Set("Access-Control-Allow-Headers", cab)
	}
	h.Set("Access-Control-Max-Age", strconv.Itoa(c.maxIdade))
	w.WriteHeader(http.StatusNoContent)
	return true
}
