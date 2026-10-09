package object

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// POO no modelo do Go (Tier 8): treta (struct com metodos), combinado
// (interface implicita) e puxadinho (embedding com promotion). A regra mora
// aqui pra os dois engines falarem igual; o que depende de engine (rodar o
// thunk de valor padrao) chega por callback.
//
// Semantica (decidida no Tier 8 do ROADMAP):
//   - tudo por referencia: instancia e objeto mutavel compartilhado (igual
//     dicionario); metodo muta o receiver, nao tem receiver por valor;
//   - campo nao tem tipo; zero-value = o padrao declarado (`x = 0`) ou nada;
//     puxadinho nasce com a treta embutida zerada (igual Go);
//   - campo e metodo com o mesmo nome na mesma treta e erro (igual Go);
//   - promotion igual Go: o mais raso ganha; empate na mesma profundidade so
//     da erro se alguem acessar o nome ambiguo;
//   - == compara campo a campo (mesma treta + campos iguais).

const (
	TRETA_OBJ     = "TRETA"
	INSTANCIA_OBJ = "INSTANCIA"
	COMBINADO_OBJ = "COMBINADO"
	DESCRITOR_OBJ = "DESCRITOR"
)

// nadaPOO e o zero-value dos campos sem padrao.
var nadaPOO = &Nada{}

// ---------------------------------------------------------------- Treta

// CampoTreta e um campo declarado. Embutida != nil = puxadinho (o campo se
// chama como a treta embutida). Padrao: nil = sem padrao; *Funcao ou
// *CompiledFunction = thunk (roda a cada instancia: `xs = []` nao divide a
// lista entre instancias); qualquer outro valor = constante.
type CampoTreta struct {
	Nome     string
	Embutida *Treta
	Padrao   Object
}

// Treta e o tipo declarado com `treta Nome ... acabou_finalmente`.
type Treta struct {
	Nome   string
	Campos []CampoTreta
	indice map[string]int // nome do campo -> posicao (fixo depois de criado)

	mu      sync.RWMutex // so com o modo concorrente ligado
	metodos map[string]Object
	// nomes: "Treta.metodo" ja montado (o MetodoLigado leva isso a cada
	// `obj.metodo`; montar na hora era uma alocacao por chamada de metodo)
	nomes map[string]string
}

func (t *Treta) Type() ObjectType { return TRETA_OBJ }
func (t *Treta) Inspect() string  { return "<treta " + t.Nome + ">" }

// NovaTreta valida e cria a treta. Devolve a mensagem de erro (ou "").
func NovaTreta(nome string, campos []CampoTreta) (*Treta, string) {
	t := &Treta{Nome: nome, Campos: campos, indice: make(map[string]int, len(campos)), metodos: map[string]Object{}}
	for i, c := range campos {
		if _, dup := t.indice[c.Nome]; dup {
			return nil, fmt.Sprintf("campo `%s` repetido na treta %s", c.Nome, nome)
		}
		t.indice[c.Nome] = i
	}
	return t, ""
}

// IndiceCampo devolve a posicao do campo PROPRIO (sem promotion).
func (t *Treta) IndiceCampo(nome string) (int, bool) {
	i, ok := t.indice[nome]
	return i, ok
}

// Metodo devolve o metodo PROPRIO (sem promotion).
func (t *Treta) Metodo(nome string) (Object, bool) {
	if concorrencia.Load() {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}
	m, ok := t.metodos[nome]
	return m, ok
}

// metodoComNome e o Metodo devolvendo tambem o nome qualificado.
func (t *Treta) metodoComNome(nome string) (Object, string, bool) {
	if concorrencia.Load() {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}
	m, ok := t.metodos[nome]
	if !ok {
		return nil, "", false
	}
	return m, t.nomes[nome], true
}

