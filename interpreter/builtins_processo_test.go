package interpreter

import (
	"strings"
	"testing"

	"gambiarrascript/object"
)

func TestRodaComandoEcho(t *testing.T) {
	out := rodar(t, `bota r = roda_comando("echo", ["salve"])
mostra r.codigo
mostra tira_espaco(r.saida)`)
	if out != "0\nsalve\n" {
		t.Fatalf("roda_comando echo: %q", out)
	}
}

func TestRodaComandoSemArgs(t *testing.T) {
	out := rodar(t, `bota r = roda_comando("true")
mostra r.codigo`)
	if out != "0\n" {
		t.Fatalf("roda_comando sem args: %q", out)
	}
}

func TestRodaComandoCodigoNaoZeroNaoEhErro(t *testing.T) {
	out := rodar(t, `bota r = roda_comando("sh", ["-c", "exit 3"])
mostra r.codigo`)
	if out != "3\n" {
		t.Fatalf("roda_comando codigo: %q", out)
	}
}

func TestRodaComandoCapturaStderr(t *testing.T) {
	out := rodar(t, `bota r = roda_comando("sh", ["-c", "echo oops 1>&2"])
mostra tira_espaco(r.erro)`)
	if out != "oops\n" {
		t.Fatalf("roda_comando stderr: %q", out)
	}
}

func TestRodaComandoInexistenteDaErro(t *testing.T) {
	out := rodarErro(t, `roda_comando("comando_que_nao_existe_zzz")`)
	if !strings.Contains(out, "roda_comando") {
		t.Fatalf("roda_comando inexistente: %q", out)
	}
}

func TestSaiParaNoTopo(t *testing.T) {
	out := rodar(t, `mostra 1
sai()
mostra 2`)
	if out != "1\n" {
		t.Fatalf("sai no topo: %q", out)
	}
}

func TestSaiDentroDeLoop(t *testing.T) {
	out := rodar(t, `pra_cada i de 1 ate 5
    se_colar i == 3
        sai()
    acabou_finalmente
    mostra i
acabou_finalmente
mostra "fim"`)
	if out != "1\n2\n" {
		t.Fatalf("sai no loop: %q", out)
	}
}

func TestSaiDentroDeFuncaoDesenrolaTudo(t *testing.T) {
	out := rodar(t, `gambiarra f()
    sai()
acabou_finalmente
mostra 1
f()
mostra 2`)
	if out != "1\n" {
		t.Fatalf("sai na funcao: %q", out)
	}
}

func TestSaiCodigoDefault(t *testing.T) {
	res := builtinSai(nil)
	s, ok := res.(*object.Sair)
	if !ok || s.Codigo != 0 {
		t.Fatalf("sai() default: %#v", res)
	}
}

func TestSaiCodigoDado(t *testing.T) {
	res := builtinSai([]object.Object{object.NumInt(3)})
	s, ok := res.(*object.Sair)
	if !ok || s.Codigo != 3 {
		t.Fatalf("sai(3): %#v", res)
	}
}
