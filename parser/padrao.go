package parser

import (
	"gambiarrascript/ast"
	"gambiarrascript/token"
)

// ---- pattern matching no caso ----

// parseValorCaso le um valor de `caso`. Comeco com `[`, `{`, `Tipo{` ou o
// curinga `_` e padrao; o resto e a expressao de sempre (comparada com ==).
// cur = primeiro token do valor; termina no ultimo token dele.
func (p *Parser) parseValorCaso() ast.Expression {
	if p.curTokenIs(token.IDENT) && p.curToken.Literal == "_" {
		return p.padraoNome()
	}
	if p.ehComecoDePadrao() {
		pad := p.parseSubPadrao()
		if pad != nil {
			p.confereNomesRepetidos(pad)
		}
		return pad
	}
	return p.parseExpression(LOWEST)
}

// ehComecoDePadrao: `[`, `{`, `Tipo{` ou `mod.Tipo{` (o `{` colado na mesma
// linha, igual o literal de treta).
func (p *Parser) ehComecoDePadrao() bool {
	switch p.curToken.Type {
	case token.LBRACKET, token.LBRACE:
		return true
	case token.IDENT:
		if ComecaMaiuscula(p.curToken.Literal) && p.peekTokenIs(token.LBRACE) &&
			p.peekToken.Line == p.curToken.Line {
			return true
		}
		if p.peekTokenIs(token.DOT) {
			nome, chave := p.espia(1), p.espia(2)
			return nome.Type == token.IDENT && ComecaMaiuscula(nome.Literal) &&
				chave.Type == token.LBRACE && chave.Line == nome.Line
		}
	}
	return false
}

// parseSubPadrao le um pedaco de padrao. cur = primeiro token.
func (p *Parser) parseSubPadrao() ast.Expression {
	switch {
	case p.curTokenIs(token.LBRACKET):
		return p.parsePadraoLista()
	case p.curTokenIs(token.LBRACE):
		return p.parsePadraoDict()
	case p.ehComecoDePadrao():
		tipo := p.embutidoNaLinha()
		if tipo == nil || !p.expectPeek(token.LBRACE) {
			return nil
		}
		return p.parsePadraoTreta(tipo)
	case p.curTokenIs(token.IDENT) && (p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.RBRACKET) || p.peekTokenIs(token.RBRACE)):
		return p.padraoNome()
	}
	tok := p.curToken
	e := p.parseExpression(LOWEST)
	if e == nil {
		return nil
	}
	if !valorDePadrao(e) {
		p.addErro(tok.Line, tok.Coluna,
			"`%s` nao vale dentro de padrao: ai so entra literal (1, \"x\", deu_bom, nada), nome (que amarra), `_` ou Tipo.membro — conta vai na guarda (`caso [x] se x > 1`)",
			e.String())
		return nil
	}
	return e
}

func (p *Parser) padraoNome() ast.Expression {
	return &ast.PadraoNome{Token: p.curToken, Nome: &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}}
}

// valorDePadrao: o que pode ser comparado com == dentro de um padrao —
// literal simples, numero negativo ou caminho com ponto (`Cor.vermelho`).
func valorDePadrao(e ast.Expression) bool {
	switch n := e.(type) {
	case *ast.NumeroLiteral, *ast.TextoLiteral, *ast.BooleanoLiteral, *ast.NadaLiteral:
		return true
	case *ast.PrefixExpression:
		_, ok := n.Right.(*ast.NumeroLiteral)
		return ok && n.Operator == "-"
	case *ast.IndexExpression:
		return caminhoComPonto(n)
	}
	return false
}

func caminhoComPonto(e ast.Expression) bool {
	switch n := e.(type) {
	case *ast.Identifier:
		return true
	case *ast.IndexExpression:
		if _, ok := n.Index.(*ast.TextoLiteral); !ok || !n.Dot || n.Safe {
			return false
		}
		return caminhoComPonto(n.Left)
	}
	return false
}

// parsePadraoLista: `[p1, p2, ...resto]`. cur = `[`; termina no `]`.
func (p *Parser) parsePadraoLista() ast.Expression {
	pad := &ast.PadraoLista{Token: p.curToken}
	for !p.peekTokenIs(token.RBRACKET) {
		p.nextToken()
		if pad.Resto != nil {
			p.addErro(p.curToken.Line, p.curToken.Coluna, "o `...resto` tem que ser o ultimo do padrao de lista")
			return nil
		}
		if p.curTokenIs(token.ELLIPSIS) {
			if !p.expectPeek(token.IDENT) {
				return nil
			}
			pad.Resto = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		} else {
			el := p.parseSubPadrao()
			if el == nil {
				return nil
			}
			pad.Elementos = append(pad.Elementos, el)
		}
		if !p.peekTokenIs(token.RBRACKET) && !p.expectPeek(token.COMMA) {
			return nil
		}
	}
	p.nextToken()
	return pad
}

