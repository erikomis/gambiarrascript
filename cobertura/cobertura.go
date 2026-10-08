// Package cobertura mede cobertura de linhas (gs testa --cobertura) em cima
// do gancho de linha dos engines (object/gancho.go).
//
// Linha executavel = linha onde comeca um statement executavel
// (ast.LinhasExecutaveis) — a mesma regra pros dois engines. A contagem de
// uma linha e quantas vezes algum statement que comeca nela rodou.
package cobertura

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

// Coletor conta as execucoes de cada sitio. Serve pra varias rodadas (um
// arquivo de teste por vez, cada um com engine novo): as contagens somam.
// Seguro pra varias goroutines (bora/paralelo chamam o gancho em paralelo).
type Coletor struct {
	mu   sync.Mutex
	hits map[*object.SitioLinha]int64
}

// Novo devolve um coletor vazio.
func Novo() *Coletor {
	return &Coletor{hits: map[*object.SitioLinha]int64{}}
}

// Gancho devolve o gancho de linha que alimenta o coletor — passa pro
// Interpreter.DefinirGancho ou pro vm.DefinirGancho.
func (c *Coletor) Gancho() object.GanchoLinha {
	return func(s *object.SitioLinha) {
		c.mu.Lock()
		c.hits[s]++
		c.mu.Unlock()
	}
}

// Contagens agrega por arquivo e linha: contagens[arquivo][linha] = execucoes.
func (c *Coletor) Contagens() map[string]map[int]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]map[int]int64{}
	for s, n := range c.hits {
		por := out[s.Arquivo]
		if por == nil {
			por = map[int]int64{}
			out[s.Arquivo] = por
		}
		por[s.Linha] += n
	}
	return out
}

// Arquivo e a cobertura de um .gs: as linhas executaveis e quantas vezes
// cada uma rodou (linha sem entrada em Hits = 0).
type Arquivo struct {
	Caminho     string // absoluto
	Fonte       []byte
	Executaveis []int // ordenadas
	Hits        map[int]int64
}

// Cobertas conta as linhas executaveis que rodaram pelo menos uma vez.
func (a *Arquivo) Cobertas() int {
	n := 0
	for _, l := range a.Executaveis {
		if a.Hits[l] > 0 {
			n++
		}
	}
	return n
}

// Relatorio e a cobertura de varios arquivos, ordenados pelo caminho.
type Relatorio struct {
	Arquivos []*Arquivo
}

// Totais soma linhas cobertas e executaveis de todos os arquivos.
func (r *Relatorio) Totais() (cobertas, executaveis int) {
	for _, a := range r.Arquivos {
		cobertas += a.Cobertas()
		executaveis += len(a.Executaveis)
	}
	return
}

// Pct devolve a porcentagem (100 quando nao tem linha executavel).
func Pct(cobertas, executaveis int) float64 {
	if executaveis == 0 {
		return 100
	}
	return 100 * float64(cobertas) / float64(executaveis)
}

// Relatorio monta o relatorio dos arquivos que rodaram (tiveram alguma linha
// executada) mais os `extras` (ex.: os .gs do diretorio testado que nenhum
// teste importou — entram com 0%). `inclui` filtra (nil = todos). A fonte de
// cada arquivo e relida (object.LeModulo) e reparseada pra achar as linhas
// executaveis — a mesma regra que os engines usaram pra instrumentar.
func (c *Coletor) Relatorio(extras []string, inclui func(caminho string) bool) (*Relatorio, error) {
	contagens := c.Contagens()
	caminhos := map[string]bool{}
	for arq := range contagens {
		if arq != "" {
			caminhos[arq] = true
		}
	}
	for _, e := range extras {
		if abs, err := filepath.Abs(e); err == nil {
			caminhos[abs] = true
		}
	}
	rel := &Relatorio{}
	for arq := range caminhos {
		if inclui != nil && !inclui(arq) {
			continue
		}
		hits, rodou := contagens[arq]
		fonte, err := object.LeModulo(arq)
		if err != nil {
			if !rodou {
				continue // extra que nao da pra ler: fica de fora
			}
			return nil, fmt.Errorf("cobertura: nao consegui ler %s: %v", arq, err)
		}
		p := parser.New(lexer.New(string(fonte)))
		prog := p.ParseProgram()
		if errs := p.Errors(); len(errs) != 0 {
			if !rodou {
				continue // extra que nem parseia nao tem linha executavel
			}
			return nil, fmt.Errorf("cobertura: %s nao parseia (mudou durante os testes?): %s", arq, errs[0])
		}
		if hits == nil {
			hits = map[int]int64{}
		}
		rel.Arquivos = append(rel.Arquivos, &Arquivo{
			Caminho:     arq,
			Fonte:       fonte,
			Executaveis: ast.LinhasExecutaveis(prog),
			Hits:        hits,
		})
	}
	sort.Slice(rel.Arquivos, func(i, j int) bool { return rel.Arquivos[i].Caminho < rel.Arquivos[j].Caminho })
	return rel, nil
}
