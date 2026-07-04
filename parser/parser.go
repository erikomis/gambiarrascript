package parser

import (
	"fmt"
	"strconv"
	"strings"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/token"
)

const (
	_ int = iota
	LOWEST
	OU          // ou
	E           // e
	RANGE       // .. (mais frouxo que aritmetica: 0..n-1 => 0..(n-1))
	BOR         // | (bitwise or)
	BXOR        // ^ (bitwise xor)
	BAND        // & (bitwise and)
	EQUALS      // == !=
	LESSGREATER // < > <= >=
	SHIFT       // << >>
	SUM         // + -
	PRODUCT     // * / %
	PREFIX      // -x  nao x  ~x
	CALL        // f(x)
	INDEX       // lista[i]
)

var precedencias = map[token.TokenType]int{
	token.OU:       OU,
	token.E:        E,
	token.BOR:      BOR,
	token.BXOR:     BXOR,
	token.BAND:     BAND,
	token.EQ:       EQUALS,
	token.NEQ:      EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.LTE:      LESSGREATER,
	token.GTE:      LESSGREATER,
	token.LSHIFT:   SHIFT,
	token.RSHIFT:   SHIFT,
	token.RANGE:    RANGE,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.STAR:     PRODUCT,
	token.SLASH:    PRODUCT,
	token.PERCENT:  PRODUCT,
	token.LPAREN:   CALL,
	token.LBRACKET: INDEX,
	token.DOT:      INDEX,
}

// baseDoComposto mapeia o token de atribuicao composta pro operador base.
// `x += 1` desugara pra `bota x = x + 1` (BotaStatement com OpComposto).
var baseDoComposto = map[token.TokenType]string{
	token.PLUSASSIGN:    "+",
	token.MINUSASSIGN:   "-",
	token.STARASSIGN:    "*",
	token.SLASHASSIGN:   "/",
	token.PERCENTASSIGN: "%",
	token.BANDASSIGN:    "&",
	token.BORASSIGN:     "|",
	token.BXORASSIGN:    "^",
	token.LSHIFTASSIGN:  "<<",
	token.RSHIFTASSIGN:  ">>",
}

// ErroParse e um erro de analise com posicao no codigo-fonte.
type ErroParse struct {
	Linha  int
	Coluna int
	Msg    string
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l    *lexer.Lexer
	errs []ErroParse

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, errs: []ErroParse{}}

	p.prefixParseFns = map[token.TokenType]prefixParseFn{}
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.NUMERO, p.parseNumero)
	p.registerPrefix(token.TEXTO, p.parseTexto)
	p.registerPrefix(token.DEU_BOM, p.parseBooleano)
	p.registerPrefix(token.DEU_RUIM, p.parseBooleano)
	p.registerPrefix(token.NADA, p.parseNada)
	p.registerPrefix(token.MINUS, p.parsePrefix)
	p.registerPrefix(token.NAO, p.parsePrefix)
	p.registerPrefix(token.BNOT, p.parsePrefix)
	p.registerPrefix(token.LPAREN, p.parseGrouped)
	p.registerPrefix(token.LBRACKET, p.parseLista)
	p.registerPrefix(token.LBRACE, p.parseDicionario)
	p.registerPrefix(token.BORA, p.parseBora)               // bora fn(args) -> Futuro
	p.registerPrefix(token.GAMBIARRA, p.parseFuncaoLiteral) // lambda anonima

	p.infixParseFns = map[token.TokenType]infixParseFn{}
	for _, tt := range []token.TokenType{
		token.PLUS, token.MINUS, token.STAR, token.SLASH, token.PERCENT,
		token.EQ, token.NEQ, token.LT, token.GT, token.LTE, token.GTE,
		token.E, token.OU,
		token.BAND, token.BOR, token.BXOR, token.LSHIFT, token.RSHIFT,
	} {
		p.registerInfix(tt, p.parseInfix)
	}
	p.registerInfix(token.LPAREN, p.parseCall)
	p.registerInfix(token.LBRACKET, p.parseIndex)
	p.registerInfix(token.RANGE, p.parseRange)
	p.registerInfix(token.DOT, p.parseDot)

	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) registerPrefix(tt token.TokenType, fn prefixParseFn) { p.prefixParseFns[tt] = fn }
func (p *Parser) registerInfix(tt token.TokenType, fn infixParseFn)   { p.infixParseFns[tt] = fn }

