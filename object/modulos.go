package object

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Sistema de modulos (importa) compartilhado pelos dois engines. Regras:
//
//   - um modulo roda UMA vez por processo, guardado pelo caminho absoluto
//     (igual Python/Node): importar de novo reaproveita o resultado;
//   - caminho relativo e relativo ao arquivo que importa; se nao achar, sobe
//     pelos diretorios procurando em gs_modulos/ (onde o `gs get` baixa);
//   - import circular vira erro com a cadeia ("importa circular: a.gs ->
//     b.gs -> a.gs"), nunca trava;
//   - varias goroutines importando o mesmo modulo: uma roda, as outras
//     esperam e recebem o mesmo resultado.

// LeModulo le a fonte de um modulo. E uma variavel pra o binario do `gs
// build` (que carrega os modulos embutidos) poder trocar a fonte.
var LeModulo = os.ReadFile

// MODULO_OBJ e o tipo do descritor de modulo compilado (nunca chega no codigo
// do usuario: so mora no pool de constantes da VM).
const MODULO_OBJ = "MODULO"

// Modulo e o descritor de um modulo compilado pra VM. O corpo roda uma vez
// (OpImporta, guardado pelo cache Modulos) e escreve nas globais dele, que
// ficam em Slots; o namespace e montado a partir de Nomes/Slots.
type Modulo struct {
	Caminho string            // absoluto
	Corpo   *CompiledFunction // nil: e o proprio principal (importar e ciclo)
	Nomes   []string
	Slots   []int
}

func (m *Modulo) Type() ObjectType { return MODULO_OBJ }
func (m *Modulo) Inspect() string  { return "<modulo " + filepath.Base(m.Caminho) + ">" }

// KindModulo e o tipo de erro de importacao (modulo que nao parseia, ciclo).
// Modulo que nao existe/nao da pra ler e "io".
const KindModulo = "parse"

