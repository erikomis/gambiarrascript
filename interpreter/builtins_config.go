package interpreter

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gambiarrascript/object"
)

// Configuracao de app de verdade: flags de linha de comando e arquivo .env.
//
//	opcoes(padroes, [ajudas]) → dicionario com os padroes sobrescritos pelos
//	                            --flags do argumentos(); posicionais em "_"
//	carrega_env([caminho=".env"], [{"sobrescreve": deu_bom}]) → dicionario
//
// Regras do opcoes:
//   - o TIPO de cada flag vem do padrao: numero, texto, booleano ou lista
//     (lista = flag repetivel, `--tag a --tag b`); nada = texto opcional
//   - `--porta 9090` e `--porta=9090`; `-` e `_` no nome valem igual
//     (`--log-nivel` preenche "log_nivel")
//   - booleano: `--verboso` liga, `--no-verboso`/`--sem-verboso` desliga,
//     `--verboso=deu_ruim` (ou true/false/sim/nao/1/0) tambem rola
//   - `--` encerra as flags: tudo depois vai pro "_" como esta
//   - flag desconhecida, numero zoado ou valor faltando → erro (o script
//     morre com a mensagem, a nao ser que tenha arruma em volta)
//   - `--ajuda`/`--help`/`-h` imprimem a ajuda gerada e saem com codigo 0
//     (a nao ser que "ajuda" seja uma das tuas opcoes)

