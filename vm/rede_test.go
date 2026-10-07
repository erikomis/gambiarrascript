package vm

import (
	"bytes"
	"strings"
	"testing"

	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Rede baixo nivel (conecta_tcp/escuta_tcp/escuta_udp...). Tudo em
// 127.0.0.1:0 — o servidor manda o endereco sorteado pelo cano `pronto` e
// para quando o cano `para` fecha, entao cada teste sobe e derruba o seu.

// rodaRedeNosDois e o esperaNosDois com o stderr capturado: os handlers que
// quebram logam la, e o teste confere o log tambem.
func rodaRedeNosDois(t *testing.T, src, saidaEsp string, stderrTem ...string) {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse: %s", strings.Join(errs, "; "))
	}
	for _, engine := range []string{"tree", "vm"} {
		var out, errOut bytes.Buffer
		interp := interpreter.New(&out)
		interp.DefinirStderr(&errOut)
		erro := ""
		if engine == "tree" {
			res := interp.Eval(prog, object.NewEnvironment())
			if e, ok := res.(*object.Erro); ok && !e.Handled {
				erro = e.Message
			}
		} else {
			comp := compiler.New()
			if err := comp.Compile(prog); err != nil {
				t.Fatalf("compile: %v", err)
			}
			if err := NovaComInterp(comp.Bytecode(), &out, interp).Run(); err != nil {
				erro = err.Error()
			}
		}
		if erro != "" {
			t.Errorf("[%s] erro inesperado: %s", engine, erro)
		}
		if out.String() != saidaEsp {
			t.Errorf("[%s] saida errada\n  veio:     %q\n  esperado: %q", engine, out.String(), saidaEsp)
		}
		for _, trecho := range stderrTem {
			if !strings.Contains(errOut.String(), trecho) {
				t.Errorf("[%s] stderr devia ter %q, veio %q", engine, trecho, errOut.String())
			}
		}
	}
}

// sobeServidor e o prelude comum: escuta_tcp em porta sorteada rodando o
// handler `atende` (definido pelo caso) com as opcoes extras.
func sobeServidor(opcoesExtras string) string {
	return `bota pronto = cano(1)
bota para = cano()
bota srv = bora escuta_tcp("127.0.0.1:0", atende, {"pronto": pronto, "para": para` + opcoesExtras + `})
bota end = recebe(pronto)
`
}

const derrubaServidor = `fecha(para)
mostra espera(srv)
`

const handlerEco = `gambiarra atende(c)
    enquanto deu_bom
        bota linha = recebe(c)
        se_colar linha == nada
            vaza
        acabou_finalmente
        envia(c, "eco: " + linha)
    acabou_finalmente
acabou_finalmente
`

func TestTcpEcoPorLinha(t *testing.T) {
	src := handlerEco + sobeServidor("") + `bota c = conecta_tcp(end)
mostra tipo(c)
envia(c, "salve")
envia(c, 42)
mostra recebe(c)
mostra recebe(c)
mostra endereco(c) == end
fecha(c)
fecha(c)
mostra recebe(c)
arruma
    envia(c, "x")
quebrou erro
    mostra erro_tipo(erro) + " | " + erro_msg(erro)
acabou_finalmente
` + derrubaServidor + `arruma
    conecta_tcp(end)
quebrou erro
    mostra "depois de parar: " + erro_tipo(erro)
acabou_finalmente
`
	rodaRedeNosDois(t, src, "nativo\neco: salve\neco: 42\ndeu_bom\nnada\n"+
		"rede | deu ruim: envia(): conexao fechada, nao da pra mandar mais nada\n"+
		"nada\ndepois de parar: rede\n")
}

// \r\n vira linha normal, a ultima linha sem \n tambem chega, e depois do
// outro lado desligar o recebe da nada.
func TestTcpEnquadramentoDeLinha(t *testing.T) {
	src := `bota linhas = cano(10)
gambiarra atende(c)
    enquanto deu_bom
        bota l = recebe(c)
        se_colar l == nada
            vaza
        acabou_finalmente
        envia(linhas, "[" + l + "]")
    acabou_finalmente
    envia(linhas, "fim")
acabou_finalmente
` + sobeServidor("") + `bota c = conecta_tcp(end, {"modo": "bruto"})
envia(c, "um" + hex_decodifica("0d0a") + "dois\n\ntres")
fecha(c)
pra_cada i de 1 ate 5
    mostra recebe(linhas)
acabou_finalmente
` + derrubaServidor
	rodaRedeNosDois(t, src, "[um]\n[dois]\n[]\n[tres]\nfim\nnada\n")
}

