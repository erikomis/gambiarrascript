package vm

import (
	"fmt"
	"io"
	"math"
	"strings"
	"sync"

	"gambiarrascript/code"
	"gambiarrascript/compiler"
	"gambiarrascript/interpreter"
	"gambiarrascript/object"
)

const (
	// StackInicial e o tamanho com que a pilha NASCE; ela cresce sob demanda
	// (dobrando no garanteEspaco, na reserva de cada frame). Antes toda VM ja
	// nascia com os 16k slots — 256 KB por VM — o que fazia cada goroutine do
	// `bora` e cada chamada de gambiarra vinda de mapeia/filtra custar um
	// quarto de mega.
	StackInicial = 512
	// GlobalsMin e o piso do array de globais. O tamanho real vem do
	// compilador (Bytecode.NumGlobals): reservar MaxGlobals de cara custava
	// 1 MB zerado em toda VM, mesmo pra um script de tres linhas.
	GlobalsMin = 16
	StackSize  = 16384 // mantido pra compatibilidade; a pilha nao tem mais teto fixo
	MaxFrames  = 1024
	MaxGlobals = 65536 // teto de globais que o compilador endereça (indice de 2 bytes)
)

var (
	DEU_BOM  = &object.Booleano{Value: true}
	DEU_RUIM = &object.Booleano{Value: false}
	NADA     = &object.Nada{}
)

type Frame struct {
	fn          *object.CompiledFunction
	ip          int
	basePointer int
	// callPos e o offset do OpCall no bytecode do frame PAI — usado pra
	// resolver a linha do call site ao montar o traço de pilha (lazy, so em
	// caminho de erro). 0 no frame raiz.
	callPos int
}

// tryHandler e uma entrada da pilha de arruma/quebrou: o endereco do catch e
// a PROFUNDIDADE de frames (framesIdx) em que o OpTry rodou — necessario pra
// desempilhar frames ate o dono do try quando o erro estoura em funcao
// chamada dentro do bloco.
type tryHandler struct {
	catchAddr int
	frameIdx  int
}

type VM struct {
	constants []object.Object
	inst      code.Instructions
	linhas    []object.LinhaPC // tabela pc->linha do fluxo principal
	maxStack  int              // teto da pilha do fluxo principal (Bytecode.MaxStack)
	stack     []object.Object
	sp        int
	globals   []object.Object
	frames    []*Frame
	framesIdx int

	// erros: pilha de handlers de arruma/quebrou. Throw pega o mais interno.
	errStack []tryHandler

	out io.Writer

	builtinIdx map[string]int
	builtins   map[string]*object.Builtin

	// num e a arena de Numeros DESTA VM. Fica por valor (nao ponteiro) pra nao
	// custar indirecao no hot path, e e por-VM porque ArenaNum nao e
	// thread-safe: cada VM roda numa goroutine so.
	num object.ArenaNum

	// subVMs reusa as VMs das chamadas SINCRONAS vindas do interpreter
	// (mapeia/filtra/reduz/ordena_com chamam a gambiarra do usuario uma vez por
	// elemento). Compartilhado entre a VM raiz e os clones; um sync.Pool porque
	// o `bora` pode disparar essas chamadas de varias goroutines.
	subVMs *sync.Pool

	// modulos e o cache do importa: cada modulo roda uma vez so por processo.
	// Compartilhado com os clones (bora, sub-VMs) — vive junto com as globais.
	modulos *object.Modulos

	// gancho de linha (object/gancho.go) e a tabela de sitios do bytecode
	// instrumentado. So o OpLinha le os dois — e ele so existe com
	// compiler.Instrumentar, entao o caminho normal nem olha pra ca.
	gancho object.GanchoLinha
	sitios []*object.SitioLinha
}

func New(bytecode *compiler.Bytecode, out io.Writer) *VM {
	return NovaComInterp(bytecode, out, interpreter.New(out))
}

// NovaComInterp cria a VM reusando um interpretador ja configurado. Util pra
// quem precisa ler estado do interp depois (ex.: `gs testa --vm` que le os
// contadores de espera()/afirma() via interp.TotaisTeste()).
func NovaComInterp(bytecode *compiler.Bytecode, out io.Writer, interp *interpreter.Interpreter) *VM {
	bidx := map[string]int{}
	for i, n := range compiler.BuiltinNomes() {
		bidx[n] = i
	}
	vm := &VM{
		constants:  bytecode.Constants,
		inst:       bytecode.Instructions,
		linhas:     bytecode.Linhas,
		maxStack:   bytecode.MaxStack,
		stack:      make([]object.Object, StackInicial),
		globals:    make([]object.Object, tamanhoGlobals(bytecode.NumGlobals)),
		frames:     novosFrames(),
		subVMs:     &sync.Pool{},
		modulos:    &object.Modulos{},
		builtinIdx: bidx,
		builtins:   interp.BuiltinsVisiveis(),
		out:        out,
		sitios:     bytecode.Sitios,
	}
	// gancho: os builtins de ordem superior (mapeia, filtra, reduz...) vem do
	// interpreter e chamam applyFunction — que delega pra ca quando a funcao
	// do usuario e uma CompiledFunction (bytecode).
	interp.ChamaCompilada = vm.chamaCompilada
	return vm
}

// disparaLinha chama o gancho com o sitio do operando do OpLinha.
func (vm *VM) disparaLinha(operando []byte) {
	if vm.gancho != nil {
		vm.gancho(vm.sitios[code.ReadUint16(operando)])
	}
}

// DefinirGancho liga o gancho de linha (nil desliga). Chama antes do Run: os
// clones (bora, sub-VMs de mapeia/importa) copiam o gancho quando nascem. So
// dispara em bytecode compilado com compiler.Instrumentar.
func (vm *VM) DefinirGancho(g object.GanchoLinha) { vm.gancho = g }

// chamaCompilada executa uma CompiledFunction de forma SINCRONA numa VM
// clonada (compartilha globals/constants/builtins). Erros de runtime viram
// *object.Erro (os builtins ja propagam via isError).
func (vm *VM) chamaCompilada(cf *object.CompiledFunction, args []object.Object) (res object.Object) {
	if len(args) != cf.NumArgs {
		return &object.Erro{
			Message: fmt.Sprintf("essa gambiarra quer %d parametro(s), voce mandou %d", cf.NumArgs, len(args)),
			Kind:    "runtime",
		}
	}
	sub := vm.pegaSubVM()
	defer vm.devolveSubVM(sub)
	topo := cf.NumLocals
	if topo < len(args) {
		topo = len(args)
	}
	sub.garanteEspaco(topo + folga(cf))
	for i, a := range args {
		sub.stack[i] = a
	}
	// reserva os slots de locals (igual OpCall)
	sub.limpaLocais(0, cf)
	sub.sp = topo
	fr0 := sub.frameEm(0)
	fr0.fn = cf
	fr0.ip = 0
	fr0.basePointer = 0
	fr0.callPos = 0
	sub.framesIdx = 1
	defer func() {
		if r := recover(); r != nil {
			if vme, ok := r.(VMError); ok {
				if vme.sai != nil {
					res = vme.sai
					return
				}
				res = vme.err
				return
			}
			res = &object.Erro{Message: fmt.Sprintf("panico na gambiarra: %v", r), Kind: "runtime"}
		}
	}()
	if err := sub.execFrame(sub.currentFrame()); err != nil {
		if enc, ok := err.(erroNaoCapturado); ok {
			return enc.err // preserva Line/Kind do erro original
		}
		// sai() dentro da gambiarra chamada por um builtin (mapeia,
		// ordena_com...): devolve o Sair pro builtin repassar e o
		// OpCallBuiltin desenrolar ate o Run — nao vira erro "sai com codigo".
		if sr, ok := err.(SaiRequisicao); ok {
			return &object.Sair{Codigo: sr.Codigo}
		}
		return &object.Erro{Message: err.Error(), Kind: "runtime"}
	}
	// apos OpReturn/OpReturnNada o valor fica em stack[sp]
	sub.sp--
	return sub.stack[sub.sp]
}

// importa devolve o namespace do modulo, rodando o corpo dele se for a
// primeira vez neste processo (cache compartilhado com as goroutines). O
// corpo roda numa sub-VM sincrona, igual uma gambiarra chamada por builtin.
func (vm *VM) importa(mod *object.Modulo, atual string, linha int) object.Object {
	valor, _, falha := vm.modulos.Importa(atual, mod.Caminho, func() (object.Object, any, object.Object) {
		if mod.Corpo == nil {
			// o principal: o cache sempre acusa o ciclo antes de chegar aqui
			return nil, nil, &object.Erro{Message: "importa circular: o modulo principal nao pode ser importado", Kind: object.KindModulo}
		}
		switch r := vm.chamaCompilada(mod.Corpo, nil).(type) {
		case *object.Erro:
			if !r.Handled {
				return nil, nil, r
			}
		case *object.Sair:
			return nil, nil, r
		}
		ns := object.NamespaceModulo(mod.Nomes, func(nome string) (object.Object, bool) {
			for i, n := range mod.Nomes {
				if n == nome {
					return vm.globals[mod.Slots[i]], true
				}
			}
			return nil, false
		})
		return ns, nil, nil
	})
	switch f := falha.(type) {
	case *object.Sair:
		panic(VMError{sai: f})
	case *object.Erro:
		panic(VMError{err: object.ComLinha(f, linha)})
	}
	return valor
}

// pegaSubVM tira uma VM do pool (ou clona uma nova) pra rodar UMA chamada
// sincrona de gambiarra vinda de um builtin de ordem superior. Antes cada
// chamada clonava — mapear 5 mil elementos alocava mais de 1 GB so de pilhas.
func (vm *VM) pegaSubVM() *VM {
	if v, ok := vm.subVMs.Get().(*VM); ok {
		return v
	}
	return vm.clone()
}

