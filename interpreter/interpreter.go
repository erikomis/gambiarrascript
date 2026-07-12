package interpreter

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/object"
	"gambiarrascript/parser"
)

var (
	DEU_BOM  = &object.Booleano{Value: true}
	DEU_RUIM = &object.Booleano{Value: false}
	NADA     = &object.Nada{}
)

type Interpreter struct {
	out               io.Writer
	erroOut           io.Writer // saída de diagnóstico (espera/afirma); default = os.Stderr
	in                io.Reader
	inBuf             *bufio.Reader
	argumentos        []string
	dirBase           string
	servidor          *servidorEstado
	builtinsInstancia map[string]*object.Builtin

	// ChamaCompilada e o gancho que a VM registra pra executar
	// *object.CompiledFunction. Permite que builtins de ordem superior
	// (mapeia, filtra, reduz, ordena_com, paralelo, handlers do rota...)
	// chamem funcoes do usuario tambem quando o engine e a VM.
	ChamaCompilada func(fn *object.CompiledFunction, args []object.Object) object.Object

	// estado de testes (gs testa): contagem de asserts passa/falha no arquivo
	// rodando agora. Zerado em resetting no rodarArquivo.
	totalEspera   int
	totalEsperaOk int

	// muOut protege i.out e i.erroOut — varias goroutines podem chamar
	// mostra/escreve/escreve_erro/espera/afirma concorrentemente. Sem o lock
	// a saida fica intercalada e podem vir pedaços pela metade.
	muOut sync.Mutex
}

func New(out io.Writer) *Interpreter {
	i := &Interpreter{out: out, erroOut: os.Stderr, in: os.Stdin}
	i.servidor = &servidorEstado{rotas: map[string]*object.Funcao{}, i: i}
	i.builtinsInstancia = map[string]*object.Builtin{
		"rota":         {Nome: "rota", Fn: i.servidor.builtinRota},
		"escuta":       {Nome: "escuta", Fn: i.servidor.builtinEscuta},
		"mapeia":       {Nome: "mapeia", Fn: i.builtinMapeia},
		"filtra":       {Nome: "filtra", Fn: i.builtinFiltra},
		"ordena_com":   {Nome: "ordena_com", Fn: i.builtinOrdenaCom},
		"agrupa_por":   {Nome: "agrupa_por", Fn: i.builtinAgrupaPor},
		"reduz":        {Nome: "reduz", Fn: i.builtinReduz},
		"acha":         {Nome: "acha", Fn: i.builtinAcha},
		"acha_indice":  {Nome: "acha_indice", Fn: i.builtinAchaIndice},
		"pergunta":     {Nome: "pergunta", Fn: i.builtinPergunta},
		"argumentos":   {Nome: "argumentos", Fn: i.builtinArgumentos},
		"le_tudo":      {Nome: "le_tudo", Fn: i.builtinLeTudo},
		"le_linhas":    {Nome: "le_linhas", Fn: i.builtinLeLinhas},
		"escreve":      {Nome: "escreve", Fn: i.builtinEscreve},
		"escreve_erro": {Nome: "escreve_erro", Fn: i.builtinEscreveErro},
		"env":          {Nome: "env", Fn: i.builtinEnv},
		"paralelo":     {Nome: "paralelo", Fn: i.builtinParalelo},
		"espera":       {Nome: "espera", Fn: i.builtinEspera},
		"afirma":       {Nome: "afirma", Fn: i.builtinAfirma},
		// concorrencia: canais (cano) e wait de Futuro
		"cano":   {Nome: "cano", Fn: i.builtinCano},
		"envia":  {Nome: "envia", Fn: i.builtinEnvia},
		"recebe": {Nome: "recebe", Fn: i.builtinRecebe},
		"fecha":  {Nome: "fecha", Fn: i.builtinFecha},
	}
	return i
}

// BuiltinsVisiveis devolve o merge das builtins globais com as instanciadas
// (rota/escuta/pergunta/etc.). Usado pela VM pra reaproveitar as mesmas
// implementacoes das builtins (incluindo estado de servidor HTTP).
func (i *Interpreter) BuiltinsVisiveis() map[string]*object.Builtin {
	out := map[string]*object.Builtin{}
	for k, v := range builtins {
		out[k] = v
	}
	for k, v := range i.builtinsInstancia {
		out[k] = v
	}
	return out
}

// DefinirStderr troca o escritor de diagnóstico usado por espera()/afirma().
// Util pra gs testa capturar o resultado dos asserts sem misturar com o out
// normal do script.
func (i *Interpreter) DefinirStderr(w io.Writer) { i.erroOut = w }

// TotaisTeste devolve (total, ok) — quantos asserts espera()/afirma() rodaram
// e quantos passaram. Usado pelo gs testa pra emitir o relatorio no fim.
func (i *Interpreter) TotaisTeste() (int, int) { return i.totalEspera, i.totalEsperaOk }

// ResetTeste reinicia os contadores de teste entre arquivos.
func (i *Interpreter) ResetTeste() { i.totalEspera = 0; i.totalEsperaOk = 0 }

// DefinirStdin troca o leitor de entrada usado por pergunta().
func (i *Interpreter) DefinirStdin(r io.Reader) { i.in = r; i.inBuf = nil }

