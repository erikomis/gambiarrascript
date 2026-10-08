package depurador

import (
	"bytes"
	"strings"
	"testing"
)

const progCLI = `bota xs = [1, 2, 3]
treta Ponto
    x
    y
acabou_finalmente
bota p = Ponto{1, 2}

gambiarra soma(a, b)
    bota r = a + b
    funciona r
acabou_finalmente

gambiarra dobra(n)
    bota t = soma(n, n)
    funciona t
acabou_finalmente
mostra dobra(4)
mostra "fim"
`

// stdin com os comandos -> transcricao esperada (igual nos dois engines).
const comandosCLI = `ajuda
para 9
para 7
para 99
para
segue
pilha
vars
ve a + 100
ve xs
ve "oi"
quadro 1
ve n
quadro 0
lista
proximo

sai
tira 1
entra
fluxos
segue
`

const transcricaoCLI = `gs debug: prog.gs (engine ENGINE). ` + "`ajuda`" + ` lista os comandos.
parado na entrada: prog.gs:1 (em <principal>)
   1 | bota xs = [1, 2, 3]
(gs) ` + textoAjuda + `(gs) breakpoint 1 em prog.gs:9
(gs) breakpoint 2 em prog.gs:8 (a linha 7 nao roda nada; foi pra 8)
(gs) breakpoint 3 em prog.gs:99 nao vale: nenhuma linha executavel da 99 pra baixo
(gs)   breakpoint 1 em prog.gs:9
  breakpoint 2 em prog.gs:8 (a linha 7 nao roda nada; foi pra 8)
  breakpoint 3 em prog.gs:99 nao vale: nenhuma linha executavel da 99 pra baixo
(gs) parou no breakpoint 2: prog.gs:8 (em <principal>)
   8 | gambiarra soma(a, b)
(gs) => #0 <principal>  prog.gs:8
(gs) globais:
  Ponto = <treta Ponto>
  p = Ponto{x: 1, y: 2}
  xs = [1, 2, 3]
(gs) ERRO_A
(gs) [1, 2, 3]
(gs) "oi"
(gs) quadro invalido (tem 1: 0 a 0)
(gs) ERRO_N
(gs) #0 <principal>  prog.gs:8
   8 | gambiarra soma(a, b)
(gs)       3 |     x
      4 |     y
      5 | acabou_finalmente
      6 | bota p = Ponto{1, 2}
      7 |
=>*   8 | gambiarra soma(a, b)
  *   9 |     bota r = a + b
     10 |     funciona r
     11 | acabou_finalmente
     12 |
     13 | gambiarra dobra(n)
(gs) prog.gs:13 (em <principal>)
  13 | gambiarra dobra(n)
(gs) prog.gs:17 (em <principal>)
  17 | mostra dobra(4)
(gs) parou no breakpoint 1: prog.gs:9 (em soma)
   9 |     bota r = a + b
(gs) breakpoint 1 tirado
(gs) prog.gs:10 (em soma)
  10 |     funciona r
(gs) * 1 principal (parado em prog.gs:10)
(gs) 8
fim
programa terminou (codigo 0)
`

// O terminal: comandos (e apelidos) de stdin, a transcricao completa.
func TestCLITranscricao(t *testing.T) {
	for _, eng := range []string{"vm", "tree"} {
		t.Run(eng, func(t *testing.T) {
			arq := escreveGs(t, "prog.gs", progCLI)
			var out bytes.Buffer
			codigo := RodaCLI(Config{Arquivo: arq, Motor: eng}, strings.NewReader(comandosCLI), &out, OpcoesCLI{})
			if codigo != 0 {
				t.Errorf("codigo %d", codigo)
			}
			quer := strings.ReplaceAll(transcricaoCLI, "ENGINE", motorNome(eng))
			erroA, erroN := "erro: nao existe nenhum `a` por aqui — confere o nome ou declara com `bota a = ...`",
				"erro: nao existe nenhum `n` por aqui — confere o nome ou declara com `bota n = ...`"
			if eng == "tree" {
				erroA, erroN = "erro: deu ruim na linha 1: cade o `a`? voce nao botou isso ainda",
					"erro: deu ruim na linha 1: cade o `n`? voce nao botou isso ainda"
			}
			quer = strings.ReplaceAll(quer, "ERRO_A", erroA)
			quer = strings.ReplaceAll(quer, "ERRO_N", erroN)
			if got := out.String(); got != quer {
				t.Errorf("transcricao diferente:\n--- tem\n%s\n--- queria\n%s", got, quer)
			}
		})
	}
}

// Os apelidos curtos (b/c/n/s/o/bt/p/v/l/q) e o breakpoint condicional no
// terminal, nos dois engines.
func TestCLIApelidos(t *testing.T) {
	cmds := "b 9 se a == 4\nb 14 se n > 100\nc\nbt\np a + b\nv\no\nn\nl\nq\n"
	for _, eng := range []string{"vm", "tree"} {
		t.Run(eng, func(t *testing.T) {
			arq := escreveGs(t, "prog.gs", progCLI)
			var out bytes.Buffer
			RodaCLI(Config{Arquivo: arq, Motor: eng}, strings.NewReader(cmds), &out, OpcoesCLI{})
			quer := strings.ReplaceAll(transcricaoApelidos, "ENGINE", motorNome(eng))
			if got := out.String(); got != quer {
				t.Errorf("transcricao diferente:\n--- tem\n%s\n--- queria\n%s", got, quer)
			}
		})
	}
}

const transcricaoApelidos = `gs debug: prog.gs (engine ENGINE). ` + "`ajuda`" + ` lista os comandos.
parado na entrada: prog.gs:1 (em <principal>)
   1 | bota xs = [1, 2, 3]
(gs) breakpoint 1 em prog.gs:9 se a == 4
(gs) breakpoint 2 em prog.gs:14 se n > 100
(gs) parou no breakpoint 1: prog.gs:9 (em soma)
   9 |     bota r = a + b
(gs) => #0 soma  prog.gs:9
   #1 dobra  prog.gs:14
   #2 <principal>  prog.gs:17
(gs) 8
(gs) locais:
  a = 4
  b = 4
globais:
  Ponto = <treta Ponto>
  dobra = gambiarra dobra(n)
  p = Ponto{x: 1, y: 2}
  soma = gambiarra soma(a, b)
  xs = [1, 2, 3]
(gs) prog.gs:15 (em dobra)
  15 |     funciona t
(gs) 8
prog.gs:18 (em <principal>)
  18 | mostra "fim"
(gs)      13 | gambiarra dobra(n)
  *  14 |     bota t = soma(n, n)
     15 |     funciona t
     16 | acabou_finalmente
     17 | mostra dobra(4)
=>   18 | mostra "fim"
(gs) fui. (programa abortado)
`