func (p *Parser) addErro(linha, coluna int, formato string, args ...interface{}) {
	p.errs = append(p.errs, ErroParse{Linha: linha, Coluna: coluna, Msg: fmt.Sprintf(formato, args...)})
}

// ErrosDetalhados devolve os erros com posicao (linha/coluna), para o LSP.
func (p *Parser) ErrosDetalhados() []ErroParse { return p.errs }

// Errors mantem a compatibilidade com o CLI, formatando "linha N: msg".
func (p *Parser) Errors() []string {
	out := make([]string, len(p.errs))
	for i, e := range p.errs {
		out[i] = fmt.Sprintf("linha %d: %s", e.Linha, e.Msg)
	}
	return out
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t token.TokenType) bool  { return p.curToken.Type == t }
func (p *Parser) peekTokenIs(t token.TokenType) bool { return p.peekToken.Type == t }

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.addErro(p.peekToken.Line, p.peekToken.Coluna,
		"esperava %q aqui, mas veio %q", t, p.peekToken.Literal)
	return false
}

func (p *Parser) peekPrecedence() int {
	if pr, ok := precedencias[p.peekToken.Type]; ok {
		return pr
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if pr, ok := precedencias[p.curToken.Type]; ok {
		return pr
	}
	return LOWEST
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"nao sei o que fazer com %q no comeco de uma expressao", p.curToken.Literal)
		return nil
	}
	left := prefix()

	for precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return left
		}
		p.nextToken()
		left = infix(left)
	}
	return left
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseNumero() ast.Expression {
	lit := p.curToken.Literal
	// numeros 0x/0o/0b: converter pro int usando a base correta.
	if len(lit) >= 2 && lit[0] == '0' {
		switch lit[1] {
		case 'x', 'X':
			if iv, err := strconv.ParseInt(lit[2:], 16, 64); err == nil {
				return &ast.NumeroLiteral{Token: p.curToken, Value: float64(iv), Int: iv, EhInt: true}
			}
		case 'o', 'O':
			if iv, err := strconv.ParseInt(lit[2:], 8, 64); err == nil {
				return &ast.NumeroLiteral{Token: p.curToken, Value: float64(iv), Int: iv, EhInt: true}
			}
		case 'b', 'B':
			if iv, err := strconv.ParseInt(lit[2:], 2, 64); err == nil {
				return &ast.NumeroLiteral{Token: p.curToken, Value: float64(iv), Int: iv, EhInt: true}
			}
		}
	}
	// Sem ponto nem expoente => inteiro exato (se couber em int64).
	if !strings.ContainsAny(lit, ".eE") {
		if iv, err := strconv.ParseInt(lit, 10, 64); err == nil {
			return &ast.NumeroLiteral{Token: p.curToken, Value: float64(iv), Int: iv, EhInt: true}
		}
	}
	val, err := strconv.ParseFloat(lit, 64)
	if err != nil {
		p.addErro(p.curToken.Line, p.curToken.Coluna, "numero estranho %q", lit)
		return nil
	}
	return &ast.NumeroLiteral{Token: p.curToken, Value: val}
}

func (p *Parser) parseTexto() ast.Expression {
	tok := p.curToken
	lit := tok.Literal
	// detecta se tem marca de interpolacao `${...}` nao escapada
	if !strings.Contains(lit, "${") {
		return &ast.TextoLiteral{Token: tok, Value: lit}
	}
	parts, ok := interpolar(p, tok, lit)
	if !ok {
		return &ast.TextoLiteral{Token: tok, Value: lit}
	}
	if len(parts) == 0 {
		return &ast.TextoLiteral{Token: tok, Value: ""}
	}
	// otimizacao: textos sem expressao vira TextoLiteral direto
	if len(parts) == 1 {
		if t, ok := parts[0].(*ast.TextoLiteral); ok {
			return t
		}
	}
	return &ast.TextoInterpolado{Token: tok, Parts: parts}
}