// DefinirArgumentos configura os argumentos de linha de comando visiveis no
// script via a builtin argumentos().
func (i *Interpreter) DefinirArgumentos(args []string) { i.argumentos = args }

// DefinirDirBase configura o diretorio base pra resolver importa "caminho.gs".
func (i *Interpreter) DefinirDirBase(dir string) { i.dirBase = dir }

func (i *Interpreter) Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	case *ast.Program:
		return i.evalProgram(node, env)
	case *ast.ExpressionStatement:
		return i.Eval(node.Expression, env)
	case *ast.MostraStatement:
		val := i.Eval(node.Value, env)
		if isError(val) {
			return val
		}
		i.muOut.Lock()
		fmt.Fprintln(i.out, val.Inspect())
		i.muOut.Unlock()
		return val

	// --- literais ---
	case *ast.NumeroLiteral:
		if node.EhInt {
			return object.NumInt(node.Int)
		}
		return object.NumFloat(node.Value)
	case *ast.TextoLiteral:
		return &object.Texto{Value: node.Value}
	case *ast.TextoInterpolado:
		var sb strings.Builder
		for _, p := range node.Parts {
			v := i.Eval(p, env)
			if isError(v) {
				return v
			}
			sb.WriteString(v.Inspect())
		}
		return &object.Texto{Value: sb.String()}
	case *ast.BooleanoLiteral:
		return boolDoNativo(node.Value)
	case *ast.NadaLiteral:
		return NADA
	case *ast.Identifier:
		return i.evalIdentifier(node, env)
	case *ast.ListaLiteral:
		elems := i.evalExpressions(node.Elements, env)
		if len(elems) == 1 && isError(elems[0]) {
			return elems[0]
		}
		return &object.Lista{Elements: elems}
	case *ast.DicionarioLiteral:
		return i.evalDicionario(node, env)

	// --- operadores ---
	case *ast.PrefixExpression:
		right := i.Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return i.evalPrefix(node.Operator, right, node.Token.Line)
	case *ast.InfixExpression:
		return i.evalInfix(node, env)
	case *ast.FuncaoLiteral:
		// lambda anonima: closure sobre o env atual, igual gambiarra nomeada.
		return &object.Funcao{Parametros: node.Parameters, Body: node.Body, Env: env}
	case *ast.DesestruturaStatement:
		return i.evalDesestrutura(node, env)
	case *ast.EscolheStatement:
		return i.evalEscolhe(node, env)
	case *ast.RangeExpression:
		start := i.Eval(node.Start, env)
		if isError(start) {
			return start
		}
		end := i.Eval(node.End, env)
		if isError(end) {
			return end
		}
		return evalRange(start, end, node.Token.Line)
	case *ast.IndexExpression:
		left := i.Eval(node.Left, env)
		if isError(left) {
			return left
		}
		// navegacao segura: obj?.campo — se left for nada, devolve nada
		if node.Safe && left.Type() == object.NADA_OBJ {
			return NADA
		}
		index := i.Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return i.evalIndex(left, index, node.Token.Line)

	case *ast.BotaStatement:
		val := i.Eval(node.Value, env)
		if isError(val) {
			return val
		}
		if node.Name != nil {
			env.Set(node.Name.Value, val)
			return NADA
		}
		return i.evalAtribuiIndice(node.Indice, val, env)
	case *ast.FuncionaStatement:
		val := i.Eval(node.Value, env)
		if isError(val) {
			return val
		}
		return &object.Retorno{Value: val}
	case *ast.VazaStatement:
		return &object.Vaza{Line: node.Token.Line}
	case *ast.ContinuaStatement:
		return &object.Continua{Line: node.Token.Line}
	case *ast.BlockStatement:
		return i.evalBlock(node, env)
	case *ast.SeColarStatement:
		return i.evalSeColar(node, env)
	case *ast.EnquantoStatement:
		return i.evalEnquanto(node, env)
	case *ast.PraCadaNumStatement:
		return i.evalPraCadaNum(node, env)
	case *ast.PraCadaListStatement:
		return i.evalPraCadaList(node, env)
	case *ast.GambiarraStatement:
		fn := &object.Funcao{Parametros: node.Parameters, Body: node.Body, Env: env}
		env.Set(node.Name.Value, fn)
		return NADA
	case *ast.ArrumaStatement:
		return i.evalArruma(node, env)
	case *ast.ImportaStatement:
		return i.evalImporta(node, env)
	case *ast.FatiaExpression:
		return i.evalFatia(node, env)
	case *ast.TernarioExpression:
		cond := i.Eval(node.Cond, env)
		if isError(cond) {
			return cond
		}
		if isTruthy(cond) {
			return i.Eval(node.SeVerdadeiro, env)
		}
		return i.Eval(node.SeFalso, env)
	case *ast.CoalesceExpression:
		left := i.Eval(node.Left, env)
		if isError(left) {
			return left
		}
		if left.Type() == object.NADA_OBJ {
			return i.Eval(node.Right, env)
		}
		return left
	case *ast.CallExpression:
		fn := i.Eval(node.Function, env)
		if isError(fn) {
			return fn
		}
		args := i.evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return i.applyFunction(fn, args, node.Token.Line, nomeDaChamada(node))
	case *ast.BoraExpression:
		return i.evalBora(node, env)
	}
	return NADA
}