// devolveSubVM limpa o estado da sub-VM (pra nao segurar referencia viva dos
// valores da chamada anterior) e devolve pro pool.
func (vm *VM) devolveSubVM(sub *VM) {
	clear(sub.stack)
	sub.sp = 0
	sub.framesIdx = 0
	sub.errStack = sub.errStack[:0]
	vm.subVMs.Put(sub)
}

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

// push empilha um valor SEM checar capacidade: quem entra num frame ja
// reservou NumLocals+MaxStack (o teto que o compilador calculou, ver
// compiler.MaxPilha), entao a pilha nunca enche no meio do frame. Funcao sem
// teto conhecido (MaxStack 0) passa pelo caminho checado: a reserva vira
// folgaSemTeto e e refeita a cada volta de laco (OpJump pra tras). Com a tag
// gsdebugpilha o push confere o teto a cada empilhada (checaPilha).
func (vm *VM) push(o object.Object) {
	if checaPilha {
		vm.confereTeto()
	}
	vm.stack[vm.sp] = o
	vm.sp++
}

// folga e quantos slots de trabalho reservar acima dos locals de um frame de
// fn: o MaxStack calculado pelo compilador ou, sem ele, a folgaSemTeto.
func folga(fn *object.CompiledFunction) int {
	if fn.MaxStack > 0 {
		return fn.MaxStack
	}
	return folgaSemTeto(fn)
}

// folgaSemTeto e a reserva do caminho checado. Nenhum opcode empilha mais de
// 1 alem do que tira (o espalhaArgs e o abreMetodo reservam o proprio
// espaco), e sem voltar pra tras cada instrucao roda no maximo uma vez — entao
// 2 slots por byte de bytecode cobrem tudo ate o proximo OpJump pra tras, onde
// a reserva e refeita.
func folgaSemTeto(fn *object.CompiledFunction) int {
	return 2*len(fn.Bytecode) + 2
}

// tamanhoGlobals decide o tamanho do array de globais a partir do que o
// compilador contou. Bytecode antigo (cache .gsc gravado por uma versao sem o
// campo) vem com 0 — cai no teto, que e o comportamento de antes.
func tamanhoGlobals(n int) int {
	if n <= 0 {
		return MaxGlobals
	}
	if n < GlobalsMin {
		return GlobalsMin
	}
	if n > MaxGlobals {
		return MaxGlobals
	}
	return n
}

// garanteEspaco cresce a pilha (dobrando) ate caber `topo` slots. E a UNICA
// forma de a pilha crescer: a reserva de cada frame (locals + MaxStack, no
// OpCall/OpTailCall/chamaCompilada/bora/Run e no unwind pos-catch) e os pontos
// que abrem argumentos de uma vez (espalhaArgs, abreMetodo). Nao tem teto
// rigido: quem limita recursao infinita e o MaxFrames, que ja da erro limpo.
func (vm *VM) garanteEspaco(topo int) {
	if topo <= len(vm.stack) {
		return
	}
	novo := len(vm.stack) * 2
	for novo < topo {
		novo *= 2
	}
	nova := make([]object.Object, novo)
	copy(nova, vm.stack)
	vm.stack = nova
}
func (vm *VM) pop() object.Object { vm.sp--; return vm.stack[vm.sp] }

func (vm *VM) currentFrame() *Frame { return vm.frames[vm.framesIdx-1] }
func (vm *VM) popFrame() *Frame     { vm.framesIdx--; return vm.frames[vm.framesIdx] }

// novosFrames devolve o array de slots de frame (ponteiros nil). Os *Frame sao
// alocados sob demanda por frameEm e REUSADOS nas chamadas seguintes na mesma
// profundidade — assim so alocamos ate a profundidade maxima de chamada do
// programa (nao os 1024 slots de uma vez).
func novosFrames() []*Frame {
	return make([]*Frame, MaxFrames)
}

// frameEm devolve o *Frame do slot, alocando na primeira vez e reusando depois.
func (vm *VM) frameEm(idx int) *Frame {
	fr := vm.frames[idx]
	if fr == nil {
		fr = &Frame{}
		vm.frames[idx] = fr
	}
	return fr
}

// empurraFrame reusa o *Frame do topo (aloca so na primeira visita aquela
// profundidade), setando seus campos sem alocar por chamada. Faz o bounds-check
// de overflow (recursao funda demais).
func (vm *VM) empurraFrame(fn *object.CompiledFunction, bp, callPos int) *Frame {
	if vm.framesIdx >= MaxFrames {
		panic(VMError{err: &object.Erro{Message: fmt.Sprintf("recursao funda demais (passou de %d chamadas) — usa recursao em cauda (funciona f(...)) ou um laco", MaxFrames), Kind: "runtime"}})
	}
	fr := vm.frameEm(vm.framesIdx)
	fr.fn = fn
	fr.ip = 0
	fr.basePointer = bp
	fr.callPos = callPos
	vm.framesIdx++
	return fr
}

// clone devolve uma VM nova pronta pra rodar em goroutine: compartilha
// constants/globals/builtins/out com a original (igual o tree-walker, que
// compartilha o mesmo Environment), mas tem stack/frames proprios.
func (vm *VM) clone() *VM {
	return &VM{
		constants:  vm.constants,
		inst:       vm.inst,
		linhas:     vm.linhas,
		maxStack:   vm.maxStack,
		stack:      make([]object.Object, StackInicial),
		sp:         0,
		globals:    vm.globals, // slice compartilhado — pagadores por concorrencia
		frames:     novosFrames(),
		subVMs:     vm.subVMs,
		modulos:    vm.modulos,
		builtinIdx: vm.builtinIdx,
		builtins:   vm.builtins,
		out:        vm.out,
		gancho:     vm.gancho,
		sitios:     vm.sitios,
	}
}

// execBoraCall dispara a chamada (callee + args ja empilhados) numa goroutine
// em VM separada e empurra o *Futuro correspondente na pilha da VM atual.
// args layout: stack[sp-1-argc .. sp-2] sao args; stack[sp-1] e o callee.
func (vm *VM) execBoraCall(argc int) {
	callee := vm.stack[vm.sp-1]
	args := make([]object.Object, argc)
	copy(args, vm.stack[vm.sp-1-argc:vm.sp-1])
	vm.sp -= argc + 1

	// liga o modo concorrente ANTES do `go` (ver object/concorrencia.go)
	object.AtivaConcorrencia()

	fut := object.NovoFuturo()
	if m, ok := callee.(*object.MetodoLigado); ok {
		// `bora obj.metodo(args)`: receiver na frente; aridade errada vai pro
		// futuro (igual o tree-walker)
		if msg := object.ChecaAridadeMetodo(m, len(args)); msg != "" {
			e := &object.Erro{Message: msg, Kind: "runtime"}
			fr := vm.currentFrame()
			if l := fr.fn.LinhaDoPC(fr.ip); l > 0 {
				e.Line = l
				e.Message = fmt.Sprintf("deu ruim na linha %d: %s", l, e.Message)
			}
			fut.Resolve(e)
			vm.push(fut)
			return
		}
		args = append([]object.Object{m.Receptor}, args...)
		callee = m.Fn
	}
	switch fn := callee.(type) {
	case *object.CompiledFunction:
		clone := vm.clone()
		// monta o frame inicial: args entram como locals a partir de bp=0,
		// com aridade/varargs/default iguais a chamada normal
		clone.garanteEspaco(len(args))
		copy(clone.stack, args)
		if e := clone.ajustaArgs(fn, 0, len(args)); e != nil {
			// igual o tree-walker: o erro de aridade vai pro futuro, com a
			// linha do `bora`
			fr := vm.currentFrame()
			if l := fr.fn.LinhaDoPC(fr.ip); l > 0 {
				e.Line = l
				e.Message = fmt.Sprintf("deu ruim na linha %d: %s", l, e.Message)
			}
			fut.Resolve(e)
			break
		}
		// reserva os slots de locals (igual OpCall) pra pilha de trabalho nao
		// pisar em cima de local do corpo.
		clone.limpaLocais(0, fn)
		clone.sp = fn.NumLocals
		frame := &Frame{fn: fn, ip: 0, basePointer: 0}
		clone.frames[0] = frame
		clone.framesIdx = 1
		// frame da goroutine no traco (`em <bora:g> (linha N)`), igual o
		// tree-walker
		quadroBora := object.StackFrame{Funcao: "<bora:" + fn.Name + ">"}
		if fr := vm.currentFrame(); fr != nil {
			quadroBora.Line = fr.fn.LinhaDoPC(fr.ip)
		}
		go func(c *VM, f *object.Futuro) {
			defer func() {
				if r := recover(); r != nil {
					if vme, ok := r.(VMError); ok {
						f.Resolve(vme.err)
						return
					}
					f.Resolve(&object.Erro{Message: fmt.Sprintf("panico dentro do `bora`: %v", r), Kind: "runtime"})
				}
			}()
			if err := c.execFrame(c.currentFrame()); err != nil {
				if enc, ok := err.(erroNaoCapturado); ok {
					// preserva Line/Kind do erro original
					enc.err.Stack = append([]object.StackFrame{quadroBora}, enc.err.Stack...)
					f.Resolve(enc.err)
					return
				}
				f.Resolve(&object.Erro{Message: err.Error(), Kind: "runtime"})
				return
			}
			// apos OpReturn, valor fica em stack[sp]
			c.sp--
			f.Resolve(c.stack[c.sp])
		}(clone, fut)
	case *object.Builtin:
		go func(f *object.Futuro, b *object.Builtin, argv []object.Object) {
			defer func() {
				if r := recover(); r != nil {
					f.Resolve(&object.Erro{Message: fmt.Sprintf("panico dentro do `bora`: %v", r), Kind: "runtime"})
				}
			}()
			f.Resolve(b.Fn(argv))
		}(fut, fn, args)
	default:
		panic(VMError{err: &object.Erro{Message: fmt.Sprintf("bora: nao da pra chamar %s", callee.Type()), Kind: "runtime"}})
	}
	vm.push(fut)
}