// interpolar percorre `lit` e separa em *TextoLiteral e Expression nos
// pontos onde ha `${expr}`. Escapes: `\${` vira `${` literal. Expressoes
// podem conter chaves aninhadas (conta balanceada).
// Erros de parse dentro de ${} viram erros do parser pai (acrescentados em p.errs).
func interpolar(p *Parser, tok token.Token, lit string) ([]ast.Expression, bool) {
	var parts []ast.Expression
	var sb strings.Builder
	i := 0
	for i < len(lit) {
		// escape \${ -> drop \ e mantem ${
		if i+2 < len(lit) && lit[i] == '\\' && lit[i+1] == '$' && lit[i+2] == '{' {
			sb.WriteByte('$')
			sb.WriteByte('{')
			i += 3
			continue
		}
		if i+1 < len(lit) && lit[i] == '$' && lit[i+1] == '{' {
			// flush literal ate aqui
			if sb.Len() > 0 {
				parts = append(parts, &ast.TextoLiteral{Token: tok, Value: sb.String()})
				sb.Reset()
			}
			// scan balanceado ate a chave que fecha
			start := i + 2
			depth := 1
			j := start
			for j < len(lit) && depth > 0 {
				switch lit[j] {
				case '{':
					depth++
				case '}':
					depth--
				}
				if depth == 0 {
					break
				}
				j++
			}
			if depth != 0 {
				// nao fechou: trata como literal
				sb.WriteString(lit[i:])
				i = len(lit)
				break
			}
			exprSrc := lit[start:j]
			// parser recursivo: sub-lexer + sub-parser (New ja carrega cur+
			// peek token, nao chamamos nextToken duas vezes aqui — senao
			// consumimos o primeiro token da expressao).
			sub := New(lexer.New(exprSrc))
			expr := sub.parseExpression(LOWEST)
			if expr == nil {
				p.errs = append(p.errs, ErroParse{Linha: tok.Line, Coluna: tok.Coluna, Msg: "expressao vazia em ${...}"})
				return nil, false
			}
			// so pra garantir: lexer sempre emite EOF; aceitar trailing EOF
			for !sub.curTokenIs(token.EOF) {
				// se sobrou algo, ignora — expressao simples
				sub.nextToken()
			}
			parts = append(parts, expr)
			// acumula erros do sub-parser pro pai
			p.errs = append(p.errs, sub.errs...)
			i = j + 1
			continue
		}
		sb.WriteByte(lit[i])
		i++
	}
	if sb.Len() > 0 {
		parts = append(parts, &ast.TextoLiteral{Token: tok, Value: sb.String()})
	}
	return parts, true
}

func (p *Parser) parseBooleano() ast.Expression {
	return &ast.BooleanoLiteral{Token: p.curToken, Value: p.curTokenIs(token.DEU_BOM)}
}

func (p *Parser) parseNada() ast.Expression {
	return &ast.NadaLiteral{Token: p.curToken}
}

func (p *Parser) parsePrefix() ast.Expression {
	exp := &ast.PrefixExpression{Token: p.curToken, Operator: p.curToken.Literal}
	p.nextToken()
	exp.Right = p.parseExpression(PREFIX)
	return exp
}

// parseBora trata `bora fn(args)`: le a expressao a direita (que tem que
// ser uma chamada de funcao) e envelopa como BoraExpression. Permite usos
// como `bora fib(40)` como statement ou `bota f = bora fib(40)`.
func (p *Parser) parseBora() ast.Expression {
	tok := p.curToken
	p.nextToken()                    // consome `bora`
	exp := p.parseExpression(PREFIX) // PRECISA vir uma chamada `fn(...)`
	call, ok := exp.(*ast.CallExpression)
	if !ok {
		p.addErro(tok.Line, tok.Coluna,
			"depois do `bora` tem que vir uma chamada de gambiarra tipo f(args), veio outra coisa")
		return nil
	}
	return &ast.BoraExpression{Token: tok, Call: call}
}

func (p *Parser) parseInfix(left ast.Expression) ast.Expression {
	exp := &ast.InfixExpression{Token: p.curToken, Operator: p.curToken.Literal, Left: left}
	prec := p.curPrecedence()
	p.nextToken()
	exp.Right = p.parseExpression(prec)
	return exp
}

// parseRange monta `inicio..fim`. Usa a propria precedencia (RANGE) no lado
// direito pra `0..n-1` virar `0..(n-1)` (subtracao mais forte que o range).
func (p *Parser) parseRange(left ast.Expression) ast.Expression {
	exp := &ast.RangeExpression{Token: p.curToken, Start: left}
	prec := p.curPrecedence()
	p.nextToken()
	exp.End = p.parseExpression(prec)
	return exp
}