// evalBora dispara a chamada em `node.Call` numa goroutine e devolve um
// Futuro imediatamente. A goroutine roda i.applyFunction (mesma funcao que
// uma chamada normal, so que em paralelo), captura o resultado (erro inclusive)
// e resolve o Futuro. Panic dentro da fn vira *Erro (Nao deixa o processo cair).
func (i *Interpreter) evalBora(node *ast.BoraExpression, env *object.Environment) object.Object {
	call := node.Call
	if call == nil {
		return newError(node.Token.Line, "bora sem chamada — isso nao devia acontecer")
	}
	fn := i.Eval(call.Function, env)
	if isError(fn) {
		return fn
	}
	args := i.evalExpressions(call.Arguments, env)
	if len(args) == 1 && isError(args[0]) {
		return args[0]
	}
	linha := call.Token.Line
	nome := nomeDaChamada(call)

	fut := object.NovoFuturo()
	go func(f *object.Futuro, fnv object.Object, argv []object.Object, lh int, nm string) {
		defer func() {
			if r := recover(); r != nil {
				f.Resolve(newError(lh, "panico dentro do `bora %s`: %v", nm, r))
			}
		}()
		res := i.applyFunction(fnv, argv, lh, "<bora:"+nm+">")
		f.Resolve(res)
	}(fut, fn, args, linha, nome)
	return fut
}

func (i *Interpreter) evalProgram(prog *ast.Program, env *object.Environment) object.Object {
	var result object.Object = NADA
	for _, stmt := range prog.Statements {
		result = i.Eval(stmt, env)
		switch r := result.(type) {
		case *object.Retorno:
			return r.Value
		case *object.Erro:
			return r
		case *object.Sair:
			return r
		case *object.Vaza:
			return newError(r.Line, "deu `vaza` fora de um loop, parca — vaza pra onde?")
		case *object.Continua:
			return newError(r.Line, "deu `continua` fora de um loop, parca")
		}
	}
	return result
}

func (i *Interpreter) evalExpressions(exps []ast.Expression, env *object.Environment) []object.Object {
	var result []object.Object
	for _, e := range exps {
		ev := i.Eval(e, env)
		if isError(ev) {
			return []object.Object{ev}
		}
		result = append(result, ev)
	}
	return result
}

func (i *Interpreter) evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}
	if b, ok := i.builtinsInstancia[node.Value]; ok {
		return b
	}
	if b, ok := builtins[node.Value]; ok {
		return b
	}
	return newError(node.Token.Line, "cade o `%s`? voce nao botou isso ainda", node.Value)
}

// evalEscolhe: casa o subject contra cada `caso` (semantica do ==, via
// iguais) e roda o primeiro corpo que bater. Sem fallthrough. Se nada casar,
// roda o se_nao_colar (se existir).
func (i *Interpreter) evalEscolhe(node *ast.EscolheStatement, env *object.Environment) object.Object {
	subject := i.Eval(node.Subject, env)
	if isError(subject) {
		return subject
	}
	for _, braco := range node.Casos {
		for _, vexpr := range braco.Values {
			v := i.Eval(vexpr, env)
			if isError(v) {
				return v
			}
			if iguais(subject, v) {
				return i.evalBlock(braco.Body, env)
			}
		}
	}
	if node.Default != nil {
		return i.evalBlock(node.Default, env)
	}
	return NADA
}

// evalDesestrutura amarra os nomes do padrao aos valores correspondentes.
// Lista: por posicao; dicionario: por chave (nome da variavel). Nome sem
// valor correspondente vira nada (lenient).
func (i *Interpreter) evalDesestrutura(node *ast.DesestruturaStatement, env *object.Environment) object.Object {
	val := i.Eval(node.Value, env)
	if isError(val) {
		return val
	}
	if node.DeDict {
		d, ok := val.(*object.Dicionario)
		if !ok {
			return newError(node.Token.Line, "pra desestruturar com {} eu quero um dicionario, veio %s", val.Type())
		}
		for _, n := range node.Names {
			chave := (&object.Texto{Value: n.Value}).ChaveHash()
			if par, ok := d.Pares[chave]; ok {
				env.Set(n.Value, par.Valor)
			} else {
				env.Set(n.Value, NADA)
			}
		}
		return NADA
	}
	l, ok := val.(*object.Lista)
	if !ok {
		return newError(node.Token.Line, "pra desestruturar com [] eu quero uma lista, veio %s", val.Type())
	}
	for idx, n := range node.Names {
		if idx < len(l.Elements) {
			env.Set(n.Value, l.Elements[idx])
		} else {
			env.Set(n.Value, NADA)
		}
	}
	return NADA
}

