package depurador

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gambiarrascript/depurador/daptest"
)

// adapterNoProcesso liga o RodaDAP a um cliente por pipes.
func adapterNoProcesso(t *testing.T) (*daptest.Cliente, chan int) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	fim := make(chan int, 1)
	go func() {
		fim <- RodaDAP(inR, outW)
		outW.Close()
	}()
	t.Cleanup(func() { inW.Close() })
	return daptest.Novo(inW, outR), fim
}

func escreveGs(t *testing.T, nome, fonte string) string {
	t.Helper()
	arq := filepath.Join(t.TempDir(), nome)
	if err := os.WriteFile(arq, []byte(fonte), 0644); err != nil {
		t.Fatal(err)
	}
	return arq
}

// O cenario completo do DAP (o mesmo do teste de ponta a ponta no cmd/gs),
// nos dois engines.
func TestDAPCenario(t *testing.T) {
	for _, eng := range []string{"vm", "tree"} {
		t.Run(eng, func(t *testing.T) {
			arq := escreveGs(t, "cenario.gs", daptest.ProgramaCenario)
			c, fim := adapterNoProcesso(t)
			daptest.Cenario(t, c, arq, eng)
			if codigo := <-fim; codigo != 0 {
				t.Errorf("adapter saiu com %d", codigo)
			}
		})
	}
}

// Breakpoint condicional (so para quando a condicao vale, avaliada no
// quadro parado) e logpoint (escreve e nao para).
func TestDAPCondicionalELogpoint(t *testing.T) {
	src := `gambiarra dobra(n)
    funciona n * 2
acabou_finalmente
bota soma = 0
pra_cada x em [1, 2, 3, 4]
    bota soma = soma + dobra(x)
acabou_finalmente
mostra soma
`
	for _, eng := range []string{"vm", "tree"} {
		t.Run(eng, func(t *testing.T) {
			arq := escreveGs(t, "cond.gs", src)
			c, _ := adapterNoProcesso(t)
			pedeOk(t, c, "initialize", nil)
			pedeOk(t, c, "launch", map[string]any{"program": arq, "engine": eng})
			pedeOk(t, c, "setBreakpoints", map[string]any{
				"source": map[string]any{"path": arq},
				"breakpoints": []any{
					map[string]any{"line": 6, "condition": "x == 3"},
					map[string]any{"line": 2, "logMessage": "dobrando {n}"},
				},
			})
			pedeOk(t, c, "configurationDone", nil)
			st, err := c.Espera("stopped", nil)
			if err != nil {
				t.Fatal(err)
			}
			if st.Corpo()["reason"] != "breakpoint" {
				t.Fatalf("parada: %v", st.Corpo())
			}
			ev := pedeOk(t, c, "evaluate", map[string]any{"expression": "x", "context": "repl"})
			if ev.Corpo()["result"] != "3" {
				t.Errorf("condicao parou com x = %v", ev.Corpo()["result"])
			}
			pedeOk(t, c, "continue", map[string]any{"threadId": 1})
			if _, err := c.Espera("terminated", nil); err != nil {
				t.Fatal(err)
			}
			if got := c.Saida("console"); got != "dobrando 1\ndobrando 2\ndobrando 3\ndobrando 4\n" {
				t.Errorf("logpoint: %q", got)
			}
			if got := c.Saida("stdout"); got != "20\n" {
				t.Errorf("stdout: %q", got)
			}
			pedeOk(t, c, "disconnect", nil)
		})
	}
}

// pause num laco infinito para o fluxo onde ele estiver; terminate encerra.
func TestDAPPause(t *testing.T) {
	src := `bota i = 0
enquanto i >= 0
    bota i = i + 1
acabou_finalmente
`
	for _, eng := range []string{"vm", "tree"} {
		t.Run(eng, func(t *testing.T) {
			arq := escreveGs(t, "laco.gs", src)
			c, fim := adapterNoProcesso(t)
			pedeOk(t, c, "initialize", nil)
			pedeOk(t, c, "launch", map[string]any{"program": arq, "engine": eng})
			pedeOk(t, c, "configurationDone", nil)
			// a 1a pausa pode pegar o programa antes do laco (linha 1); a 2a
			// com certeza pega dentro dele
			for volta := 0; volta < 2; volta++ {
				if volta > 0 {
					pedeOk(t, c, "continue", map[string]any{"threadId": 1})
				}
				pedeOk(t, c, "pause", map[string]any{"threadId": 1})
				st, err := c.Espera("stopped", nil)
				if err != nil {
					t.Fatal(err)
				}
				if st.Corpo()["reason"] != "pause" {
					t.Fatalf("parada: %v", st.Corpo())
				}
			}
			ev := pedeOk(t, c, "evaluate", map[string]any{"expression": "i >= 0", "context": "repl"})
			if ev.Corpo()["result"] != "deu_bom" {
				t.Errorf("i > 0 = %v", ev.Corpo()["result"])
			}
			pedeOk(t, c, "terminate", nil)
			if _, err := c.Espera("terminated", nil); err != nil {
				t.Fatal(err)
			}
			<-fim
		})
	}
}

// Erro de parse aparece na resposta do launch.
func TestDAPLaunchComErro(t *testing.T) {
	arq := escreveGs(t, "ruim.gs", "bota = 5\n")
	c, _ := adapterNoProcesso(t)
	pedeOk(t, c, "initialize", nil)
	r, err := c.Pede("launch", map[string]any{"program": arq})
	if err != nil {
		t.Fatal(err)
	}
	if r["success"] != false || !strings.Contains(r["message"].(string), "perrengue") {
		t.Errorf("launch: %v", r)
	}
	pedeOk(t, c, "disconnect", nil)
}

func pedeOk(t *testing.T, c *daptest.Cliente, cmd string, args any) daptest.Msg {
	t.Helper()
	m, err := c.Pede(cmd, args)
	if err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(c.Log, "\n"))
	}
	if m["success"] != true {
		t.Fatalf("%s falhou: %v", cmd, m["message"])
	}
	return m
}