// parseDot monta `obj.campo` como acucar pra `obj["campo"]`: vira uma
// IndexExpression com Dot=true (engines nao mudam; formatter reimprime com
// ponto). Funciona pra leitura, escrita (`bota obj.campo = v`) e chamada de
// metodo (`obj.metodo(args)`).
func (p *Parser) parseDot(left ast.Expression) ast.Expression {
	tok := p.curToken // o '.'
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	return &ast.IndexExpression{
		Token: tok,
		Left:  left,
		Index: &ast.TextoLiteral{Token: p.curToken, Value: p.curToken.Literal},
		Dot:   true,
	}
}

func (p *Parser) parseGrouped() ast.Expression {
	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) parseLista() ast.Expression {
	lista := &ast.ListaLiteral{Token: p.curToken}
	lista.Elements = p.parseExpressionList(token.RBRACKET)
	return lista
}

func (p *Parser) parseDicionario() ast.Expression {
	dic := &ast.DicionarioLiteral{Token: p.curToken}
	for !p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		chave := p.parseExpression(LOWEST)
		if !p.expectPeek(token.COLON) {
			return nil
		}
		p.nextToken()
		valor := p.parseExpression(LOWEST)
		dic.Pares = append(dic.Pares, ast.ParAST{Chave: chave, Valor: valor})
		if !p.peekTokenIs(token.RBRACE) && !p.expectPeek(token.COMMA) {
			return nil
		}
	}
	if !p.expectPeek(token.RBRACE) {
		return nil
	}
	return dic
}

func (p *Parser) parseCall(fn ast.Expression) ast.Expression {
	return &ast.CallExpression{Token: p.curToken, Function: fn, Arguments: p.parseExpressionList(token.RPAREN)}
}

func (p *Parser) parseIndex(left ast.Expression) ast.Expression {
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}
	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)
	if !p.expectPeek(token.RBRACKET) {
		return nil
	}
	return exp
}

// parseExpressionList le elementos separados por virgula ate o token de fechamento.
// curToken deve estar no token de abertura ( ou [ ; ao final fica no token de fechamento.
func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	lista := []ast.Expression{}
	if p.peekTokenIs(end) {
		p.nextToken()
		return lista
	}
	p.nextToken()
	lista = append(lista, p.parseExpression(LOWEST))
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		lista = append(lista, p.parseExpression(LOWEST))
	}
	if !p.expectPeek(end) {
		return nil
	}
	return lista
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{Statements: []ast.Statement{}}
	for !p.curTokenIs(token.EOF) {
		if stmt := p.parseStatement(); stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}
	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.BOTA:
		return p.parseBota()
	case token.MOSTRA:
		return p.parseMostra()
	case token.FUNCIONA:
		return p.parseFunciona()
	case token.VAZA:
		return &ast.VazaStatement{Token: p.curToken}
	case token.CONTINUA:
		return &ast.ContinuaStatement{Token: p.curToken}
	case token.SE_COLAR:
		return p.parseSeColar()
	case token.ENQUANTO:
		return p.parseEnquanto()
	case token.PRA_CADA:
		return p.parsePraCada()
	case token.GAMBIARRA:
		// `gambiarra nome(...)` e declaracao; `gambiarra(...)` e lambda
		// anonima em posicao de expressao.
		if p.peekTokenIs(token.IDENT) {
			return p.parseGambiarra()
		}
		return p.parseExpressionStatement()
	case token.ARRUMA:
		return p.parseArruma()
	case token.ESCOLHE:
		return p.parseEscolhe()
	case token.IMPORTA:
		return p.parseImporta()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseExpressionStatement() ast.Statement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	// atribuicao composta sem `bota`: `x += 1`, `xs[i] <<= 2`, `obj.n -= 3`.
	// O pratt parser para no token composto (nao e infix registrado), entao
	// ele fica no peek. Desugar: BotaStatement{alvo, Value: alvo op rhs}.
	if op, ok := baseDoComposto[p.peekToken.Type]; ok {
		return p.parseAtribuicaoComposta(stmt.Expression, op)
	}
	return stmt
}

