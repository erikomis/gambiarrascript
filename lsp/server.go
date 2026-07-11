package lsp

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/parser"
	"gambiarrascript/token"
)

// ---- tipos do protocolo (subconjunto) ----

type Posicao struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Faixa struct {
	Start Posicao `json:"start"`
	End   Posicao `json:"end"`
}

type Diagnostico struct {
	Range    Faixa  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

type ItemCompletion struct {
	Label string `json:"label"`
	Kind  int    `json:"kind"`
}

// keywords da GambiarraScript, sem acento, para o autocomplete.
var keywords = []string{
	"bota", "mostra", "se_colar", "se_nao_colar", "enquanto", "pra_cada",
	"de", "ate", "em", "gambiarra", "funciona", "arruma", "quebrou",
	"vaza", "continua", "deu_bom", "deu_ruim", "nada", "acabou_finalmente",
	"finalmente", "escolhe", "caso",
	"e", "ou", "nao", "importa",
	"bora", // bora fn(args) -> Futuro (concorrencia)
}

// builtinsCompletion são as funções nativas da linguagem expostas no autocomplete.
var builtinsCompletion = []string{
	"tamanho", "chaves", "tem", "texto", "numero", "busca", "rota", "escuta",
	"de_json", "pra_json",
	"formata",
	"separa", "junta", "maiusculo", "minusculo", "substitui", "fatia",
	"contem", "comeca_com", "termina_com", "tira_espaco",
	"adiciona", "remove", "ordena", "inverte", "mapeia", "filtra",
	"reduz", "acha", "acha_indice", "unicos", "achatada", "ordena_com",
	// estatistica / munging
	"soma", "media", "zip", "enumera", "ordena_por", "agrupa_por",
	"raiz", "aleatorio", "arredonda", "teto", "chao", "abs", "min", "max",
	// aleatoriedade
	"semente", "embaralha", "escolhe_um", "uuid",
	"le_arquivo", "escreve_arquivo", "anexa_arquivo",
	"existe", "eh_dir", "deleta", "cria_dir", "le_dir",
	"copia", "move", "tamanho_arquivo", "modificado_em", "glob",
	"caminho_junta", "caminho_base", "caminho_dir", "caminho_ext", "caminho_abs",
	"pergunta", "argumentos", "le_tudo", "le_linhas",
	"escreve", "escreve_erro", "env",
	// concorrencia
	"cano", "envia", "recebe", "fecha", "espera", "afirma", "paralelo",
	// banco
	"conecta", "consulta", "executa",
	// regex
	"busca_regex", "acha_regex", "combina_regex", "substitui_regex", "separa_regex",
	// tempo
	"agora", "agora_num", "agora_ns", "formata_tempo", "parse_tempo",
	"duracao", "espera_ms",
	// crypto / codificacao
	"md5", "sha1", "sha256", "sha512", "hmac_sha256",
	"base64_codifica", "base64_decodifica",
	"base32_codifica", "base32_decodifica",
	"hex_codifica", "hex_decodifica",
	// set
	"conjunto", "contem_conjunto", "adiciona_conjunto", "remove_conjunto",
	"uniao", "intersecao", "diferenca",
	// erros
	"quebra", "erro_msg", "erro_linha", "erro_tipo", "erro_pilha",
	"erro_causa", "envolve_erro",
}

// builtinsSet espelha builtinsCompletion num map pra lookup rapido.
var builtinsSet = func() map[string]bool {
	m := make(map[string]bool, len(builtinsCompletion))
	for _, b := range builtinsCompletion {
		m[b] = true
	}
	return m
}()

// docsBuiltin descreve cada builtin pro hover do LSP.
var docsBuiltin = map[string]string{
	"tamanho":         "tamanho(x) -> numero: devolve o tamanho de lista, dicionario ou texto.",
	"chaves":          "chaves(dicionario) -> lista: devolve as chaves do dicionario.",
	"tem":             "tem(dicionario, chave) -> booleano: checa se a chave existe.",
	"texto":           "texto(valor) -> texto: converte qualquer valor em texto.",
	"numero":          "numero(texto) -> numero: converte texto em numero.",
	"busca":           "busca(url, [opcoes]) -> dicionario: faz uma requisicao HTTP.",
	"rota":            "rota(metodo, caminho, handler): registra uma rota no servidor HTTP.",
	"escuta":          "escuta(porta): sobe o servidor HTTP e bloqueia.",
	"de_json":         "de_json(texto) -> valor: converte JSON em valor GambiarraScript.",
	"pra_json":        "pra_json(valor) -> texto: serializa um valor pra JSON.",
	"separa":          "separa(texto, separador) -> lista: quebra o texto em partes.",
	"junta":           "junta(lista, separador) -> texto: junta os itens da lista num texto.",
	"maiusculo":       "maiusculo(texto) -> texto: converte pra maiusculas.",
	"minusculo":       "minusculo(texto) -> texto: converte pra minusculas.",
	"substitui":       "substitui(texto, antigo, novo) -> texto: troca todas as ocorrencias.",
	"fatia":           "fatia(texto, inicio, [fim]) -> texto: devolve um pedaco do texto.",
	"contem":          "contem(texto, pedaco) -> booleano: checa se o texto contem o pedaco.",
	"comeca_com":      "comeca_com(texto, prefixo) -> booleano.",
	"termina_com":     "termina_com(texto, sufixo) -> booleano.",
	"tira_espaco":     "tira_espaco(texto) -> texto: remove espacos nas pontas (trim).",
	"adiciona":        "adiciona(lista, item): adiciona item ao final da lista (muda a lista).",
	"remove":          "remove(lista, item): remove a primeira ocorrencia de item.",
	"ordena":          "ordena(lista): ordena a lista in-place (numeros ou textos).",
	"inverte":         "inverte(lista): inverte a lista in-place.",
	"formata":         "formata(modelo, valores...) -> texto: printf com verbos do Go (%v %s %d %f, padding %05d, casas %.2f).",
	"mapeia":          "mapeia(lista, gambiarra) -> lista: aplica a gambiarra em cada item.",
	"filtra":          "filtra(lista, gambiarra) -> lista: keep itens em que a gambiarra devolve verdadeiro.",
	"ordena_com":      "ordena_com(lista, fn): ordena a lista in-place usando fn(a, b) comparator (devolve booleano menor-que OU numero <0/0/>0).",
	"soma":            "soma(lista) -> numero: soma os numeros (inteiro se todos forem; vazia = 0).",
	"media":           "media(lista) -> numero: media aritmetica (float). Lista vazia da erro.",
	"zip":             "zip(a, b) -> lista: pares [a[i], b[i]], parando na lista menor.",
	"enumera":         "enumera(lista) -> lista: pares [indice, valor] pra iterar com o indice.",
	"ordena_por":      "ordena_por(lista, campo) -> lista: ordena dicts por campo (crescente); lista nova, nao muda a original.",
	"agrupa_por":      "agrupa_por(lista, fn) -> dict: agrupa {chave: [itens]}, chave = fn(item).",
	"raiz":            "raiz(numero) -> numero: raiz quadrada.",
	"aleatorio":       "aleatorio([max]) -> numero: numero aleatorio em [0, max).",
	"semente":         "semente(numero): fixa a semente do aleatorio/embaralha/escolhe_um/uuid (reprodutivel).",
	"embaralha":       "embaralha(lista) -> lista: nova lista embaralhada (Fisher-Yates); nao muda a original.",
	"escolhe_um":      "escolhe_um(lista) -> item: um elemento aleatorio. Lista vazia da erro.",
	"uuid":            "uuid() -> texto: UUID v4 aleatorio (nao use pra seguranca).",
	"arredonda":       "arredonda(numero) -> numero: arredonda pro inteiro mais proximo.",
	"teto":            "teto(numero) -> numero: arredonda pra cima.",
	"chao":            "chao(numero) -> numero: arredonda pra baixo.",
	"abs":             "abs(numero) -> numero: valor absoluto.",
	"min":             "min(n1, n2, ...) -> numero: o menor dos numeros.",
	"max":             "max(n1, n2, ...) -> numero: o maior dos numeros.",
	"le_arquivo":      "le_arquivo(caminho) -> texto: le todo o conteudo de um arquivo.",
	"escreve_arquivo": "escreve_arquivo(caminho, texto): escreve texto num arquivo.",
	// fs
	"existe":        "existe(caminho) -> booleano: devuelve deu_bom se o caminho existe (stat ok).",
	"eh_dir":        "eh_dir(caminho) -> booleano: devolve deu_bom se o caminho e um diretorio.",
	"deleta":        "deleta(caminho): apaga arquivo ou diretorio recursivo (idempotente).",
	"cria_dir":      "cria_dir(caminho): mkdir -p (cria todos os pais).",
	"le_dir":        "le_dir(dir) -> lista: lista os nomes do diretorio (1 nivel, ordem alfabetica).",
	"copia":           "copia(de, pra): copia o arquivo de -> pra (preserva o modo).",
	"move":            "move(de, pra): renomeia/move o arquivo de -> pra.",
	"tamanho_arquivo": "tamanho_arquivo(caminho) -> numero: tamanho do arquivo em bytes.",
	"modificado_em":   "modificado_em(caminho) -> numero: ultima modificacao em unix-segundos (usavel no formata_tempo).",
	"glob":            "glob(padrao) -> lista: caminhos que casam com o padrao (*, ?, [...]). Sem match = lista vazia.",
	"caminho_junta": "caminho_junta(p1, p2, ...) -> texto: filepath.Join (caminho valido pro SO).",
	"caminho_base":  "caminho_base(caminho) -> texto: ultimo componente do caminho.",
	"caminho_dir":   "caminho_dir(caminho) -> texto: diretorio do caminho (sem o nome final).",
	"caminho_ext":   "caminho_ext(caminho) -> texto: extensao com ponto (ex: .gs) ou \"\".",
	"caminho_abs":   "caminho_abs(caminho) -> texto: caminho absoluto (limpa . e .. e prefixa o cwd).",
	"pergunta":      "pergunta([prompt]) -> texto: le uma linha do stdin.",
	"argumentos":    "argumentos() -> lista: argumentos de linha de comando passados ao script.",
	// concorrencia
	"cano":   "cano([capacidade]) -> cano: cria um canal (channel). Sem args = sincrono.",
	"envia":  "envia(cano, valor): manda um valor pro cano. Bloqueia se o cano estiver cheio ou se nao houver receptor.",
	"recebe": "recebe(cano) -> valor: pega o proximo valor do cano. Bloqueia ate ter algo (ou cano ser fechado -> nada).",
	"fecha":  "fecha(cano_ou_conexao): fecha um cano (channel) ou uma conexao de banco. Idempotente.",
	"espera": "espera(futuro|lista_de_futuros) -> valor|lista: aguarda o(s) futuro(s) e devolve o(s) valor(es). Tambem: espera(a, b) = assert de teste.",
	"afirma": "afirma(cond, [msg]): assert de teste pra gs testa.",
}

// docsKeyword descreve cada keyword pro hover do LSP.
var docsKeyword = map[string]string{
	"bota":              "bota nome = valor: declara (ou reatribui) uma variavel. Tambem desestrutura: `bota [a, b] = lista` (posicao) e `bota {x, y} = dict` (chave). Atribuicao composta dispensa o bota: `x += 1`, `n <<= 2`.",
	"mostra":            "mostra valor: imprime no stdout.",
	"se_colar":          "se_colar condicao ... se_nao_colar ... acabou_finalmente: condicional.",
	"se_nao_colar":      "se_nao_colar: ramo alternativo (else / else-if).",
	"enquanto":          "enquanto condicao ... acabou_finalmente: laco while.",
	"pra_cada":          "pra_cada var de A ate B / pra_cada var em lista ... acabou_finalmente: laco for.",
	"gambiarra":         "gambiarra nome(params) ... acabou_finalmente: declara uma funcao. Sem nome (`gambiarra(x) ... acabou_finalmente`) e uma lambda anonima usavel como expressao.",
	"escolhe":           "escolhe x / caso v1, v2 <bloco> / se_nao_colar <bloco> / acabou_finalmente: switch — casa o primeiro caso igual (==) e sai, sem fallthrough.",
	"caso":              "caso v1[, v2...]: um braco do escolhe. Aceita varios valores separados por virgula.",
	"funciona":          "funciona valor: return de uma gambiarra.",
	"arruma":            "arruma ... quebrou erro ... acabou_finalmente: try/catch.",
	"quebrou":           "quebrou nome: captura o erro do arruma.",
	"vaza":              "vaza: break de um loop.",
	"continua":          "continua: continue de um loop.",
	"deu_bom":           "deu_bom: booleano verdadeiro.",
	"deu_ruim":          "deu_ruim: booleano falso.",
	"nada":              "nada: valor nulo.",
	"acabou_finalmente": "acabou_finalmente: fecha um bloco.",
	"finalmente":        "finalmente <bloco> acabou_finalmente: bloco opcional do arruma, sempre roda (com ou sem erro). Combinacao: `arruma <try> quebrou err <catch> finalmente <finally> acabou_finalmente` ou `arruma <try> finalmente <finally> acabou_finalmente` (catch opcional).",
	"e":                 "e: operador logico AND (curto-circuito).",
	"ou":                "ou: operador logico OR (curto-circuito).",
	"nao":               "nao: operador logico NOT.",
	"importa":           "importa \"caminho.gs\": carrega outro script e traz suas definicoes.",
	"bora":              "bora fn(args) -> Futuro: dispara a gambiarra numa goroutine e devolve um Futuro imediatamente. Use espera(futuro) pra aguardar o valor.",
}

// ---- servidor ----

type Servidor struct {
	docs map[string]string
	out  io.Writer
}

func NovoServidor(out io.Writer) *Servidor {
	return &Servidor{docs: map[string]string{}, out: out}
}

// Rodar processa mensagens do stdin ate o EOF ou um 'exit'.
func (s *Servidor) Rodar(in io.Reader) error {
	r := bufio.NewReader(in)
	for {
		msg, err := LerMensagem(r)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if s.tratar(msg) {
			return nil // exit
		}
	}
}

// tratar despacha uma mensagem; devolve true se for 'exit'.
func (s *Servidor) tratar(msg *Mensagem) bool {
	switch msg.Method {
	case "initialize":
		s.responder(msg.ID, map[string]interface{}{
			"capabilities": map[string]interface{}{
				"textDocumentSync":   1, // Full
				"completionProvider": map[string]interface{}{},
				"hoverProvider":      true,
			},
			"serverInfo": map[string]interface{}{"name": "gambiarrascript-lsp"},
		})
	case "shutdown":
		s.responder(msg.ID, nil)
	case "exit":
		return true
	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"textDocument"`
		}
		json.Unmarshal(msg.Params, &p)
		s.docs[p.TextDocument.URI] = p.TextDocument.Text
		s.PublicarDiagnosticos(p.TextDocument.URI)
	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		json.Unmarshal(msg.Params, &p)
		if len(p.ContentChanges) > 0 {
			s.docs[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
			s.PublicarDiagnosticos(p.TextDocument.URI)
		}
	case "textDocument/didClose":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		json.Unmarshal(msg.Params, &p)
		delete(s.docs, p.TextDocument.URI)
	case "textDocument/completion":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		json.Unmarshal(msg.Params, &p)
		s.responder(msg.ID, map[string]interface{}{
			"isIncomplete": false,
			"items":        s.itensCompletion(s.docs[p.TextDocument.URI]),
		})
	case "textDocument/hover":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		json.Unmarshal(msg.Params, &p)
		doc := s.docs[p.TextDocument.URI]
		conteudo := hoverConteudo(doc, p.Position.Line, p.Position.Character)
		if conteudo == "" {
			s.responder(msg.ID, nil)
		} else {
			s.responder(msg.ID, map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": conteudo,
				},
			})
		}
	}
	// 'initialized' e outras notifications sem id sao ignoradas.
	return false
}

// PublicarDiagnosticos reparseia o documento e envia publishDiagnostics.
func (s *Servidor) PublicarDiagnosticos(uri string) {
	texto := s.docs[uri]
	p := parser.New(lexer.New(texto))
	prog := p.ParseProgram()

	diags := []Diagnostico{}
	for _, e := range p.ErrosDetalhados() {
		linha := e.Linha - 1
		if linha < 0 {
			linha = 0
		}
		col := e.Coluna - 1
		if col < 0 {
			col = 0
		}
		diags = append(diags, Diagnostico{
			Range: Faixa{
				Start: Posicao{Line: linha, Character: col},
				End:   Posicao{Line: linha, Character: col + 1},
			},
			Severity: 1, // Error
			Source:   "gambiarrascript",
			Message:  e.Msg,
		})
	}
	// typechecker basico — só prossegue sem erros de parse
	if len(p.Errors()) == 0 && prog != nil {
		diags = append(diags, typecheck(prog)...)
	}
	s.notificar("textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         uri,
		"diagnostics": diags,
	})
}

// itensCompletion devolve keywords + builtins + identificadores vistos no texto.
func (s *Servidor) itensCompletion(texto string) []ItemCompletion {
	vistos := map[string]bool{}
	var itens []ItemCompletion
	for _, kw := range keywords {
		itens = append(itens, ItemCompletion{Label: kw, Kind: 14}) // 14 = Keyword
		vistos[kw] = true
	}
	for _, b := range builtinsCompletion {
		if !vistos[b] {
			itens = append(itens, ItemCompletion{Label: b, Kind: 3}) // 3 = Function
			vistos[b] = true
		}
	}
	l := lexer.New(texto)
	for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
		if tok.Type == token.IDENT && !vistos[tok.Literal] {
			vistos[tok.Literal] = true
			itens = append(itens, ItemCompletion{Label: tok.Literal, Kind: 6}) // 6 = Variable
		}
	}
	return itens
}

// hoverConteudo devolve o texto markdown de hover pro word na posicao dada,
// ou "" se nao houver documentacao. linha e coluna sao 0-indexadas (LSP).
func hoverConteudo(doc string, linha, coluna int) string {
	linhas := strings.Split(doc, "\n")
	if linha < 0 || linha >= len(linhas) {
		return ""
	}
	linhaTexto := linhas[linha]
	if coluna < 0 || coluna > len([]rune(linhaTexto)) {
		return ""
	}
	runes := []rune(linhaTexto)
	inicio := coluna
	for inicio > 0 && isWordChar(runes[inicio-1]) {
		inicio--
	}
	fim := coluna
	for fim < len(runes) && isWordChar(runes[fim]) {
		fim++
	}
	if inicio >= fim {
		return ""
	}
	palavra := string(runes[inicio:fim])
	if d, ok := docsBuiltin[palavra]; ok {
		return "`" + palavra + "`\n\n" + d
	}
	if d, ok := docsKeyword[palavra]; ok {
		return "`" + palavra + "` — keyword\n\n" + d
	}
	return ""
}

func isWordChar(r rune) bool {
	return r == '_' ||
		('a' <= r && r <= 'z') ||
		('A' <= r && r <= 'Z') ||
		('0' <= r && r <= '9')
}

// ---- typechecker basico (warnings) ----
//
// Faz um passe leve no AST rastreando nomes definidos (bota, gambiarra,
// params, quebrou) e emite diagnosticos severidade=2 (Warning) quando um
// Identifier em posicao de expressao nao parece resolvivel. Nao e um
// typechecker real: so um "linter" de uso de variavel/funcao.

func typecheck(prog *ast.Program) []Diagnostico {
	tc := &typechecker{
		scopes: []map[string]bool{{}},
	}
	tc.walkProgram(prog)
	return tc.diags
}

// Typecheck expoe o linter de identificadores pra fora do LSP (usado pelo
// `gs check`). Devolve os mesmos diagnosticos que a extensao mostra.
func Typecheck(prog *ast.Program) []Diagnostico {
	return typecheck(prog)
}

type typechecker struct {
	scopes []map[string]bool
	diags  []Diagnostico
}

func (tc *typechecker) pushScope() {
	tc.scopes = append(tc.scopes, map[string]bool{})
}
func (tc *typechecker) popScope() {
	if len(tc.scopes) > 1 {
		tc.scopes = tc.scopes[:len(tc.scopes)-1]
	}
}
func (tc *typechecker) define(nome string) {
	if len(tc.scopes) == 0 {
		tc.scopes = []map[string]bool{{nome: true}}
		return
	}
	tc.scopes[len(tc.scopes)-1][nome] = true
}
func (tc *typechecker) resolvivel(nome string) bool {
	for i := len(tc.scopes) - 1; i >= 0; i-- {
		if tc.scopes[i][nome] {
			return true
		}
	}
	return false
}

func (tc *typechecker) warn(linha int, coluna int, msg string) {
	if linha < 1 {
		linha = 1
	}
	if coluna < 1 {
		coluna = 1
	}
	tc.diags = append(tc.diags, Diagnostico{
		Range: Faixa{
			Start: Posicao{Line: linha - 1, Character: coluna - 1},
			End:   Posicao{Line: linha - 1, Character: coluna},
		},
		Severity: 2, // Warning
		Source:   "gambiarrascript-tc",
		Message:  msg,
	})
}

func (tc *typechecker) walkProgram(prog *ast.Program) {
	for _, s := range prog.Statements {
		tc.walkStmt(s)
	}
}

func (tc *typechecker) walkStmt(s ast.Statement) {
	switch n := s.(type) {
	case *ast.BotaStatement:
		tc.walkExpr(n.Value)
		if n.Name != nil {
			tc.define(n.Name.Value)
		}
	case *ast.MostraStatement:
		tc.walkExpr(n.Value)
	case *ast.GambiarraStatement:
		tc.define(n.Name.Value)
		tc.pushScope()
		for _, p := range n.Parameters {
			tc.define(p.Value)
		}
		tc.walkBlock(n.Body)
		tc.popScope()
	case *ast.SeColarStatement:
		for i, c := range n.Conditions {
			tc.walkExpr(c)
			tc.pushScope()
			tc.walkBlock(n.Consequences[i])
			tc.popScope()
		}
		if n.Alternative != nil {
			tc.pushScope()
			tc.walkBlock(n.Alternative)
			tc.popScope()
		}
	case *ast.EnquantoStatement:
		tc.walkExpr(n.Condition)
		tc.pushScope()
		tc.walkBlock(n.Body)
		tc.popScope()
	case *ast.PraCadaNumStatement:
		tc.walkExpr(n.Start)
		tc.walkExpr(n.End)
		tc.pushScope()
		tc.define(n.Var.Value)
		tc.walkBlock(n.Body)
		tc.popScope()
	case *ast.PraCadaListStatement:
		tc.walkExpr(n.Iterable)
		tc.pushScope()
		tc.define(n.Var.Value)
		tc.walkBlock(n.Body)
		tc.popScope()
	case *ast.ArrumaStatement:
		tc.pushScope()
		tc.walkBlock(n.Try)
		tc.popScope()
		if n.Catch != nil {
			tc.pushScope()
			if n.ErrName != nil {
				tc.define(n.ErrName.Value)
			}
			tc.walkBlock(n.Catch)
			tc.popScope()
		}
		if n.Finally != nil {
			tc.pushScope()
			tc.walkBlock(n.Finally)
			tc.popScope()
		}
	case *ast.DesestruturaStatement:
		tc.walkExpr(n.Value)
		for _, nome := range n.Names {
			tc.define(nome.Value)
		}
	case *ast.EscolheStatement:
		tc.walkExpr(n.Subject)
		for _, braco := range n.Casos {
			for _, v := range braco.Values {
				tc.walkExpr(v)
			}
			tc.pushScope()
			tc.walkBlock(braco.Body)
			tc.popScope()
		}
		if n.Default != nil {
			tc.pushScope()
			tc.walkBlock(n.Default)
			tc.popScope()
		}
	case *ast.FuncionaStatement:
		if n.Value != nil {
			tc.walkExpr(n.Value)
		}
	case *ast.VazaStatement, *ast.ContinuaStatement:
		// nada a checar
	case *ast.ExpressionStatement:
		tc.walkExpr(n.Expression)
	case *ast.ImportaStatement:
		// sem checagem — caminho dinâmico
	}
}

func (tc *typechecker) walkBlock(b *ast.BlockStatement) {
	if b == nil {
		return
	}
	for _, s := range b.Statements {
		tc.walkStmt(s)
	}
}

func (tc *typechecker) walkExpr(e ast.Expression) {
	switch n := e.(type) {
	case *ast.Identifier:
		nome := n.Value
		if !tc.resolvivel(nome) && !builtinsSet[nome] && !ehKeyword(nome) {
			tc.warn(n.Token.Line, n.Token.Coluna, "`"+nome+"` pode estar indefinido (nao e builtin nem keyword)")
		}
	case *ast.PrefixExpression:
		tc.walkExpr(n.Right)
	case *ast.InfixExpression:
		tc.walkExpr(n.Left)
		tc.walkExpr(n.Right)
	case *ast.CallExpression:
		tc.walkExpr(n.Function)
		for _, a := range n.Arguments {
			tc.walkExpr(a)
		}
	case *ast.IndexExpression:
		tc.walkExpr(n.Left)
		tc.walkExpr(n.Index)
	case *ast.ListaLiteral:
		for _, el := range n.Elements {
			tc.walkExpr(el)
		}
	case *ast.DicionarioLiteral:
		for _, p := range n.Pares {
			tc.walkExpr(p.Chave)
			tc.walkExpr(p.Valor)
		}
	case *ast.BoraExpression:
		if n.Call != nil {
			tc.walkExpr(n.Call.Function)
			for _, a := range n.Call.Arguments {
				tc.walkExpr(a)
			}
		}
	case *ast.FuncaoLiteral:
		tc.pushScope()
		for _, p := range n.Parameters {
			tc.define(p.Value)
		}
		tc.walkBlock(n.Body)
		tc.popScope()
	case *ast.RangeExpression:
		tc.walkExpr(n.Start)
		tc.walkExpr(n.End)
	case *ast.TextoInterpolado:
		for _, parte := range n.Parts {
			tc.walkExpr(parte)
		}
	case *ast.NumeroLiteral, *ast.TextoLiteral, *ast.BooleanoLiteral, *ast.NadaLiteral:
		// literais: nada
	}
}

func ehKeyword(nome string) bool {
	for _, k := range keywords {
		if k == nome {
			return true
		}
	}
	return false
}

func (s *Servidor) notificar(metodo string, params interface{}) {
	raw, _ := json.Marshal(params)
	EscreverMensagem(s.out, &Mensagem{Method: metodo, Params: raw})
}

func (s *Servidor) responder(id *json.RawMessage, result interface{}) {
	if result == nil {
		result = json.RawMessage("null")
	}
	EscreverMensagem(s.out, &Mensagem{ID: id, Result: result})
}
