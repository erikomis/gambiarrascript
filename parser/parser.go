package parser

import (
	"fmt"
	"strconv"

	"gambiarrascript/ast"
	"gambiarrascript/lexer"
	"gambiarrascript/token"
)

const (
	_ int = iota
	LOWEST
	OU          // ou
	E           // e
	EQUALS      // == !=
	LESSGREATER // < > <= >=
	SUM         // + -
	PRODUCT     // * / %
	PREFIX      // -x  nao x
	CALL        // f(x)
	INDEX       // lista[i]
)

var precedencias = map[token.TokenType]int{
	token.OU:       OU,
	token.E:        E,
	token.EQ:       EQUALS,
	token.NEQ:      EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.LTE:      LESSGREATER,
	token.GTE:      LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.STAR:     PRODUCT,
	token.SLASH:    PRODUCT,
	token.PERCENT:  PRODUCT,
	token.LPAREN:   CALL,
	token.LBRACKET: INDEX,
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
	p.registerPrefix(token.LPAREN, p.parseGrouped)
	p.registerPrefix(token.LBRACKET, p.parseLista)
	p.registerPrefix(token.LBRACE, p.parseDicionario)

	p.infixParseFns = map[token.TokenType]infixParseFn{}
	for _, tt := range []token.TokenType{
		token.PLUS, token.MINUS, token.STAR, token.SLASH, token.PERCENT,
		token.EQ, token.NEQ, token.LT, token.GT, token.LTE, token.GTE,
		token.E, token.OU,
	} {
		p.registerInfix(tt, p.parseInfix)
	}
	p.registerInfix(token.LPAREN, p.parseCall)
	p.registerInfix(token.LBRACKET, p.parseIndex)

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
	val, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		p.addErro(p.curToken.Line, p.curToken.Coluna, "numero estranho %q", p.curToken.Literal)
		return nil
	}
	return &ast.NumeroLiteral{Token: p.curToken, Value: val}
}

func (p *Parser) parseTexto() ast.Expression {
	return &ast.TextoLiteral{Token: p.curToken, Value: p.curToken.Literal}
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

func (p *Parser) parseInfix(left ast.Expression) ast.Expression {
	exp := &ast.InfixExpression{Token: p.curToken, Operator: p.curToken.Literal, Left: left}
	prec := p.curPrecedence()
	p.nextToken()
	exp.Right = p.parseExpression(prec)
	return exp
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
		return p.parseGambiarra()
	case token.ARRUMA:
		return p.parseArruma()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseExpressionStatement() ast.Statement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	return stmt
}

func (p *Parser) parseBota() ast.Statement {
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

func (p *Parser) parseArruma() ast.Statement {
	stmt := &ast.ArrumaStatement{Token: p.curToken}
	p.nextToken()
	stmt.Try = p.parseBlockStatement()
	if !p.curTokenIs(token.QUEBROU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"cade o 'quebrou' pra pegar o erro do arruma?")
		return stmt
	}
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.ErrName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	p.nextToken()
	stmt.Catch = p.parseBlockStatement()
	if !p.curTokenIs(token.ACABOU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"cade o acabou_finalmente pra fechar o arruma?")
	}
	return stmt
}