func (p *Parser) parseAtribuicaoComposta(alvo ast.Expression, op string) ast.Statement {
	p.nextToken() // consome o alvo; cur = token composto (+=, <<=, ...)
	opTok := p.curToken
	stmt := &ast.BotaStatement{Token: opTok, OpComposto: opTok.Literal}
	switch a := alvo.(type) {
	case *ast.Identifier:
		stmt.Name = a
	case *ast.IndexExpression:
		stmt.Indice = a
	default:
		p.addErro(opTok.Line, opTok.Coluna,
			"o %s so cola em variavel ou alvo[indice], nao em expressao solta", opTok.Literal)
		return nil
	}
	p.nextToken()
	rhs := p.parseExpression(LOWEST)
	stmt.Value = &ast.InfixExpression{Token: opTok, Operator: op, Left: alvo, Right: rhs}
	return stmt
}

func (p *Parser) parseBota() ast.Statement {
	// desestruturacao: `bota [a, b] = lista` / `bota {x, y} = dict`
	if p.peekTokenIs(token.LBRACKET) || p.peekTokenIs(token.LBRACE) {
		return p.parseDesestrutura()
	}
	stmt := &ast.BotaStatement{Token: p.curToken}
	p.nextToken()
	alvo := p.parseExpression(LOWEST)
	switch a := alvo.(type) {
	case *ast.Identifier:
		stmt.Name = a
	case *ast.IndexExpression:
		stmt.Indice = a
	default:
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"depois do bota eu esperava um nome ou um alvo[indice], veio outra coisa")
		return nil
	}
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	return stmt
}

// parseEscolhe monta o switch:
//
//	escolhe <expr>
//	caso <v1>[, <v2>...]  <bloco>
//	...
//	se_nao_colar <bloco>      (opcional)
//	acabou_finalmente
func (p *Parser) parseEscolhe() ast.Statement {
	stmt := &ast.EscolheStatement{Token: p.curToken}
	p.nextToken()
	stmt.Subject = p.parseExpression(LOWEST)
	p.nextToken()

	if !p.curTokenIs(token.CASO) {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"escolhe sem nenhum `caso`? escolhe o que entao?")
		return nil
	}
	for p.curTokenIs(token.CASO) {
		braco := ast.CasoBraco{}
		p.nextToken()
		braco.Values = append(braco.Values, p.parseExpression(LOWEST))
		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // vai pra virgula
			p.nextToken() // vai pro proximo valor
			braco.Values = append(braco.Values, p.parseExpression(LOWEST))
		}
		p.nextToken()
		braco.Body = p.parseBlockStatement()
		stmt.Casos = append(stmt.Casos, braco)
	}
	if p.curTokenIs(token.SE_NAO_COLAR) {
		p.nextToken()
		stmt.Default = p.parseBlockStatement()
	}
	if !p.curTokenIs(token.ACABOU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"cade o acabou_finalmente do escolhe?")
	}
	return stmt
}

// parseDesestrutura monta `bota [a, b] = expr` ou `bota {x, y} = expr`.
// cur = bota; peek = [ ou {.
func (p *Parser) parseDesestrutura() ast.Statement {
	stmt := &ast.DesestruturaStatement{Token: p.curToken}
	p.nextToken() // cur = [ ou {
	fecha := token.TokenType(token.RBRACKET)
	if p.curTokenIs(token.LBRACE) {
		stmt.DeDict = true
		fecha = token.RBRACE
	}
	// lista de nomes separada por virgula
	for {
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Names = append(stmt.Names, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			continue
		}
		break
	}
	if !p.expectPeek(fecha) {
		return nil
	}
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	return stmt
}

func (p *Parser) parseMostra() ast.Statement {
	stmt := &ast.MostraStatement{Token: p.curToken}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	return stmt
}

func (p *Parser) parseFunciona() ast.Statement {
	stmt := &ast.FuncionaStatement{Token: p.curToken}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	return stmt
}

// parseBlockStatement le statements ate um terminador, sem consumi-lo.
func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken, Statements: []ast.Statement{}}
	for !p.curTokenIs(token.ACABOU) &&
		!p.curTokenIs(token.SE_NAO_COLAR) &&
		!p.curTokenIs(token.QUEBROU) &&
		!p.curTokenIs(token.FINALMENTE) &&
		!p.curTokenIs(token.CASO) &&
		!p.curTokenIs(token.EOF) {
		if stmt := p.parseStatement(); stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}
	return block
}

