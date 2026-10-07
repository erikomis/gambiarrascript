package object

import (
	"strconv"
	"sync"
	"testing"
)

// Com o modo concorrente ligado, varias goroutines mexendo na mesma colecao
// nao podem perder escrita nem derrubar o processo (rode com -race).
func TestColecoesConcorrentes(t *testing.T) {
	AtivaConcorrencia()
	l := NovaLista(nil)
	d := NovoDicionario()
	c := NovoConjunto()
	const gs, n = 8, 2000
	var wg sync.WaitGroup
	for g := 0; g < gs; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < n; i++ {
				v := NumInt(int64(i))
				l.Adiciona(v)
				k := &Texto{Value: "k" + strconv.Itoa(i%50)}
				d.Bota(k.ChaveHash(), ParDic{Chave: k, Valor: v})
				c.Adiciona(NumInt(int64(i % 30)))
				if i%3 == 0 {
					c.Remove(NumInt(int64(i % 30)))
					l.Poe(-1, v)
				}
				if i%200 == 0 {
					_ = l.Inspect()
					_ = d.Inspect()
					_ = c.Inspect()
					_ = d.Pares()
					l.Ordena(func(a, b Object) bool { return a.(*Numero).Int < b.(*Numero).Int })
					_ = FatiaLista(l, NumInt(0), NumInt(10))
				}
			}
		}(g)
	}
	wg.Wait()
	if got := l.Tamanho(); got != gs*n {
		t.Fatalf("lista perdeu item: %d, queria %d", got, gs*n)
	}
	if got := d.Tamanho(); got != 50 || len(d.Chaves()) != 50 {
		t.Fatalf("dicionario: %d pares, %d chaves", got, len(d.Chaves()))
	}
	if got := c.Tamanho(); got > 30 {
		t.Fatalf("conjunto com %d itens", got)
	}
}

func TestTravaSeguraEReentrada(t *testing.T) {
	tr := NovaTrava()
	res, reentrou := tr.Segura(func() Object {
		_, dentro := tr.Segura(func() Object { return nil })
		return &Booleano{Value: dentro}
	})
	if reentrou || !res.(*Booleano).Value {
		t.Fatalf("reentrada nao detectada: %v %v", res, reentrou)
	}
	// panico dentro solta a trava
	func() {
		defer func() { recover() }()
		tr.Segura(func() Object { panic("ops") })
	}()
	if tr.Inspect() != "<trava>" {
		t.Fatalf("trava ficou presa: %s", tr.Inspect())
	}
	n := 0
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				tr.Segura(func() Object { n++; return nil })
			}
		}()
	}
	wg.Wait()
	if n != 4000 {
		t.Fatalf("contador %d, queria 4000", n)
	}
}

// ordenado (miolo do dicionario/conjunto): depois de muito bota/tira, com os
// buracos e as compactacoes no meio, a ordem tem que ser exatamente a de um
// modelo ingenuo (slice de chaves, tira arrastando o resto).
func TestOrdenadoMantemOrdemComBuracos(t *testing.T) {
	o := novoOrdenado[int]()
	var modelo []string
	valores := map[string]int{}
	chave := func(s string) HashKey { return HashKey{Tipo: TEXTO_OBJ, Valor: s} }
	for passo := 0; passo < 5000; passo++ {
		k := "k" + strconv.Itoa((passo*7919)%97)
		if passo%3 == 0 {
			if o.tira(chave(k)) {
				for i, m := range modelo {
					if m == k {
						modelo = append(modelo[:i], modelo[i+1:]...)
						break
					}
				}
				delete(valores, k)
			}
			continue
		}
		if _, ja := valores[k]; !ja {
			modelo = append(modelo, k)
		}
		valores[k] = passo
		o.bota(chave(k), passo)
	}
	var got []string
	o.cada(func(k HashKey, v int) {
		got = append(got, k.Valor)
		if valores[k.Valor] != v {
			t.Errorf("%s = %d, queria %d", k.Valor, v, valores[k.Valor])
		}
	})
	if len(got) != len(modelo) || o.tamanho() != len(modelo) {
		t.Fatalf("tamanho %d/%d, modelo %d", len(got), o.tamanho(), len(modelo))
	}
	for i := range got {
		if got[i] != modelo[i] {
			t.Fatalf("ordem diverge na posicao %d: %v vs %v", i, got, modelo)
		}
	}
	if len(o.slots) > 2*len(modelo)+17 {
		t.Fatalf("buracos nao foram compactados: %d slots pra %d vivos", len(o.slots), len(modelo))
	}
}

// Inspect de estrutura que contem ela mesma nao pode descer pra sempre
func TestInspectComCiclo(t *testing.T) {
	xs := NovaLista(nil)
	xs.Adiciona(xs)
	if got := xs.Inspect(); got != "[[...]]" {
		t.Fatalf("lista: %q", got)
	}
	d := NovoDicionario()
	k := &Texto{Value: "eu"}
	d.Bota(k.ChaveHash(), ParDic{Chave: k, Valor: d})
	if got := d.Inspect(); got != `{"eu": {...}}` {
		t.Fatalf("dicionario: %q", got)
	}
}
