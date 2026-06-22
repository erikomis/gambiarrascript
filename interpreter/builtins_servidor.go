package interpreter

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"gambiarrascript/object"
)

type servidorEstado struct {
	rotas map[string]*object.Funcao
	mu    sync.Mutex
	i     *Interpreter
}

func chaveRota(metodo, caminho string) string {
	return strings.ToUpper(metodo) + " " + caminho
}

func (s *servidorEstado) builtinRota(args []object.Object) object.Object {
	if len(args) != 3 {
		return erroBuiltin("rota() quer 3 argumentos (metodo, caminho, handler), veio %d", len(args))
	}
	metodo, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("rota(): o metodo tem que ser texto, veio %s", args[0].Type())
	}
	caminho, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("rota(): o caminho tem que ser texto, veio %s", args[1].Type())
	}
	handler, ok := args[2].(*object.Funcao)
	if !ok {
		return erroBuiltin("rota(): o handler tem que ser uma gambiarra, veio %s", args[2].Type())
	}
	s.rotas[chaveRota(metodo.Value, caminho.Value)] = handler
	return NADA
}

func (s *servidorEstado) builtinEscuta(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("escuta() quer 1 argumento (porta), veio %d", len(args))
	}
	porta, ok := args[0].(*object.Numero)
	if !ok {
		return erroBuiltin("escuta(): a porta tem que ser numero, veio %s", args[0].Type())
	}
	endereco := ":" + strconv.Itoa(int(porta.Value))
	if err := http.ListenAndServe(endereco, s.Handler()); err != nil {
		return erroBuiltin("nao consegui escutar na porta %d: %v", int(porta.Value), err)
	}
	return NADA
}

// ServidorHandler expoe o http.Handler do servidor (usado em teste).
func (i *Interpreter) ServidorHandler() http.Handler {
	return i.servidor.Handler()
}

func (s *servidorEstado) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		handler, existe := s.rotas[chaveRota(r.Method, r.URL.Path)]
		if !existe {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, "rota nao encontrada, parca")
			return
		}

		pedido := s.montaPedido(r)
		resultado := s.i.applyFunction(handler, []object.Object{pedido}, 0)
		s.escreveResposta(w, resultado)
	})
}

func (s *servidorEstado) montaPedido(r *http.Request) *object.Dicionario {
	corpo, _ := io.ReadAll(r.Body)

	pares := map[object.HashKey]object.ParDic{}
	set := func(chave string, valor object.Object) {
		k := &object.Texto{Value: chave}
		pares[k.ChaveHash()] = object.ParDic{Chave: k, Valor: valor}
	}
	set("metodo", &object.Texto{Value: r.Method})
	set("caminho", &object.Texto{Value: r.URL.Path})
	set("corpo", &object.Texto{Value: string(corpo)})
	set("cabecalhos", dicDeMultimap(r.Header))
	set("query", dicDeMultimap(r.URL.Query()))
	return &object.Dicionario{Pares: pares}
}

// dicDeMultimap converte um map[string][]string (header/query) num dicionario
// texto->texto, juntando multiplos valores com ", ".
func dicDeMultimap(m map[string][]string) *object.Dicionario {
	pares := map[object.HashKey]object.ParDic{}
	for nome, valores := range m {
		k := &object.Texto{Value: nome}
		pares[k.ChaveHash()] = object.ParDic{Chave: k, Valor: &object.Texto{Value: strings.Join(valores, ", ")}}
	}
	return &object.Dicionario{Pares: pares}
}

func (s *servidorEstado) escreveResposta(w http.ResponseWriter, resultado object.Object) {
	switch r := resultado.(type) {
	case *object.Texto:
		io.WriteString(w, r.Value)
	case *object.Nada:
		// 200 com corpo vazio
	case *object.Erro:
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, r.Message)
	case *object.Dicionario:
		s.escreveRespostaDict(w, r)
	default:
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, fmt.Sprintf("o handler devolveu algo que nao da pra responder: %s", resultado.Type()))
	}
}

func (s *servidorEstado) escreveRespostaDict(w http.ResponseWriter, d *object.Dicionario) {
	// cabecalhos primeiro (antes de WriteHeader)
	if par, ok := d.Pares[(&object.Texto{Value: "cabecalhos"}).ChaveHash()]; ok {
		if cab, ok := par.Valor.(*object.Dicionario); ok {
			for _, p := range cab.Pares {
				nome, nok := p.Chave.(*object.Texto)
				valor, vok := p.Valor.(*object.Texto)
				if nok && vok {
					w.Header().Set(nome.Value, valor.Value)
				}
			}
		}
	}
	// status (default 200)
	status := http.StatusOK
	if par, ok := d.Pares[(&object.Texto{Value: "status"}).ChaveHash()]; ok {
		if n, ok := par.Valor.(*object.Numero); ok {
			status = int(n.Value)
		}
	}
	w.WriteHeader(status)
	// corpo (default vazio)
	if par, ok := d.Pares[(&object.Texto{Value: "corpo"}).ChaveHash()]; ok {
		if c, ok := par.Valor.(*object.Texto); ok {
			io.WriteString(w, c.Value)
		}
	}
}