func (i *Interpreter) builtinOpcoes(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("opcoes() quer (padroes, [ajudas]), veio %d", len(args))
	}
	padroes, ok := args[0].(*object.Dicionario)
	if !ok {
		return erroBuiltin("opcoes: padroes tem que ser dicionario, veio %s", args[0].Type())
	}
	var ajudas *object.Dicionario
	if len(args) == 2 {
		if ajudas, ok = args[1].(*object.Dicionario); !ok {
			return erroBuiltin("opcoes: ajudas tem que ser dicionario {flag: descricao}, veio %s", args[1].Type())
		}
	}

	// valida os padroes e copia pro resultado (na mesma ordem)
	res := object.NovoDicionario()
	var nomes []string
	tipos := map[string]object.Object{}
	var erro *object.Erro
	padroes.Itera(func(par object.ParDic) {
		if erro != nil {
			return
		}
		k, ok := par.Chave.(*object.Texto)
		if !ok || k.Value == "" || k.Value == "_" || strings.HasPrefix(k.Value, "-") {
			erro = erroBuiltin("opcoes: nome de flag invalido %s (usa texto tipo \"porta\")", par.Chave.Inspect())
			return
		}
		switch v := par.Valor.(type) {
		case *object.Numero, *object.Texto, *object.Booleano, *object.Nada:
		case *object.Lista:
			// copia: o resultado nao pode ser o mesmo objeto do padrao
			par.Valor = object.NovaLista(v.Copia())
		default:
			erro = erroBuiltin("opcoes: padrao de --%s tem que ser numero, texto, booleano, lista ou nada, veio %s", k.Value, par.Valor.Type())
			return
		}
		nomes = append(nomes, k.Value)
		tipos[k.Value] = par.Valor
		res.Bota(k.ChaveHash(), par)
	})
	if erro != nil {
		return erro
	}

	_, ajudaEhOpcao := tipos["ajuda"]
	acha := func(nome string) (string, bool) {
		if _, ok := tipos[nome]; ok {
			return nome, true
		}
		alt := strings.ReplaceAll(nome, "-", "_")
		if _, ok := tipos[alt]; ok {
			return alt, true
		}
		return "", false
	}

	var posicionais []object.Object
	listaTocada := map[string]bool{} // 1a vez que a flag de lista aparece, troca o padrao
	argv := i.argumentos
	for idx := 0; idx < len(argv); idx++ {
		a := argv[idx]
		if a == "--" {
			for _, resto := range argv[idx+1:] {
				posicionais = append(posicionais, &object.Texto{Value: resto})
			}
			break
		}
		if !ajudaEhOpcao && (a == "--ajuda" || a == "--help" || a == "-h") {
			i.muOut.Lock()
			io.WriteString(i.out, ajudaOpcoes(nomes, tipos, ajudas))
			i.muOut.Unlock()
			return &object.Sair{Codigo: 0}
		}
		if !strings.HasPrefix(a, "-") || a == "-" || ehNumeroTexto(a) {
			posicionais = append(posicionais, &object.Texto{Value: a})
			continue
		}
		if !strings.HasPrefix(a, "--") {
			return erroBuiltin("flag curta %s nao rola, usa o nome inteiro (%s)", a, listaFlags(nomes, ajudaEhOpcao))
		}

		nome, valor, temValor := strings.Cut(a[2:], "=")
		chave, achou := acha(nome)
		negada := false
		if !achou {
			// --no-x / --sem-x desligam um booleano
			for _, pre := range []string{"no-", "sem-"} {
				if base, ok := strings.CutPrefix(nome, pre); ok {
					if c, ok := acha(base); ok {
						if _, ehBool := tipos[c].(*object.Booleano); ehBool && !temValor {
							chave, achou, negada = c, true, true
						}
					}
				}
			}
		}
		if !achou {
			return erroBuiltin("flag desconhecida --%s (as que rolam: %s)", nome, listaFlags(nomes, ajudaEhOpcao))
		}

		k := &object.Texto{Value: chave}
		padrao := tipos[chave]
		if _, ehBool := padrao.(*object.Booleano); ehBool {
			v := !negada
			if temValor {
				b, ok := boolDeTexto(valor)
				if !ok {
					return erroBuiltin("--%s espera booleano (deu_bom/deu_ruim, true/false, sim/nao), veio %s", chave, valor)
				}
				v = b
			}
			res.Bota(k.ChaveHash(), object.ParDic{Chave: k, Valor: boolDoNativo(v)})
			continue
		}

		if !temValor {
			if idx+1 >= len(argv) || (strings.HasPrefix(argv[idx+1], "--") && len(argv[idx+1]) > 2) {
				return erroBuiltin("--%s precisa de um valor (%s)", chave, nomeTipoFlag(padrao))
			}
			idx++
			valor = argv[idx]
		}

		var v object.Object
		switch p := padrao.(type) {
		case *object.Numero:
			n, ok := numeroDeTexto(valor)
			if !ok {
				return erroBuiltin("--%s espera numero, veio %s", chave, valor)
			}
			v = n
		case *object.Lista:
			if !listaTocada[chave] {
				listaTocada[chave] = true
				p = object.NovaLista(nil)
				res.Bota(k.ChaveHash(), object.ParDic{Chave: k, Valor: p})
			} else {
				par, _ := res.Pega(k.ChaveHash())
				p = par.Valor.(*object.Lista)
			}
			p.Adiciona(&object.Texto{Value: valor})
			continue
		default: // texto ou nada
			v = &object.Texto{Value: valor}
		}
		res.Bota(k.ChaveHash(), object.ParDic{Chave: k, Valor: v})
	}

	if posicionais == nil {
		posicionais = []object.Object{}
	}
	botaTexto(res, "_", object.NovaLista(posicionais))
	return res
}

func ehNumeroTexto(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// numeroDeTexto: inteiro vira inteiro exato (igual literal), resto float.
func numeroDeTexto(s string) (*object.Numero, bool) {
	s = strings.TrimSpace(s)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return object.NumInt(n), true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, false
	}
	return object.NumFloat(f), true
}

func boolDeTexto(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "deu_bom", "true", "sim", "s", "1", "yes", "y", "on":
		return true, true
	case "deu_ruim", "false", "nao", "n", "0", "no", "off":
		return false, true
	}
	return false, false
}

func nomeTipoFlag(o object.Object) string {
	switch o.(type) {
	case *object.Numero:
		return "numero"
	case *object.Booleano:
		return "booleano"
	case *object.Lista:
		return "texto, repetivel"
	default:
		return "texto"
	}
}

func listaFlags(nomes []string, ajudaEhOpcao bool) string {
	fs := make([]string, 0, len(nomes)+1)
	for _, n := range nomes {
		fs = append(fs, "--"+n)
	}
	if !ajudaEhOpcao {
		fs = append(fs, "--ajuda")
	}
	if len(fs) == 0 {
		return "nenhuma"
	}
	return strings.Join(fs, ", ")
}

