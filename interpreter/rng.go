package interpreter

import (
	"math/rand"
	"sync"
	"time"
)

// Gerador aleatorio compartilhado por aleatorio/semente/embaralha/escolhe_um/
// uuid. Protegido por mutex porque builtins podem rodar em goroutines (bora,
// paralelo, handlers do rota) e *rand.Rand nao e seguro pra uso concorrente.
// `semente` troca a fonte pra deixar a sequencia reprodutivel.
var (
	rngMu sync.Mutex
	rng   = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func rngSemente(n int64) {
	rngMu.Lock()
	rng = rand.New(rand.NewSource(n))
	rngMu.Unlock()
}

func rngFloat() float64 {
	rngMu.Lock()
	defer rngMu.Unlock()
	return rng.Float64()
}

func rngIntn(n int) int {
	rngMu.Lock()
	defer rngMu.Unlock()
	return rng.Intn(n)
}

func rngRead(b []byte) {
	rngMu.Lock()
	defer rngMu.Unlock()
	rng.Read(b)
}