// espalhaArgs abre os args `...lista` da chamada no topo da pilha. Layout de
// entrada: [a0 .. an-1, callee] com n = len(mascara) ('1' = espalhado); sai
// [args abertos..., callee] e devolve o argc novo.
func (vm *VM) espalhaArgs(mascara string) int {
	n := len(mascara)
	base := vm.sp - 1 - n
	callee := vm.stack[vm.sp-1]
	args := make([]object.Object, 0, n)
	for i := 0; i < n; i++ {
		v := vm.stack[base+i]
		if mascara[i] != '1' {
			args = append(args, v)
			continue
		}
		l, ok := v.(*object.Lista)
		if !ok {
			panic(VMError{err: &object.Erro{Message: "so da pra espalhar lista, veio " + object.NomeTipo(v), Kind: "runtime"}})
		}
		args = append(args, l.Visao()...)
	}
	vm.garanteEspaco(base + len(args) + 1)
	copy(vm.stack[base:], args)
	vm.stack[base+len(args)] = callee
	vm.sp = base + len(args) + 1
	return len(args)
}

// erroAridade confere argc contra a gambiarra (mesmas mensagens do OpCall):
// variadic aceita >= MinArgs; com default, entre MinArgs e NumArgs; sem
// nenhum dos dois, exatamente NumArgs.
func erroAridade(cf *object.CompiledFunction, argc int) *object.Erro {
	if cf.Variadic {
		if argc < cf.MinArgs {
			return &object.Erro{Message: fmt.Sprintf("essa gambiarra quer no minimo %d parametro(s), voce mandou %d", cf.MinArgs, argc), Kind: "runtime"}
		}
		return nil
	}
	minA := cf.MinArgs
	if minA == 0 {
		minA = cf.NumArgs
	}
	if argc >= minA && argc <= cf.NumArgs {
		return nil
	}
	if cf.MinArgs > 0 && cf.MinArgs < cf.NumArgs {
		return &object.Erro{Message: fmt.Sprintf("essa gambiarra quer entre %d e %d parametro(s), voce mandou %d", cf.MinArgs, cf.NumArgs, argc), Kind: "runtime"}
	}
	return &object.Erro{Message: fmt.Sprintf("essa gambiarra quer %d parametro(s), voce mandou %d", cf.NumArgs, argc), Kind: "runtime"}
}

// ajustaArgs valida a aridade e arruma os argc args em stack[bp:] pros slots
// da gambiarra: junta os extras no ...resto e completa os que faltam com NADA
// (o prologo troca pelo default). Mesmo trabalho que o OpCall faz inline;
// devolve o erro em vez de jogar porque o bora entrega ele no futuro.
func (vm *VM) ajustaArgs(cf *object.CompiledFunction, bp, argc int) *object.Erro {
	if e := erroAridade(cf, argc); e != nil {
		return e
	}
	topo := bp + cf.NumLocals
	if topo < bp+argc {
		topo = bp + argc
	}
	vm.garanteEspaco(topo + folga(cf))
	if cf.Variadic && argc >= cf.NumArgs {
		variadicIdx := cf.NumArgs - 1
		resto := make([]object.Object, argc-variadicIdx)
		copy(resto, vm.stack[bp+variadicIdx:bp+argc])
		vm.stack[bp+variadicIdx] = object.NovaLista(resto)
		argc = cf.NumArgs
	}
	for i := argc; i < cf.NumArgs; i++ {
		if cf.Variadic && i == cf.NumArgs-1 {
			vm.stack[bp+i] = object.NovaLista([]object.Object{})
		} else {
			vm.stack[bp+i] = NADA
		}
	}
	return nil
}

// Run executa o bytecode. frame e ip reciclados entre chamadas via execFrame.
func (vm *VM) Run() error {
	main := &object.CompiledFunction{Name: "<main>", Bytecode: vm.inst, NumLocals: 0, Linhas: vm.linhas, MaxStack: vm.maxStack}
	vm.garanteEspaco(folga(main))
	fr0 := vm.frameEm(0)
	fr0.fn = main
	fr0.ip = 0
	fr0.basePointer = 0
	fr0.callPos = 0
	vm.framesIdx = 1

	err := vm.execFrame(vm.frames[0])
	// Programa que termina via `funciona` no top-level sai por OpReturn: o valor
	// fica em stack[sp-1] (push) e o frame principal e desempilhado (framesIdx=0).
	// O fim normal (fallthrough/OpPop) deixa o valor em stack[sp], onde
	// LastPoppedStackElem le. Ajusta o sp pra os dois casos convergirem.
	if err == nil && vm.framesIdx == 0 {
		vm.sp--
	}
	return err
}

// erroNaoCapturado e o Go error devolvido quando um erro de runtime da VM
// estoura sem handler. Carrega o *object.Erro original (com Line/Kind/...)
// pra quem chamou (chamaCompilada, execBoraCall) nao perder a posicao.
type erroNaoCapturado struct{ err *object.Erro }

func (e erroNaoCapturado) Error() string { return e.err.Message }

// ErroDoRun extrai o *object.Erro de um erro devolvido por Run (nil se o
// erro nao veio do runtime do script). O CLI usa pra imprimir o traço de
// pilha igual o tree-walker.
func ErroDoRun(err error) *object.Erro {
	if enc, ok := err.(erroNaoCapturado); ok {
		return enc.err
	}
	return nil
}

// execFrame executa a partir de um frame ate o retorno dele (ou OpHalt).
// Erros propagam como VMError ate achar um handler (OpTry) ou ate o top
// (Run devolve como Go error).
func (vm *VM) execFrame(frame *Frame) error {
	return vm.execDesde(frame, vm.framesIdx)
}

