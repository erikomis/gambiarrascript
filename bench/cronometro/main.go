// Command cronometro roda um comando N vezes e imprime a mediana do tempo de
// parede (em segundos) e um resumo da saida padrao. O bench/roda.sh usa isso
// pra medir cada carga em cada linguagem e conferir que todas imprimem a
// mesma coisa (o checksum do programa).
//
// Uso: cronometro [-n 5] comando [args...]
// Saida (uma linha): "<mediana> <resumo>" — resumo e o sha256 curto do stdout.
// Se alguma rodada falhar, imprime "erro <codigo>" e sai com 1.
package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"
)

func main() {
	n := flag.Int("n", 5, "quantas rodadas")
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 || *n < 1 {
		fmt.Fprintln(os.Stderr, "uso: cronometro [-n 5] comando [args...]")
		os.Exit(2)
	}
	var tempos []float64
	var resumo string
	for i := 0; i < *n; i++ {
		var saida bytes.Buffer
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = &saida
		cmd.Stderr = os.Stderr
		ini := time.Now()
		err := cmd.Run()
		dur := time.Since(ini).Seconds()
		if err != nil {
			fmt.Printf("erro %v\n", err)
			os.Exit(1)
		}
		soma := sha256.Sum256(saida.Bytes())
		r := fmt.Sprintf("%x", soma[:4])
		if resumo != "" && r != resumo {
			fmt.Println("erro saida-mudou-entre-rodadas")
			os.Exit(1)
		}
		resumo = r
		tempos = append(tempos, dur)
	}
	sort.Float64s(tempos)
	fmt.Printf("%.3f %s\n", mediana(tempos), resumo)
}

func mediana(xs []float64) float64 {
	m := len(xs) / 2
	if len(xs)%2 == 1 {
		return xs[m]
	}
	return (xs[m-1] + xs[m]) / 2
}
