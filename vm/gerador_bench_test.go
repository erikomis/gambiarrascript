package vm

import "testing"

// pra_cada sobre lista (o laco mudou pro OpIterProx) e o custo de um valor
// de gerador (retomar a sub-VM) — pra comparar com o laco de lista puro.
const fontePraCadaLista = `bota xs = 1..200000
bota s = 0
pra_cada x em xs
    s += x
acabou_finalmente
mostra s`

const fontePraCadaGerador = `gambiarra ate(n)
    bota i = 1
    enquanto i <= n
        rende i
        i += 1
    acabou_finalmente
acabou_finalmente
bota s = 0
pra_cada x em ate(200000)
    s += x
acabou_finalmente
mostra s`

func BenchmarkPraCadaLista(b *testing.B)   { rodaBench(b, compilaBench(b, fontePraCadaLista)) }
func BenchmarkPraCadaGerador(b *testing.B) { rodaBench(b, compilaBench(b, fontePraCadaGerador)) }