func (p *Parser) parseSeColar() ast.Statement {
	stmt := &ast.SeColarStatement{Token: p.curToken}

	p.nextToken()
	stmt.Conditions = append(stmt.Conditions, p.parseExpression(LOWEST))
	p.nextToken()
	stmt.Consequences = append(stmt.Consequences, p.parseBlockStatement())

	for p.curTokenIs(token.SE_NAO_COLAR) {
		p.nextToken()
		if p.curTokenIs(token.SE_COLAR) {
			p.nextToken()
			stmt.Conditions = append(stmt.Conditions, p.parseExpression(LOWEST))
			p.nextToken()
			stmt.Consequences = append(stmt.Consequences, p.parseBlockStatement())
		} else {
			stmt.Alternative = p.parseBlockStatement()
			break
		}
	}

	if !p.curTokenIs(token.ACABOU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"cade o acabou_finalmente pra fechar o se_colar?")
	}
	return stmt
}

func (p *Parser) parseEnquanto() ast.Statement {
	stmt := &ast.EnquantoStatement{Token: p.curToken}
	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)
	p.nextToken()
	stmt.Body = p.parseBlockStatement()
	if !p.curTokenIs(token.ACABOU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"cade o acabou_finalmente pra fechar o enquanto?")
	}
	return stmt
}

func (p *Parser) parsePraCada() ast.Statement {
	tok := p.curToken
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	varName := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken() // de | em
	switch p.curToken.Type {
	case token.DE:
		stmt := &ast.PraCadaNumStatement{Token: tok, Var: varName}
		p.nextToken()
		stmt.Start = p.parseExpression(LOWEST)
		if !p.expectPeek(token.ATE) {
			return nil
		}
		p.nextToken()
		stmt.End = p.parseExpression(LOWEST)
		p.nextToken()
		stmt.Body = p.parseBlockStatement()
		return stmt
	case token.EM:
		stmt := &ast.PraCadaListStatement{Token: tok, Var: varName}
		p.nextToken()
		stmt.Iterable = p.parseExpression(LOWEST)
		p.nextToken()
		stmt.Body = p.parseBlockStatement()
		return stmt
	default:
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"depois do pra_cada eu esperava 'de' ou 'em', veio %q", p.curToken.Literal)
		return nil
	}
}

// parseFuncaoLiteral monta a lambda anonima `gambiarra(params) <corpo>
// acabou_finalmente` como expressao. Deixa cur no acabou_finalmente (ultimo
// token da expressao), seguindo a convencao das outras parse fns.
func (p *Parser) parseFuncaoLiteral() ast.Expression {
	lit := &ast.FuncaoLiteral{Token: p.curToken}
	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	lit.Parameters = p.parseFunctionParameters()
	p.nextToken() // sai do ) para o corpo
	lit.Body = p.parseBlockStatement()
	if !p.curTokenIs(token.ACABOU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"lambda sem acabou_finalmente? fecha ela, parca")
		return nil
	}
	return lit
}

func (p *Parser) parseGambiarra() ast.Statement {
	stmt := &ast.GambiarraStatement{Token: p.curToken}
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	stmt.Parameters = p.parseFunctionParameters()
	p.nextToken() // sai do ) para o corpo
	stmt.Body = p.parseBlockStatement()
	return stmt
}

// parseFunctionParameters: curToken deve estar em '(' ; ao final fica em ')'.
func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	params := []*ast.Identifier{}
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return params
	}
	p.nextToken()
	params = append(params, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		params = append(params, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}
	p.nextToken() // curToken = )
	return params
}

func (p *Parser) parseImporta() ast.Statement {
	stmt := &ast.ImportaStatement{Token: p.curToken}
	p.nextToken()
	stmt.Path = p.parseExpression(LOWEST)
	return stmt
}

func (p *Parser) parseArruma() ast.Statement {
	stmt := &ast.ArrumaStatement{Token: p.curToken}
	p.nextToken()
	stmt.Try = p.parseBlockStatement()
	// catch opcional: `quebrou err <catch-block>`
	if p.curTokenIs(token.QUEBROU) {
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.ErrName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		p.nextToken()
		stmt.Catch = p.parseBlockStatement()
	}
	// bloco finally opcional: `finalmente <block> acabou_finalmente`
	if p.curTokenIs(token.FINALMENTE) {
		p.nextToken()
		stmt.Finally = p.parseBlockStatement()
	}
	if stmt.Catch == nil && stmt.Finally == nil {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"arruma sem 'quebrou' nem 'finalmente'? cade o resto?")
	}
	if !p.curTokenIs(token.ACABOU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"cade o acabou_finalmente pra fechar o arruma?")
	}
	return stmt
}