// ajudaOpcoes gera o texto do --ajuda a partir dos padroes (e das descricoes
// opcionais), alinhado em colunas.
func ajudaOpcoes(nomes []string, tipos map[string]object.Object, ajudas *object.Dicionario) string {
	type linha struct{ flag, desc string }
	var linhas []linha
	for _, n := range nomes {
		p := tipos[n]
		flag := "--" + n
		padrao := ""
		switch v := p.(type) {
		case *object.Booleano:
			flag += ", --no-" + n
			padrao = v.Inspect()
		case *object.Numero:
			flag += " <numero>"
			padrao = v.Inspect()
		case *object.Lista:
			flag += " <texto>..."
			if v.Tamanho() > 0 {
				padrao = v.Inspect()
			}
		case *object.Texto:
			flag += " <texto>"
			padrao = strconv.Quote(v.Value)
		default:
			flag += " <texto>"
		}
		desc := ""
		if ajudas != nil {
			if d, ok := pegaTexto(ajudas, n); ok {
				desc = textoDe(d)
			}
		}
		if padrao != "" {
			if desc != "" {
				desc += " "
			}
			desc += "(padrao: " + padrao + ")"
		}
		linhas = append(linhas, linha{flag, desc})
	}
	linhas = append(linhas, linha{"--ajuda, -h", "mostra essa ajuda e sai"})

	largura := 0
	for _, l := range linhas {
		if len(l.flag) > largura {
			largura = len(l.flag)
		}
	}
	var b strings.Builder
	b.WriteString("uso: [opcoes] [--] [argumentos...]\n\nopcoes:\n")
	for _, l := range linhas {
		b.WriteString(strings.TrimRight(fmt.Sprintf("  %-*s  %s", largura, l.flag, l.desc), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// ---- .env ----

func builtinCarregaEnv(args []object.Object) object.Object {
	if len(args) > 2 {
		return erroBuiltin("carrega_env() quer ([caminho], [opcoes]), veio %d", len(args))
	}
	caminho := ".env"
	var op *object.Dicionario
	for idx, a := range args {
		switch v := a.(type) {
		case *object.Texto:
			if idx != 0 {
				return erroBuiltin("carrega_env: caminho vem primeiro, opcoes depois")
			}
			caminho = v.Value
		case *object.Dicionario:
			op = v
		default:
			return erroBuiltin("carrega_env: arg %d tem que ser texto (caminho) ou dicionario (opcoes), veio %s", idx+1, a.Type())
		}
	}
	sobrescreve := false
	if op != nil {
		var erro *object.Erro
		op.Itera(func(par object.ParDic) {
			if erro != nil {
				return
			}
			if textoDe(par.Chave) != "sobrescreve" {
				erro = erroBuiltin("carrega_env: opcao desconhecida %s (a que rola: sobrescreve)", par.Chave.Inspect())
				return
			}
			b, ok := par.Valor.(*object.Booleano)
			if !ok {
				erro = erroBuiltin("carrega_env: sobrescreve quer booleano, veio %s", par.Valor.Inspect())
				return
			}
			sobrescreve = b.Value
		})
		if erro != nil {
			return erro
		}
	}

	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		return erroBuiltinKind(KindIO, "carrega_env: nao consegui ler %q: %v", caminho, err)
	}
	pares, err := parseDotenv(string(conteudo))
	if err != nil {
		return erroBuiltin("carrega_env %q: %v", caminho, err)
	}

	// so mexe no ambiente depois de parsear o arquivo inteiro: arquivo com
	// erro na linha 30 nao deixa as 29 primeiras aplicadas pela metade.
	res := object.NovoDicionario()
	for _, p := range pares {
		if _, existe := os.LookupEnv(p.chave); !existe || sobrescreve {
			if err := os.Setenv(p.chave, p.valor); err != nil {
				return erroBuiltin("carrega_env: nao consegui setar %s: %v", p.chave, err)
			}
		}
		botaTexto(res, p.chave, &object.Texto{Value: os.Getenv(p.chave)})
	}
	return res
}

type parEnv struct{ chave, valor string }

// parseDotenv entende o dialeto comum de .env:
//
//	# comentario
//	CHAVE=valor            (espacos nas pontas somem; ` #...` e comentario)
//	export CHAVE=valor
//	CHAVE="com \"aspas\", \n e ${nada de expansao}"  (pode quebrar linha)
//	CHAVE='literal, sem escape'                       (pode quebrar linha)
//
// Nao expande ${VAR} de proposito: o valor e o que ta escrito.
func parseDotenv(s string) ([]parEnv, error) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimPrefix(s, "\xef\xbb\xbf") // BOM do Bloco de Notas
	var pares []parEnv
	pos, linha := 0, 1
	for pos < len(s) {
		fim := strings.IndexByte(s[pos:], '\n')
		if fim < 0 {
			fim = len(s)
		} else {
			fim += pos
		}
		inicioLinha := pos
		bruta := s[pos:fim]
		linhaAtual := linha
		pos = fim + 1
		linha++

		l := strings.TrimSpace(bruta)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if r, ok := strings.CutPrefix(l, "export"); ok && r != "" && (r[0] == ' ' || r[0] == '\t') {
			l = strings.TrimSpace(r)
		}
		chave, resto, ok := strings.Cut(l, "=")
		if !ok {
			return nil, fmt.Errorf("linha %d sem '=': %q", linhaAtual, l)
		}
		chave = strings.TrimSpace(chave)
		if !chaveEnvValida(chave) {
			return nil, fmt.Errorf("linha %d: nome de variavel invalido %q", linhaAtual, chave)
		}
		resto = strings.TrimLeft(resto, " \t")

		var valor string
		if resto != "" && (resto[0] == '"' || resto[0] == '\'') {
			aspa := resto[0]
			// o valor entre aspas pode continuar nas linhas seguintes:
			// junta o resto do arquivo e procura a aspa que fecha.
			// (chave e "export" nao tem aspas: a 1a aspa da linha e a que abre)
			inicio := inicioLinha + strings.IndexByte(bruta, aspa) + 1
			v, depois, consumidas, err := leAspas(s[inicio:], aspa)
			if err != nil {
				return nil, fmt.Errorf("linha %d: %v", linhaAtual, err)
			}
			valor = v
			// o que sobra na linha da aspa final tem que ser vazio ou comentario
			cauda := depois
			if nl := strings.IndexByte(cauda, '\n'); nl >= 0 {
				cauda = cauda[:nl]
			}
			if c := strings.TrimSpace(cauda); c != "" && !strings.HasPrefix(c, "#") {
				return nil, fmt.Errorf("linha %d: lixo depois das aspas: %q", linhaAtual, c)
			}
			// pula pro fim da linha onde a aspa fechou
			fimAspa := inicio + consumidas
			if nl := strings.IndexByte(s[fimAspa:], '\n'); nl >= 0 {
				pos = fimAspa + nl + 1
			} else {
				pos = len(s)
			}
			linha = linhaAtual + strings.Count(s[inicioLinha:pos], "\n")
		} else {
			valor = resto
			for idx := 0; idx < len(valor); idx++ {
				if valor[idx] == '#' && idx > 0 && (valor[idx-1] == ' ' || valor[idx-1] == '\t') {
					valor = valor[:idx]
					break
				}
			}
			valor = strings.TrimSpace(valor)
		}
		pares = append(pares, parEnv{chave, valor})
	}
	return pares, nil
}

// leAspas le ate a aspa que fecha. Com aspa dupla trata \n \t \r \" \\ (e
// \$); com simples e tudo literal. Devolve (valor, o que sobrou depois da
// aspa, quantos bytes de s foram consumidos incluindo a aspa final).
func leAspas(s string, aspa byte) (string, string, int, error) {
	var b strings.Builder
	for idx := 0; idx < len(s); idx++ {
		c := s[idx]
		if c == aspa {
			return b.String(), s[idx+1:], idx + 1, nil
		}
		if c == '\\' && aspa == '"' && idx+1 < len(s) {
			idx++
			switch s[idx] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"', '\\', '$':
				b.WriteByte(s[idx])
			default:
				b.WriteByte('\\')
				b.WriteByte(s[idx])
			}
			continue
		}
		b.WriteByte(c)
	}
	return "", "", 0, fmt.Errorf("aspa %c abriu e nunca fechou", aspa)
}

func chaveEnvValida(k string) bool {
	if k == "" {
		return false
	}
	for idx, r := range k {
		ok := r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(idx > 0 && ((r >= '0' && r <= '9') || r == '.' || r == '-'))
		if !ok {
			return false
		}
	}
	return true
}
