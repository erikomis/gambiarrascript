package object

import (
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func nsVazio() (Object, any, Object) { return NovoDicionario(), nil, nil }

// Muitas goroutines importando o mesmo modulo: o corpo roda uma vez so e
// todas recebem o MESMO namespace.
func TestModulosRodaUmaVezConcorrente(t *testing.T) {
	var m Modulos
	var rodou int32
	ns := NovoDicionario()
	var wg sync.WaitGroup
	resultados := make([]Object, 50)
	for i := range resultados {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, _, falha := m.Importa("/p/main.gs", "/p/x.gs", func() (Object, any, Object) {
				atomic.AddInt32(&rodou, 1)
				time.Sleep(20 * time.Millisecond)
				return ns, nil, nil
			})
			if falha != nil {
				t.Errorf("falha inesperada: %v", falha.Inspect())
			}
			resultados[i] = v
		}(i)
	}
	wg.Wait()
	if rodou != 1 {
		t.Fatalf("modulo rodou %d vezes, esperava 1", rodou)
	}
	for _, r := range resultados {
		if r != ns {
			t.Fatal("goroutine recebeu namespace diferente")
		}
	}
}

// a -> b -> a na mesma goroutine: erro com a cadeia, sem travar.
func TestModulosCircular(t *testing.T) {
	var m Modulos
	_, _, falha := m.Importa("/p/a.gs", "/p/b.gs", func() (Object, any, Object) {
		_, _, f := m.Importa("/p/b.gs", "/p/a.gs", nsVazio)
		return nil, nil, f
	})
	e, ok := falha.(*Erro)
	if !ok || e.Message != "importa circular: a.gs -> b.gs -> a.gs" || e.Kind != KindModulo {
		t.Fatalf("esperava erro de ciclo, veio %#v", falha)
	}
}

// Ciclo entre goroutines: uma roda a e quer b, a outra roda b e quer a. Sem
// o grafo de espera as duas ficariam esperando pra sempre.
func TestModulosCircularEntreGoroutines(t *testing.T) {
	var m Modulos
	comecouA, comecouB := make(chan struct{}), make(chan struct{})
	falhas := make(chan Object, 2)
	go func() {
		_, _, f := m.Importa("/p/main.gs", "/p/a.gs", func() (Object, any, Object) {
			close(comecouA)
			<-comecouB
			_, _, f := m.Importa("/p/a.gs", "/p/b.gs", nsVazio)
			return nil, nil, f
		})
		falhas <- f
	}()
	go func() {
		_, _, f := m.Importa("/p/main.gs", "/p/b.gs", func() (Object, any, Object) {
			close(comecouB)
			<-comecouA
			_, _, f := m.Importa("/p/b.gs", "/p/a.gs", nsVazio)
			return nil, nil, f
		})
		falhas <- f
	}()
	for i := 0; i < 2; i++ {
		select {
		case f := <-falhas:
			e, ok := f.(*Erro)
			if !ok || !strings.Contains(e.Message, "importa circular:") {
				t.Fatalf("esperava erro de ciclo, veio %#v", f)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("travou: ciclo entre goroutines nao foi detectado")
		}
	}
}

// Modulo que falhou sai do cache: o proximo importa tenta de novo. E um
// importa do modulo que ja terminou nao e ciclo (cadeia velha nao conta).
func TestModulosFalhaNaoFicaNoCache(t *testing.T) {
	var m Modulos
	vezes := 0
	roda := func() (Object, any, Object) {
		vezes++
		if vezes == 1 {
			return nil, nil, &Erro{Message: "deu ruim na linha 1: explodiu", Line: 1, Kind: "runtime"}
		}
		return NovoDicionario(), nil, nil
	}
	_, _, f := m.Importa("/p/main.gs", "/p/x.gs", roda)
	if e, ok := f.(*Erro); !ok || e.Message != `deu ruim na linha 1: explodiu (no modulo "x.gs")` {
		t.Fatalf("primeira falha errada: %#v", f)
	}
	if _, _, f := m.Importa("/p/main.gs", "/p/x.gs", roda); f != nil || vezes != 2 {
		t.Fatalf("segunda tentativa: falha=%v vezes=%d", f, vezes)
	}
	if _, _, f := m.Importa("/p/x.gs", "/p/x.gs", roda); f != nil || vezes != 2 {
		t.Fatalf("modulo pronto importando a si mesmo depois: falha=%v vezes=%d", f, vezes)
	}
}

func TestCandidatosModulo(t *testing.T) {
	dir := filepath.FromSlash("/proj/sub")
	got := CandidatosModulo(dir, "x.gs")
	want := []string{
		filepath.FromSlash("/proj/sub/x.gs"),
		filepath.FromSlash("/proj/sub/gs_modulos/x.gs"),
		filepath.FromSlash("/proj/gs_modulos/x.gs"),
		filepath.FromSlash("/gs_modulos/x.gs"),
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("candidatos:\n  veio %q\n  quer %q", got, want)
	}
	if got := CandidatosModulo(dir, filepath.FromSlash("/abs/y.gs")); len(got) != 1 {
		t.Fatalf("absoluto deveria ser usado direto: %q", got)
	}
}
