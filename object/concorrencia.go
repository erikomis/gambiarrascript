package object

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// Modo concorrente (ver colecoes.go).
//
// Enquanto o programa tem uma goroutine so rodando gambiarra, as colecoes nao
// travam nada: cada operacao paga so uma leitura atomica de bool. Na primeira
// vez que alguem cria outra goroutine que enxerga objetos do usuario (`bora`,
// `paralelo`, servidor, handler de socket...), AtivaConcorrencia liga o modo
// concorrente pro processo inteiro — e dali pra frente TODA operacao em TODA
// colecao trava o mutex dela. Nunca desliga.
//
// Por que global e nao por colecao: a primeira versao marcava so as colecoes
// "publicadas" pra outra goroutine, mas os caminhos de publicacao sao muitos
// e alguns invisiveis (closure Go de builtin segurando lista, freevar de
// handler, global lida por clone da VM...). Esquecer um caminho = data race =
// processo morto de novo. A regra global so tem UM ponto pra auditar:
//
//	TODO `go` (ou servidor/timer do Go que chama gambiarra ou mexe em objeto
//	do usuario) chama AtivaConcorrencia ANTES de criar a goroutine.
//
// O `go` da o happens-before: a goroutine nova ja nasce vendo o modo ligado,
// e a goroutine que ligou ve a propria escrita. Operacao que comecou sem
// trava terminou antes do `go` (nenhuma operacao de colecao cria goroutine).
var concorrencia atomic.Bool

// AtivaConcorrencia liga o modo concorrente. Chame ANTES de criar a goroutine.
func AtivaConcorrencia() {
	if !concorrencia.Load() {
		concorrencia.Store(true)
	}
}

// ConcorrenciaAtiva diz se o processo ja ligou o modo concorrente.
func ConcorrenciaAtiva() bool { return concorrencia.Load() }

// ---------------------------------------------------------------- Trava

// Trava e o lock explicito da linguagem (`trava()` / `com_trava(t, fn)`), pra
// operacao composta tipo `bota d["n"] = d["n"] + 1` virar atomica. NAO e
// reentrante (igual o sync.Mutex do Go): pegar de novo a mesma trava dentro
// do com_trava, na mesma goroutine, daria deadlock eterno — em vez disso
// Segura avisa (reentrou=true) e o builtin vira erro claro. Pra saber "mesma
// goroutine" guardamos o id da dona.
type Trava struct {
	mu   sync.Mutex
	dono atomic.Int64 // id da goroutine que segura (0 = livre)
}

func NovaTrava() *Trava { return &Trava{} }

func (t *Trava) Type() ObjectType { return TRAVA_OBJ }
func (t *Trava) Inspect() string {
	if t.dono.Load() != 0 {
		return "<trava ocupada>"
	}
	return "<trava>"
}

// Dono e o id da goroutine que segura a trava agora (0 = livre).
func (t *Trava) Dono() int64 { return t.dono.Load() }

// Segura roda f com a trava fechada e solta no fim, mesmo se f entrar em
// panico. reentrou=true (e f nao roda) se esta goroutine ja segura a trava.
func (t *Trava) Segura(f func() Object) (res Object, reentrou bool) {
	eu := idGoroutine()
	if t.dono.Load() == eu {
		return nil, true
	}
	t.mu.Lock()
	t.dono.Store(eu)
	defer func() {
		t.dono.Store(0)
		t.mu.Unlock()
	}()
	return f(), false
}

// idGoroutine le o id da goroutine atual do cabecalho do runtime.Stack
// ("goroutine 123 [running]:"). Custa ~1us — so o com_trava usa, pra trocar
// um deadlock silencioso por um erro claro. O Go nunca reusa esse id.
func idGoroutine() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	const prefixo = "goroutine "
	var id int64
	for _, c := range buf[len(prefixo):n] {
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + int64(c-'0')
	}
	return id
}