// parsePadraoDict: `{"chave": padrao, nome}`. cur = `{`; termina no `}`.
func (p *Parser) parsePadraoDict() ast.Expression {
	pad := &ast.PadraoDict{Token: p.curToken}
	for !p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		switch {
		case p.curTokenIs(token.IDENT) && !p.peekTokenIs(token.COLON):
			// `{nome}` = `{"nome": nome}`
			id := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			pad.Chaves = append(pad.Chaves, &ast.TextoLiteral{Token: p.curToken, Value: id.Value})
			pad.Valores = append(pad.Valores, &ast.PadraoNome{Token: p.curToken, Nome: id})
			pad.Curto = append(pad.Curto, true)
		case p.curTokenIs(token.TEXTO) || p.curTokenIs(token.NUMERO) || p.curTokenIs(token.DEU_BOM) || p.curTokenIs(token.DEU_RUIM):
			chave := p.parseExpression(INDEX)
			if _, interp := chave.(*ast.TextoInterpolado); interp || chave == nil {
				p.addErro(p.curToken.Line, p.curToken.Coluna, "chave de padrao de dicionario e literal (sem ${})")
				return nil
			}
			if !p.expectPeek(token.COLON) {
				return nil
			}
			p.nextToken()
			v := p.parseSubPadrao()
			if v == nil {
				return nil
			}
			pad.Chaves = append(pad.Chaves, chave)
			pad.Valores = append(pad.Valores, v)
			pad.Curto = append(pad.Curto, false)
		default:
			p.addErro(p.curToken.Line, p.curToken.Coluna,
				"no padrao de dicionario a chave e literal (\"tipo\": ...) ou so um nome ({nome}), veio %q", p.curToken.Literal)
			return nil
		}
		if !p.peekTokenIs(token.RBRACE) && !p.expectPeek(token.COMMA) {
			return nil
		}
	}
	p.nextToken()
	return pad
}

// parsePadraoTreta le o que vem depois do tipo, igual o literal de treta:
// `{x: padrao, y}` por nome (so os campos listados; `y` sozinho = `y: y`) ou
// `{p1, p2}` na ordem dos campos (sem nenhum `:`; tem que passar todos).
// cur = `{`; termina no `}`.
func (p *Parser) parsePadraoTreta(tipo ast.Expression) ast.Expression {
	pad := &ast.PadraoTreta{Token: p.curToken, Tipo: tipo}
	var nomes []*ast.Identifier // nil na entrada sem `campo:`
	for !p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		var nome *ast.Identifier
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.COLON) {
			nome = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			p.nextToken()
			p.nextToken()
		}
		v := p.parseSubPadrao()
		if v == nil {
			return nil
		}
		nomes = append(nomes, nome)
		pad.Valores = append(pad.Valores, v)
		if !p.peekTokenIs(token.RBRACE) && !p.expectPeek(token.COMMA) {
			return nil
		}
	}
	p.nextToken()
	porNome := false
	for _, n := range nomes {
		if n != nil {
			porNome = true
		}
	}
	if !porNome {
		pad.Posicional = true
		return pad
	}
	for i, n := range nomes {
		if n != nil {
			pad.Nomes = append(pad.Nomes, n)
			pad.Curto = append(pad.Curto, false)
			continue
		}
		// forma curta: so um nome (`y` = `y: y`)
		pn, ok := pad.Valores[i].(*ast.PadraoNome)
		if !ok || pn.Curinga() {
			p.addErro(pad.Token.Line, pad.Token.Coluna,
				"no padrao %s{...} ou vai tudo com nome (x: 0, y) ou tudo na ordem (0, y), nao mistura", tipo.String())
			return nil
		}
		pad.Nomes = append(pad.Nomes, pn.Nome)
		pad.Curto = append(pad.Curto, true)
	}
	return pad
}

// confereNomesRepetidos: o mesmo nome duas vezes no mesmo padrao e erro
// (`[x, x]` nao quer dizer "os dois iguais" — use guarda).
func (p *Parser) confereNomesRepetidos(pad ast.Expression) {
	visto := map[string]bool{}
	ast.PercorrePadrao(pad, func(id *ast.Identifier) {
		if visto[id.Value] {
			p.addErro(id.Token.Line, id.Token.Coluna,
				"`%s` aparece duas vezes no mesmo padrao — pra exigir igual, use guarda (`caso [a, b] se a == b`)", id.Value)
		}
		visto[id.Value] = true
	}, nil)
}

// ---- cardapio (enum) ----

// parseCardapio monta:
//
//	cardapio Cor
//	    vermelho
//	    verde
//	acabou_finalmente
func (p *Parser) parseCardapio() ast.Statement {
	stmt := &ast.CardapioDecl{Token: p.curToken}
	topo := p.declaracaoNoTopo("cardapio")
	if stmt.Nome = p.nomeDeTipo("cardapio"); stmt.Nome == nil {
		return nil
	}
	if p.pos != nil {
		p.pos.Cabeca[stmt] = p.curToken
	}
	p.nextToken()
	visto := map[string]bool{}
	for !p.curTokenIs(token.ACABOU) && !p.curTokenIs(token.EOF) {
		ini := p.curToken
		if !p.curTokenIs(token.IDENT) {
			p.addErro(p.curToken.Line, p.curToken.Coluna,
				"no cardapio %s eu esperava o nome de uma opcao (um por linha), veio %q",
				stmt.Nome.Value, p.curToken.Literal)
			return nil
		}
		nome := p.curToken.Literal
		if visto[nome] {
			p.addErro(ini.Line, ini.Coluna, "o cardapio %s ja tem %s", stmt.Nome.Value, nome)
			return nil
		}
		visto[nome] = true
		m := &ast.MembroCardapio{Token: p.curToken, Nome: &ast.Identifier{Token: p.curToken, Value: nome}}
		p.marcaPosicao(m, ini)
		stmt.Membros = append(stmt.Membros, m)
		p.nextToken()
	}
	if !p.curTokenIs(token.ACABOU) {
		p.addErro(p.curToken.Line, p.curToken.Coluna, "cade o acabou_finalmente do cardapio %s?", stmt.Nome.Value)
		return nil
	}
	if len(stmt.Membros) == 0 {
		p.addErro(stmt.Token.Line, stmt.Token.Coluna, "cardapio %s vazio? bota pelo menos uma opcao", stmt.Nome.Value)
		return nil
	}
	if !topo {
		return nil
	}
	return stmt
}
