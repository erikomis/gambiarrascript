package vm

import (
	"fmt"
	"io"
	"net/http/httptest"
	"sync"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Colecoes da linguagem mexidas por varias goroutines ao mesmo tempo. Antes
// isso matava o processo com o "concurrent map writes" do Go (fatal, nem o
// recover pega). Rode tambem com -race.

// sobeNosDoisMotores roda a fonte (que so define rotas/globais) no motor
// pedido e devolve o interpretador que hospeda o servidor.
func sobeNosDoisMotores(t *testing.T, motor, fonte string) *interpreter.Interpreter {
	t.Helper()
	p := parser.New(lexer.New(fonte))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	interp := interpreter.New(io.Discard)
	if motor == "arvore" {
		if res := interp.Eval(prog, object.NewEnvironment()); res != nil {
			if e, ok := res.(*object.Erro); ok {
				t.Fatalf("tree-walker: %s", e.Message)
			}
		}
		return interp
	}
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := NovaComInterp(comp.Bytecode(), io.Discard, interp).Run(); err != nil {
		t.Fatalf("vm: %v", err)
	}
	return interp
}

// rodaNosDoisMotores roda a fonte nos dois motores e devolve a saida de cada.
func rodaNosDoisMotores(t *testing.T, fonte string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_, saidaTW, errTW := rodaTWComp(t, fonte)
	if errTW != "" {
		t.Fatalf("tree-walker: %s", errTW)
	}
	out["arvore"] = saidaTW
	_, saidaVM, errVM := rodaVMComp(t, fonte)
	if errVM != "" {
		t.Fatalf("vm: %s", errVM)
	}
	out["vm"] = saidaVM
	return out
}

func TestColecoesConcorrentesComBora(t *testing.T) {
	fonte := `
bota d = {}
bota l = []
bota c = conjunto([])
bota aninhado = {"lista": []}
gambiarra martela(id)
    pra_cada i de 1 ate 1500
        bota d["k" + (i % 97)] = id
        adiciona(l, i)
        adiciona(aninhado["lista"], id)
        adiciona_conjunto(c, i % 50)
        se_colar i % 3 == 0
            remove_conjunto(c, i % 50)
        acabou_finalmente
        se_colar i % 100 == 0
            pra_cada k, v em d
                bota _x = v
            acabou_finalmente
            bota _t = pra_json(aninhado)
            bota _s = texto(l)
            bota _o = ordena(l[0:50])
            bota _m = mapeia(l[0:20], gambiarra(x) funciona x * 2 acabou_finalmente)
            bota _r = contem_conjunto(c, 3)
        acabou_finalmente
    acabou_finalmente
    funciona id
acabou_finalmente
bota fs = []
pra_cada j de 1 ate 8
    adiciona(fs, bora martela(j))
acabou_finalmente
espera(fs)
mostra tamanho(l)
mostra tamanho(aninhado["lista"])
mostra tamanho(d)
mostra contem_conjunto(c, 999)
`
	for motor, saida := range rodaNosDoisMotores(t, fonte) {
		if saida != "12000\n12000\n97\ndeu_ruim\n" {
			t.Fatalf("%s: adiciona perdeu item ou quebrou: %q", motor, saida)
		}
	}
}

// Iterar enquanto outra goroutine adiciona/remove: o laco anda num retrato e
// nunca estoura indice nem repete/pula por causa da outra goroutine.
func TestIteraEnquantoOutraMexe(t *testing.T) {
	fonte := `
bota l = []
bota d = {}
pra_cada i de 1 ate 200
    adiciona(l, i)
    bota d["k" + i] = i
acabou_finalmente
gambiarra mexe()
    pra_cada i de 1 ate 3000
        adiciona(l, i)
        remove(l, i)
        bota d["x" + (i % 10)] = i
    acabou_finalmente
    funciona 1
acabou_finalmente
gambiarra le()
    bota total = 0
    pra_cada volta de 1 ate 30
        pra_cada x em l
            bota total = total + 1
        acabou_finalmente
        pra_cada k em d
            bota total = total + 1
        acabou_finalmente
        bota _f = filtra(l, gambiarra(x) funciona x > 100 acabou_finalmente)
    acabou_finalmente
    funciona total > 0
acabou_finalmente
bota fs = [bora mexe(), bora mexe(), bora le(), bora le()]
mostra espera(fs)
mostra tamanho(l)
`
	for motor, saida := range rodaNosDoisMotores(t, fonte) {
		if saida != "[1, 1, deu_bom, deu_bom]\n200\n" {
			t.Fatalf("%s: %q", motor, saida)
		}
	}
}

func TestComTravaContadorExato(t *testing.T) {
	fonte := `
bota t = trava()
bota c = {"n": 0}
bota global_n = 0
gambiarra soma_muito(k)
    pra_cada i de 1 ate k
        com_trava(t, gambiarra()
            bota c["n"] = c["n"] + 1
        acabou_finalmente)
    acabou_finalmente
    funciona k
acabou_finalmente
bota fs = []
pra_cada j de 1 ate 10
    adiciona(fs, bora soma_muito(1000))
acabou_finalmente
espera(fs)
mostra c["n"]
mostra tipo(t)
`
	for motor, saida := range rodaNosDoisMotores(t, fonte) {
		if saida != "10000\ntrava\n" {
			t.Fatalf("%s: com_trava deixou incremento escapar: %q", motor, saida)
		}
	}
}

// com_trava devolve o resultado, solta a trava mesmo com erro (o erro sobe
// pra quem chamou) e acusa a reentrada em vez de travar pra sempre.
func TestComTravaErroEReentrada(t *testing.T) {
	fonte := `
bota t = trava()
mostra com_trava(t, gambiarra() funciona 42 acabou_finalmente)
arruma
    com_trava(t, gambiarra() quebra("ops") acabou_finalmente)
quebrou err
    mostra "subiu: " + erro_msg(err)
acabou_finalmente
mostra com_trava(t, gambiarra() funciona "soltou" acabou_finalmente)
arruma
    com_trava(t, gambiarra()
        funciona com_trava(t, gambiarra() funciona 1 acabou_finalmente)
    acabou_finalmente)
quebrou err
    mostra contem(erro_msg(err), "nao e reentrante")
acabou_finalmente
mostra espera(bora com_trava(t, gambiarra() funciona "outra goroutine pegou" acabou_finalmente))
arruma
    com_trava(42, gambiarra() funciona 1 acabou_finalmente)
quebrou err
    mostra contem(erro_msg(err), "trava()")
acabou_finalmente
`
	want := "42\nsubiu: quebra: ops\nsoltou\ndeu_bom\noutra goroutine pegou\ndeu_bom\n"
	for motor, saida := range rodaNosDoisMotores(t, fonte) {
		if saida != want {
			t.Fatalf("%s:\n got %q\nwant %q", motor, saida, want)
		}
	}
}

// Servidor HTTP de verdade (httptest abre porta TCP) com requisicoes em
// paralelo: cada handler mexe nas MESMAS colecoes globais.
func TestServidorColecoesConcorrentes(t *testing.T) {
	fonte := `
bota visitas = {}
bota log = []
bota vistos = conjunto([])
bota t = trava()
bota contador = {"n": 0}
rota("GET", "/oi", gambiarra(pedido)
    bota id = pedido["query"]["id"]
    bota visitas[id] = (visitas[id] ?? 0) + 1
    adiciona(log, id)
    adiciona_conjunto(vistos, id)
    se_colar tamanho(log) % 7 == 0
        remove_conjunto(vistos, id)
    acabou_finalmente
    bota n = 0
    pra_cada k, v em visitas
        bota n = n + 1
    acabou_finalmente
    bota _j = pra_json({"log": log[0:10], "n": n})
    com_trava(t, gambiarra() bota contador["n"] = contador["n"] + 1 acabou_finalmente)
    funciona "ok"
acabou_finalmente)
rota("GET", "/total", gambiarra(pedido)
    funciona texto(contador["n"]) + " " + texto(tamanho(log))
acabou_finalmente)
`
	const clientes, porCliente = 20, 25
	for _, motor := range []string{"vm", "arvore"} {
		t.Run(motor, func(t *testing.T) {
			interp := sobeNosDoisMotores(t, motor, fonte)
			srv := httptest.NewServer(interp.ServidorHandler())
			defer srv.Close()
			cli := srv.Client()

			var wg sync.WaitGroup
			erros := make(chan string, clientes*porCliente)
			for c := 0; c < clientes; c++ {
				wg.Add(1)
				go func(c int) {
					defer wg.Done()
					for r := 0; r < porCliente; r++ {
						resp, err := cli.Get(fmt.Sprintf("%s/oi?id=c%d", srv.URL, (c*porCliente+r)%13))
						if err != nil {
							erros <- err.Error()
							return
						}
						corpo, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						if resp.StatusCode != 200 || string(corpo) != "ok" {
							erros <- fmt.Sprintf("status %d corpo %q", resp.StatusCode, corpo)
						}
					}
				}(c)
			}
			wg.Wait()
			close(erros)
			for e := range erros {
				t.Fatalf("requisicao falhou: %s", e)
			}

			resp, err := cli.Get(srv.URL + "/total")
			if err != nil {
				t.Fatal(err)
			}
			corpo, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			want := fmt.Sprintf("%d %d", clientes*porCliente, clientes*porCliente)
			if string(corpo) != want {
				t.Fatalf("total: got %q want %q", corpo, want)
			}
		})
	}
}

// paralelo() com a gambiarra escrevendo no mesmo dicionario.
func TestParaleloEscreveMesmoDicionario(t *testing.T) {
	fonte := `
bota d = {}
bota t = trava()
bota soma = {"n": 0}
bota r = paralelo(1..500, gambiarra(i)
    bota d["k" + (i % 20)] = i
    com_trava(t, gambiarra() bota soma["n"] = soma["n"] + i acabou_finalmente)
    funciona i
acabou_finalmente)
mostra tamanho(r)
mostra tamanho(d)
mostra soma["n"]
`
	for motor, saida := range rodaNosDoisMotores(t, fonte) {
		if saida != "500\n20\n125250\n" {
			t.Fatalf("%s: %q", motor, saida)
		}
	}
}
