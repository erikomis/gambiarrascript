package lsp

import (
	"strings"
	"testing"
)

const srcPOO = `treta Animal
    nome = "anonimo"
acabou_finalmente
treta Cachorro
    Animal
    raca
acabou_finalmente
combinado Falante
    fala()
acabou_finalmente
gambiarra (c Cachorro) fala(alto = deu_ruim)
    bota frase = c.nome + " late"
    funciona frase
acabou_finalmente
bota rex = Cachorro{Animal: Animal{nome: "rex"}, raca: "vira"}
mostra satisfaz(rex, Falante)
mostra rex.fala()`

// O linter nao pode acusar nome de treta/combinado, receiver, campo do
// literal nem metodo como indefinido.
func TestLintPOOSemFalsoPositivo(t *testing.T) {
	for _, d := range diagsDeTypecheck(t, srcPOO) {
		t.Errorf("aviso inesperado: %s", d.Message)
	}
	diags := diagsDeTypecheck(t, "bota p = Fantasma{x: 1}")
	if !contemMsg(diags, "`Fantasma` pode estar indefinido") {
		t.Fatalf("treta inexistente devia avisar: %v", diags)
	}
}

func TestOutlinePOO(t *testing.T) {
	s := servidorCom(map[string]string{uriMain: srcPOO})
	sims := s.simbolosDoDocumento(uriMain)
	por := map[string]SimboloDoc{}
	for _, x := range sims {
		por[x.Name] = x
	}
	cach, ok := por["Cachorro"]
	if !ok || cach.Kind != kindStruct || len(cach.Children) != 2 || cach.Children[0].Detail != "puxadinho" {
		t.Fatalf("Cachorro errado no outline: %+v", cach)
	}
	if cach.Range.End.Line != 6 {
		t.Fatalf("range da treta devia ir ate o acabou_finalmente: %+v", cach.Range)
	}
	if f, ok := por["Falante"]; !ok || f.Kind != kindInterface || len(f.Children) != 1 || f.Children[0].Kind != kindMetodo {
		t.Fatalf("Falante errado: %+v", f)
	}
	m, ok := por["Cachorro.fala"]
	if !ok || m.Kind != kindMetodo || m.Range.End.Line != 13 || !strings.Contains(m.Detail, "alto") {
		t.Fatalf("metodo errado: %+v", m)
	}
	if _, ok := por["rex"]; !ok {
		t.Fatalf("variavel de topo sumiu: %+v", sims)
	}
}

func TestDefinicaoPOO(t *testing.T) {
	s := servidorCom(map[string]string{uriMain: srcPOO})
	// `Cachorro{` no literal vai pra declaracao da treta
	loc := definicaoEm(t, s, uriMain, acha(t, srcPOO, "Cachorro", 2))
	if loc == nil || loc.Range.Start != (Posicao{3, 6}) {
		t.Fatalf("definicao de Cachorro errada: %+v", loc)
	}
	// o puxadinho `Animal` aponta pra treta Animal
	loc = definicaoEm(t, s, uriMain, acha(t, srcPOO, "Animal", 1))
	if loc == nil || loc.Range.Start != (Posicao{0, 6}) {
		t.Fatalf("definicao do puxadinho errada: %+v", loc)
	}
	// o receiver e parametro do metodo
	loc = definicaoEm(t, s, uriMain, acha(t, srcPOO, "c.nome", 0))
	if loc == nil || loc.Range.Start != (Posicao{10, 11}) {
		t.Fatalf("definicao do receiver errada: %+v", loc)
	}
	// renomear a treta pega declaracao, puxadinho e literais
	refs := referenciasEm(t, s, uriMain, acha(t, srcPOO, "Animal", 0), true)
	if len(refs) != 3 {
		t.Fatalf("referencias de Animal: %d (%+v)", len(refs), refs)
	}
}

func TestHoverPOO(t *testing.T) {
	if h := hoverConteudo("treta X", 0, 2); !strings.Contains(h, "struct") {
		t.Fatalf("hover de treta: %q", h)
	}
	if h := hoverConteudo("satisfaz(a, B)", 0, 3); !strings.Contains(h, "implicita") {
		t.Fatalf("hover de satisfaz: %q", h)
	}
}