// execDesde e o loop ITERATIVO da VM: OpCall/OpReturn trocam o frame local
// sem recursao Go, e a execucao devolve o controle quando framesIdx cair
// abaixo de baseIdx (o frame que iniciou a invocacao retornou). O baseIdx e
// repassado no resume pos-catch — um catch dentro de funcao continua depois
// no chamador, nao para no retorno da funcao.
func (vm *VM) execDesde(frame *Frame, baseIdx int) (errRet error) {
	// Recupera panics transformando em VMError — toda construcao de erro
	// runtime usa panic(VMError{...}) por simplicidade.
	defer func() {
		if r := recover(); r != nil {
			if vme, ok := r.(VMError); ok {
				// sai(codigo): nao e erro nem passa por handler (nao da pra
				// capturar com arruma/quebrou). Desenrola direto pro runner.
				if vme.sai != nil {
					errRet = SaiRequisicao{Codigo: vme.sai.Codigo}
					return
				}
				// amarra a linha do fonte (tabela pc->linha) e formata a
				// mensagem igual o tree-walker ("deu ruim na linha N: ...").
				// So pra erro cru de runtime: builtins ja vem formatados.
				// `frame` e capturado por referencia: aponta pro frame que
				// estava rodando na hora do panic (o loop reatribui a var).
				// Erro cru de runtime (sem linha e sem prefixo) ganha a linha e o
				// prefixo aqui. Erros que ja vem formatados de um builtin (ex.:
				// funcao chamada dentro de reduz/mapeia) comecam com "deu ruim"
				// e NAO devem ser re-prefixados — senao vira "deu ruim ... deu ruim".
				if vme.err.Line == 0 && vme.err.Kind == "runtime" && !strings.HasPrefix(vme.err.Message, "deu ruim") {
					if l := frame.fn.LinhaDoPC(frame.ip); l > 0 {
						vme.err.Line = l
						vme.err.Message = fmt.Sprintf("deu ruim na linha %d: %s", l, vme.err.Message)
					}
				}
				vm.handleVMError(vme.err, vme.quadro)
				// apos handle: ou temos handler (continua) ou propaga
				errRet = nil
				// se ainda ha erro pendente (sem handler), sinalizamos
				if vm.framesIdx == 0 {
					// top-level sem handler: erro fatal
					errRet = erroNaoCapturado{err: vme.err}
					return
				}
				// handler achado — resume no frame que registrou o try (o
				// unwinding ja desempilhou os intermediarios), MANTENDO o
				// baseIdx original desta invocacao.
				errRet = vm.execDesde(vm.currentFrame(), baseIdx)
				return
			}
			panic(r)
		}
	}()

	fn := frame.fn
	ip := frame.ip
	for ip < len(fn.Bytecode) {
		frame.ip = ip // sync pro recover saber onde o erro estourou
		op := code.Opcode(fn.Bytecode[ip])
		switch op {
		case code.OpConstant:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			vm.push(vm.constants[idx])
		case code.OpPop:
			vm.pop()
			ip++
		case code.OpHalt:
			return nil
		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod, code.OpPow,
			code.OpBAnd, code.OpBOr, code.OpBXor, code.OpLShift, code.OpRShift:
			vm.execBinario(op)
			ip++
		case code.OpTrue:
			vm.push(DEU_BOM)
			ip++
		case code.OpFalse:
			vm.push(DEU_RUIM)
			ip++
		case code.OpNada:
			vm.push(NADA)
			ip++
		case code.OpBinConst:
			// superinstrucao: aplica a operacao entre o topo e a constante NO
			// LUGAR — sem push/pop da constante e com um dispatch so.
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			sub := code.Opcode(fn.Bytecode[ip+3])
			ip += 4
			esq := vm.stack[vm.sp-1]
			dir := vm.constants[idx]
			if ehComparacao(sub) {
				vm.stack[vm.sp-1] = vm.comparacao(sub, esq, dir)
			} else {
				vm.stack[vm.sp-1] = vm.binario(sub, esq, dir)
			}
		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan, code.OpGreaterEqual, code.OpMenor, code.OpMenorEqual:
			vm.execComparacao(op)
			ip++
		case code.OpMinus:
			vm.execMinus()
			ip++
		case code.OpNao:
			vm.push(boolNativo(!ehVerdade(vm.pop())))
			ip++
		case code.OpBNot:
			o := vm.pop()
			n, ok := o.(*object.Numero)
			if !ok || !n.EhInt {
				panic(VMError{err: &object.Erro{Message: "~ espera inteiro", Kind: "runtime"}})
			}
			vm.push(vm.num.Int(^n.Int))
			ip++
		case code.OpMostra:
			fmt.Fprintln(vm.out, vm.pop().Inspect())
			ip++
		case code.OpGetGlobal:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			if idx >= len(vm.globals) {
				// global enderecada mas nunca escrita (ex.: so atribuida num
				// ramo que nao rodou): vale `nada`, igual ao tree-walker.
				vm.push(NADA)
			} else if object.ConcorrenciaAtiva() {
				vm.push(pegaGlobalTravado(vm.globals, idx))
			} else {
				vm.push(vm.globals[idx])
			}
		case code.OpSetGlobal:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			v := vm.pop()
			if object.ConcorrenciaAtiva() {
				poeGlobalTravado(vm.globals, idx, v)
			} else {
				vm.globals[idx] = v
			}
		case code.OpJump:
			pos := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			if pos < ip && fn.MaxStack == 0 {
				// caminho checado (funcao sem teto): refaz a reserva a cada
				// volta de laco, ver folgaSemTeto
				vm.garanteEspaco(vm.sp + folgaSemTeto(fn))
			}
			ip = pos
		case code.OpJumpIfFalse:
			pos := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			val := vm.pop()
			if !ehVerdade(val) {
				ip = pos
			} else {
				ip += 3
			}
		case code.OpJumpIfTrue:
			pos := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			val := vm.pop()
			if ehVerdade(val) {
				ip = pos
			} else {
				ip += 3
			}
		case code.OpArray:
			n := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			elems := make([]object.Object, n)
			copy(elems, vm.stack[vm.sp-n:vm.sp])
			vm.sp -= n
			vm.push(object.NovaLista(elems))
		case code.OpHash:
			n := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			dic := object.NovoDicionario()
			base := vm.sp - 2*n
			for i := 0; i < n; i++ {
				chave := vm.stack[base+2*i]
				valor := vm.stack[base+2*i+1]
				c, ok := chave.(object.Chaveavel)
				if !ok {
					panic(VMError{err: &object.Erro{Message: "chave de dicionario inaceitavel: " + string(chave.Type()), Kind: "runtime"}})
				}
				dic.Bota(c.ChaveHash(), object.ParDic{Chave: chave, Valor: valor})
			}
			vm.sp = base
			vm.push(dic)
		case code.OpIndex:
			idx := vm.pop()
			cont := vm.pop()
			r, perr := vmIndex(cont, idx)
			if perr != nil {
				panic(VMError{err: &object.Erro{Message: perr.Error(), Kind: "runtime"}})
			}
			vm.push(r)
			ip++
		case code.OpIndexSet:
			val := vm.pop()
			idx := vm.pop()
			cont := vm.pop()
			if perr := vmIndexSet(cont, idx, val); perr != nil {
				panic(VMError{err: &object.Erro{Message: perr.Error(), Kind: "runtime"}})
			}
			// `bota d[k] = v` e statement: nao deixa valor na pilha (igual o
			// caminho `bota nome = v`, que o OpSetGlobal/Local consome).
			ip++
		case code.OpRange:
			hi := vm.pop()
			lo := vm.pop()
			ln, lok := lo.(*object.Numero)
			hn, hok := hi.(*object.Numero)
			if !lok || !ln.EhInt || !hok || !hn.EhInt {
				panic(VMError{err: &object.Erro{Message: "range .. quer inteiros dos dois lados", Kind: "runtime"}})
			}
			elems, ok := object.RangeInts(ln.Int, hn.Int)
			if !ok {
				panic(VMError{err: &object.Erro{Message: fmt.Sprintf("range .. de %d..%d e gigante demais", ln.Int, hn.Int), Kind: "runtime"}})
			}
			vm.push(object.NovaLista(elems))
			ip++
		case code.OpIndexOuNada:
			idx := vm.pop()
			cont := vm.pop()
			switch cc := cont.(type) {
			case *object.Lista:
				var v object.Object
				if n, ok := idx.(*object.Numero); ok && n.EhInt && n.Int >= 0 {
					v, _ = cc.Pega(int(n.Int))
				}
				if v == nil {
					v = NADA
				}
				vm.push(v)
			case *object.Dicionario:
				if ch, ok := idx.(object.Chaveavel); ok {
					if par, existe := cc.Pega(ch.ChaveHash()); existe {
						vm.push(par.Valor)
					} else {
						vm.push(NADA)
					}
				} else {
					vm.push(NADA)
				}
			case *object.Instancia:
				// `bota {x, y} = ponto`: pelos campos (o que nao tem vira nada)
				if _, ehNome := idx.(*object.Texto); !ehNome {
					panic(VMError{err: &object.Erro{Message: fmt.Sprintf("pra desestruturar com [] eu quero uma lista, veio %s", cont.Type()), Kind: "runtime"}})
				}
				v, perr := membroVM(cc, idx)
				if perr != nil {
					v = NADA
				}
				vm.push(v)
			default:
				panic(VMError{err: &object.Erro{Message: fmt.Sprintf("so da pra desestruturar lista ou dicionario, veio %s", cont.Type()), Kind: "runtime"}})
			}
			ip++
		case code.OpIterSeq:
			it := vm.pop()
			switch c := it.(type) {
			case *object.Lista:
				// com concorrencia o laco percorre um retrato tirado agora
				vm.push(c.ParaIterar())
			case *object.Dicionario:
				chaves := make([]object.Object, 0, c.Tamanho())
				c.Itera(func(par object.ParDic) { chaves = append(chaves, par.Chave) })
				vm.push(object.NovaLista(chaves))
			case *object.Conjunto:
				// retrato dos itens na ordem de insercao; com dois nomes o
				// OpIterPar trata como lista (indice, item)
				vm.push(object.NovaLista(c.Valores()))
			default:
				panic(VMError{err: &object.Erro{Message: fmt.Sprintf("pra_cada ... em ... so funciona com lista, dicionario ou conjunto, e isso ai e %s", it.Type()), Kind: "runtime"}})
			}
			ip++
		case code.OpGetLocal:
			idx := int(fn.Bytecode[ip+1])
			ip += 2
			vm.push(vm.stack[frame.basePointer+idx])
		case code.OpSetLocal:
			idx := int(fn.Bytecode[ip+1])
			ip += 2
			vm.stack[frame.basePointer+idx] = vm.pop()
		case code.OpGetFree:
			idx := int(fn.Bytecode[ip+1])
			ip += 2
			if idx >= len(fn.Free) {
				panic(VMError{err: &object.Erro{Message: "freevar fora do range", Kind: "runtime"}})
			}
			vm.push(valorLivre(fn.Free[idx]))
		case code.OpGetLocalOu:
			// cadeia de leitura (escopo.go no compilador): achou valor, pula
			// pro fim da cadeia; senao a proxima instrucao le o escopo de fora
			if v := vm.stack[frame.basePointer+int(fn.Bytecode[ip+3])]; v != nil {
				vm.push(v)
				ip = int(code.ReadUint16(fn.Bytecode[ip+1:]))
			} else {
				ip += 4
			}
		case code.OpGetCelulaOu:
			if v := vm.stack[frame.basePointer+int(fn.Bytecode[ip+3])].(*celula).pega(); v != nil {
				vm.push(v)
				ip = int(code.ReadUint16(fn.Bytecode[ip+1:]))
			} else {
				ip += 4
			}
		case code.OpGetFreeOu:
			idx := int(fn.Bytecode[ip+3])
			if idx >= len(fn.Free) {
				panic(VMError{err: &object.Erro{Message: "freevar fora do range", Kind: "runtime"}})
			}
			if v := valorLivre(fn.Free[idx]); v != nil {
				vm.push(v)
				ip = int(code.ReadUint16(fn.Bytecode[ip+1:]))
			} else {
				ip += 4
			}
		case code.OpGetGlobalOu:
			idx := int(code.ReadUint16(fn.Bytecode[ip+3:]))
			var v object.Object
			if idx < len(vm.globals) {
				if object.ConcorrenciaAtiva() {
					v = pegaGlobalTravado(vm.globals, idx)
				} else {
					v = vm.globals[idx]
				}
			}
			if v != nil {
				vm.push(v)
				ip = int(code.ReadUint16(fn.Bytecode[ip+1:]))
			} else {
				ip += 5
			}
		case code.OpGetLocalChk:
			v := vm.stack[frame.basePointer+int(fn.Bytecode[ip+1])]
			if v == nil {
				panic(vm.erroNaoBotou(fn.Bytecode[ip+2:]))
			}
			ip += 4
			vm.push(v)
		case code.OpGetGlobalChk:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			var v object.Object
			if idx < len(vm.globals) {
				if object.ConcorrenciaAtiva() {
					v = pegaGlobalTravado(vm.globals, idx)
				} else {
					v = vm.globals[idx]
				}
			}
			if v == nil {
				panic(vm.erroNaoBotou(fn.Bytecode[ip+3:]))
			}
			ip += 5
			vm.push(v)
		case code.OpGetCelulaChk:
			v := vm.stack[frame.basePointer+int(fn.Bytecode[ip+1])].(*celula).pega()
			if v == nil {
				panic(vm.erroNaoBotou(fn.Bytecode[ip+2:]))
			}
			ip += 4
			vm.push(v)
		case code.OpGetFreeChk:
			idx := int(fn.Bytecode[ip+1])
			if idx >= len(fn.Free) {
				panic(VMError{err: &object.Erro{Message: "freevar fora do range", Kind: "runtime"}})
			}
			v := valorLivre(fn.Free[idx])
			if v == nil {
				panic(vm.erroNaoBotou(fn.Bytecode[ip+2:]))
			}
			ip += 4
			vm.push(v)
		case code.OpCelula:
			idx := frame.basePointer + int(fn.Bytecode[ip+1])
			ip += 2
			vm.stack[idx] = &celula{v: vm.stack[idx]}
		case code.OpGetCelula:
			idx := int(fn.Bytecode[ip+1])
			ip += 2
			vm.push(vm.stack[frame.basePointer+idx].(*celula).pega())
		case code.OpSetCelula:
			idx := int(fn.Bytecode[ip+1])
			ip += 2
			vm.stack[frame.basePointer+idx].(*celula).poe(vm.pop())
		case code.OpGetFreeCelula:
			idx := int(fn.Bytecode[ip+1])
			ip += 2
			if idx >= len(fn.Free) {
				panic(VMError{err: &object.Erro{Message: "freevar fora do range", Kind: "runtime"}})
			}
			vm.push(fn.Free[idx])
		case code.OpClosure:
			constIdx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			numFree := int(fn.Bytecode[ip+3])
			ip += 4
			cf, ok := vm.constants[constIdx].(*object.CompiledFunction)
			if !ok {
				panic(VMError{err: &object.Erro{Message: "OpClosure nao aponta pra CompiledFunction", Kind: "runtime"}})
			}
			// popula freevars com os ultimos `numFree` valores da pilha
			// (empilhados pelo compiler antes de emitir OpClosure).
			var free []object.Object
			if numFree > 0 {
				free = make([]object.Object, numFree)
				copy(free, vm.stack[vm.sp-numFree:vm.sp])
				vm.sp -= numFree
			}
			vm.push(&object.CompiledFunction{
				Name: cf.Name, NumArgs: cf.NumArgs, NumLocals: cf.NumLocals,
				MinArgs: cf.MinArgs, Variadic: cf.Variadic,
				Bytecode: cf.Bytecode, Free: free, Linhas: cf.Linhas,
				MaxStack: cf.MaxStack,
			})
		case code.OpCall:
			opPos := ip // offset do OpCall (call site) pro traço de pilha
			argc := int(fn.Bytecode[ip+1])
			ip += 2
			callee := vm.stack[vm.sp-1]
			if cf, ok := callee.(*object.CompiledFunction); ok {
				// valida argc: variadic aceita >= minArgs; default aceita
				// entre minArgs e numArgs; sem nenhum = estritamente numArgs.
				if cf.Variadic {
					if argc < cf.MinArgs {
						panic(VMError{err: &object.Erro{Message: fmt.Sprintf("essa gambiarra quer no minimo %d parametro(s), voce mandou %d", cf.MinArgs, argc), Kind: "runtime"}})
					}
				} else {
					minA := cf.MinArgs
					if minA == 0 {
						minA = cf.NumArgs
					}
					if argc < minA || argc > cf.NumArgs {
						if cf.MinArgs > 0 && cf.MinArgs < cf.NumArgs {
							panic(VMError{err: &object.Erro{Message: fmt.Sprintf("essa gambiarra quer entre %d e %d parametro(s), voce mandou %d", cf.MinArgs, cf.NumArgs, argc), Kind: "runtime"}})
						}
						panic(VMError{err: &object.Erro{Message: fmt.Sprintf("essa gambiarra quer %d parametro(s), voce mandou %d", cf.NumArgs, argc), Kind: "runtime"}})
					}
				}
				bp := vm.sp - 1 - argc
				// a reserva do frame inteiro sai aqui, uma vez: locals + o teto
				// da pilha de operandos do chamado
				vm.garanteEspaco(bp + cf.NumLocals + folga(cf))
				// varargs: coleta extras numa lista (antes de ajustar sp)
				if cf.Variadic && argc >= cf.NumArgs {
					variadicIdx := cf.NumArgs - 1
					nExtras := argc - variadicIdx
					restElems := make([]object.Object, nExtras)
					copy(restElems, vm.stack[bp+variadicIdx:bp+argc])
					vm.stack[bp+variadicIdx] = object.NovaLista(restElems)
					argc = cf.NumArgs
				}
				// default params: pad missing slots com NADA. Excecao: se o
				// ultimo param e variadico e ficou sem nenhum extra, o slot dele
				// vira lista VAZIA (nao NADA) — paridade com o tree-walker.
				if argc < cf.NumArgs {
					for i := argc; i < cf.NumArgs; i++ {
						if cf.Variadic && i == cf.NumArgs-1 {
							vm.stack[bp+i] = object.NovaLista([]object.Object{})
						} else {
							vm.stack[bp+i] = NADA
						}
					}
					argc = cf.NumArgs
				}
				vm.limpaLocais(bp, cf)
				vm.sp = bp + cf.NumLocals
				frame.ip = ip
				frame = vm.empurraFrame(cf, bp, opPos)
				fn = cf
				ip = 0
				continue
			}
			if b, ok := callee.(*object.Builtin); ok {
				args := make([]object.Object, argc)
				// args em stack[sp-1-argc .. sp-2]
				copy(args, vm.stack[vm.sp-1-argc:vm.sp-1])
				vm.sp -= argc + 1 // popa args + callee
				res := b.Fn(args)
				if s, ok := res.(*object.Sair); ok {
					panic(VMError{sai: s}) // sai(codigo): desenrola tudo ate o Run
				}
				if e, ok := res.(*object.Erro); ok && e != nil && !e.Handled {
					panic(VMError{err: e, quadro: quadroBuiltin(b.Nome, fn, opPos)})
				}
				vm.push(res)
				continue
			}
			if m, ok := callee.(*object.MetodoLigado); ok {
				// `obj.metodo(args)`: o receiver entra como 1o argumento
				cf, argcM := vm.abreMetodo(m, argc)
				bp := vm.sp - 1 - argcM
				if e := vm.ajustaArgs(cf, bp, argcM); e != nil {
					panic(VMError{err: e})
				}
				vm.limpaLocais(bp, cf)
				vm.sp = bp + cf.NumLocals
				frame.ip = ip
				frame = vm.empurraFrame(cf, bp, opPos)
				fn = cf
				ip = 0
				continue
			}
			panic(VMError{err: &object.Erro{Message: fmt.Sprintf("isso ai (%s) nao e gambiarra pra voce sair chamando", callee.Type()), Kind: "runtime"}})
		case code.OpTailCall:
			argc := int(fn.Bytecode[ip+1])
			ip += 2
			callee := vm.stack[vm.sp-1]
			if cf, ok := callee.(*object.CompiledFunction); ok {
				// mesma validacao de aridez do OpCall
				if cf.Variadic {
					if argc < cf.MinArgs {
						panic(VMError{err: &object.Erro{Message: fmt.Sprintf("essa gambiarra quer no minimo %d parametro(s), voce mandou %d", cf.MinArgs, argc), Kind: "runtime"}})
					}
				} else {
					minA := cf.MinArgs
					if minA == 0 {
						minA = cf.NumArgs
					}
					if argc < minA || argc > cf.NumArgs {
						if cf.MinArgs > 0 && cf.MinArgs < cf.NumArgs {
							panic(VMError{err: &object.Erro{Message: fmt.Sprintf("essa gambiarra quer entre %d e %d parametro(s), voce mandou %d", cf.MinArgs, cf.NumArgs, argc), Kind: "runtime"}})
						}
						panic(VMError{err: &object.Erro{Message: fmt.Sprintf("essa gambiarra quer %d parametro(s), voce mandou %d", cf.NumArgs, argc), Kind: "runtime"}})
					}
				}
				bpCall := vm.sp - 1 - argc
				vm.garanteEspaco(bpCall + cf.NumLocals)
				if cf.Variadic && argc >= cf.NumArgs {
					variadicIdx := cf.NumArgs - 1
					nExtras := argc - variadicIdx
					restElems := make([]object.Object, nExtras)
					copy(restElems, vm.stack[bpCall+variadicIdx:bpCall+argc])
					vm.stack[bpCall+variadicIdx] = object.NovaLista(restElems)
					argc = cf.NumArgs
				}
				if argc < cf.NumArgs {
					for i := argc; i < cf.NumArgs; i++ {
						if cf.Variadic && i == cf.NumArgs-1 {
							vm.stack[bpCall+i] = object.NovaLista([]object.Object{})
						} else {
							vm.stack[bpCall+i] = NADA
						}
					}
					argc = cf.NumArgs
				}
				// TAIL CALL: reusa o frame ATUAL — move os args pra base do frame
				// corrente e troca a funcao, sem empilhar. Recursao em cauda roda em
				// profundidade constante de frames.
				bp := frame.basePointer
				vm.garanteEspaco(bp + cf.NumLocals + folga(cf))
				copy(vm.stack[bp:bp+cf.NumArgs], vm.stack[bpCall:bpCall+cf.NumArgs])
				vm.limpaLocais(bp, cf)
				vm.sp = bp + cf.NumLocals
				frame.fn = cf
				frame.ip = 0
				fn = cf
				ip = 0
				continue
			}
			if b, ok := callee.(*object.Builtin); ok {
				// tail call a builtin: chama e retorna (builtin nao recursa via frames)
				args := make([]object.Object, argc)
				copy(args, vm.stack[vm.sp-1-argc:vm.sp-1])
				res := b.Fn(args)
				if sr, ok := res.(*object.Sair); ok {
					panic(VMError{sai: sr})
				}
				if e, ok := res.(*object.Erro); ok && e != nil && !e.Handled {
					panic(VMError{err: e, quadro: quadroBuiltin(b.Nome, fn, ip-2)})
				}
				returnedFn := vm.popFrame()
				vm.sp = returnedFn.basePointer
				vm.push(res)
				vm.limpaTriesOrfaos()
				if vm.framesIdx < baseIdx {
					return nil
				}
				frame = vm.currentFrame()
				fn = frame.fn
				ip = frame.ip
				continue
			}
			if m, ok := callee.(*object.MetodoLigado); ok {
				// raro (`funciona g()` com g = metodo ligado): roda sincrono e
				// retorna, igual o tail call de builtin — fora do caminho quente
				args := make([]object.Object, argc)
				copy(args, vm.stack[vm.sp-1-argc:vm.sp-1])
				res := vm.chamaMetodoSincrono(m, args)
				returnedFn := vm.popFrame()
				vm.sp = returnedFn.basePointer
				vm.push(res)
				vm.limpaTriesOrfaos()
				if vm.framesIdx < baseIdx {
					return nil
				}
				frame = vm.currentFrame()
				fn = frame.fn
				ip = frame.ip
				continue
			}
			panic(VMError{err: &object.Erro{Message: fmt.Sprintf("isso ai (%s) nao e gambiarra pra voce sair chamando", callee.Type()), Kind: "runtime"}})
		case code.OpCallBuiltin:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			argc := int(fn.Bytecode[ip+3])
			ip += 4
			args := make([]object.Object, argc)
			copy(args, vm.stack[vm.sp-argc:vm.sp])
			vm.sp -= argc
			nome := compiler.BuiltinNomes()[idx]
			b := vm.builtins[nome]
			if b == nil {
				panic(VMError{err: &object.Erro{Message: "builtin " + nome + " nao registrada na VM", Kind: "runtime"}})
			}
			res := b.Fn(args)
			if s, ok := res.(*object.Sair); ok {
				panic(VMError{sai: s}) // sai(codigo): desenrola tudo ate o Run
			}
			if e, ok := res.(*object.Erro); ok && e != nil && !e.Handled {
				panic(VMError{err: e})
			}
			vm.push(res)
		case code.OpReturn:
			val := vm.pop()
			returnedFn := vm.popFrame()
			vm.sp = returnedFn.basePointer
			vm.push(val)
			vm.limpaTriesOrfaos()
			if vm.framesIdx < baseIdx {
				return nil // o frame que esta invocacao comecou retornou
			}
			frame = vm.currentFrame()
			fn = frame.fn
			ip = frame.ip
		case code.OpReturnNada:
			returnedFn := vm.popFrame()
			vm.sp = returnedFn.basePointer
			vm.push(NADA)
			vm.limpaTriesOrfaos()
			if vm.framesIdx < baseIdx {
				return nil
			}
			frame = vm.currentFrame()
			fn = frame.fn
			ip = frame.ip
		case code.OpIterPar:
			it := vm.pop()
			seq := vm.pop()
			orig := vm.pop()
			idx, ok := it.(*object.Numero)
			if !ok || !idx.EhInt {
				panic(VMError{err: &object.Erro{Message: "IterPar: indice tem que ser inteiro", Kind: "runtime"}})
			}
			i := int(idx.Int)
			seqList, sok := seq.(*object.Lista)
			if !sok {
				panic(VMError{err: &object.Erro{Message: "IterPar: __seq tem que ser lista", Kind: "runtime"}})
			}
			mid, dentro := seqList.Pega(i)
			if !dentro {
				panic(VMError{err: &object.Erro{Message: "IterPar: indice fora do range", Kind: "runtime"}})
			}
			// orig pode ser lista ou dict
			if dOrig, isDict := orig.(*object.Dicionario); isDict {
				// mid e a chave
				chave, ok := mid.(object.Chaveavel)
				if !ok {
					panic(VMError{err: &object.Erro{Message: "IterPar: chave nao e chaveavel", Kind: "runtime"}})
				}
				par, existe := dOrig.Pega(chave.ChaveHash())
				var valor object.Object = NADA
				if existe {
					valor = par.Valor
				}
				vm.push(mid)   // chave
				vm.push(valor) // valor
			} else {
				// lista: 1o nome = indice (i), 2o = elemento (mid)
				vm.push(vm.num.Int(int64(i)))
				vm.push(mid)
			}
			ip++
		case code.OpFatia:
			fimRaw := vm.pop()
			inicioRaw := vm.pop()
			left := vm.pop()
			var inicio, fim *object.Numero
			if n, ok := inicioRaw.(*object.Numero); ok {
				inicio = n
			} else if _, ok := inicioRaw.(*object.Nada); !ok {
				panic(VMError{err: &object.Erro{Message: "fatia so aceita numero como inicio, veio " + string(inicioRaw.Type()), Kind: "runtime"}})
			}
			if n, ok := fimRaw.(*object.Numero); ok {
				fim = n
			} else if _, ok := fimRaw.(*object.Nada); !ok {
				panic(VMError{err: &object.Erro{Message: "fatia so aceita numero como fim, veio " + string(fimRaw.Type()), Kind: "runtime"}})
			}
			switch c := left.(type) {
			case *object.Lista:
				vm.push(object.FatiaLista(c, inicio, fim))
			case *object.Texto:
				runes := []rune(c.Value)
				lo, hi := object.NormalizarFatia(inicio, fim, len(runes))
				vm.push(&object.Texto{Value: string(runes[lo:hi])})
			default:
				panic(VMError{err: &object.Erro{Message: "so da pra fatiar lista ou texto, e isso ai e " + string(left.Type()), Kind: "runtime"}})
			}
			ip++
		case code.OpBoraCall:
			argc := int(fn.Bytecode[ip+1])
			ip += 2
			vm.execBoraCall(argc)
		case code.OpCallEspalha:
			// chamada com `...lista`: abre as listas na pilha e segue igual o
			// OpCall (que fica intocado pra nao pesar nas chamadas normais).
			opPos := ip
			mascara := vm.constants[int(code.ReadUint16(fn.Bytecode[ip+1:]))].(*object.Texto).Value
			ip += 3
			argc := vm.espalhaArgs(mascara)
			callee := vm.stack[vm.sp-1]
			if m, ok := callee.(*object.MetodoLigado); ok {
				callee, argc = vm.abreMetodo(m, argc)
			}
			if cf, ok := callee.(*object.CompiledFunction); ok {
				bp := vm.sp - 1 - argc
				if e := vm.ajustaArgs(cf, bp, argc); e != nil {
					panic(VMError{err: e})
				}
				vm.limpaLocais(bp, cf)
				vm.sp = bp + cf.NumLocals
				frame.ip = ip
				frame = vm.empurraFrame(cf, bp, opPos)
				fn = cf
				ip = 0
				continue
			}
			if b, ok := callee.(*object.Builtin); ok {
				args := make([]object.Object, argc)
				copy(args, vm.stack[vm.sp-1-argc:vm.sp-1])
				vm.sp -= argc + 1
				res := b.Fn(args)
				if s, ok := res.(*object.Sair); ok {
					panic(VMError{sai: s})
				}
				if e, ok := res.(*object.Erro); ok && e != nil && !e.Handled {
					panic(VMError{err: e, quadro: quadroBuiltin(b.Nome, fn, opPos)})
				}
				vm.push(res)
				continue
			}
			panic(VMError{err: &object.Erro{Message: fmt.Sprintf("isso ai (%s) nao e gambiarra pra voce sair chamando", callee.Type()), Kind: "runtime"}})
		case code.OpBoraEspalha:
			mascara := vm.constants[int(code.ReadUint16(fn.Bytecode[ip+1:]))].(*object.Texto).Value
			ip += 3
			vm.execBoraCall(vm.espalhaArgs(mascara))
		case code.OpImporta:
			mod := vm.constants[int(code.ReadUint16(fn.Bytecode[ip+1:]))].(*object.Modulo)
			atual := vm.constants[int(code.ReadUint16(fn.Bytecode[ip+3:]))].(*object.Texto).Value
			linha := fn.LinhaDoPC(ip)
			ip += 5
			vm.push(vm.importa(mod, atual, linha))
		case code.OpDup:
			val := vm.stack[vm.sp-1]
			vm.push(val)
			ip++
		case code.OpIsNada:
			o := vm.pop()
			if _, ok := o.(*object.Nada); ok {
				vm.push(DEU_BOM)
			} else {
				vm.push(DEU_RUIM)
			}
			ip++
		case code.OpGetBuiltin:
			idx := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			nome := compiler.BuiltinNomes()[idx]
			b := vm.builtins[nome]
			if b == nil {
				panic(VMError{err: &object.Erro{Message: "builtin " + nome + " nao registrada", Kind: "runtime"}})
			}
			vm.push(b)
		case code.OpThrow:
			val := vm.pop()
			e, ok := val.(*object.Erro)
			if !ok {
				panic(VMError{err: &object.Erro{Message: "so da pra jogar Erro, veio " + string(val.Type()), Kind: "runtime"}})
			}
			// relancado (finalmente sem quebrou): volta a ser erro levantado
			e.Handled = false
			panic(VMError{err: e})
		case code.OpTry:
			catchAddr := int(code.ReadUint16(fn.Bytecode[ip+1:]))
			ip += 3
			vm.errStack = append(vm.errStack, tryHandler{catchAddr: catchAddr, frameIdx: vm.framesIdx})
		case code.OpTryEnd:
			if len(vm.errStack) > 0 {
				vm.errStack = vm.errStack[:len(vm.errStack)-1]
			}
			ip++
		// POO (poo.go): treta, combinado, metodo e literal
		case code.OpTreta, code.OpCombinado, code.OpMetodo, code.OpInstancia:
			vm.execPOO(op, int(code.ReadUint16(fn.Bytecode[ip+1:])))
			ip += 3
		default:
			// OpLinha (gancho de linha) so aparece em bytecode instrumentado:
			// fica no default pra o switch quente nao mudar nada
			if op == code.OpLinha {
				vm.disparaLinha(fn.Bytecode[ip+1:])
				ip += 3
				continue
			}
			return fmt.Errorf("opcode desconhecido: %d", op)
		}
	}
	// fallthrough: frame esgotou sem return
	return nil
}