// NomesMetodos lista os metodos proprios em ordem alfabetica.
func (t *Treta) NomesMetodos() []string {
	if concorrencia.Load() {
		t.mu.RLock()
		defer t.mu.RUnlock()
	}
	out := make([]string, 0, len(t.metodos))
	for n := range t.metodos {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// DefineMetodo pendura o metodo na treta (redeclarar troca, igual gambiarra).
func (t *Treta) DefineMetodo(nome string, fn Object) string {
	if _, ehCampo := t.indice[nome]; ehCampo {
		return fmt.Sprintf("a treta %s ja tem o campo `%s`: metodo nao pode ter o mesmo nome de campo (igual Go)", t.Nome, nome)
	}
	if concorrencia.Load() {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	t.metodos[nome] = fn
	if t.nomes == nil {
		t.nomes = map[string]string{}
	}
	t.nomes[nome] = t.Nome + "." + nome
	return ""
}

// MsgNaoETreta e o erro de `X{...}`/metodo/puxadinho em cima de algo que nao
// e treta.
func MsgNaoETreta(nome string, v Object) string {
	return fmt.Sprintf("`%s` nao e treta (e %s)", nome, NomeTipo(v))
}

// ---------------------------------------------------------------- Combinado

// AssinaturaMetodo e uma linha do combinado: nome + quantos parametros
// (sem contar o receiver).
type AssinaturaMetodo struct {
	Nome      string
	NumParams int
}

// Combinado e a interface: um conjunto de assinaturas. Satisfeito de forma
// implicita por qualquer treta que tenha os metodos.
type Combinado struct {
	Nome    string
	Metodos []AssinaturaMetodo // ja com os dos combinados embutidos, sem repetir
}

func (c *Combinado) Type() ObjectType { return COMBINADO_OBJ }
func (c *Combinado) Inspect() string  { return "<combinado " + c.Nome + ">" }

// NovoCombinado junta as assinaturas proprias com as dos embutidos.
func NovoCombinado(nome string, proprios []AssinaturaMetodo, embutidos []*Combinado) (*Combinado, string) {
	c := &Combinado{Nome: nome}
	visto := map[string]int{}
	add := func(a AssinaturaMetodo) string {
		if n, ok := visto[a.Nome]; ok {
			if n != a.NumParams {
				return fmt.Sprintf("o combinado %s tem o metodo `%s` duas vezes com numero de parametros diferente", nome, a.Nome)
			}
			return ""
		}
		visto[a.Nome] = a.NumParams
		c.Metodos = append(c.Metodos, a)
		return ""
	}
	for _, e := range embutidos {
		for _, a := range e.Metodos {
			if msg := add(a); msg != "" {
				return nil, msg
			}
		}
	}
	for _, a := range proprios {
		if msg := add(a); msg != "" {
			return nil, msg
		}
	}
	return c, ""
}

// ---------------------------------------------------------------- Instancia

// Instancia e um valor de treta (`Ponto{1, 2}`). Os campos ficam na ordem da
// declaracao. Mutavel e por referencia, com a mesma trava das colecoes
// (colecoes.go): so trava com o modo concorrente ligado, e nunca segura a
// propria trava enquanto mexe em outro objeto.
type Instancia struct {
	Tipo   *Treta
	mu     sync.Mutex
	campos []Object
}

func (i *Instancia) Type() ObjectType { return INSTANCIA_OBJ }
func (i *Instancia) Inspect() string  { return i.inspect(nil) }

func (i *Instancia) inspect(emCurso map[Object]bool) string {
	if emCurso[i] {
		return i.Tipo.Nome + "{...}"
	}
	vals := i.Campos()
	partes := make([]string, len(vals))
	for k, v := range vals {
		txt := ""
		if ehColecaoAninhavel(v) {
			if emCurso == nil {
				emCurso = map[Object]bool{}
			}
			emCurso[i] = true
			txt = inspectDentro(v, emCurso)
		} else {
			txt = inspectComAspas(v)
		}
		partes[k] = i.Tipo.Campos[k].Nome + ": " + txt
	}
	delete(emCurso, i)
	return i.Tipo.Nome + "{" + strings.Join(partes, ", ") + "}"
}

// Campos devolve uma COPIA dos valores, na ordem da declaracao.
func (i *Instancia) Campos() []Object {
	if concorrencia.Load() {
		i.mu.Lock()
		defer i.mu.Unlock()
	}
	out := make([]Object, len(i.campos))
	copy(out, i.campos)
	return out
}

// PegaCampo le o campo proprio k.
func (i *Instancia) PegaCampo(k int) Object {
	if concorrencia.Load() {
		i.mu.Lock()
		defer i.mu.Unlock()
	}
	return i.campos[k]
}

// PoeCampo escreve o campo proprio k.
func (i *Instancia) PoeCampo(k int, v Object) {
	if concorrencia.Load() {
		i.mu.Lock()
		defer i.mu.Unlock()
	}
	i.campos[k] = v
}

// ---------------------------------------------------------------- metodo ligado

// MetodoLigado e `obj.metodo` sem chamar ainda: o metodo com o receiver
// grudado. Chamar = chamar Fn com o receiver no primeiro argumento.
type MetodoLigado struct {
	Receptor *Instancia
	Fn       Object // *Funcao (tree-walker) ou *CompiledFunction (VM)
	Nome     string // "Ponto.distancia" (traco de pilha e mensagem)
}

func (m *MetodoLigado) Type() ObjectType { return FUNCAO_OBJ }
func (m *MetodoLigado) Inspect() string  { return "<metodo " + m.Nome + ">" }

// Aridade devolve quantos argumentos a funcao aceita: min, max e se e
// variadica (max nao vale). ok=false se nao for funcao do usuario.
func Aridade(fn Object) (min, max int, variadica, ok bool) {
	switch f := fn.(type) {
	case *Funcao:
		for _, p := range f.Parametros {
			if p.Variadico {
				variadica = true
				continue
			}
			if p.Padrao == nil {
				min++
			}
			max++
		}
		return min, max, variadica, true
	case *CompiledFunction:
		max = f.NumArgs
		if f.Variadic {
			max--
		}
		return f.MinArgs, max, f.Variadic, true
	}
	return 0, 0, false, false
}

// ChecaAridadeMetodo confere argc (SEM o receiver) contra o metodo. Devolve a
// mensagem de erro ou "" — mesma nos dois engines.
func ChecaAridadeMetodo(m *MetodoLigado, argc int) string {
	min, max, variadica, ok := Aridade(m.Fn)
	if !ok {
		return ""
	}
	min, max = min-1, max-1 // o receiver nao conta pra quem chama
	switch {
	case variadica && argc < min:
		return fmt.Sprintf("o metodo %s quer no minimo %d parametro(s), voce mandou %d", m.Nome, min, argc)
	case variadica:
		return ""
	case argc >= min && argc <= max:
		return ""
	case min < max:
		return fmt.Sprintf("o metodo %s quer entre %d e %d parametro(s), voce mandou %d", m.Nome, min, max, argc)
	}
	return fmt.Sprintf("o metodo %s quer %d parametro(s), voce mandou %d", m.Nome, max, argc)
}

// ---------------------------------------------------------------- membros

// alvoMembro e onde `obj.nome` caiu: um campo (caminho ate ele, o ultimo
// indice e o campo) ou um metodo (caminho ate a instancia dona).
type alvoMembro struct {
	caminho []int
	metodo  Object
	dono    *Treta
}

type noBusca struct {
	t       *Treta
	caminho []int
}

// profMaxPuxadinho limita a descida (puxadinho dentro de puxadinho...).
const profMaxPuxadinho = 64

// resolveMembro acha `nome` na treta com promotion (igual Go): procura nivel
// por nivel; no primeiro nivel que achar, um so = esse; mais de um = ambiguo
// (devolve os donos pra mensagem).
func resolveMembro(t *Treta, nome string) (alvoMembro, []string, bool) {
	nivel := []noBusca{{t: t}}
	for prof := 0; len(nivel) > 0 && prof < profMaxPuxadinho; prof++ {
		var achados []alvoMembro
		var donos []string
		var prox []noBusca
		for _, n := range nivel {
			if k, ok := n.t.indice[nome]; ok {
				achados = append(achados, alvoMembro{caminho: junta(n.caminho, k), dono: n.t})
				donos = append(donos, n.t.Nome)
			} else if m, ok := n.t.Metodo(nome); ok {
				achados = append(achados, alvoMembro{caminho: n.caminho, metodo: m, dono: n.t})
				donos = append(donos, n.t.Nome)
			}
			for k, c := range n.t.Campos {
				if c.Embutida != nil {
					prox = append(prox, noBusca{t: c.Embutida, caminho: junta(n.caminho, k)})
				}
			}
		}
		switch len(achados) {
		case 0:
			nivel = prox
			continue
		case 1:
			return achados[0], nil, true
		}
		return alvoMembro{}, donos, false
	}
	return alvoMembro{}, nil, false
}

func junta(caminho []int, k int) []int {
	out := make([]int, len(caminho)+1)
	copy(out, caminho)
	out[len(caminho)] = k
	return out
}

// desce segue o caminho de puxadinhos a partir da instancia.
func (i *Instancia) desce(caminho []int) *Instancia {
	atual := i
	for _, k := range caminho {
		atual = atual.PegaCampo(k).(*Instancia)
	}
	return atual
}

func msgSemMembro(t *Treta, nome string) string {
	return fmt.Sprintf("treta %s nao tem campo nem metodo `%s`", t.Nome, nome)
}

func msgAmbiguo(t *Treta, nome string, donos []string) string {
	return fmt.Sprintf("`%s` e ambiguo em %s: tem em %s na mesma profundidade — usa o caminho completo (tipo obj.%s.%s)",
		nome, t.Nome, strings.Join(donos, " e "), donos[0], nome)
}

// MsgCampoNaoTexto e o erro de `obj[3]` numa instancia.
func MsgCampoNaoTexto(idx Object) string {
	return fmt.Sprintf("campo de treta e por nome (texto), veio %s", NomeTipo(idx))
}

// Membro le `obj.nome`: campo (proprio ou promovido) ou metodo ligado.
// Devolve a mensagem de erro (ou "").
func (i *Instancia) Membro(nome string) (Object, string) {
	t := i.Tipo
	if k, ok := t.indice[nome]; ok {
		return i.PegaCampo(k), ""
	}
	if m, qual, ok := t.metodoComNome(nome); ok {
		return &MetodoLigado{Receptor: i, Fn: m, Nome: qual}, ""
	}
	alvo, donos, ok := resolveMembro(t, nome)
	if !ok {
		if donos != nil {
			return nil, msgAmbiguo(t, nome, donos)
		}
		return nil, msgSemMembro(t, nome)
	}
	if alvo.metodo != nil {
		return &MetodoLigado{Receptor: i.desce(alvo.caminho), Fn: alvo.metodo, Nome: alvo.dono.Nome + "." + nome}, ""
	}
	ult := len(alvo.caminho) - 1
	return i.desce(alvo.caminho[:ult]).PegaCampo(alvo.caminho[ult]), ""
}

// PoeMembro escreve `obj.nome = v` (campo proprio ou promovido). Campo novo
// nao nasce: so os declarados na treta.
func (i *Instancia) PoeMembro(nome string, v Object) string {
	// caminho rapido: campo proprio que nao e puxadinho (o caso comum,
	// `p.x = ...`). Da o mesmo que o resolveMembro, sem as alocacoes da busca.
	if k, ok := i.Tipo.indice[nome]; ok && i.Tipo.Campos[k].Embutida == nil {
		i.PoeCampo(k, v)
		return ""
	}
	alvo, donos, ok := resolveMembro(i.Tipo, nome)
	if !ok {
		if donos != nil {
			return msgAmbiguo(i.Tipo, nome, donos)
		}
		return fmt.Sprintf("treta %s nao tem campo `%s` (campo novo so declarando na treta)", i.Tipo.Nome, nome)
	}
	if alvo.metodo != nil {
		return fmt.Sprintf("`%s` e metodo da treta %s, nao campo — nao da pra atribuir", nome, alvo.dono.Nome)
	}
	ult := len(alvo.caminho) - 1
	dono := i.desce(alvo.caminho[:ult])
	campo := dono.Tipo.Campos[alvo.caminho[ult]]
	if campo.Embutida != nil {
		if msg := confereEmbutida(dono.Tipo, campo, v); msg != "" {
			return msg
		}
	}
	dono.PoeCampo(alvo.caminho[ult], v)
	return ""
}

// confereEmbutida: o campo do puxadinho so aceita instancia daquela treta
// (senao a promotion quebrava depois).
func confereEmbutida(t *Treta, campo CampoTreta, v Object) string {
	if inst, ok := v.(*Instancia); ok && inst.Tipo == campo.Embutida {
		return ""
	}
	return fmt.Sprintf("o campo %s de %s e o puxadinho da treta %s: so aceita %s{...}, veio %s",
		campo.Nome, t.Nome, campo.Embutida.Nome, campo.Embutida.Nome, NomeTipo(v))
}

// ---------------------------------------------------------------- instanciar

// ChamaThunk roda um valor padrao que e thunk (gambiarra sem parametro) no
// engine da vez. Pode devolver *Erro.
type ChamaThunk func(fn Object) Object

func ehThunk(o Object) bool {
	switch o.(type) {
	case *Funcao, *CompiledFunction:
		return true
	}
	return false
}

// zeroDoCampo e o valor do campo que o literal nao passou.
func zeroDoCampo(c CampoTreta, chama ChamaThunk) (Object, string) {
	switch {
	case c.Embutida != nil:
		return MontaInstancia(c.Embutida, nil, nil, chama)
	case c.Padrao == nil:
		return nadaPOO, ""
	case ehThunk(c.Padrao):
		return chama(c.Padrao), ""
	}
	return c.Padrao, ""
}

// MontaInstancia cria a instancia do literal `T{...}`. nomes == nil e
// posicional (valores na ordem; vazio = tudo zero). Devolve *Instancia, ou
// *Erro vindo de um thunk de padrao, ou a mensagem de erro do literal.
func MontaInstancia(t *Treta, nomes []string, valores []Object, chama ChamaThunk) (Object, string) {
	campos := make([]Object, len(t.Campos))
	if nomes == nil && len(valores) > 0 {
		if len(valores) != len(t.Campos) {
			nomesCampos := make([]string, len(t.Campos))
			for k, c := range t.Campos {
				nomesCampos[k] = c.Nome
			}
			return nil, fmt.Sprintf("%s{...} na ordem quer os %d campo(s) (%s), veio %d — ou passa por nome: %s{campo: valor}",
				t.Nome, len(t.Campos), strings.Join(nomesCampos, ", "), len(valores), t.Nome)
		}
		for k, v := range valores {
			if c := t.Campos[k]; c.Embutida != nil {
				if msg := confereEmbutida(t, c, v); msg != "" {
					return nil, msg
				}
			}
			campos[k] = v
		}
		return &Instancia{Tipo: t, campos: campos}, ""
	}
	passou := make([]bool, len(t.Campos))
	for j, nome := range nomes {
		k, ok := t.indice[nome]
		if !ok {
			if alvo, _, achou := resolveMembro(t, nome); achou && alvo.metodo == nil {
				via := t.Campos[alvo.caminho[0]].Nome
				return nil, fmt.Sprintf("`%s` e campo promovido do puxadinho %s: no literal passa ele dentro, tipo %s{%s: %s{%s: ...}}",
					nome, via, t.Nome, via, via, nome)
			}
			return nil, fmt.Sprintf("treta %s nao tem campo `%s`", t.Nome, nome)
		}
		if passou[k] {
			return nil, fmt.Sprintf("campo `%s` repetido no %s{...}", nome, t.Nome)
		}
		if c := t.Campos[k]; c.Embutida != nil {
			if msg := confereEmbutida(t, c, valores[j]); msg != "" {
				return nil, msg
			}
		}
		passou[k] = true
		campos[k] = valores[j]
	}
	for k, c := range t.Campos {
		if passou[k] {
			continue
		}
		v, msg := zeroDoCampo(c, chama)
		if msg != "" {
			return nil, msg
		}
		switch v.(type) {
		case *Erro, *Sair: // o thunk quebrou (ou chamou sai): sobe do jeito que veio
			return v, ""
		}
		campos[k] = v
	}
	return &Instancia{Tipo: t, campos: campos}, ""
}

// ---------------------------------------------------------------- combinado

// Satisfaz diz se o valor cumpre o tipo: treta = e instancia DESSA treta;
// combinado = tem todos os metodos (proprios ou promovidos) com aridade que
// aceita o numero de parametros da assinatura. motivo explica o nao.
func Satisfaz(v Object, tipo Object) (ok bool, motivo string, valido bool) {
	switch tp := tipo.(type) {
	case *Treta:
		if inst, ehInst := v.(*Instancia); ehInst && inst.Tipo == tp {
			return true, "", true
		}
		return false, fmt.Sprintf("esperava %s, veio %s", tp.Nome, NomeTipo(v)), true
	case *Combinado:
		if len(tp.Metodos) == 0 {
			return true, "", true // combinado vazio = qualquer coisa (o `any` do Go)
		}
		inst, ehInst := v.(*Instancia)
		if !ehInst {
			return false, fmt.Sprintf("%s nao tem metodo, entao nao satisfaz %s", NomeTipo(v), tp.Nome), true
		}
		for _, a := range tp.Metodos {
			alvo, _, achou := resolveMembro(inst.Tipo, a.Nome)
			if !achou || alvo.metodo == nil {
				return false, fmt.Sprintf("%s nao satisfaz %s: falta o metodo %s", inst.Tipo.Nome, tp.Nome, a.Nome), true
			}
			min, max, variadica, _ := Aridade(alvo.metodo)
			min, max = min-1, max-1
			if a.NumParams < min || (!variadica && a.NumParams > max) {
				return false, fmt.Sprintf("%s nao satisfaz %s: o metodo %s nao aceita %d parametro(s)", inst.Tipo.Nome, tp.Nome, a.Nome, a.NumParams), true
			}
		}
		return true, "", true
	}
	return false, "", false
}

// ---------------------------------------------------------------- json

// ParCampo e um campo pronto pra virar JSON.
type ParCampo struct {
	Nome  string
	Valor Object
}

// CamposJson achata a instancia igual o encoding/json do Go: campos do
// puxadinho sobem pro objeto de fora; nome repetido fica com o mais raso, e
// empate na mesma profundidade some (igual Go). Ordem: a da declaracao.
func (i *Instancia) CamposJson() []ParCampo {
	type folha struct {
		par  ParCampo
		prof int
	}
	var folhas []folha
	var anda func(inst *Instancia, prof int)
	anda = func(inst *Instancia, prof int) {
		if prof > profMaxPuxadinho {
			return
		}
		vals := inst.Campos()
		for k, c := range inst.Tipo.Campos {
			if c.Embutida != nil {
				if sub, ok := vals[k].(*Instancia); ok {
					anda(sub, prof+1)
				}
				continue
			}
			folhas = append(folhas, folha{ParCampo{c.Nome, vals[k]}, prof})
		}
	}
	anda(i, 0)
	melhor := map[string]int{} // nome -> menor profundidade
	conta := map[string]int{}  // quantos nessa menor profundidade
	for _, f := range folhas {
		p, ok := melhor[f.par.Nome]
		switch {
		case !ok || f.prof < p:
			melhor[f.par.Nome], conta[f.par.Nome] = f.prof, 1
		case f.prof == p:
			conta[f.par.Nome]++
		}
	}
	out := make([]ParCampo, 0, len(folhas))
	for _, f := range folhas {
		if f.prof == melhor[f.par.Nome] && conta[f.par.Nome] == 1 {
			out = append(out, f.par)
		}
	}
	return out
}

// ---------------------------------------------------------------- descritores (VM)

// DescTreta, DescCombinado e DescLiteral sao constantes do bytecode: o que o
// compilador sabe de uma declaracao/literal. Nunca aparecem pro usuario. Tem
// campos exportados pra irem no cache .gsc (gob).
type DescTreta struct {
	Nome   string
	Campos []DescCampo
}

// DescCampo: Embutida = puxadinho (o valor vem da pilha); TemPadrao = o valor
// padrao (constante ou thunk) vem da pilha. Sem nenhum dos dois = zero-value nada.
type DescCampo struct {
	Nome      string
	Embutida  bool
	TemPadrao bool
	Texto     string // como o puxadinho foi escrito (mensagem de erro)
}

type DescCombinado struct {
	Nome      string
	Metodos   []AssinaturaMetodo
	Embutidos []string // como cada embutido foi escrito (os valores vem da pilha)
}

type DescLiteral struct {
	Tipo       string // como o tipo foi escrito (mensagem de erro)
	Nomes      []string
	Posicional bool
	N          int
}

func (d *DescTreta) Type() ObjectType     { return DESCRITOR_OBJ }
func (d *DescTreta) Inspect() string      { return "<descritor treta " + d.Nome + ">" }
func (d *DescCombinado) Type() ObjectType { return DESCRITOR_OBJ }
func (d *DescCombinado) Inspect() string  { return "<descritor combinado " + d.Nome + ">" }
func (d *DescLiteral) Type() ObjectType   { return DESCRITOR_OBJ }
func (d *DescLiteral) Inspect() string    { return "<descritor literal " + d.Tipo + ">" }