// evalRange monta a lista [inicio, ..., fim] inclusive. So com inteiros.
// Se inicio > fim, gera decrescente (10..1 => [10, 9, ..., 1]).
func evalRange(start, end object.Object, linha int) object.Object {
	lo, ok := start.(*object.Numero)
	if !ok || !lo.EhInt {
		return newError(linha, "range .. quer inteiro na esquerda, veio %s", start.Type())
	}
	hi, ok := end.(*object.Numero)
	if !ok || !hi.EhInt {
		return newError(linha, "range .. quer inteiro na direita, veio %s", end.Type())
	}
	elems, ok := object.RangeInts(lo.Int, hi.Int)
	if !ok {
		return newError(linha, "range .. de %d..%d e gigante demais, vai estourar a memoria", lo.Int, hi.Int)
	}
	return &object.Lista{Elements: elems}
}

func (i *Interpreter) evalPrefix(op string, right object.Object, linha int) object.Object {
	switch op {
	case "nao":
		return boolDoNativo(!isTruthy(right))
	case "-":
		num, ok := right.(*object.Numero)
		if !ok {
			return newError(linha, "nao da pra colocar - na frente de %s", right.Type())
		}
		if num.EhInt {
			return object.NumInt(-num.Int)
		}
		return object.NumFloat(-num.Value)
	case "~":
		// NOT bitwise: so pra inteiros (^int em Go = complemento)
		num, ok := right.(*object.Numero)
		if !ok {
			return newError(linha, "~ espera inteiro, veio %s", right.Type())
		}
		if !num.EhInt {
			return newError(linha, "~ so funciona com inteiro (nao com ponto flutuante)")
		}
		return object.NumInt(^num.Int)
	}
	return newError(linha, "operador prefixo desconhecido: %s", op)
}

func (i *Interpreter) evalInfix(node *ast.InfixExpression, env *object.Environment) object.Object {
	// operadores logicos com curto-circuito
	if node.Operator == "e" || node.Operator == "ou" {
		left := i.Eval(node.Left, env)
		if isError(left) {
			return left
		}
		if node.Operator == "e" && !isTruthy(left) {
			return DEU_RUIM
		}
		if node.Operator == "ou" && isTruthy(left) {
			return DEU_BOM
		}
		right := i.Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return boolDoNativo(isTruthy(right))
	}

	left := i.Eval(node.Left, env)
	if isError(left) {
		return left
	}
	right := i.Eval(node.Right, env)
	if isError(right) {
		return right
	}

	ln, lok := left.(*object.Numero)
	rn, rok := right.(*object.Numero)
	if lok && rok {
		return i.evalInfixNumero(node.Operator, ln, rn, node.Token.Line)
	}

	if node.Operator == "+" && (left.Type() == object.TEXTO_OBJ || right.Type() == object.TEXTO_OBJ) {
		return &object.Texto{Value: left.Inspect() + right.Inspect()}
	}

	switch node.Operator {
	case "==":
		return boolDoNativo(iguais(left, right))
	case "!=":
		return boolDoNativo(!iguais(left, right))
	}

	return newError(node.Token.Line, "nao da pra fazer %s %s %s", left.Type(), node.Operator, right.Type())
}

func (i *Interpreter) evalInfixNumero(op string, lo, ro *object.Numero, linha int) object.Object {
	// Caminho inteiro exato: so quando os dois lados sao inteiros. Mantem
	// precisao acima de 2^53 (somas, contagens, multiplicacoes gigantes).
	bothInt := lo.EhInt && ro.EhInt
	l, r := lo.Value, ro.Value
	switch op {
	case "+":
		if bothInt {
			return object.NumInt(lo.Int + ro.Int)
		}
		return object.NumFloat(l + r)
	case "-":
		if bothInt {
			return object.NumInt(lo.Int - ro.Int)
		}
		return object.NumFloat(l - r)
	case "*":
		if bothInt {
			return object.NumInt(lo.Int * ro.Int)
		}
		return object.NumFloat(l * r)
	case "/":
		if r == 0 {
			return newError(linha, "nao da pra dividir por zero, parca — nem na gambiarra")
		}
		// divisao exata entre inteiros continua inteiro; senao vira float.
		if bothInt && lo.Int%ro.Int == 0 {
			return object.NumInt(lo.Int / ro.Int)
		}
		return object.NumFloat(l / r)
	case "%":
		if r == 0 {
			return newError(linha, "resto de divisao por zero? ai voce quer demais")
		}
		if bothInt {
			return object.NumInt(lo.Int % ro.Int)
		}
		return object.NumFloat(math.Mod(l, r))
	case "<":
		if bothInt {
			return boolDoNativo(lo.Int < ro.Int)
		}
		return boolDoNativo(l < r)
	case ">":
		if bothInt {
			return boolDoNativo(lo.Int > ro.Int)
		}
		return boolDoNativo(l > r)
	case "<=":
		if bothInt {
			return boolDoNativo(lo.Int <= ro.Int)
		}
		return boolDoNativo(l <= r)
	case ">=":
		if bothInt {
			return boolDoNativo(lo.Int >= ro.Int)
		}
		return boolDoNativo(l >= r)
	case "==":
		if bothInt {
			return boolDoNativo(lo.Int == ro.Int)
		}
		return boolDoNativo(l == r)
	case "!=":
		if bothInt {
			return boolDoNativo(lo.Int != ro.Int)
		}
		return boolDoNativo(l != r)
	case "&":
		if bothInt {
			return object.NumInt(lo.Int & ro.Int)
		}
		return newError(linha, "& bitwise so faz sentido com inteiros")
	case "|":
		if bothInt {
			return object.NumInt(lo.Int | ro.Int)
		}
		return newError(linha, "| bitwise so faz sentido com inteiros")
	case "^":
		if bothInt {
			return object.NumInt(lo.Int ^ ro.Int)
		}
		return newError(linha, "^ bitwise so faz sentido com inteiros")
	case "<<":
		if bothInt {
			if ro.Int < 0 {
				return newError(linha, "<< por valor negativo? naoiedade")
			}
			return object.NumInt(lo.Int << uint(ro.Int))
		}
		return newError(linha, "shift so faz sentido com inteiros")
	case ">>":
		if bothInt {
			if ro.Int < 0 {
				return newError(linha, ">> por valor negativo? naoiedade")
			}
			return object.NumInt(lo.Int >> uint(ro.Int))
		}
		return newError(linha, "shift so faz sentido com inteiros")
	}
	return newError(linha, "operador desconhecido pra numeros: %s", op)
}