// handleVMError recebe um erro runtime: monta o traço de pilha (call sites
// dos frames entre o try e o ponto do erro), acha o handler mais interno e
// desempilha frames ate o dono do try (unwinding real — o erro pode ter
// estourado em funcao chamada dentro do bloco arruma). Sem handler:
// desempilha tudo e marca framesIdx=0 (erro nao capturado).
func (vm *VM) handleVMError(e *object.Erro, quadro *object.StackFrame) {
	// profundidade do try mais interno (1 = frame raiz, quando nao ha try:
	// traço cobre todos os frames alem do raiz)
	inicio := 1
	if len(vm.errStack) > 0 {
		inicio = vm.errStack[len(vm.errStack)-1].frameIdx
	}
	// traço externo->interno (igual o tree-walker): frames[j] foi chamado de
	// frames[j-1] no offset callPos — a linha vem da tabela do PAI. Entram so
	// os frames entre o try (ou a raiz) e onde estourou, NA FRENTE do que o
	// erro ja trazia: igual o tree-walker, que empilha um frame a cada funcao
	// que o erro atravessa. Relancado pelo finalmente, ganha os frames de
	// fora; o builtin que falhou (quadro) e o mais de dentro.
	var novos []object.StackFrame
	for j := inicio; j < vm.framesIdx; j++ {
		novos = append(novos, object.StackFrame{
			Funcao: vm.frames[j].fn.Name,
			Line:   vm.frames[j-1].fn.LinhaDoPC(vm.frames[j].callPos),
		})
	}
	if quadro != nil {
		novos = append(novos, *quadro)
	}
	if len(novos) > 0 {
		e.Stack = append(novos, e.Stack...)
	}

	if len(vm.errStack) == 0 {
		// sem handler: destroi frames e marca erro nao capturado
		vm.framesIdx = 0
		return
	}
	h := vm.errStack[len(vm.errStack)-1]
	vm.errStack = vm.errStack[:len(vm.errStack)-1]
	// desempilha frames ate o que registrou o try
	for vm.framesIdx > h.frameIdx {
		vm.popFrame()
	}
	alvo := vm.currentFrame()
	// descarta operandos pendentes e restabelece o espaco de locals
	vm.garanteEspaco(alvo.basePointer + alvo.fn.NumLocals + folga(alvo.fn))
	vm.sp = alvo.basePointer + alvo.fn.NumLocals
	// pego: dali pra frente e so um valor (igual o tree-walker) — passar pra
	// builtin, devolver de gambiarra ou o erro_causa nao relancam.
	e.Handled = true
	vm.push(e)
	alvo.ip = h.catchAddr
}

