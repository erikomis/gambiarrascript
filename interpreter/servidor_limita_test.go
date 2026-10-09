//go:build !js

package interpreter

import (
	"container/list"
	"strings"
	"testing"
	"time"
)

// O balde volta a encher com o tempo e a memoria nao passa de max_chaves
// (sai o mais esquecido).
func TestLimitadorBaldeEDespejo(t *testing.T) {
	agora := time.Unix(1000, 0)
	l := &limitador{porSeg: 1, rajada: 2, maxChave: 2, baldes: map[string]*list.Element{}, ordem: list.New(),
		agora: func() time.Time { return agora }}
	for n, esp := range []bool{true, true, false} {
		if ok, _ := l.pega("a"); ok != esp {
			t.Fatalf("pedido %d de a: %v", n, ok)
		}
	}
	if _, espera := l.pega("a"); espera != 1 {
		t.Errorf("Retry-After: %d", espera)
	}
	agora = agora.Add(1500 * time.Millisecond)
	if ok, _ := l.pega("a"); !ok {
		t.Error("a ficha voltou e nao deixou passar")
	}
	l.pega("b")
	l.pega("c") // despeja o "a" (o mais esquecido)
	if len(l.baldes) != 2 || l.ordem.Len() != 2 {
		t.Fatalf("memoria: %d baldes", len(l.baldes))
	}
	if _, ok := l.baldes["a"]; ok {
		t.Error("o mais antigo devia ter saido")
	}
	if ok, _ := l.pega("a"); !ok {
		t.Error("chave despejada volta de balde cheio")
	}
}

func TestSanitizaNomeArquivo(t *testing.T) {
	casos := map[string]string{
		"../../etc/passwd":    "passwd",
		`C:\Users\x\foto.png`: "foto.png",
		".htaccess":           "htaccess",
		"..":                  "arquivo",
		"":                    "arquivo",
		"a\x00b\nc.txt":       "abc.txt",
		`que<>:"|?*.txt`:      "que.txt",
		"relatório final.pdf": "relatório final.pdf",
	}
	for in, esp := range casos {
		if got := sanitizaNomeArquivo(in); got != esp {
			t.Errorf("%q: veio %q, esperado %q", in, got, esp)
		}
	}
	longo := sanitizaNomeArquivo(strings.Repeat("á", 300) + ".txt")
	if len(longo) > tamMaxNomeArquivo || !strings.HasSuffix(longo, ".txt") {
		t.Errorf("longo: %d bytes %q", len(longo), longo)
	}
}
