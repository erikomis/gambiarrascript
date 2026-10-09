package main

import (
	"fmt"
	"os"
	"runtime/pprof"
)

// iniciaPprof liga o perfil de CPU do `gs roda` quando a variavel escondida
// GS_PPROF aponta um arquivo (GS_PPROF=cpu.out gs roda x.gs, depois
// `go tool pprof cpu.out`). Serve pra cacar gargalo do motor sem montar
// benchmark: o servidor do `escuta` tambem, porque o ctrl+c desliga ele com
// calma e o roda termina normal. Saida por erro/`sai` (os.Exit) perde o perfil.
// Devolve a funcao que fecha o perfil.
func iniciaPprof() func() {
	caminho := os.Getenv("GS_PPROF")
	if caminho == "" {
		return func() {}
	}
	f, err := os.Create(caminho)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GS_PPROF: %v\n", err)
		return func() {}
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Fprintf(os.Stderr, "GS_PPROF: %v\n", err)
		f.Close()
		return func() {}
	}
	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}
}