func (i *Interpreter) evalAtribuiIndice(alvo *ast.IndexExpression, val object.Object, env *object.Environment) object.Object {
	cont := i.Eval(alvo.Left, env)
	if isError(cont) {
		return cont
	}
	idx := i.Eval(alvo.Index, env)
	if isError(idx) {
		return idx
	}
	linha := alvo.Token.Line
	switch c := cont.(type) {
	case *object.Lista:
		n, ok := idx.(*object.Numero)
		if !ok {
			return newError(linha, "indice de lista tem que ser numero, veio %s", idx.Type())
		}
		pos, dentro := object.IndiceNormalizado(int(n.Value), len(c.Elements))
		if !dentro {
			return newError(linha, "esse indice (%d) ta fora da lista, o", int(n.Value))
		}
		c.Elements[pos] = val
		return NADA
	case *object.Dicionario:
		chave, ok := idx.(object.Chaveavel)
		if !ok {
			return newError(linha, "essa chave (%s) nao da pra usar num dicionario", idx.Type())
		}
		c.Pares[chave.ChaveHash()] = object.ParDic{Chave: idx, Valor: val}
		return NADA
	default:
		return newError(linha, "so da pra atribuir indice em lista ou dicionario, e isso ai e %s", cont.Type())
	}
}

func (i *Interpreter) evalDicionario(node *ast.DicionarioLiteral, env *object.Environment) object.Object {
	pares := map[object.HashKey]object.ParDic{}
	for _, par := range node.Pares {
		chave := i.Eval(par.Chave, env)
		if isError(chave) {
			return chave
		}
		chaveavel, ok := chave.(object.Chaveavel)
		if !ok {
			return newError(node.Token.Line, "nao da pra usar %s como chave de dicionario", chave.Type())
		}
		valor := i.Eval(par.Valor, env)
		if isError(valor) {
			return valor
		}
		pares[chaveavel.ChaveHash()] = object.ParDic{Chave: chave, Valor: valor}
	}
	return &object.Dicionario{Pares: pares}
}

func (i *Interpreter) evalIndex(left, index object.Object, linha int) object.Object {
	switch c := left.(type) {
	case *object.Lista:
		idx, ok := index.(*object.Numero)
		if !ok {
			return newError(linha, "indice de lista tem que ser numero, veio %s", index.Type())
		}
		pos, dentro := object.IndiceNormalizado(int(idx.Value), len(c.Elements))
		if !dentro {
			return newError(linha, "esse indice (%d) ta fora da lista, o", int(idx.Value))
		}
		return c.Elements[pos]
	case *object.Texto:
		idx, ok := index.(*object.Numero)
		if !ok {
			return newError(linha, "indice de texto tem que ser numero, veio %s", index.Type())
		}
		runes := []rune(c.Value)
		pos, dentro := object.IndiceNormalizado(int(idx.Value), len(runes))
		if !dentro {
			return newError(linha, "esse indice (%d) ta fora do texto, o", int(idx.Value))
		}
		return &object.Texto{Value: string(runes[pos])}
	case *object.Dicionario:
		chave, ok := index.(object.Chaveavel)
		if !ok {
			return newError(linha, "essa chave (%s) nao da pra usar num dicionario", index.Type())
		}
		par, existe := c.Pares[chave.ChaveHash()]
		if !existe {
			return NADA
		}
		return par.Valor
	default:
		return newError(linha, "so da pra indexar lista, texto ou dicionario, e isso ai e %s", left.Type())
	}
}

func boolDoNativo(b bool) *object.Booleano {
	if b {
		return DEU_BOM
	}
	return DEU_RUIM
}

func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Nada:
		return false
	case *object.Booleano:
		return obj.Value
	default:
		return true
	}
}

