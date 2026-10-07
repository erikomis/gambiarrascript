package parser

import (
	"unicode"
	"unicode/utf8"

	"gambiarrascript/ast"
	"gambiarrascript/token"
)

// ---- POO no modelo do Go: treta, combinado, metodo e literal Tipo{...} ----

// ComecaMaiuscula diz se o nome comeca com letra maiuscula. E a regra que
// separa tipo de valor na sintaxe: so nome maiusculo vira literal de treta
// (`Ponto{...}`) e so linha maiuscula sozinha na treta vira puxadinho.
func ComecaMaiuscula(nome string) bool {
	r, _ := utf8.DecodeRuneInString(nome)
	return unicode.IsUpper(r)
}

// ehTipoDeLiteral decide se `left {` e um literal de treta: left tem que ser
// um nome com maiuscula (`Ponto`) ou `modulo.Nome`, e o `{` tem que estar na
// MESMA linha do fim dele. Como statement nao tem separador, sem a regra da
// linha um dicionario solto na linha de baixo de um `mostra LIMITE` grudaria
// nele; sem a maiuscula, `mostra x {` mudaria de sentido.
func (p *Parser) ehTipoDeLiteral(left ast.Expression) bool {
	if p.peekToken.Line != p.curToken.Line {
		return false
	}
	switch n := left.(type) {
	case *ast.Identifier:
		return ComecaMaiuscula(n.Value)
	case *ast.IndexExpression:
		t, ok := n.Index.(*ast.TextoLiteral)
		if !n.Dot || n.Safe || !ok {
			return false
		}
		_, esqNome := n.Left.(*ast.Identifier)
		return esqNome && ComecaMaiuscula(t.Value)
	}
	return false
}

// parseTretaLiteral le `{x: 1, y: 2}` / `{1, 2}` / `{}` depois do tipo. cur
// = `{`; termina no `}`. Nomeado ou posicional, sem misturar (igual Go).
func (p *Parser) parseTretaLiteral(tipo ast.Expression) ast.Expression {
	lit := &ast.TretaLiteral{Token: p.curToken, Tipo: tipo}
	nomeado := p.peekTokenIs(token.IDENT) && p.espia(1).Type == token.COLON
	if nomeado {
		lit.Nomes = []*ast.Identifier{}
	}
	misturou := func() ast.Expression {
		p.addErro(p.peekToken.Line, p.peekToken.Coluna,
			"no %s{...} ou vai tudo com nome (x: 1) ou tudo na ordem (1, 2), nao mistura", tipo.String())
		return nil
	}
	for !p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		if nomeado {
			if !p.curTokenIs(token.IDENT) || !p.peekTokenIs(token.COLON) {
				p.addErro(p.curToken.Line, p.curToken.Coluna,
					"no %s{...} ou vai tudo com nome (x: 1) ou tudo na ordem (1, 2), nao mistura", tipo.String())
				return nil
			}
			lit.Nomes = append(lit.Nomes, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
			p.nextToken() // :
			p.nextToken() // valor
		}
		lit.Valores = append(lit.Valores, p.parseExpression(LOWEST))
		if !nomeado && p.peekTokenIs(token.COLON) {
			return misturou()
		}
		if p.peekTokenIs(token.RBRACE) {
			break
		}
		if !p.expectPeek(token.COMMA) {
			return nil
		}
	}
	if !p.expectPeek(token.RBRACE) {
		return nil
	}
	if p.pos != nil {
		p.pos.Fecha[lit] = p.curToken
	}
	return lit
}

// declaracaoNoTopo reclama de treta/combinado/metodo dentro de bloco: o tipo
// e a tabela de metodos sao do arquivo, nao de uma chamada.
func (p *Parser) declaracaoNoTopo(oque string) bool {
	if p.prof == 0 {
		return true
	}
	p.addErro(p.curToken.Line, p.curToken.Coluna,
		"%s so pode ser declarada no topo do arquivo (fora de gambiarra, laco, se_colar...)", oque)
	return false
}

// nomeDeTipo le o nome depois do `treta`/`combinado` (tem que ser maiusculo).
func (p *Parser) nomeDeTipo(oque string) *ast.Identifier {
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	nome := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !ComecaMaiuscula(nome.Value) {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"nome de %s comeca com maiuscula (tipo %s): e assim que `Nome{...}` vira literal e nao dicionario",
			oque, maiusculaNoComeco(nome.Value))
		return nil
	}
	return nome
}

