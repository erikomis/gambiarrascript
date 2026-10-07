package repl

import (
	"strings"
	"testing"

	"gambiarrascript/vm"
)

func TestPalavraAntes(t *testing.T) {
	p, i := palavraAntes("mostra tam", 10)
	if p != "tam" || i != 7 {
		t.Fatalf("palavraAntes: got %q %d", p, i)
	}
	// pos no meio de espaco: palavra vazia
	p2, _ := palavraAntes("mostra ", 7)
	if p2 != "" {
		t.Fatalf("esperava vazio, got %q", p2)
	}
}

func TestAutocompletaUnico(t *testing.T) {
	nova, pos, ok := autocompleta("mostra tama", 11, []string{"tamanho", "tem"})
	if !ok || nova != "mostra tamanho" || pos != 14 {
		t.Fatalf("autocompleta unico: %q %d %v", nova, pos, ok)
	}
}

func TestAutocompletaPrefixoComum(t *testing.T) {
	// "ca" -> "cano"/"canoa" -> prefixo comum "cano" (avanca ate ali)
	nova, _, ok := autocompleta("ca", 2, []string{"cano", "canoa"})
	if !ok || nova != "cano" {
		t.Fatalf("autocompleta prefixo comum: %q %v", nova, ok)
	}
}

func TestAutocompletaSemMatch(t *testing.T) {
	if _, _, ok := autocompleta("xyz", 3, []string{"tamanho"}); ok {
		t.Fatalf("nao devia completar sem match")
	}
}

// Os temporarios que o compilador cria (`__it_gs0`, `__cont_gs0`,
// `__erro_gsN`...) moram no escopo global da sessao, mas nao sao do usuario:
// nao podem aparecer no TAB.
func TestAutocompleteEscondeTemporarios(t *testing.T) {
	var out strings.Builder
	s := vm.NovaSessao(&out)
	fonte := "bota meu_valor = 1\n" +
		"pra_cada i em [1, 2]\n    mostra i\nacabou_finalmente\n" +
		"pra_cada k, v em {\"a\": 1}\n    mostra k\nacabou_finalmente\n" +
		"enquanto meu_valor < 3\n    meu_valor += 1\nacabou_finalmente\n" +
		"arruma\n    quebra(\"x\")\nquebrou err\n    mostra 1\nfinalmente\n    mostra 2\nacabou_finalmente\n" +
		"escolhe meu_valor\ncaso 3\n    mostra 3\nacabou_finalmente\n"
	avalia(s, fonte, &out)
	if strings.Contains(out.String(), "deu ruim") {
		t.Fatalf("fonte do teste quebrou: %s", out.String())
	}
	nomes := nomesCompletaveis(s)
	achouMeu := false
	for _, n := range nomes {
		if strings.HasPrefix(n, "__") {
			t.Errorf("temporario %q apareceu no autocomplete", n)
		}
		if n == "meu_valor" {
			achouMeu = true
		}
	}
	if !achouMeu {
		t.Errorf("a variavel do usuario sumiu do autocomplete: %v", nomes)
	}
	if _, _, ok := autocompleta("__", 2, nomes); ok {
		t.Errorf("TAB depois de __ completou temporario")
	}
}

func TestTrataComandoAjudaLimpa(t *testing.T) {
	var b strings.Builder
	if !trataComando(":ajuda", &b) {
		t.Fatalf(":ajuda devia ser tratado")
	}
	if !trataComando(":limpa", &b) {
		t.Fatalf(":limpa devia ser tratado")
	}
	if !trataComando(":naoexiste", &b) {
		t.Fatalf(": prefix sempre e tratado (msg de desconhecido)")
	}
}
