package object

import "sync"

// Environment e um escopo lexico encadeado. Desde a introducao de concorrencia
// (escuta paralelo + paralelo()) o store precisa ser seguro pra acesso
// concorrente — handlers de HTTP rodando em goroutines separadas leem e
// escrevem no mesmo Environment global. O RWMutex permite multiplas leituras
// simultaneas (comum: handlers so consultam estado via closure) e escrita
// exclusiva. O `outer` e imutavel depois de construido.
type Environment struct {
	mu    sync.RWMutex
	store map[string]Object
	outer *Environment
}

func NewEnvironment() *Environment {
	return &Environment{store: map[string]Object{}}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

// Get caminha pela cadeia de escopos. Seguro pra chamada concorrente.
func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.buscaLocal(name)
	if !ok && e.outer != nil {
		return e.outer.Get(name)
	}
	return obj, ok
}

// buscaLocal tenta achar a chave so neste nivel (sem descer pro outer).
func (e *Environment) buscaLocal(name string) (Object, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	obj, ok := e.store[name]
	return obj, ok
}

// Set escreve no escopo atual. Seguro pra chamada concorrente.
func (e *Environment) Set(name string, val Object) Object {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.store[name] = val
	return val
}

// Locais devolve os nomes definidos neste proprio escopo (ignora outer),
// usado pelo importa pra mesclar as definicoes do modulo no escopo importador.
// Snapshot consistente sob lock.
func (e *Environment) Locais() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	nomes := make([]string, 0, len(e.store))
	for k := range e.store {
		nomes = append(nomes, k)
	}
	return nomes
}