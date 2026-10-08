package interpreter

import (
	"sync"

	"gambiarrascript/ast"
	"gambiarrascript/object"
)

// Gancho de linha no tree-walker (contrato em object/gancho.go). Desligado
// custa um teste de nil por statement no evalBlock/evalProgram. Ligado, cada
// programa que roda (o principal e cada modulo, na primeira vez) registra um
// sitio por statement executavel; o laco de statements procura o node no
// mapa e chama o gancho. Statement sintetico (thunk de padrao de campo) nao
// esta no mapa e nao dispara — igual a VM, que so emite OpLinha pros nodes
// do parser.
type ganchoLinha struct {
	fn     object.GanchoLinha
	mu     sync.RWMutex // modulo pode ser importado de varias goroutines
	sitios map[ast.Statement]*object.SitioLinha
}

// DefinirGancho liga o gancho de linha (nil desliga). Chama antes do Eval do
// programa: os sitios sao registrados quando cada programa comeca a rodar.
func (i *Interpreter) DefinirGancho(g object.GanchoLinha) {
	if g == nil {
		i.gancho = nil
		return
	}
	i.gancho = &ganchoLinha{fn: g, sitios: map[ast.Statement]*object.SitioLinha{}}
}

// registraSitios cria os sitios do programa (principal ou modulo).
func (g *ganchoLinha) registraSitios(prog *ast.Program, arquivo string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, s := range ast.StatementsExecutaveis(prog) {
		if _, ok := g.sitios[s]; !ok {
			g.sitios[s] = &object.SitioLinha{Arquivo: arquivo, Linha: ast.LinhaDoStatement(s)}
		}
	}
}

// dispara chama o gancho se o statement e do usuario.
func (g *ganchoLinha) dispara(s ast.Statement) {
	g.mu.RLock()
	sitio := g.sitios[s]
	g.mu.RUnlock()
	if sitio != nil {
		g.fn(sitio)
	}
}
