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