func iguais(a, b object.Object) bool {
	if a.Type() != b.Type() {
		return false
	}
	switch av := a.(type) {
	case *object.Texto:
		return av.Value == b.(*object.Texto).Value
	case *object.Booleano:
		return av.Value == b.(*object.Booleano).Value
	case *object.Numero:
		bn := b.(*object.Numero)
		if av.EhInt && bn.EhInt {
			return av.Int == bn.Int
		}
		return av.Value == bn.Value
	case *object.Nada:
		return true
	case *object.Lista:
		bl, ok := b.(*object.Lista)
		if !ok || len(av.Elements) != len(bl.Elements) {
			return false
		}
		for j, e := range av.Elements {
			if !iguais(e, bl.Elements[j]) {
				return false
			}
		}
		return true
	case *object.Dicionario:
		bd := b.(*object.Dicionario)
		if len(av.Pares) != len(bd.Pares) {
			return false
		}
		for k, pa := range av.Pares {
			pb, ok := bd.Pares[k]
			if !ok || !iguais(pa.Valor, pb.Valor) {
				return false
			}
		}
		return true
	}
	return a == b
}

// evalBlock avalia statements em sequencia e propaga sinais de controle
// (Retorno, Erro, Vaza, Continua) sem desembrulhar — quem propaga decide.
func (i *Interpreter) evalBlock(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object = NADA
	for _, stmt := range block.Statements {
		result = i.Eval(stmt, env)
		if result != nil {
			switch result.Type() {
			case object.RETORNO_OBJ, object.ERRO_OBJ, object.VAZA_OBJ, object.CONTINUA_OBJ, object.SAIR_OBJ:
				return result
			}
		}
	}
	return result
}

func (i *Interpreter) evalSeColar(node *ast.SeColarStatement, env *object.Environment) object.Object {
	for idx, cond := range node.Conditions {
		c := i.Eval(cond, env)
		if isError(c) {
			return c
		}
		if isTruthy(c) {
			return i.evalBlock(node.Consequences[idx], env)
		}
	}
	if node.Alternative != nil {
		return i.evalBlock(node.Alternative, env)
	}
	return NADA
}

func (i *Interpreter) evalEnquanto(node *ast.EnquantoStatement, env *object.Environment) object.Object {
	for {
		c := i.Eval(node.Condition, env)
		if isError(c) {
			return c
		}
		if !isTruthy(c) {
			break
		}
		res := i.evalBlock(node.Body, env)
		if res != nil {
			switch res.Type() {
			case object.ERRO_OBJ, object.RETORNO_OBJ, object.SAIR_OBJ:
				return res
			case object.VAZA_OBJ:
				return NADA
			case object.CONTINUA_OBJ:
				continue
			}
		}
	}
	return NADA
}

func (i *Interpreter) evalPraCadaNum(node *ast.PraCadaNumStatement, env *object.Environment) object.Object {
	inicio := i.Eval(node.Start, env)
	if isError(inicio) {
		return inicio
	}
	fim := i.Eval(node.End, env)
	if isError(fim) {
		return fim
	}
	ni, ok1 := inicio.(*object.Numero)
	nf, ok2 := fim.(*object.Numero)
	if !ok1 || !ok2 {
		return newError(node.Token.Line, "no pra_cada de..ate eu preciso de numeros, parca")
	}

	// corpo roda o bloco com a variavel = val. Devolve (resultado, parar): se
	// parar for true o loop aborta devolvendo resultado (erro/retorno) ou NADA
	// (vaza). continua simplesmente cai pra proxima iteracao.
	corpo := func(val *object.Numero) (object.Object, bool) {
		env.Set(node.Var.Value, val)
		res := i.evalBlock(node.Body, env)
		if res != nil {
			switch res.Type() {
			case object.ERRO_OBJ, object.RETORNO_OBJ, object.SAIR_OBJ:
				return res, true
			case object.VAZA_OBJ:
				return NADA, true
			}
		}
		return nil, false
	}

	// Caminho inteiro exato quando os dois limites sao inteiros — evita a perda
	// de precisao do float64 em contadores gigantes.
	if ni.EhInt && nf.EhInt {
		for v := ni.Int; v <= nf.Int; v++ {
			if res, parar := corpo(object.NumInt(v)); parar {
				return res
			}
		}
		return NADA
	}
	for v := ni.Value; v <= nf.Value; v++ {
		if res, parar := corpo(object.NumFloat(v)); parar {
			return res
		}
	}
	return NADA
}

func (i *Interpreter) evalPraCadaList(node *ast.PraCadaListStatement, env *object.Environment) object.Object {
	it := i.Eval(node.Iterable, env)
	if isError(it) {
		return it
	}

	doisNomes := len(node.Vars) == 2

	switch c := it.(type) {
	case *object.Lista:
		for idx, item := range c.Elements {
			if doisNomes {
				env.Set(node.Vars[0].Value, object.NumInt(int64(idx)))
				env.Set(node.Vars[1].Value, item)
			} else {
				env.Set(node.Vars[0].Value, item)
			}
			res := i.evalBlock(node.Body, env)
			if res != nil {
				switch res.Type() {
				case object.ERRO_OBJ, object.RETORNO_OBJ, object.SAIR_OBJ:
					return res
				case object.VAZA_OBJ:
					return NADA
				case object.CONTINUA_OBJ:
					continue
				}
			}
		}
	case *object.Dicionario:
		for _, par := range c.Pares {
			if doisNomes {
				env.Set(node.Vars[0].Value, par.Chave)
				env.Set(node.Vars[1].Value, par.Valor)
			} else {
				env.Set(node.Vars[0].Value, par.Chave)
			}
			res := i.evalBlock(node.Body, env)
			if res != nil {
				switch res.Type() {
				case object.ERRO_OBJ, object.RETORNO_OBJ, object.SAIR_OBJ:
					return res
				case object.VAZA_OBJ:
					return NADA
				case object.CONTINUA_OBJ:
					continue
				}
			}
		}
	default:
		return newError(node.Token.Line, "pra_cada ... em ... so funciona com lista ou dicionario, e isso ai e %s", it.Type())
	}
	return NADA
}