// limpaTriesOrfaos descarta handlers de try registrados por frames que ja
// retornaram (um `funciona` dentro de `arruma` sai da funcao sem passar pelo
// OpTryEnd). Sem isso, um erro futuro saltaria pra um catchAddr de bytecode
// de outro frame.
func (vm *VM) limpaTriesOrfaos() {
	for len(vm.errStack) > 0 && vm.errStack[len(vm.errStack)-1].frameIdx > vm.framesIdx {
		vm.errStack = vm.errStack[:len(vm.errStack)-1]
	}
}

type VMError struct {
	err *object.Erro
	sai *object.Sair // preenchido quando o panic e um sai(codigo), nao um erro
	// quadro: o erro veio de um builtin chamado pelo programa — entra no
	// traco como o frame mais de dentro (`em tamanho (linha N)`), igual o
	// tree-walker.
	quadro *object.StackFrame
}

// quadroBuiltin monta o frame do traco pra um builtin chamado no offset pc.
func quadroBuiltin(nome string, fn *object.CompiledFunction, pc int) *object.StackFrame {
	return &object.StackFrame{Funcao: nome, Line: fn.LinhaDoPC(pc)}
}

func (v VMError) Error() string {
	if v.sai != nil {
		return fmt.Sprintf("sai com codigo %d", v.sai.Codigo)
	}
	return v.err.Message
}

