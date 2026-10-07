package repl

import (
	"bytes"
	"strings"
	"testing"
)

func TestStartAvaliaLinhas(t *testing.T) {
	entrada := strings.NewReader("bota x = 21\nmostra x * 2\n")
	var out bytes.Buffer
	Start(entrada, &out)
	if !strings.Contains(out.String(), "42") {
		t.Fatalf("REPL nao avaliou as linhas mantendo estado; saida: %q", out.String())
	}
}

func TestStartMultiline(t *testing.T) {
	// bloco aberto atravessa linhas: so avalia no acabou_finalmente
	entrada := strings.NewReader("gambiarra dobra(n)\nfunciona n * 2\nacabou_finalmente\ndobra(21)\n")
	var out bytes.Buffer
	Start(entrada, &out)
	if !strings.Contains(out.String(), "=> 42") {
		t.Fatalf("REPL multiline nao funcionou; saida: %q", out.String())
	}
	if !strings.Contains(out.String(), promptCont) {
		t.Fatalf("cade o prompt de continuacao? saida: %q", out.String())
	}
}

func TestStartMultilineTreta(t *testing.T) {
	// treta/combinado abrem bloco; metodo e instancia sobrevivem entre entradas
	entrada := strings.NewReader("treta P\nx = 20\nacabou_finalmente\n" +
		"combinado C\nm()\nacabou_finalmente\n" +
		"gambiarra (p P) m()\nfunciona p.x + 1\nacabou_finalmente\n" +
		"bota p = P{}\np.m() * 2\nsatisfaz(p, C)\n")
	var out bytes.Buffer
	Start(entrada, &out)
	if !strings.Contains(out.String(), "=> 42") || !strings.Contains(out.String(), "=> deu_bom") {
		t.Fatalf("treta no REPL nao funcionou; saida: %q", out.String())
	}
}

func TestStartMultilineElif(t *testing.T) {
	// `se_nao_colar se_colar` (elif) compartilha o MESMO acabou_finalmente —
	// o contador de blocos nao pode contar o se_colar do elif como abertura.
	entrada := strings.NewReader("bota x = 5\nse_colar x > 10\nmostra \"grande\"\nse_nao_colar se_colar x > 3\nmostra \"medio\"\nacabou_finalmente\n")
	var out bytes.Buffer
	Start(entrada, &out)
	if !strings.Contains(out.String(), "medio") {
		t.Fatalf("elif multiline nao avaliou; saida: %q", out.String())
	}
}

// TestReplRodaNaVM: o REPL passou a rodar na VM (era o ultimo caminho no
// tree-walker). O que importa e o estado sobreviver entre entradas — global,
// gambiarra e mutacao.
func TestReplRodaNaVM(t *testing.T) {
	entrada := strings.NewReader(
		"bota x = 10\n" +
			"gambiarra dobra(n)\n    funciona n * 2\nacabou_finalmente\n" +
			"dobra(x)\n" +
			"bota x = x + 1\n" +
			"x\n")
	var out bytes.Buffer
	Start(entrada, &out)
	got := out.String()
	for _, quer := range []string{"=> 20", "=> 11"} {
		if !strings.Contains(got, quer) {
			t.Errorf("saida nao tem %q:\n%s", quer, got)
		}
	}
}

// TestReplMostraNaoDuplica: `mostra` e COMANDO, nao expressao. No tree-walker o
// REPL imprimia o valor mostrado uma segunda vez como `=> valor` — `mostra "oi"`
// saia como "oi" seguido de "=> oi".
func TestReplMostraNaoDuplica(t *testing.T) {
	entrada := strings.NewReader("mostra \"oi\"\n")
	var out bytes.Buffer
	Start(entrada, &out)
	if strings.Contains(out.String(), "=> oi") {
		t.Errorf("mostra virou expressao de novo (saida duplicada):\n%s", out.String())
	}
	if !strings.Contains(out.String(), "oi") {
		t.Errorf("mostra nao imprimiu:\n%s", out.String())
	}
}

// TestReplSobreviveAErro: erro numa linha nao pode derrubar a sessao nem levar
// junto o que ja tinha sido definido.
func TestReplSobreviveAErro(t *testing.T) {
	entrada := strings.NewReader("bota x = 3\nnaoexiste\nx / 0\nx + 1\n")
	var out bytes.Buffer
	Start(entrada, &out)
	got := out.String()
	if !strings.Contains(got, "nao existe nenhum `naoexiste`") {
		t.Errorf("faltou o erro de nome desconhecido:\n%s", got)
	}
	if !strings.Contains(got, "dividir por zero") {
		t.Errorf("faltou o erro de divisao por zero:\n%s", got)
	}
	if !strings.Contains(got, "=> 4") {
		t.Errorf("sessao nao sobreviveu aos erros:\n%s", got)
	}
}
