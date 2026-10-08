package object

import (
	"runtime"
	"sort"
)

// ---- Gancho de linha (API Go, compartilhada pelos dois engines) ----
//
// E a infraestrutura que a cobertura do `gs testa --cobertura` usa hoje e que
// um depurador (breakpoint, passo a passo) vai usar depois. Contrato:
//
//   - O gancho e chamado ANTES de cada statement executavel do usuario rodar
//     (a lista sai de ast.StatementsExecutaveis: o que conta e o mesmo nos
//     dois engines, e cada execucao do statement dispara uma vez — volta de
//     laco dispara de novo, ramo que nao rodou nao dispara).
//   - Recebe um *SitioLinha estavel: o mesmo ponteiro toda vez que o mesmo
//     statement roda (da pra usar como chave de mapa). Arquivo e o caminho
//     absoluto do .gs ("" quando o programa nao veio de arquivo); Linha e a
//     linha onde o statement comeca. Nao altere o sitio.
//   - Roda SINCRONO, na goroutine que esta executando o script: bloquear
//     dentro do gancho pausa so aquele fluxo (e o que um breakpoint faz).
//     Com `bora`/paralelo/servidor o gancho e chamado de varias goroutines
//     ao mesmo tempo — quem implementa sincroniza o proprio estado.
//   - Gancho nil = desligado. Tree-walker: Interpreter.DefinirGancho(g) antes
//     do Eval (custo desligado: um teste de nil por statement). VM: compila
//     com compiler.Instrumentar = true (o compilador emite um OpLinha antes de
//     cada statement executavel; sem instrumentar o bytecode e identico ao de
//     sempre, custo zero) e chama vm.DefinirGancho(g) antes do Run. Bytecode
//     instrumentado nunca vai pro cache .gsc.
//
// Pro depurador: o sitio da o ponto de parada (arquivo:linha); a pilha de
// chamadas e as variaveis vem pelo Depurador/Fluxo (mais abaixo), que liga no
// mesmo ponto dos engines e convive com o gancho de linha.

// SitioLinha identifica um statement executavel: arquivo e linha.
type SitioLinha struct {
	Arquivo string
	Linha   int
}

// GanchoLinha e o callback chamado antes de cada statement executavel.
type GanchoLinha func(s *SitioLinha)

// ---- Depurador (API Go de inspecao, compartilhada pelos dois engines) ----
//
// Em cima do mesmo ponto do gancho de linha, o depurador (gs debug) recebe
// tambem uma visao do fluxo que esta rodando: a pilha de chamadas e as
// variaveis de cada quadro. Contrato:
//
//   - Depurador.Linha e chamado ANTES de cada statement executavel (os mesmos
//     sitios do GanchoLinha), sincrono, na goroutine do fluxo. Bloquear ali
//     pausa so aquele fluxo — e assim que o breakpoint para.
//   - O Fluxo so vale DURANTE a chamada (enquanto o fluxo esta parado ali
//     dentro) e os metodos dele devem ser chamados NA PROPRIA goroutine do
//     fluxo (o depurador manda o trabalho pra la enquanto esta pausado).
//   - Fluxo = goroutine do programa: o principal e o 1; cada `bora`, cada
//     chamada de handler do servidor e cada tarefa do paralelo vira um fluxo
//     novo. Gambiarra chamada por builtin (mapeia, filtra...) roda no fluxo de
//     quem chamou o builtin (empilha quadro, nao cria fluxo).
//   - Depurador.FluxoAcabou avisa que um fluxo (que nao o principal) terminou.
//   - Avaliar codigo pelo Fluxo roda codigo do programa: o gancho dispara de
//     novo (reentrante) na mesma goroutine — quem implementa ignora.
//   - Custo: desligado (nil) nada muda. Ligado, os engines rastreiam os
//     quadros por goroutine (o tree-walker descobre a goroutine a cada
//     statement) — so se paga com o depurador ligado. Na VM o depurador
//     exige bytecode instrumentado (compiler.Instrumentar), igual o gancho.

// Quadro e um nivel da pilha de chamadas visto pelo depurador.
type Quadro struct {
	Nome    string // gambiarra (ou "<principal>", "<modulo>", "<anonima>"...)
	Arquivo string // .gs onde o codigo do quadro mora ("" = sem arquivo)
	Linha   int    // linha atual do quadro (no mais interno, a do statement)
}

// Variavel e um nome visivel num quadro e o valor dele agora.
type Variavel struct {
	Nome  string
	Valor Object
}

// Fluxo e a visao do depurador sobre o fluxo parado no gancho. Quadro 0 e o
// mais interno (onde parou); o ultimo e o de baixo (<principal> no fluxo 1).
type Fluxo interface {
	ID() int64
	// Profundidade e len(Quadros()) sem montar a lista (o passo a passo
	// consulta a cada statement).
	Profundidade() int
	Quadros() []Quadro
	// Locais devolve as variaveis do quadro q (parametros, locais e as
	// capturadas pela closure), ordenadas pelo nome. Quadro de topo de
	// programa/modulo nao tem locais: o que ele ve sao as Globais.
	Locais(q int) []Variavel
	// Globais devolve as globais do arquivo do quadro q (principal ou modulo),
	// ordenadas pelo nome.
	Globais(q int) []Variavel
	// Avalia roda `fonte` no contexto do quadro q e devolve o valor (um
	// *Erro levantado se nao deu).
	Avalia(q int, fonte string) Object
}

// Depurador recebe os statements e o fim dos fluxos.
type Depurador interface {
	Linha(s *SitioLinha, f Fluxo)
	FluxoAcabou(id int64)
}

// InfoDepuracao acompanha uma CompiledFunction compilada com instrumentacao
// (compiler.Instrumentar): o que a VM precisa pra mostrar nomes no depurador.
// nil no bytecode normal.
type InfoDepuracao struct {
	Arquivo string   // .gs onde a gambiarra foi escrita
	Locais  []string // slot do local -> nome ("" = temporario do compilador)
	Livres  []string // indice da freevar -> nome
	Globais *TabelaGlobais
}

// TabelaGlobais lista as globais de um arquivo (principal ou modulo): nome e
// slot no array de globais da VM. Preenchida no fim da compilacao do arquivo.
type TabelaGlobais struct {
	Nomes []string
	Slots []int
}

// IDGoroutine devolve o id da goroutine atual (lido do cabecalho do
// runtime.Stack: "goroutine 123 [running]:..."). So o depurador usa.
func IDGoroutine() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	b := buf[:n]
	const pref = "goroutine "
	if len(b) < len(pref) {
		return 0
	}
	var id int64
	for _, c := range b[len(pref):] {
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + int64(c-'0')
	}
	return id
}

// VariaveisOrdenadas ordena pelo nome (e o formato que os engines devolvem).
func VariaveisOrdenadas(vs []Variavel) []Variavel {
	sort.Slice(vs, func(a, b int) bool { return vs[a].Nome < vs[b].Nome })
	return vs
}