// SaiRequisicao e o que o Run devolve quando o script chamou sai(codigo). Nao
// e um erro de verdade: o runner (cmd/gs) traduz pra os.Exit(codigo).
type SaiRequisicao struct{ Codigo int }

func (s SaiRequisicao) Error() string { return fmt.Sprintf("sai com codigo %d", s.Codigo) }

func (vm *VM) execBinario(op code.Opcode) {
	right := vm.pop()
	left := vm.pop()
	vm.push(vm.binario(op, left, right))
}

// binario aplica a operacao e DEVOLVE o resultado, sem tocar na pilha — assim
// o OpBinConst (que ja tem os dois operandos em maos) reusa a mesma logica sem
// pagar push/pop da constante.
func (vm *VM) binario(op code.Opcode, left, right object.Object) object.Object {
	ln, lok := left.(*object.Numero)
	rn, rok := right.(*object.Numero)
	if lok && rok {
		if op == code.OpPow {
			return vm.potencia(ln, rn)
		}
		// bitwise so faz sentido com inteiros; tratamos antes do fast-path
		// float pra nao contaminar caminho aritmetico. Igual ao tree-walker,
		// operando nao-inteiro num op bitwise e erro (nao cai no caminho float).
		if ehBitwise(op) {
			if !ln.EhInt || !rn.EhInt {
				msg := simboloBinario(op) + " bitwise so faz sentido com inteiros"
				if ehShift(op) {
					msg = "shift so faz sentido com inteiros"
				}
				panic(VMError{err: &object.Erro{Message: msg, Kind: "runtime"}})
			}
			r, _ := vm.execBinarioBitwise(op, ln.Int, rn.Int)
			return r
		}
		// fast path inteiros exatos
		if r, ok := vm.execBinarioIntShort(op, ln, rn); ok {
			return r
		}
		return vm.execBinarioNumero(op, ln.Value, rn.Value)
	}
	if op == code.OpAdd && (left.Type() == object.TEXTO_OBJ || right.Type() == object.TEXTO_OBJ) {
		return &object.Texto{Value: left.Inspect() + right.Inspect()}
	}
	// mesma mensagem do tree-walker: "nao da pra fazer TEXTO - NUMERO"
	panic(VMError{err: &object.Erro{Message: fmt.Sprintf("nao da pra fazer %s %s %s", left.Type(), simboloBinario(op), right.Type()), Kind: "runtime"}})
}

// simboloBinario devolve o simbolo textual do operador aritmetico/bitwise, pra
// que as mensagens de erro da VM fiquem identicas as do interpretador (que usa
// node.Operator).
func simboloBinario(op code.Opcode) string {
	switch op {
	case code.OpAdd:
		return "+"
	case code.OpSub:
		return "-"
	case code.OpMul:
		return "*"
	case code.OpDiv:
		return "/"
	case code.OpMod:
		return "%"
	case code.OpPow:
		return "**"
	case code.OpBAnd:
		return "&"
	case code.OpBOr:
		return "|"
	case code.OpBXor:
		return "^"
	case code.OpLShift:
		return "<<"
	case code.OpRShift:
		return ">>"
	case code.OpGreaterThan:
		return ">"
	case code.OpGreaterEqual:
		return ">="
	case code.OpMenor:
		return "<"
	case code.OpMenorEqual:
		return "<="
	}
	return "?"
}

// ehBitwise diz se o opcode e uma operacao bitwise (&, |, ^, <<, >>).
func ehBitwise(op code.Opcode) bool {
	switch op {
	case code.OpBAnd, code.OpBOr, code.OpBXor, code.OpLShift, code.OpRShift:
		return true
	}
	return false
}

// ehShift diz se o opcode e um deslocamento (<< ou >>).
func ehShift(op code.Opcode) bool {
	return op == code.OpLShift || op == code.OpRShift
}

// potencia usa a mesma conta do tree-walker (object.Potencia), so que
// alocando o resultado na arena da VM.
func (vm *VM) potencia(base, exp *object.Numero) object.Object {
	iv, fv, ehInt, err := object.Potencia(base, exp)
	if err != nil {
		panic(VMError{err: &object.Erro{Message: err.Error(), Kind: "runtime"}})
	}
	if ehInt {
		return vm.num.Int(iv)
	}
	return vm.num.Float(fv)
}

func (vm *VM) execBinarioNumero(op code.Opcode, l, r float64) object.Object {
	if op == code.OpDiv && r == 0 {
		panic(VMError{err: &object.Erro{Message: "nao da pra dividir por zero, parca — nem na gambiarra", Kind: "runtime"}})
	}
	if op == code.OpMod && r == 0 {
		panic(VMError{err: &object.Erro{Message: "resto de divisao por zero? ai voce quer demais", Kind: "runtime"}})
	}
	var res float64
	switch op {
	case code.OpAdd:
		res = l + r
	case code.OpSub:
		res = l - r
	case code.OpMul:
		res = l * r
	case code.OpDiv:
		res = l / r
	case code.OpMod:
		res = math.Mod(l, r)
	}
	return vm.num.Float(res)
}

// execBinarioIntShort usa aritmetica int64 exata quando AMBOS operandos sao
// inteiros exatos (EhInt=true): +, -, *, % e a divisao exata (6 / 3). Conta
// que estoura o int64 (ou divisao com resto, ou por zero) devolve false e o
// chamador refaz em float64 — a mesma regra do tree-walker (object.SomaInt).
func (vm *VM) execBinarioIntShort(op code.Opcode, lo, ro *object.Numero) (object.Object, bool) {
	if !lo.EhInt || !ro.EhInt {
		return nil, false
	}
	var r int64
	var ok bool
	switch op {
	case code.OpAdd:
		r, ok = object.SomaInt(lo.Int, ro.Int)
	case code.OpSub:
		r, ok = object.SubInt(lo.Int, ro.Int)
	case code.OpMul:
		r, ok = object.MulInt(lo.Int, ro.Int)
	case code.OpDiv:
		r, ok = object.DivInt(lo.Int, ro.Int)
	case code.OpMod:
		r, ok = object.RestoInt(lo.Int, ro.Int)
	}
	if !ok {
		return nil, false
	}
	return vm.num.Int(r), true
}

func (vm *VM) execComparacao(op code.Opcode) {
	right := vm.pop()
	left := vm.pop()
	vm.push(vm.comparacao(op, left, right))
}

