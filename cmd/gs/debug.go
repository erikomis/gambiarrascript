package main

import (
	"fmt"
	"os"

	"gambiarrascript/depurador"
)

const usoDebug = `uso: gs debug [--tree] <arquivo.gs> [argumentos...]   # depurador no terminal
     gs debug --dap                                    # Debug Adapter Protocol no stdio (VSCode)
  o programa comeca parado na entrada; digita ` + "`ajuda`" + ` pra ver os comandos.`

// cmdDebug: `gs debug` (terminal interativo) e `gs debug --dap` (adapter do
// VSCode). Depois do arquivo, tudo e argumento do script, igual o gs roda.
func cmdDebug(args []string) {
	motor := "vm"
	dap := false
	arquivo := ""
	var scriptArgs []string
	for _, a := range args {
		if arquivo != "" {
			scriptArgs = append(scriptArgs, a)
			continue
		}
		switch a {
		case "--tree":
			motor = "tree"
		case "--vm":
			motor = "vm"
		case "--dap":
			dap = true
		case "-h", "--help":
			fmt.Println(usoDebug)
			os.Exit(0)
		default:
			arquivo = a
		}
	}
	if dap {
		os.Exit(depurador.RodaDAP(os.Stdin, os.Stdout))
	}
	if arquivo == "" {
		fmt.Println(usoDebug)
		os.Exit(1)
	}
	cfg := depurador.Config{Arquivo: arquivo, Args: scriptArgs, Motor: motor}
	os.Exit(depurador.RodaCLI(cfg, os.Stdin, os.Stdout, depurador.OpcoesCLI{CtrlCPausa: true}))
}