// Modo bruto: o texto carrega os bytes exatos, inclusive os que nao sao
// UTF-8 — hex_decodifica/hex_codifica fazem a ponte.
func TestTcpModoBruto(t *testing.T) {
	src := `gambiarra atende(c)
    enquanto deu_bom
        bota pedaco = recebe(c)
        se_colar pedaco == nada
            vaza
        acabou_finalmente
        envia(c, pedaco)
    acabou_finalmente
acabou_finalmente
` + sobeServidor(`, "modo": "bruto"`) + `bota c = conecta_tcp(end, {"modo": "bruto", "timeout": 5})
envia(c, "sem quebra de linha")
mostra recebe(c)
envia(c, hex_decodifica("00ff10fe"))
mostra hex_codifica(recebe(c))
fecha(c)
` + derrubaServidor
	rodaRedeNosDois(t, src, "sem quebra de linha\n00ff10fe\nnada\n")
}

func TestTcpTimeoutNoRecebe(t *testing.T) {
	src := `gambiarra atende(c)
    recebe(c)
acabou_finalmente
` + sobeServidor("") + `bota c = conecta_tcp(end, {"timeout": 0.2})
arruma
    recebe(c)
    mostra "nao devia chegar aqui"
quebrou erro
    mostra erro_tipo(erro)
    mostra erro_msg(erro)
acabou_finalmente
# a conexao continua de pe depois do timeout
envia(c, "ainda to aqui")
fecha(c)
` + derrubaServidor
	rodaRedeNosDois(t, src, "rede\ndeu ruim: recebe(): ninguem falou nada em 0.2s (timeout)\nnada\n")
}

// Handler que quebra loga no stderr e fecha so aquela conexao; o servidor
// segue atendendo.
func TestTcpHandlerQuebradoNaoDerrubaServidor(t *testing.T) {
	src := `gambiarra atende(c)
    bota l = recebe(c)
    se_colar l == "quebra"
        mostra 1 / 0
    acabou_finalmente
    envia(c, "vivo: " + l)
acabou_finalmente
` + sobeServidor("") + `bota c1 = conecta_tcp(end)
envia(c1, "quebra")
mostra recebe(c1)
bota c2 = conecta_tcp(end)
envia(c2, "oi")
mostra recebe(c2)
` + derrubaServidor
	rodaRedeNosDois(t, src, "nada\nvivo: oi\nnada\n", "escuta_tcp: o handler de 127.0.0.1:", "quebrou")
}

// Dois clientes ao mesmo tempo: o 2o e atendido antes do 1o mandar qualquer
// coisa — so funciona se cada conexao roda na propria goroutine.
func TestTcpDoisClientesAoMesmoTempo(t *testing.T) {
	src := `gambiarra atende(c)
    bota nome = recebe(c)
    envia(c, "salve, " + nome)
acabou_finalmente
` + sobeServidor("") + `bota c1 = conecta_tcp(end, {"timeout": 5})
bota c2 = conecta_tcp(end, {"timeout": 5})
envia(c2, "dois")
mostra recebe(c2)
envia(c1, "um")
mostra recebe(c1)
` + derrubaServidor
	rodaRedeNosDois(t, src, "salve, dois\nsalve, um\nnada\n")
}

// envia de varias goroutines na mesma conexao nao intercala bytes: cada linha
// chega inteira.
func TestTcpEnviaConcorrenteNaoIntercala(t *testing.T) {
	src := `gambiarra rajada(c, letra)
    bota linha = ""
    pra_cada i de 1 ate 2000
        linha += letra
    acabou_finalmente
    pra_cada i de 1 ate 50
        envia(c, linha)
    acabou_finalmente
acabou_finalmente
gambiarra atende(c)
    espera([bora rajada(c, "a"), bora rajada(c, "b"), bora rajada(c, "c")])
acabou_finalmente
` + sobeServidor("") + `bota c = conecta_tcp(end, {"timeout": 5})
bota inteiras = 0
bota tortas = 0
enquanto deu_bom
    bota l = recebe(c)
    se_colar l == nada
        vaza
    acabou_finalmente
    se_colar tamanho(l) == 2000 e (l == substitui(l, "b", "") e l == substitui(l, "c", "") ou l == substitui(l, "a", "") e l == substitui(l, "c", "") ou l == substitui(l, "a", "") e l == substitui(l, "b", ""))
        inteiras += 1
    se_nao_colar
        tortas += 1
    acabou_finalmente
acabou_finalmente
mostra "inteiras: " + inteiras + ", tortas: " + tortas
` + derrubaServidor
	rodaRedeNosDois(t, src, "inteiras: 150, tortas: 0\nnada\n")
}