func (i *Interpreter) evalImporta(node *ast.ImportaStatement, env *object.Environment) object.Object {
	caminhoVal := i.Eval(node.Path, env)
	if isError(caminhoVal) {
		return caminhoVal
	}
	caminho, ok := caminhoVal.(*object.Texto)
	if !ok {
		return newError(node.Token.Line, "importa quer um texto (caminho), veio %s", caminhoVal.Type())
	}
	resolvido := caminho.Value
	if !filepath.IsAbs(resolvido) && i.dirBase != "" {
		resolvido = filepath.Join(i.dirBase, resolvido)
	}
	fonte, err := os.ReadFile(resolvido)
	if err != nil {
		return newError(node.Token.Line, "nao consegui importar %q: %v", caminho.Value, err)
	}
	p := parser.New(lexer.New(string(fonte)))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return newError(node.Token.Line, "o modulo %q ta com perrengue: %s", caminho.Value, errs[0])
	}

	dirAntes := i.dirBase
	i.DefinirDirBase(filepath.Dir(resolvido))

	if node.Alias != nil {
		// importa "x.gs" como alias — modulo vira namespace isolado
		modEnv := object.NewEnclosedEnvironment(env)
		res := i.evalProgram(prog, modEnv)
		i.DefinirDirBase(dirAntes)
		if isError(res) {
			return res
		}
		// Cria um dicionario com todas as definicoes do modulo
		pares := map[object.HashKey]object.ParDic{}
		for _, nome := range modEnv.Locais() {
			if v, ok := modEnv.Get(nome); ok {
				chave := &object.Texto{Value: nome}
				pares[chave.ChaveHash()] = object.ParDic{Chave: chave, Valor: v}
			}
		}
		modulo := &object.Dicionario{Pares: pares}
		env.Set(node.Alias.Value, modulo)
		return NADA
	}

	// importa sem alias — despeja tudo no escopo (comportamento classico)
	modEnv := object.NewEnclosedEnvironment(env)
	res := i.evalProgram(prog, modEnv)
	i.DefinirDirBase(dirAntes)
	if isError(res) {
		return res
	}
	for _, nome := range modEnv.Locais() {
		if v, ok := modEnv.Get(nome); ok {
			env.Set(nome, v)
		}
	}
	return NADA
}

// evalFatia executa xs[inicio:fim] pra lista e texto. nil = omitido.
func (i *Interpreter) evalFatia(node *ast.FatiaExpression, env *object.Environment) object.Object {
	left := i.Eval(node.Left, env)
	if isError(left) {
		return left
	}
	linha := node.Token.Line

	var inicioVal, fimVal *object.Numero
	if node.Inicio != nil {
		v := i.Eval(node.Inicio, env)
		if isError(v) {
			return v
		}
		n, ok := v.(*object.Numero)
		if !ok {
			return newError(linha, "fatia so aceita numero como inicio, veio %s", v.Type())
		}
		inicioVal = n
	}
	if node.Fim != nil {
		v := i.Eval(node.Fim, env)
		if isError(v) {
			return v
		}
		n, ok := v.(*object.Numero)
		if !ok {
			return newError(linha, "fatia so aceita numero como fim, veio %s", v.Type())
		}
		fimVal = n
	}

	switch c := left.(type) {
	case *object.Lista:
		lo, hi := object.NormalizarFatia(inicioVal, fimVal, len(c.Elements))
		return &object.Lista{Elements: c.Elements[lo:hi]}
	case *object.Texto:
		runes := []rune(c.Value)
		lo, hi := object.NormalizarFatia(inicioVal, fimVal, len(runes))
		return &object.Texto{Value: string(runes[lo:hi])}
	default:
		return newError(linha, "so da pra fatiar lista ou texto, e isso ai e %s", left.Type())
	}
}

func (i *Interpreter) evalArruma(node *ast.ArrumaStatement, env *object.Environment) object.Object {
	res := i.evalBlock(node.Try, env)

	// se houve erro e existe catch: captura e amarra ao ErrName.
	if res != nil && res.Type() == object.ERRO_OBJ && node.Catch != nil {
		erro := res.(*object.Erro)
		erro.Handled = true
		if node.ErrName != nil {
			env.Set(node.ErrName.Value, erro)
		}
		res = i.evalBlock(node.Catch, env)
	}

	// finally sempre roda, com erro/return/vaza/continua pendente. O valor
	// pendente (res) ainda sera propagado, a menos que o finally troque (retorn
	// novo). Simplificacao: finally so roda *antes* de propagar; nao captura
	// erro/return — deve apenas executar cleanup efeito colateral.
	if node.Finally != nil {
		fin := i.evalBlock(node.Finally, env)
		// se finally devolveu algo (return/erro/vaza/continua) explicitamente,
		// toma precedencia — sobrepoe o res do try/catch.
		if fin != nil && fin.Type() != object.NADA_OBJ {
			res = fin
		}
	}
	return res
}