// comparacao devolve o resultado sem tocar na pilha — mesma razao do binario:
// o OpBinConst reusa a logica com os operandos ja em maos.
func (vm *VM) comparacao(op code.Opcode, left, right object.Object) object.Object {
	ln, lok := left.(*object.Numero)
	rn, rok := right.(*object.Numero)
	if lok && rok {
		if ln.EhInt && rn.EhInt {
			// inteiro com inteiro compara exato (o float64 erra acima de
			// 2^53), igual o tree-walker
			return boolNativo(comparaInt(op, ln.Int, rn.Int))
		}
		switch op {
		case code.OpGreaterThan:
			return boolNativo(ln.Value > rn.Value)
		case code.OpGreaterEqual:
			return boolNativo(ln.Value >= rn.Value)
		case code.OpMenor:
			return boolNativo(ln.Value < rn.Value)
		case code.OpMenorEqual:
			return boolNativo(ln.Value <= rn.Value)
		case code.OpEqual:
			return boolNativo(ln.Value == rn.Value)
		case code.OpNotEqual:
			return boolNativo(ln.Value != rn.Value)
		}
	}
	switch op {
	case code.OpEqual:
		return boolNativo(iguais(left, right))
	case code.OpNotEqual:
		return boolNativo(!iguais(left, right))
	}
	// mesma mensagem do tree-walker: "nao da pra fazer TEXTO > NUMERO"
	panic(VMError{err: &object.Erro{Message: fmt.Sprintf("nao da pra fazer %s %s %s", left.Type(), simboloBinario(op), right.Type()), Kind: "runtime"}})
}

// comparaInt compara dois inteiros exatos.
func comparaInt(op code.Opcode, a, b int64) bool {
	switch op {
	case code.OpGreaterThan:
		return a > b
	case code.OpGreaterEqual:
		return a >= b
	case code.OpMenor:
		return a < b
	case code.OpMenorEqual:
		return a <= b
	case code.OpEqual:
		return a == b
	}
	return a != b // OpNotEqual
}

// ehComparacao diz se o opcode e de comparacao (vai pra vm.comparacao) em vez
// de aritmetica/bitwise (vm.binario).
func ehComparacao(op code.Opcode) bool {
	switch op {
	case code.OpEqual, code.OpNotEqual, code.OpGreaterThan, code.OpGreaterEqual,
		code.OpMenor, code.OpMenorEqual:
		return true
	}
	return false
}

func (vm *VM) execMinus() {
	o := vm.pop()
	if n, ok := o.(*object.Numero); ok {
		// preserva a inteireza exata: -1 tem que continuar EhInt, senao vira
		// float e escapa de checagens como o shift por valor negativo.
		if n.EhInt {
			if v, ok := object.NegInt(n.Int); ok {
				vm.push(vm.num.Int(v))
				return
			}
		}
		vm.push(vm.num.Float(-n.Value)) // inteiro que estoura vira real
		return
	}
	panic(VMError{err: &object.Erro{Message: fmt.Sprintf("nao da pra colocar - na frente de %s", o.Type()), Kind: "runtime"}})
}

// vmExecBinarioBitwise trata operacoes bitwise. Devolve (r, true) se o op
// for bitwise; (nil, false) caso contrario — caller segue o fluxo normal.
func (vm *VM) execBinarioBitwise(op code.Opcode, l, r int64) (object.Object, bool) {
	switch op {
	case code.OpBAnd:
		return vm.num.Int(l & r), true
	case code.OpBOr:
		return vm.num.Int(l | r), true
	case code.OpBXor:
		return vm.num.Int(l ^ r), true
	case code.OpLShift:
		if r < 0 {
			panic(VMError{err: &object.Erro{Message: "<< por valor negativo? naoiedade", Kind: "runtime"}})
		}
		return vm.num.Int(l << uint(r)), true
	case code.OpRShift:
		if r < 0 {
			panic(VMError{err: &object.Erro{Message: ">> por valor negativo? naoiedade", Kind: "runtime"}})
		}
		return vm.num.Int(l >> uint(r)), true
	}
	return nil, false
}

func vmIndex(cont, idx object.Object) (object.Object, error) {
	switch c := cont.(type) {
	case *object.Lista:
		n, ok := idx.(*object.Numero)
		if !ok {
			return nil, fmt.Errorf("indice de lista tem que ser numero")
		}
		v, dentro := c.Indice(int(n.Value))
		if !dentro {
			return nil, fmt.Errorf("esse indice (%d) ta fora da lista, o", int(n.Value))
		}
		return v, nil
	case *object.Texto:
		n, ok := idx.(*object.Numero)
		if !ok {
			return nil, fmt.Errorf("indice de texto tem que ser numero")
		}
		runes := []rune(c.Value)
		pos, dentro := object.IndiceNormalizado(int(n.Value), len(runes))
		if !dentro {
			return nil, fmt.Errorf("esse indice (%d) ta fora do texto, o", int(n.Value))
		}
		return &object.Texto{Value: string(runes[pos])}, nil
	case *object.Dicionario:
		chave, ok := idx.(object.Chaveavel)
		if !ok {
			return nil, fmt.Errorf("chave de dicionario invalida")
		}
		par, existe := c.Pega(chave.ChaveHash())
		if !existe {
			return NADA, nil
		}
		return par.Valor, nil
	case *object.Instancia:
		return membroVM(c, idx)
	}
	// lista, texto e dicionario sao indexaveis.
	return nil, fmt.Errorf("so da pra indexar lista, texto ou dicionario, e isso ai e %s", cont.Type())
}

func vmIndexSet(cont, idx, val object.Object) error {
	switch c := cont.(type) {
	case *object.Lista:
		n, ok := idx.(*object.Numero)
		if !ok {
			return fmt.Errorf("indice de lista tem que ser numero, veio %s", idx.Type())
		}
		if !c.Poe(int(n.Value), val) {
			return fmt.Errorf("esse indice (%d) ta fora da lista, o", int(n.Value))
		}
	case *object.Dicionario:
		chave, ok := idx.(object.Chaveavel)
		if !ok {
			return fmt.Errorf("essa chave (%s) nao da pra usar num dicionario", idx.Type())
		}
		c.Bota(chave.ChaveHash(), object.ParDic{Chave: idx, Valor: val})
	case *object.Instancia:
		return poeMembroVM(c, idx, val)
	default:
		// mesmas mensagens do tree-walker (evalAtribuiIndice)
		return fmt.Errorf("so da pra atribuir indice em lista ou dicionario, e isso ai e %s", cont.Type())
	}
	return nil
}

func boolNativo(b bool) *object.Booleano {
	if b {
		return DEU_BOM
	}
	return DEU_RUIM
}

func ehVerdade(o object.Object) bool {
	switch o := o.(type) {
	case *object.Nada:
		return false
	case *object.Booleano:
		return o.Value
	default:
		return true
	}
}

// iguais compara por valor. Estrutura que contem ela mesma nao pode descer pra
// sempre: mesma logica do interpreter.iguais (mesma colecao dos dois lados e
// igual de cara; passado de profSemMemoria niveis, par (a, b) que ja esta em
// comparacao e "igual ate aqui").
func iguais(a, b object.Object) bool { return iguaisRec(a, b, 0, nil) }

const profSemMemoria = 64

func iguaisRec(a, b object.Object, prof int, vistos map[[2]object.Object]bool) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch a.(type) {
	case *object.Lista, *object.Dicionario, *object.Instancia:
		if a == b {
			return true
		}
		if prof >= profSemMemoria {
			if vistos == nil {
				vistos = map[[2]object.Object]bool{}
			}
			par := [2]object.Object{a, b}
			if vistos[par] {
				return true
			}
			vistos[par] = true
		}
		prof++
	}
	switch av := a.(type) {
	case *object.Texto:
		return av.Value == b.(*object.Texto).Value
	case *object.Booleano:
		return av.Value == b.(*object.Booleano).Value
	case *object.Numero:
		return av.Value == b.(*object.Numero).Value
	case *object.Nada:
		return true
	case *object.Lista:
		// retratos: comparar elemento aninhado trava outra colecao, e nunca
		// seguramos duas travas ao mesmo tempo
		ae, be := av.Visao(), b.(*object.Lista).Visao()
		if len(ae) != len(be) {
			return false
		}
		for i, e := range ae {
			if !iguaisRec(e, be[i], prof, vistos) {
				return false
			}
		}
		return true
	case *object.Dicionario:
		bd := b.(*object.Dicionario)
		pares := av.Pares()
		if len(pares) != bd.Tamanho() {
			return false
		}
		for _, pa := range pares {
			pb, ok := bd.Pega(pa.Chave.(object.Chaveavel).ChaveHash())
			if !ok || !iguaisRec(pa.Valor, pb.Valor, prof, vistos) {
				return false
			}
		}
		return true
	case *object.Instancia:
		// == de treta compara campo a campo (igual struct do Go)
		bi := b.(*object.Instancia)
		if av.Tipo != bi.Tipo {
			return false
		}
		ac, bc := av.Campos(), bi.Campos()
		for j := range ac {
			if !iguaisRec(ac[j], bc[j], prof, vistos) {
				return false
			}
		}
		return true
	}
	return a == b
}

// globaisMu protege o slice de globais (dividido entre a VM raiz, os clones do
// `bora` e as VMs de cada entrada da Sessao) depois que o modo concorrente
// liga. Sem ela duas goroutines gravando global ao mesmo tempo podiam deixar
// uma interface pela metade (tipo de um valor, dado do outro) pra quem le — e
// isso derruba o processo. Antes da concorrencia ninguem trava. E uma so pro
// processo porque o slice da Sessao sobrevive as VMs que o usam.
var globaisMu sync.RWMutex

func pegaGlobalTravado(globals []object.Object, idx int) object.Object {
	globaisMu.RLock()
	v := globals[idx]
	globaisMu.RUnlock()
	return v
}

func poeGlobalTravado(globals []object.Object, idx int, v object.Object) {
	globaisMu.Lock()
	globals[idx] = v
	globaisMu.Unlock()
}