func TestTcpErrosDeUso(t *testing.T) {
	src := `gambiarra tenta(f)
    arruma
        f()
    quebrou erro
        mostra erro_tipo(erro) + " | " + erro_msg(erro)
    acabou_finalmente
acabou_finalmente
tenta(gambiarra() conecta_tcp("127.0.0.1:1") acabou_finalmente)
tenta(gambiarra() conecta_tcp("127.0.0.1:1", {"modo": "turbo"}) acabou_finalmente)
tenta(gambiarra() conecta_tcp("127.0.0.1:1", {"tempo": 1}) acabou_finalmente)
tenta(gambiarra() conecta_tcp("127.0.0.1:1", {"timeout": 0}) acabou_finalmente)
tenta(gambiarra() escuta_tcp(8080, 42) acabou_finalmente)
tenta(gambiarra() endereco(cano()) acabou_finalmente)
`
	rodaRedeNosDois(t, src, ""+
		"rede | deu ruim: conecta_tcp(): nao rolou conectar em 127.0.0.1:1: dial tcp 127.0.0.1:1: connect: connection refused\n"+
		"builtin | deu ruim: conecta_tcp(): modo \"turbo\" nao existe — e \"linha\" (padrao) ou \"bruto\"\n"+
		"builtin | deu ruim: conecta_tcp(): opcao \"tempo\" nao existe aqui (vale: modo, timeout, tls)\n"+
		"builtin | deu ruim: conecta_tcp(): timeout tem que ser numero de segundos maior que zero, veio 0\n"+
		"builtin | deu ruim: escuta_tcp(): o handler tem que ser uma gambiarra(conexao), veio NUMERO\n"+
		"builtin | deu ruim: endereco() espera uma conexao tcp/udp, veio CANO\n")
}

func TestTcpEnviaSoTexto(t *testing.T) {
	src := handlerEco + sobeServidor("") + `bota c = conecta_tcp(end)
arruma
    envia(c, [1, 2])
quebrou erro
    mostra erro_msg(erro)
acabou_finalmente
fecha(c)
` + derrubaServidor
	rodaRedeNosDois(t, src, "deu ruim: envia(): conexao so manda texto, veio LISTA — usa pra_json(x) ou texto(x)\nnada\n")
}

func TestUdpIdaEVolta(t *testing.T) {
	src := `bota chegou = cano(10)
gambiarra atende(msg, remetente)
    envia(chegou, msg + " de " + tipo(remetente))
    se_colar msg == "calado"
        funciona nada
    acabou_finalmente
    funciona maiusculo(msg)
acabou_finalmente
bota pronto = cano(1)
bota para = cano()
bota srv = bora escuta_udp("127.0.0.1:0", atende, {"pronto": pronto, "para": para})
bota end = recebe(pronto)

bota u = conecta_udp(end, {"timeout": 5})
envia(u, "salve")
mostra recebe(u)
mostra endereco(u) == end
mostra recebe(chegou)

envia_udp(end, "ping")
mostra recebe(chegou)

bota mudo = conecta_udp(end, {"timeout": 0.2})
envia(mudo, "calado")
mostra recebe(chegou)
arruma
    recebe(mudo)
quebrou erro
    mostra erro_tipo(erro) + " | " + erro_msg(erro)
acabou_finalmente
fecha(mudo)
mostra recebe(mudo)
fecha(u)
` + derrubaServidor
	rodaRedeNosDois(t, src, "SALVE\ndeu_bom\nsalve de texto\nping de texto\ncalado de texto\n"+
		"rede | deu ruim: recebe(): ninguem falou nada em 0.2s (timeout)\nnada\nnada\n")
}