// CandidatosModulo lista, em ordem, onde `importa caminho` procura o arquivo
// a partir do diretorio de quem importa: absoluto e usado direto; relativo
// tenta dir/caminho e depois <dir e cada pai>/gs_modulos/caminho.
func CandidatosModulo(dir, caminho string) []string {
	if filepath.IsAbs(caminho) {
		return []string{filepath.Clean(caminho)}
	}
	if dir == "" {
		dir = "."
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	out := []string{filepath.Join(dir, caminho)}
	for d := dir; ; {
		out = append(out, filepath.Join(d, "gs_modulos", caminho))
		pai := filepath.Dir(d)
		if pai == d {
			break
		}
		d = pai
	}
	return out
}

// ResolveModulo acha e le o modulo. Devolve o caminho absoluto (a chave do
// cache) e a fonte. O erro ja vem com a mensagem pro usuario e Kind "io".
func ResolveModulo(dir, caminho string) (string, []byte, *Erro) {
	for _, c := range CandidatosModulo(dir, caminho) {
		fonte, err := LeModulo(c)
		if err == nil {
			return c, fonte, nil
		}
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return "", nil, &Erro{Message: fmt.Sprintf("nao consegui ler o modulo %q: %v", caminho, err), Kind: "io"}
	}
	msg := fmt.Sprintf("nao achei o modulo %q (procurei do lado de quem importa e em gs_modulos/)", caminho)
	return "", nil, &Erro{Message: msg, Kind: "io"}
}

// ErroDeModulo monta o erro de um modulo que nao parseia/compila. `detalhe`
// ja traz a linha dentro do modulo ("linha 3: ...").
func ErroDeModulo(caminho, detalhe string) *Erro {
	return &Erro{Message: fmt.Sprintf("o modulo %q ta com perrengue: %s", caminho, detalhe), Kind: KindModulo}
}

// ComLinha devolve o erro de importacao com o prefixo "deu ruim na linha N:"
// da linha do `importa` — so se ele ainda nao tiver posicao (erro que veio de
// dentro do modulo ja tem a dele). Nunca mexe no original (pode estar no
// cache, compartilhado).
func ComLinha(e *Erro, linha int) *Erro {
	if e.Line != 0 || strings.HasPrefix(e.Message, "deu ruim") {
		return e
	}
	novo := *e
	if linha > 0 {
		novo.Message = fmt.Sprintf("deu ruim na linha %d: %s", linha, e.Message)
		novo.Line = linha
	} else {
		novo.Message = "deu ruim: " + e.Message
	}
	return &novo
}

// NamespaceModulo monta o dicionario do `importa ... como m`: os nomes de
// topo do modulo em ordem alfabetica (igual nos dois engines). Temporarios
// do compilador (__*) e valores nao definidos ficam de fora.
func NamespaceModulo(nomes []string, valor func(string) (Object, bool)) *Dicionario {
	ordenados := append([]string(nil), nomes...)
	sort.Strings(ordenados)
	d := NovoDicionario()
	for _, nome := range ordenados {
		if strings.HasPrefix(nome, "__") {
			continue
		}
		v, ok := valor(nome)
		if !ok || v == nil {
			continue
		}
		chave := &Texto{Value: nome}
		d.Bota(chave.ChaveHash(), ParDic{Chave: chave, Valor: v})
	}
	return d
}

// Modulos e o cache de modulos de UM processo (um interpretador ou uma VM com
// suas goroutines). O valor zero ja serve.
type Modulos struct {
	mu      sync.Mutex
	estados map[string]*estadoModulo
	// espera[a][b] > 0: codigo do modulo `a` esta parado esperando o modulo
	// `b` terminar (rodando ele ou esperando outra goroutine). E o grafo que
	// pega o ciclo entre goroutines (a espera b numa, b espera a noutra).
	espera map[string]map[string]int
}

type estadoModulo struct {
	pronto chan struct{}
	feito  bool
	cadeia []string // quem importou quem ate chegar nele (termina nele)
	valor  Object
	extra  any
	falha  Object // *Erro ou *Sair
}

// RodaModulo executa o corpo do modulo e devolve o namespace, um dado extra
// do engine (o tree-walker guarda o Environment) e a falha (*Erro/*Sair).
type RodaModulo func() (Object, any, Object)

// Importa devolve o modulo `alvo` (caminho absoluto), rodando-o se for a
// primeira vez. `atual` e o arquivo onde o `importa` esta escrito ("" quando
// o programa principal nao tem arquivo). A falha e *Erro (sem linha quando e
// do proprio importa: quem chama poe a linha com ComLinha) ou *Sair.
func (m *Modulos) Importa(atual, alvo string, roda RodaModulo) (Object, any, Object) {
	m.mu.Lock()
	if m.estados == nil {
		m.estados = map[string]*estadoModulo{}
		m.espera = map[string]map[string]int{}
	}
	// ja rodou: devolve do cache (antes de olhar a cadeia — um importa dentro
	// de gambiarra, chamada depois que tudo terminou, nao e ciclo)
	if st, ok := m.estados[alvo]; ok && st.feito {
		m.mu.Unlock()
		return st.valor, st.extra, nil
	}
	cadeia := m.cadeiaDe(atual)
	if i := indice(cadeia, alvo); i >= 0 {
		m.mu.Unlock()
		return nil, nil, erroCircular(append(append([]string(nil), cadeia[i:]...), alvo), cadeia[0])
	}
	if st, ok := m.estados[alvo]; ok {
		// outra goroutine esta rodando: se quem roda o alvo (direta ou
		// indiretamente) espera por algo da nossa cadeia, esperar travaria.
		if volta := m.caminhoAte(alvo, cadeia); volta != nil {
			m.mu.Unlock()
			ciclo := append(append([]string(nil), cadeia[indice(cadeia, volta[len(volta)-1]):]...), volta...)
			return nil, nil, erroCircular(ciclo, cadeia[0])
		}
		m.marcaEspera(atual, alvo, 1)
		m.mu.Unlock()
		<-st.pronto
		m.mu.Lock()
		m.marcaEspera(atual, alvo, -1)
		m.mu.Unlock()
		if st.falha != nil {
			return nil, nil, st.falha
		}
		return st.valor, st.extra, nil
	}
	st := &estadoModulo{pronto: make(chan struct{}), cadeia: append(append([]string(nil), cadeia...), alvo)}
	m.estados[alvo] = st
	m.marcaEspera(atual, alvo, 1)
	m.mu.Unlock()

	var valor Object
	var extra any
	var falha Object
	func() {
		// panico no corpo nao pode deixar quem espera travado pra sempre
		defer func() {
			if r := recover(); r != nil {
				falha = &Erro{Message: fmt.Sprintf("panico rodando o modulo: %v", r), Kind: "runtime"}
			}
		}()
		valor, extra, falha = roda()
	}()
	if e, ok := falha.(*Erro); ok {
		falha = AnotaModulo(e, NomeCurto(alvo, st.cadeia[0]))
	}

	m.mu.Lock()
	m.marcaEspera(atual, alvo, -1)
	if falha != nil {
		// modulo que falhou sai do cache: o proximo importa tenta de novo
		delete(m.estados, alvo)
		st.falha = falha
	} else {
		st.feito, st.valor, st.extra = true, valor, extra
	}
	close(st.pronto)
	m.mu.Unlock()
	if falha != nil {
		return nil, nil, falha
	}
	return valor, extra, nil
}

// cadeiaDe devolve a cadeia de importacao do arquivo `atual`: a gravada
// quando o modulo comecou a rodar, ou so ele mesmo (programa principal).
func (m *Modulos) cadeiaDe(atual string) []string {
	if st, ok := m.estados[atual]; ok {
		return st.cadeia
	}
	if atual == "" {
		return nil
	}
	return []string{atual}
}

func (m *Modulos) marcaEspera(de, para string, delta int) {
	if de == "" {
		return
	}
	arestas := m.espera[de]
	if arestas == nil {
		arestas = map[string]int{}
		m.espera[de] = arestas
	}
	arestas[para] += delta
	if arestas[para] <= 0 {
		delete(arestas, para)
	}
}

// caminhoAte procura, no grafo de espera, um caminho de `de` ate algum
// modulo da cadeia que ainda esta rodando. Devolve o caminho (comeca em `de`,
// termina no modulo da cadeia) ou nil.
func (m *Modulos) caminhoAte(de string, cadeia []string) []string {
	alvos := map[string]bool{}
	for _, c := range cadeia {
		if st, ok := m.estados[c]; !ok || !st.feito {
			alvos[c] = true
		}
	}
	visto := map[string]bool{}
	var anda func(n string) []string
	anda = func(n string) []string {
		if alvos[n] {
			return []string{n}
		}
		if visto[n] {
			return nil
		}
		visto[n] = true
		proximos := make([]string, 0, len(m.espera[n]))
		for p := range m.espera[n] {
			proximos = append(proximos, p)
		}
		sort.Strings(proximos)
		for _, p := range proximos {
			if resto := anda(p); resto != nil {
				return append([]string{n}, resto...)
			}
		}
		return nil
	}
	return anda(de)
}

func indice(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// NomeCurto mostra o caminho relativo ao diretorio do arquivo raiz da cadeia
// (o principal), que e o que a pessoa reconhece.
func NomeCurto(caminho, raiz string) string {
	if raiz != "" {
		if rel, err := filepath.Rel(filepath.Dir(raiz), caminho); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.Base(caminho)
}

func erroCircular(ciclo []string, raiz string) *Erro {
	nomes := make([]string, len(ciclo))
	for i, c := range ciclo {
		nomes[i] = NomeCurto(c, raiz)
	}
	return &Erro{Message: "importa circular: " + strings.Join(nomes, " -> "), Kind: KindModulo}
}

// AnotaModulo marca de qual modulo veio um erro que estourou no topo dele
// (so o mais interno: erro que atravessa varios modulos nao vira uma fila).
func AnotaModulo(e *Erro, nome string) *Erro {
	// erro de parse do proprio modulo ("o modulo ... ta com perrengue") ja diz
	// de onde veio
	if strings.HasPrefix(e.Message, "o modulo ") || strings.Contains(e.Message, "(no modulo ") || strings.Contains(e.Message, "importa circular:") {
		return e
	}
	novo := *e
	novo.Message = fmt.Sprintf("%s (no modulo %q)", e.Message, nome)
	return &novo
}