func (i *Interpreter) applyFunction(fn object.Object, args []object.Object, linha int, nome string) object.Object {
	if b, ok := fn.(*object.Builtin); ok {
		res := b.Fn(args)
		// Builtins nao tem call-site no AST; registra so o nome da builtin
		// como frame, na linha onde foi chamada.
		if err, ok := res.(*object.Erro); ok && err != nil {
			empilhaFrame(err, b.Nome, linha)
			kind := err.Kind
			if kind == "" {
				kind = KindBuiltin
			}
			err.Kind = kind
		}
		return res
	}
	// CompiledFunction (bytecode da VM): builtins de ordem superior (mapeia,
	// filtra, reduz, ordena_com, paralelo...) recebem esse tipo quando o
	// engine e a VM. A VM registra ChamaCompilada no boot; sem ela (tree-
	// walker puro) e erro mesmo.
	if cf, ok := fn.(*object.CompiledFunction); ok {
		if i.ChamaCompilada != nil {
			return i.ChamaCompilada(cf, args)
		}
		return newError(linha, "essa gambiarra e compilada (VM) e o engine atual nao sabe rodar ela")
	}
	funcao, ok := fn.(*object.Funcao)
	if !ok {
		return newError(linha, "isso ai (%s) nao e gambiarra pra voce sair chamando", fn.Type())
	}
	// Valida argc: Varargs aceita >= minReq; padrao aceita entre minReq e
	// total. Sem padrao/varargs: strictly igual.
	minReq := 0
	temVariadic := false
	for _, p := range funcao.Parametros {
		if p.Variadico {
			temVariadic = true
			continue
		}
		if p.Padrao == nil {
			minReq++
		}
	}
	totalParams := len(funcao.Parametros)
	if temVariadic {
		if len(args) < minReq {
			return newError(linha, "essa gambiarra quer no minimo %d parametro(s), voce mandou %d", minReq, len(args))
		}
	} else if totalParams > 0 {
		hasDefaults := false
		for _, p := range funcao.Parametros {
			if p.Padrao != nil {
				hasDefaults = true
				break
			}
		}
		if hasDefaults {
			if len(args) < minReq || len(args) > totalParams {
				return newError(linha, "essa gambiarra quer entre %d e %d parametro(s), voce mandou %d", minReq, totalParams, len(args))
			}
		} else if len(args) != totalParams {
			return newError(linha, "essa gambiarra quer %d parametro(s), voce mandou %d", totalParams, len(args))
		}
	} else if totalParams == 0 && len(args) != 0 {
		return newError(linha, "essa gambiarra nao quer parametro(s), voce mandou %d", len(args))
	}
	escopo := object.NewEnclosedEnvironment(funcao.Env)
	for idx, p := range funcao.Parametros {
		if p.Variadico {
			// coleta args extras (do idx em diante) numa lista
			resto := []object.Object{}
			if idx < len(args) {
				resto = args[idx:]
			}
			escopo.Set(p.Nome.Value, &object.Lista{Elements: resto})
		} else if idx < len(args) {
			escopo.Set(p.Nome.Value, args[idx])
		} else if p.Padrao != nil {
			// valor padrao: avalia no escopo da funcao (nao do caller)
			defaultVal := i.Eval(p.Padrao, escopo)
			escopo.Set(p.Nome.Value, defaultVal)
		} else {
			escopo.Set(p.Nome.Value, NADA)
		}
	}
	avaliado := i.evalBlock(funcao.Body, escopo)
	if s, ok := avaliado.(*object.Sair); ok {
		return s // sai() desenrola a funcao inteira ate o topo
	}
	if ret, ok := avaliado.(*object.Retorno); ok {
		return ret.Value
	}
	if isError(avaliado) {
		// Empilha o frame desta chamada no erro que bubble up. Cada nivel de
		// applyFunction prepended o seu proprio frame, mantendo ordem externo->
		// interno.
		if err, ok := avaliado.(*object.Erro); ok && err != nil {
			empilhaFrame(err, nome, linha)
		}
		return avaliado
	}
	if v, ok := avaliado.(*object.Vaza); ok {
		return newError(v.Line, "deu `vaza` fora de um loop, parca — vaza pra onde?")
	}
	if c, ok := avaliado.(*object.Continua); ok {
		return newError(c.Line, "deu `continua` fora de um loop, parca")
	}
	return NADA
}

// nomeDaChamada extrai o nome "amigavel" de uma chamada pra usar no traço de
// pilha. Se for `foo(...)` devolve "foo", otherwise "<anonima>".
func nomeDaChamada(node *ast.CallExpression) string {
	if ident, ok := node.Function.(*ast.Identifier); ok {
		return ident.Value
	}
	return "<anonima>"
}
