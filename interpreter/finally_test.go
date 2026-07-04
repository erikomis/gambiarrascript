package interpreter

import "testing"

func TestFinalmenteRodaSemErro(t *testing.T) {
	out := rodar(t, `arruma
    mostra "try"
finalmente
    mostra "finally"
acabou_finalmente`)
	if out != "try\nfinally\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestFinalmenteRodaComErroECatch(t *testing.T) {
	out := rodar(t, `arruma
    mostra "try"
    quebra("pfvr")
quebrou err
    mostra "catch: " + texto(err)
finalmente
    mostra "finally"
acabou_finalmente`)
	esp := "try\ncatch: quebra: pfvr\nfinally\n"
	if out != esp {
		t.Fatalf("saida %q, esperado %q", out, esp)
	}
}

func TestFinalmenteSemQuebrou(t *testing.T) {
	out := rodar(t, `arruma
    mostra "try sozinho"
finalmente
    mostra "cleanup"
acabou_finalmente`)
	if out != "try sozinho\ncleanup\n" {
		t.Fatalf("saida %q", out)
	}
}

func TestCatchSemFinalmenteContinuaComoAntes(t *testing.T) {
	out := rodar(t, `arruma
    quebra("b Meng")
quebrou err
    mostra "capturado: " + texto(err)
acabou_finalmente`)
	if out != "capturado: quebra: b Meng\n" {
		t.Fatalf("saida %q", out)
	}
}
