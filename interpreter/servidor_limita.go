//go:build !js

package interpreter

import (
	"container/list"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gambiarrascript/object"
)

// Limite de pedidos: limita({"por_minuto": 60}) segura cada cliente (por ip,
// ou pela chave que uma gambiarra(pedido) devolver) num balde de fichas: o
// balde comeca cheio com "rajada" fichas (padrao = por_minuto), cada pedido
// gasta uma e elas voltam aos poucos (por_minuto/60 por segundo). Sem ficha
// → 429 com Retry-After. Da pra chamar varias vezes (um geral por ip e um
// mais apertado com "prefixo": "/login", por exemplo): todos valem.
//
// Memoria limitada: no maximo max_chaves baldes (padrao 10000); passou, sai
// o que ficou mais tempo sem pedido (o cliente dele volta de balde cheio).

const maxChavesPadrao = 10000

type limitador struct {
	porSeg   float64 // fichas que voltam por segundo
	rajada   float64 // tamanho do balde
	chaveFn  object.Object
	prefixo  string
	maxChave int

	mu     sync.Mutex
	baldes map[string]*list.Element
	ordem  *list.List // frente = usado agora; fundo = o mais esquecido
	agora  func() time.Time
}

type balde struct {
	chave  string
	fichas float64
	ultimo time.Time
}

func (s *servidorEstado) builtinLimita(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("limita() quer 1 argumento (opcoes), tipo limita({\"por_minuto\": 60}), veio %d", len(args))
	}
	d, ok := args[0].(*object.Dicionario)
	if !ok {
		return erroBuiltin("limita(): as opcoes tem que ser dicionario, tipo {\"por_minuto\": 60}, veio %s", object.NomeTipo(args[0]))
	}
	l := &limitador{maxChave: maxChavesPadrao, baldes: map[string]*list.Element{}, ordem: list.New(), agora: time.Now}
	porMinuto := 0.0
	for _, par := range d.Pares() {
		k, _ := par.Chave.(*object.Texto)
		if k == nil {
			return erroBuiltin("limita(): as opcoes tem que ter chave texto, veio %s", par.Chave.Inspect())
		}
		numero := func() (float64, *object.Erro) {
			n, _ := par.Valor.(*object.Numero)
			if n == nil || !ehInteiro(n) || n.Value < 1 {
				return 0, erroBuiltin("limita(): %q tem que ser numero inteiro maior que 0, veio %s", k.Value, par.Valor.Inspect())
			}
			return n.Value, nil
		}
		var erro *object.Erro
		switch k.Value {
		case "por_minuto":
			porMinuto, erro = numero()
		case "rajada":
			l.rajada, erro = numero()
		case "max_chaves":
			var n float64
			n, erro = numero()
			l.maxChave = int(n)
		case "chave":
			if t, ok := par.Valor.(*object.Texto); ok && t.Value == "ip" {
				l.chaveFn = nil
			} else if ehChamavel(par.Valor) {
				l.chaveFn = par.Valor
			} else {
				erro = erroBuiltin("limita(): \"chave\" tem que ser \"ip\" ou uma gambiarra(pedido) que devolve a chave, veio %s", par.Valor.Inspect())
			}
		case "prefixo":
			t, _ := par.Valor.(*object.Texto)
			if t == nil || !strings.HasPrefix(t.Value, "/") {
				erro = erroBuiltin("limita(): \"prefixo\" tem que ser texto comecando com /, veio %s", par.Valor.Inspect())
			} else {
				l.prefixo = t.Value
			}
		default:
			erro = erroBuiltin("limita(): opcao %q nao existe (as que existem: por_minuto, rajada, chave, prefixo, max_chaves)", k.Value)
		}
		if erro != nil {
			return erro
		}
	}
	if porMinuto == 0 {
		return erroBuiltin("limita(): falta o \"por_minuto\" (quantos pedidos por minuto cada cliente pode fazer)")
	}
	l.porSeg = porMinuto / 60
	if l.rajada == 0 {
		l.rajada = porMinuto
	}
	s.mu.Lock()
	s.limites = append(s.limites, l)
	s.mu.Unlock()
	return NADA
}

// vale diz se o caminho cai nesse limitador ("/api" pega "/api" e "/api/x",
// nao "/apix").
func (l *limitador) vale(caminho string) bool {
	if l.prefixo == "" || l.prefixo == "/" {
		return true
	}
	p := strings.TrimSuffix(l.prefixo, "/")
	return caminho == p || strings.HasPrefix(caminho, p+"/")
}

// pega gasta uma ficha da chave. Sem ficha: devolve false e quantos segundos
// faltam pra proxima.
func (l *limitador) pega(chave string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	agora := l.agora()
	var b *balde
	if el, ok := l.baldes[chave]; ok {
		b = el.Value.(*balde)
		l.ordem.MoveToFront(el)
		b.fichas = math.Min(l.rajada, b.fichas+agora.Sub(b.ultimo).Seconds()*l.porSeg)
		b.ultimo = agora
	} else {
		for len(l.baldes) >= l.maxChave {
			velho := l.ordem.Back()
			delete(l.baldes, velho.Value.(*balde).chave)
			l.ordem.Remove(velho)
		}
		b = &balde{chave: chave, fichas: l.rajada, ultimo: agora}
		l.baldes[chave] = l.ordem.PushFront(b)
	}
	if b.fichas >= 1 {
		b.fichas--
		return true, 0
	}
	espera := int(math.Ceil((1 - b.fichas) / l.porSeg))
	if espera < 1 {
		espera = 1
	}
	return false, espera
}

// rodaLimites passa o pedido por todos os limita(). Devolve a resposta que
// barrou (429, ou 500 se a gambiarra da chave estourou) ou nil.
func (s *servidorEstado) rodaLimites(r *http.Request, rota *rotaHTTP, pedido *object.Dicionario) *respostaHTTP {
	s.mu.RLock()
	limites := s.limites
	s.mu.RUnlock()
	for _, l := range limites {
		if !l.vale(r.URL.Path) {
			continue
		}
		var chave string
		if l.chaveFn == nil {
			v, _ := dicPega(pedido, "ip")
			if t, ok := v.(*object.Texto); ok {
				chave = t.Value
			}
		} else {
			switch v := s.chamaAdaptado(l.chaveFn, []object.Object{pedido}, "<limita>").(type) {
			case *object.Nada:
				continue // sem chave = esse pedido nao entra no limite
			case *object.Erro:
				s.logaErroHandler(r, "chave do limita() de "+rota.rotulo(), v)
				return respostaErroInterno()
			case *object.Texto:
				chave = v.Value
			default:
				chave = v.Inspect()
			}
		}
		if ok, espera := l.pega(chave); !ok {
			h := http.Header{}
			h.Set("Content-Type", "text/plain; charset=utf-8")
			h.Set("Retry-After", strconv.Itoa(espera))
			corpo := "calma la, parca: pedido demais (tenta de novo em " + strconv.Itoa(espera) + "s)"
			return &respostaHTTP{status: http.StatusTooManyRequests, cabecalhos: h, corpo: []byte(corpo)}
		}
	}
	return nil
}