func maiusculaNoComeco(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// embutidoNaLinha le `Nome` ou `modulo.Nome` (cur no primeiro nome) como
// expressao — e o puxadinho da treta e o combinado embutido.
func (p *Parser) embutidoNaLinha() ast.Expression {
	id := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.peekTokenIs(token.DOT) {
		return id
	}
	p.nextToken() // .
	tokPonto := p.curToken
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	return &ast.IndexExpression{
		Token: tokPonto,
		Left:  id,
		Index: &ast.TextoLiteral{Token: p.curToken, Value: p.curToken.Literal},
		Dot:   true,
	}
}

// marcaPosicao guarda inicio/fim de uma linha de treta/combinado (formatter).
func (p *Parser) marcaPosicao(s ast.Statement, ini token.Token) {
	if p.pos != nil {
		p.pos.Inicio[s] = ini
		p.pos.Fim[s] = p.curToken
	}
}

// parseTreta monta:
//
//	treta Nome
//	    campo
//	    campo = padrao
//	    Outra          # puxadinho (embedding)
//	acabou_finalmente
func (p *Parser) parseTreta() ast.Statement {
	stmt := &ast.TretaDecl{Token: p.curToken}
	topo := p.declaracaoNoTopo("treta")
	if stmt.Nome = p.nomeDeTipo("treta"); stmt.Nome == nil {
		return nil
	}
	if p.pos != nil {
		p.pos.Cabeca[stmt] = p.curToken
	}
	p.nextToken()
	for !p.curTokenIs(token.ACABOU) && !p.curTokenIs(token.EOF) {
		ini := p.curToken
		if !p.curTokenIs(token.IDENT) {
			p.addErro(p.curToken.Line, p.curToken.Coluna,
				"na treta %s eu esperava um campo (nome, nome = padrao ou Outra treta), veio %q",
				stmt.Nome.Value, p.curToken.Literal)
			return nil
		}
		campo := &ast.CampoTreta{Token: p.curToken, Nome: &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}}
		switch {
		case p.peekTokenIs(token.ASSIGN):
			p.nextToken() // =
			p.nextToken()
			campo.Padrao = p.parseExpression(LOWEST)
		case p.peekTokenIs(token.DOT) || ComecaMaiuscula(campo.Nome.Value):
			// puxadinho: o campo se chama como a treta embutida
			campo.Embutida = p.embutidoNaLinha()
			if campo.Embutida == nil {
				return nil
			}
			if campo.Nome.Value == stmt.Nome.Value {
				p.addErro(ini.Line, ini.Coluna, "a treta %s nao pode ser puxadinho dela mesma", stmt.Nome.Value)
				return nil
			}
			if ix, ok := campo.Embutida.(*ast.IndexExpression); ok {
				t := ix.Index.(*ast.TextoLiteral)
				campo.Nome = &ast.Identifier{Token: t.Token, Value: t.Value}
				if !ComecaMaiuscula(t.Value) {
					p.addErro(t.Token.Line, t.Token.Coluna,
						"puxadinho %s: o nome da treta embutida comeca com maiuscula", ix.String())
					return nil
				}
			}
		}
		p.marcaPosicao(campo, ini)
		stmt.Campos = append(stmt.Campos, campo)
		p.nextToken()
	}
	if !p.curTokenIs(token.ACABOU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna, "cade o acabou_finalmente da treta %s?", stmt.Nome.Value)
		return nil
	}
	if !topo {
		return nil
	}
	return stmt
}

// parseCombinado monta:
//
//	combinado Nome
//	    metodo(params)
//	    Outro          # combinado embutido
//	acabou_finalmente
func (p *Parser) parseCombinado() ast.Statement {
	stmt := &ast.CombinadoDecl{Token: p.curToken}
	topo := p.declaracaoNoTopo("combinado")
	if stmt.Nome = p.nomeDeTipo("combinado"); stmt.Nome == nil {
		return nil
	}
	if p.pos != nil {
		p.pos.Cabeca[stmt] = p.curToken
	}
	p.nextToken()
	for !p.curTokenIs(token.ACABOU) && !p.curTokenIs(token.EOF) {
		ini := p.curToken
		if !p.curTokenIs(token.IDENT) {
			p.addErro(p.curToken.Line, p.curToken.Coluna,
				"no combinado %s eu esperava a assinatura de um metodo (tipo escreve(texto)), veio %q",
				stmt.Nome.Value, p.curToken.Literal)
			return nil
		}
		ass := &ast.AssinaturaMetodo{Token: p.curToken, Nome: &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}}
		switch {
		case p.peekTokenIs(token.LPAREN):
			p.nextToken()
			ass.Parametros = p.parseFunctionParameters()
		case p.peekTokenIs(token.DOT) || ComecaMaiuscula(ass.Nome.Value):
			ass.Embutido = p.embutidoNaLinha()
			if ass.Embutido == nil {
				return nil
			}
		default:
			p.addErro(p.curToken.Line, p.curToken.Coluna,
				"no combinado o metodo vem com os parenteses: %s(...)", ass.Nome.Value)
			return nil
		}
		p.marcaPosicao(ass, ini)
		stmt.Metodos = append(stmt.Metodos, ass)
		p.nextToken()
	}
	if !p.curTokenIs(token.ACABOU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna, "cade o acabou_finalmente do combinado %s?", stmt.Nome.Value)
		return nil
	}
	if !topo {
		return nil
	}
	return stmt
}

// ehMetodo: cur = gambiarra, peek = `(`; metodo e `( nome Tipo )`.
func (p *Parser) ehMetodo() bool {
	return p.peekTokenIs(token.LPAREN) &&
		p.espia(1).Type == token.IDENT &&
		p.espia(2).Type == token.IDENT &&
		p.espia(3).Type == token.RPAREN
}

// parseMetodo monta `gambiarra (p Ponto) nome(params) <corpo> acabou_finalmente`.
func (p *Parser) parseMetodo() ast.Statement {
	stmt := &ast.MetodoDecl{Token: p.curToken}
	topo := p.declaracaoNoTopo("gambiarra com receiver (metodo)")
	p.nextToken() // (
	p.nextToken()
	stmt.Receptor = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	p.nextToken()
	stmt.Tipo = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	p.nextToken() // )
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Nome = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	stmt.Parameters = p.parseFunctionParameters()
	p.nextToken() // sai do ) para o corpo
	stmt.Body = p.parseBlockStatement()
	if !p.curTokenIs(token.ACABOU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna,
			"cade o acabou_finalmente do metodo %s?", stmt.Nome.Value)
		return nil
	}
	for _, par := range stmt.Parameters {
		if par != nil && par.Nome != nil && par.Nome.Value == stmt.Receptor.Value {
			p.addErro(par.Nome.Token.Line, par.Nome.Token.Coluna,
				"o parametro %s tem o mesmo nome do receiver", par.Nome.Value)
			return nil
		}
	}
	if !topo {
		return nil
	}
	return stmt
}
