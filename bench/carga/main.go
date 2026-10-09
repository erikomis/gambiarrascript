// Command carga e um gerador de carga HTTP minimo (no espirito do hey/wrk)
// pro bench/http.sh: C conexoes keep-alive martelando um endpoint por D
// segundos, e no fim req/s e latencia p50/p99.
//
// Uso: carga -url http://127.0.0.1:8080 -modo ping|item|cria [-c 50] [-d 10s]
//
//	ping: GET /ping
//	item: GET /itens/<1..1000 sorteado>
//	cria: POST /itens com {"nome":"x","preco":3}
//
// Saida (uma linha): "<req/s> <p50 ms> <p99 ms> <falhas>". Resposta fora de
// 2xx ou erro de rede conta como falha (e nao entra na latencia).
package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func main() {
	base := flag.String("url", "http://127.0.0.1:8080", "endereco do servidor")
	modo := flag.String("modo", "ping", "ping, item ou cria")
	conc := flag.Int("c", 50, "conexoes simultaneas")
	dur := flag.Duration("d", 10*time.Second, "duracao")
	espera := flag.Duration("espera", 15*time.Second, "quanto esperar o servidor subir")
	flag.Parse()

	tr := &http.Transport{MaxIdleConns: *conc, MaxIdleConnsPerHost: *conc, MaxConnsPerHost: *conc}
	cli := &http.Client{Transport: tr, Timeout: 10 * time.Second}

	if !esperaSubir(cli, *base, *espera) {
		fmt.Println("erro servidor-nao-subiu")
		os.Exit(1)
	}

	var (
		mu      sync.Mutex
		lat     []time.Duration
		falhas  int
		wg      sync.WaitGroup
		fimEm   = time.Now().Add(*dur)
		inicio  = time.Now()
		semente = time.Now().UnixNano()
	)
	for w := 0; w < *conc; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(semente + int64(w)))
			var minhas []time.Duration
			minhasFalhas := 0
			for time.Now().Before(fimEm) {
				req := monta(*base, *modo, r)
				t0 := time.Now()
				resp, err := cli.Do(req)
				if err != nil {
					minhasFalhas++
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode/100 != 2 {
					minhasFalhas++
					continue
				}
				minhas = append(minhas, time.Since(t0))
			}
			mu.Lock()
			lat = append(lat, minhas...)
			falhas += minhasFalhas
			mu.Unlock()
		}(w)
	}
	wg.Wait()
	total := time.Since(inicio).Seconds()
	if len(lat) == 0 {
		fmt.Printf("erro nenhuma-resposta-ok falhas=%d\n", falhas)
		os.Exit(1)
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	fmt.Printf("%.0f %.2f %.2f %d\n", float64(len(lat))/total, ms(pct(lat, 0.50)), ms(pct(lat, 0.99)), falhas)
}

func monta(base, modo string, r *rand.Rand) *http.Request {
	switch modo {
	case "item":
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/itens/%d", base, 1+r.Intn(1000)), nil)
		return req
	case "cria":
		req, _ := http.NewRequest("POST", base+"/itens", strings.NewReader(`{"nome":"x","preco":3}`))
		req.Header.Set("Content-Type", "application/json")
		return req
	}
	req, _ := http.NewRequest("GET", base+"/ping", nil)
	return req
}

func esperaSubir(cli *http.Client, base string, ate time.Duration) bool {
	limite := time.Now().Add(ate)
	for time.Now().Before(limite) {
		if resp, err := cli.Get(base + "/ping"); err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func pct(xs []time.Duration, p float64) time.Duration {
	i := int(float64(len(xs)-1) * p)
	return xs[i]
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
