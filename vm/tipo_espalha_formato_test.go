package vm

import "testing"

// tipo(x): nome do tipo igual nos 2 engines (gambiarra, lambda, closure e
// builtin sao todas "funcao").
func TestTipo(t *testing.T) {
	casos := []struct{ src, saida string }{
		{`mostra tipo(1)`, "numero\n"},
		{`mostra tipo(1.5)`, "numero\n"},
		{`mostra tipo("a")`, "texto\n"},
		{"mostra tipo(`cru`)", "texto\n"},
		{`mostra tipo(deu_bom)`, "booleano\n"},
		{`mostra tipo(nada)`, "nada\n"},
		{`mostra tipo([1, 2])`, "lista\n"},
		{`mostra tipo({"a": 1})`, "dicionario\n"},
		{`mostra tipo(conjunto([1, 2]))`, "conjunto\n"},
		{`gambiarra f() funciona 1 acabou_finalmente
mostra tipo(f)`, "funcao\n"},
		{`mostra tipo(gambiarra(x) funciona x acabou_finalmente)`, "funcao\n"},
		{`gambiarra fora(n)
    funciona gambiarra(x) funciona x + n acabou_finalmente
acabou_finalmente
mostra tipo(fora(1))`, "funcao\n"},
		{`mostra tipo(tamanho)`, "funcao\n"},
		{`mostra tipo(mapeia)`, "funcao\n"},
		{`mostra tipo(tipo)`, "funcao\n"},
		{`mostra tipo(cano(1))`, "cano\n"},
		{`gambiarra f() funciona 1 acabou_finalmente
bota fu = bora f()
mostra tipo(fu)
espera(fu)`, "futuro\n"},
		{`arruma
    quebra("x")
quebrou err
    mostra tipo(err)
acabou_finalmente`, "erro\n"},
		{`mostra tipo(tipo(1))`, "texto\n"},
		// interpolacao sempre da texto (a VM devolvia o valor cru)
		{`mostra tipo("${1}")`, "texto\n"},
		{`mostra "${1}${2}"`, "12\n"},
		// type switch com escolhe
		{`gambiarra descreve(v)
    escolhe tipo(v)
        caso "numero"
            funciona "num"
        caso "texto"
            funciona "txt"
    acabou_finalmente
    funciona "outro"
acabou_finalmente
mostra descreve(1) + descreve("a") + descreve([])`, "numtxtoutro\n"},
		// binding do usuario sombreia o builtin
		{`bota tipo = "meu"
mostra tipo`, "meu\n"},
		{`gambiarra tipo(v) funciona "sombra" acabou_finalmente
mostra tipo(1)`, "sombra\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
	esperaNosDois(t, `mostra tipo()`, "", "tipo() quer 1 argumento, veio 0")
	esperaNosDois(t, `mostra tipo(1, 2)`, "", "tipo() quer 1 argumento, veio 2")
}

// f(...lista): espalha a lista em argumentos posicionais.
func TestEspalhaNaChamada(t *testing.T) {
	casos := []struct{ src, saida string }{
		{`gambiarra soma3(a, b, c) funciona a + b + c acabou_finalmente
bota xs = [1, 2, 3]
mostra soma3(...xs)`, "6\n"},
		// misturado com args normais e varios spreads
		{`gambiarra lista4(a, b, c, d) funciona [a, b, c, d] acabou_finalmente
bota xs = [2, 3]
mostra lista4(1, ...xs, 4)
mostra lista4(...[1], ...xs, ...[4])
mostra lista4(...[], 1, 2, ...[3, 4])`, "[1, 2, 3, 4]\n[1, 2, 3, 4]\n[1, 2, 3, 4]\n"},
		// builtins
		{`bota xs = [3, 9, 4]
mostra max(...xs)
mostra min(...xs)`, "9\n3\n"},
		{`bota par = [7, 42]
mostra formata("%d-%d", ...par)`, "7-42\n"},
		{`mostra tamanho(...[[1, 2, 3]])`, "3\n"},
		// lambda
		{`bota f = gambiarra(a, b) funciona a * b acabou_finalmente
mostra f(...[6, 7])`, "42\n"},
		// metodo via ponto
		{`bota o = {"soma": gambiarra(a, b) funciona a + b acabou_finalmente}
mostra o.soma(...[1, 2])`, "3\n"},
		// varargs: os extras viram o ...resto
		{`gambiarra g(a, ...resto) funciona [a, resto] acabou_finalmente
bota xs = [1, 2, 3]
mostra g(...xs)
mostra g(0, ...xs)
mostra g(...[9])`, "[1, [2, 3]]\n[0, [1, 2, 3]]\n[9, []]\n"},
		// default completa o que faltou
		{`gambiarra h(a, b = 10, c = 20) funciona [a, b, c] acabou_finalmente
mostra h(...[1])
mostra h(...[1, 2])
mostra h(...[1, 2, 3])`, "[1, 10, 20]\n[1, 2, 20]\n[1, 2, 3]\n"},
		// lista vazia espalhada = sem argumento
		{`gambiarra z() funciona "zero" acabou_finalmente
mostra z(...[])`, "zero\n"},
		// espalhar nao muda a lista original
		{`gambiarra g(...r)
    adiciona(r, 99)
    funciona r
acabou_finalmente
bota xs = [1, 2]
mostra g(...xs)
mostra xs`, "[1, 2, 99]\n[1, 2]\n"},
		// bora
		{`gambiarra soma2(a, b) funciona a + b acabou_finalmente
bota fu = bora soma2(...[20, 22])
mostra espera(fu)`, "42\n"},
		{`gambiarra g(a, ...resto) funciona [a, resto] acabou_finalmente
mostra espera(bora g(...[1, 2, 3]))`, "[1, [2, 3]]\n"},
		{`mostra espera(bora max(...[1, 5, 2]))`, "5\n"},
		// funciona f(...xs): recursao continua certa
		{`gambiarra conta(n, acc)
    se_colar n == 0
        funciona acc
    acabou_finalmente
    funciona conta(...[n - 1, acc + n])
acabou_finalmente
mostra conta(100, 0)`, "5050\n"},
		// spread dentro de interpolacao e de argumento aninhado
		{`bota xs = [1, 2]
mostra "max=${max(...xs)}"`, "max=2\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
}

// bora com varargs/default sem spread: a VM nao juntava o resto nem
// completava os defaults (slot ficava nil).
func TestBoraVarargsDefault(t *testing.T) {
	esperaNosDois(t, `gambiarra g(a, ...resto) funciona [a, resto] acabou_finalmente
mostra espera(bora g(1, 2, 3))`, "[1, [2, 3]]\n", "")
	esperaNosDois(t, `gambiarra h(a, b = 5) funciona a + b acabou_finalmente
mostra espera(bora h(1))`, "6\n", "")
}

func TestEspalhaErros(t *testing.T) {
	esperaNosDois(t, `gambiarra f(a) funciona a acabou_finalmente
mostra f(...5)`, "", "so da pra espalhar lista, veio numero")
	esperaNosDois(t, `mostra max(..."abc")`, "", "so da pra espalhar lista, veio texto")
	esperaNosDois(t, `gambiarra f(a) funciona a acabou_finalmente
mostra f(...{"a": 1})`, "", "so da pra espalhar lista, veio dicionario")
	// aridade: mesma mensagem da chamada normal
	esperaNosDois(t, `gambiarra f(a, b) funciona a acabou_finalmente
mostra f(...[1, 2, 3])`, "", "essa gambiarra quer 2 parametro(s), voce mandou 3")
	esperaNosDois(t, `gambiarra f(a, b = 1) funciona a acabou_finalmente
mostra f(...[])`, "", "essa gambiarra quer entre 1 e 2 parametro(s), voce mandou 0")
	esperaNosDois(t, `gambiarra f(a, b, ...c) funciona a acabou_finalmente
mostra f(...[1])`, "", "essa gambiarra quer no minimo 2 parametro(s), voce mandou 1")
	// espalhar da pra pegar com arruma
	esperaNosDois(t, `arruma
    mostra max(...nada)
quebrou err
    mostra "pegou"
acabou_finalmente`, "pegou\n", "")
	// `...` fora de chamada: erro de parse claro
	esperaNosDois(t, `bota xs = ...[1]`, "", "`...` so vale")
	esperaNosDois(t, `mostra [...[1]]`, "", "`...` so vale")
	esperaNosDois(t, `mostra max(...)`, "", "faltou a lista depois do `...`")
}

// "${expr:fmt}": formato com os verbos do formata, sem o %.
func TestInterpolacaoComFormato(t *testing.T) {
	casos := []struct{ src, saida string }{
		{`mostra "${3.14159:.2f}"`, "3.14\n"},
		{`bota preco = 10
mostra "R$ ${preco:.2f}"`, "R$ 10.00\n"},
		{`bota n = 42
mostra "[${n:05d}]"`, "[00042]\n"},
		{`mostra "[${10 / 2:03d}]"`, "[005]\n"},
		{`mostra "[${"ab":>5}]"`, "parse"},
		{`mostra "[${"ab":5}]"`, "[   ab]\n"},
		{`mostra "[${"ab":-5}]"`, "[ab   ]\n"},
		{`mostra "${255:x} ${255:X} ${5:b} ${8:o}"`, "ff FF 101 10\n"},
		{`mostra "${1234.5:e}"`, "1.234500e+03\n"},
		{`mostra "${"oi":q}"`, "\"oi\"\n"},
		{`mostra "${2:+d}"`, "+2\n"},
		{`mostra "${ 3.14159 : .3f }"`, "3.142\n"},
		// `:` legitimo dentro da expressao nao e formato
		{`mostra "${ {"a": 1}["a"] }"`, "1\n"},
		{`bota xs = [1, 2, 3, 4]
mostra "${xs[1:3]}"`, "[2, 3]\n"},
		{`mostra "${"a:b"}"`, "a:b\n"},
		{"mostra \"${`a:b`}\"", "a:b\n"},
		{`bota d = {"x": 1.5}
mostra "${d["x"]:.1f} e ${ {"y": 2}["y"]:03d}"`, "1.5 e 002\n"},
		{`bota xs = [1.25, 2.5]
mostra "${xs[0:1][0]:.1f}"`, "1.2\n"},
		// ternario e chamada com formato
		{`bota x = 3
mostra "${se_colar x > 2 entao 1.5 se_nao_colar 2.5:.2f}"`, "1.50\n"},
		{`mostra "${max(1.5, 2):.1f}"`, "2.0\n"},
		// crase tambem interpola
		{"bota v = 2.5\nmostra `v=${v:.2f}`", "v=2.50\n"},
		// formato devolve texto e o resto da string segue normal
		{`bota a = 1
bota b = 2.5
mostra "${a}+${b:.1f}=${a + b:.2f}!"`, "1+2.5=3.50!\n"},
		// \${ continua literal
		{`mostra "\${x:.2f}"`, "${x:.2f}\n"},
		// sombrear formata nao muda a interpolacao
		{`bota formata = 1
mostra "${3.14159:.2f}"`, "3.14\n"},
	}
	for _, c := range casos {
		if c.saida == "parse" {
			esperaNosDois(t, c.src, "", "formato")
			continue
		}
		esperaNosDois(t, c.src, c.saida, "")
	}
}

// lixo dentro de ${...} agora e erro de parse (antes sumia calado).
func TestInterpolacaoLixoEErro(t *testing.T) {
	esperaNosDois(t, `mostra "${1 2}"`, "", "sobrou")
	esperaNosDois(t, `bota x = 1
mostra "${x y}"`, "", "sobrou")
	esperaNosDois(t, `mostra "${3.14:zz}"`, "", "formato")
	esperaNosDois(t, `mostra "${3.14:}"`, "", "formato")
	esperaNosDois(t, `mostra "${:.2f}"`, "", "expressao vazia")
	esperaNosDois(t, `mostra "${1:.2f:.2f}"`, "", "sobrou")
}

// formata converte numero conforme o verbo: inteiro com %f e float inteiro
// com %d funcionam (antes saia %!f(int64=3)).
func TestFormataConverteNumero(t *testing.T) {
	esperaNosDois(t, `mostra formata("%.2f", 3)`, "3.00\n", "")
	esperaNosDois(t, `mostra formata("%d", 10 / 2)`, "5\n", "")
	esperaNosDois(t, `mostra formata("%d|%.1f|%s|%%|%x", 1, 2, "a", 255.0)`, "1|2.0|a|%|ff\n", "")
	// float nao inteiro com %d continua do jeito do Go
	esperaNosDois(t, `mostra formata("%d", 2.5)`, "%!d(float64=2.5)\n", "")
}

// casos que estressam a pilha da VM: muitos args, closure, recursao.
func TestEspalhaPilha(t *testing.T) {
	casos := []struct{ src, saida string }{
		// mais de 255 args (o OpCall guarda argc num byte; o spread nao)
		{`mostra max(...1..1000)`, "1000\n"},
		{`gambiarra conta(...r) funciona tamanho(r) acabou_finalmente
mostra conta(...1..5000)`, "5000\n"},
		// closure com freevar
		{`gambiarra somador(n)
    funciona gambiarra(a, b) funciona a + b + n acabou_finalmente
acabou_finalmente
bota s = somador(100)
mostra s(...[1, 2])`, "103\n"},
		// recursao sem cauda espalhando a cada nivel
		{`gambiarra soma(xs)
    se_colar tamanho(xs) == 0
        funciona 0
    acabou_finalmente
    funciona xs[0] + soma(...[xs[1:]])
acabou_finalmente
mostra soma(1..300)`, "45150\n"},
		// locals da funcao chamada nao pisam nos args espalhados
		{`gambiarra f(a, b)
    bota x = a * 10
    bota y = b * 10
    funciona [x, y, a, b]
acabou_finalmente
mostra f(...[1, 2])
mostra f(1, ...[2])`, "[10, 20, 1, 2]\n[10, 20, 1, 2]\n"},
		// erro de spread dentro de funcao, pego fora
		{`gambiarra f(v) funciona max(...v) acabou_finalmente
arruma
    f(3)
quebrou err
    mostra erro_msg(err)
acabou_finalmente`, "deu ruim na linha 1: so da pra espalhar lista, veio numero\n"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, "")
	}
}

// bora com aridade errada: o erro vem no espera, igual nos 2 engines.
func TestBoraAridade(t *testing.T) {
	esperaNosDois(t, `gambiarra f(a) funciona a acabou_finalmente
bota fu = bora f(1, 2)
mostra "seguiu"
espera(fu)`, "seguiu\n", "essa gambiarra quer 1 parametro(s), voce mandou 2")
	esperaNosDois(t, `gambiarra f(a) funciona a acabou_finalmente
espera(bora f(...[1, 2]))`, "", "essa gambiarra quer 1 parametro(s), voce mandou 2")
}
