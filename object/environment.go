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
	// cravadas sao os nomes declarados com `crava` NESTE escopo (nil ate o
	// primeiro). A checagem de verdade e estatica (ast.ChecaCravadas); isto
	// so serve pro importa saber o que o modulo e o importador cravaram.
	cravadas map[string]bool
	// modulo: caminho absoluto do arquivo dono deste escopo — so no escopo
	// raiz de um modulo importado (o principal fica "").
	modulo string
}

// MarcaModulo diz que este escopo e o topo do modulo `caminho` (absoluto).
// Chamado so na criacao do escopo do modulo, antes de qualquer uso.
func (e *Environment) MarcaModulo(caminho string) { e.modulo = caminho }

// Modulo devolve o arquivo do modulo dono deste escopo (subindo pelos
// escopos externos) ou "" quando o codigo e do programa principal. O
// `importa` usa pra resolver caminho relativo ao arquivo que importa.
func (e *Environment) Modulo() string {
	for env := e; env != nil; env = env.outer {
		if env.modulo != "" {
			return env.modulo
		}
	}
	return ""
}

func NewEnvironment() *Environment {
	return &Environment{store: map[string]Object{}}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

// Externo devolve o escopo de fora (nil na raiz). O depurador usa pra separar
// locais de globais.
func (e *Environment) Externo() *Environment { return e.outer }

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

// Crava marca o nome como cravado neste escopo.
func (e *Environment) Crava(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cravadas == nil {
		e.cravadas = map[string]bool{}
	}
	e.cravadas[name] = true
}

// EhCravada diz se o nome foi cravado NESTE escopo (ignora outer).
func (e *Environment) EhCravada(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cravadas[name]
}

// Cravadas devolve uma copia dos nomes cravados neste escopo.
func (e *Environment) Cravadas() map[string]bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string]bool, len(e.cravadas))
	for k := range e.cravadas {
		out[k] = true
	}
	return out
}
