package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gambiarrascript/depurador/daptest"
)

// TestMain deixa o binario de teste virar o `gs`: com GS_TESTE_MAIN=1 ele
// roda o main() com os argumentos depois do "--". Assim o teste de ponta a
// ponta sobe um `gs debug --dap` de verdade (processo separado, stdio).
func TestMain(m *testing.M) {
	if os.Getenv("GS_TESTE_MAIN") == "1" {
		for k, a := range os.Args {
			if a == "--" {
				os.Args = append([]string{"gs"}, os.Args[k+1:]...)
				break
			}
		}
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// gsDeVerdade prepara o comando `gs <args>` (o proprio binario de teste).
func gsDeVerdade(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^$", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "GS_TESTE_MAIN=1")
	return cmd
}

// Ponta a ponta: `gs debug --dap` num processo, falando DAP por pipes, com o
// cenario completo (stopOnEntry, breakpoints, pilha, variaveis, passos,
// evaluate, breakpoint dentro de bora) nos dois engines.
func TestDebugDAPPontaAPonta(t *testing.T) {
	for _, eng := range []string{"vm", "tree"} {
		t.Run(eng, func(t *testing.T) {
			arq := filepath.Join(t.TempDir(), "cenario.gs")
			if err := os.WriteFile(arq, []byte(daptest.ProgramaCenario), 0644); err != nil {
				t.Fatal(err)
			}
			cmd := gsDeVerdade(t, "debug", "--dap")
			in, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr strings.Builder
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			// nunca deixa processo pra tras
			defer func() {
				in.Close()
				fim := make(chan error, 1)
				go func() { fim <- cmd.Wait() }()
				select {
				case <-fim:
				case <-time.After(10 * time.Second):
					cmd.Process.Kill()
					<-fim
					t.Errorf("gs debug --dap nao saiu depois do disconnect")
				}
				if stderr.Len() > 0 {
					t.Logf("stderr do adapter: %s", stderr.String())
				}
			}()
			c := daptest.Novo(in, out)
			daptest.Cenario(t, c, arq, eng)
		})
	}
}

// O terminal de verdade: `gs debug` com stdin roteirizado.
func TestDebugCLIProcesso(t *testing.T) {
	arq := filepath.Join(t.TempDir(), "x.gs")
	os.WriteFile(arq, []byte("bota a = 1\nbota b = a + 1\nmostra b\n"), 0644)
	cmd := gsDeVerdade(t, "debug", "--tree", arq)
	cmd.Stdin = strings.NewReader("proximo\nve a * 10\nsegue\n")
	saida, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, saida)
	}
	quer := "gs debug: x.gs (engine tree-walker). `ajuda` lista os comandos.\n" +
		"parado na entrada: x.gs:1 (em <principal>)\n   1 | bota a = 1\n" +
		"(gs) x.gs:2 (em <principal>)\n   2 | bota b = a + 1\n" +
		"(gs) 10\n(gs) 2\nprograma terminou (codigo 0)\n"
	if string(saida) != quer {
		t.Errorf("saida:\n%s\nqueria:\n%s", saida, quer)
	}
}
