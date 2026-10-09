package object

import (
	"strings"
	"sync"
	"testing"
)

func TestConcatenaIgualAoSimples(t *testing.T) {
	var s Object = &Texto{Value: ""}
	esperado := ""
	for i := 0; i < 5000; i++ {
		p := string(rune('a' + i%26))
		s = Concatena(s, &Texto{Value: p})
		esperado += p
		if s.Inspect() != esperado {
			t.Fatalf("volta %d: diverge (len %d vs %d)", i, len(s.Inspect()), len(esperado))
		}
	}
	// operando nao-texto dos dois lados
	if got := Concatena(NumInt(1), &Texto{Value: "a"}).Value; got != "1a" {
		t.Fatalf("1 + a = %q", got)
	}
	if got := Concatena(&Texto{Value: "a"}, NumInt(2)).Value; got != "a2" {
		t.Fatalf("a + 2 = %q", got)
	}
}

// Dois textos esticando do MESMO prefixo nao podem pisar um no outro: o
// segundo tem que copiar, e o primeiro continua intacto.
func TestConcatenaPrefixoCompartilhado(t *testing.T) {
	base := Concatena(&Texto{Value: strings.Repeat("x", 300)}, &Texto{Value: "!"})
	a := Concatena(base, &Texto{Value: "AAA"})
	b := Concatena(base, &Texto{Value: "BBB"})
	c := Concatena(a, &Texto{Value: "CCC"})
	pre := strings.Repeat("x", 300) + "!"
	if base.Value != pre || a.Value != pre+"AAA" || b.Value != pre+"BBB" || c.Value != pre+"AAACCC" {
		t.Fatalf("textos pisados: base=%q... a=%q b=%q c=%q",
			base.Value[295:], a.Value[295:], b.Value[295:], c.Value[295:])
	}
}

// Varios fluxos concatenando no mesmo texto ao mesmo tempo (bora/escuta):
// cada um tem que ver o proprio resultado. Rode com -race.
func TestConcatenaConcorrente(t *testing.T) {
	base := Concatena(&Texto{Value: strings.Repeat("y", 300)}, &Texto{Value: "-"})
	var wg sync.WaitGroup
	erros := make(chan string, 64)
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			marca := string(rune('a' + g))
			s := base
			for i := 0; i < 200; i++ {
				s = Concatena(s, &Texto{Value: marca})
			}
			if s.Value != base.Value+strings.Repeat(marca, 200) {
				erros <- "fluxo " + marca + " viu texto de outro"
			}
		}(g)
	}
	wg.Wait()
	close(erros)
	for e := range erros {
		t.Fatal(e)
	}
}
