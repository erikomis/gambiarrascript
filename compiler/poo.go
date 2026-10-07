package compiler

import (
	"fmt"

	"gambiarrascript/ast"
	"gambiarrascript/code"
	"gambiarrascript/object"
)

// POO na VM (Tier 8). A declaracao vira um opcode com descritor no pool de
// constantes (os valores que dependem de runtime — puxadinho, padrao,
// metodo — vem da pilha); a regra mora em object/poo.go, igual o tree-walker.

// textoDoTipo e como o tipo foi escrito (`Ponto`, `geo.Ponto`), pras mensagens.
func textoDoTipo(e ast.Expression) string {
	switch n := e.(type) {
	case *ast.Identifier:
		return n.Value
	case *ast.IndexExpression:
		if t, ok := n.Index.(*ast.TextoLiteral); ok {
			return textoDoTipo(n.Left) + "." + t.Value
		}
	}
	return e.String()
}

// ehLiteralSimples: padrao que nao precisa de thunk (valor imutavel). Tem que
// casar com o interpreter.thunkDoPadrao.
func ehLiteralSimples(e ast.Expression) bool {
	switch e.(type) {
	case *ast.NumeroLiteral, *ast.TextoLiteral, *ast.BooleanoLiteral, *ast.NadaLiteral:
		return true
	}
	return false
}

func (c *Compiler) exigeTopo(oque string, linha int) error {
	if c.scope.outer != nil {
		return fmt.Errorf("linha %d: %s so pode ser declarada no topo do arquivo", linha, oque)
	}
	return nil
}

func (c *Compiler) compileTretaDecl(node *ast.TretaDecl) error {
	if err := c.exigeTopo("treta", node.Token.Line); err != nil {
		return err
	}
	// o nome existe antes dos padroes compilarem: um thunk pode citar a
	// propria treta (roda so na hora de instanciar), igual o tree-walker
	sym := c.defineVar(node.Nome.Value)
	desc := &object.DescTreta{Nome: node.Nome.Value}
	for _, campo := range node.Campos {
		d := object.DescCampo{Nome: campo.Nome.Value}
		switch {
		case campo.Embutida != nil:
			d.Embutida, d.Texto = true, textoDoTipo(campo.Embutida)
			if err := c.compile(campo.Embutida); err != nil {
				return err
			}
		case campo.Padrao != nil:
			d.TemPadrao = true
			if ehLiteralSimples(campo.Padrao) {
				if err := c.compile(campo.Padrao); err != nil {
					return err
				}
				break
			}
			corpo := &ast.BlockStatement{Statements: []ast.Statement{
				&ast.FuncionaStatement{Token: campo.Token, Value: campo.Padrao},
			}}
			nome := node.Nome.Value + "." + campo.Nome.Value + "(padrao)"
			if err := c.compileFuncaoValor(nome, nil, corpo); err != nil {
				return err
			}
		}
		desc.Campos = append(desc.Campos, d)
	}
	c.linhaAtual = node.Token.Line
	c.emit(code.OpTreta, c.addConstant(desc))
	c.emitVarSet(sym)
	return nil
}

func (c *Compiler) compileCombinadoDecl(node *ast.CombinadoDecl) error {
	if err := c.exigeTopo("combinado", node.Token.Line); err != nil {
		return err
	}
	desc := &object.DescCombinado{Nome: node.Nome.Value}
	for _, m := range node.Metodos {
		if m.Embutido != nil {
			if err := c.compile(m.Embutido); err != nil {
				return err
			}
			desc.Embutidos = append(desc.Embutidos, textoDoTipo(m.Embutido))
			continue
		}
		desc.Metodos = append(desc.Metodos, object.AssinaturaMetodo{Nome: m.Nome.Value, NumParams: len(m.Parametros)})
	}
	c.linhaAtual = node.Token.Line
	c.emit(code.OpCombinado, c.addConstant(desc))
	c.emitVarSet(c.defineVar(node.Nome.Value))
	return nil
}

func (c *Compiler) compileMetodoDecl(node *ast.MetodoDecl) error {
	if err := c.exigeTopo("gambiarra com receiver (metodo)", node.Token.Line); err != nil {
		return err
	}
	if err := c.compileIdent(node.Tipo); err != nil {
		return err
	}
	nome := node.Tipo.Value + "." + node.Nome.Value
	if err := c.compileFuncaoValor(nome, node.ParametrosComReceptor(), node.Body); err != nil {
		return err
	}
	c.linhaAtual = node.Token.Line
	c.emit(code.OpMetodo, c.addConstant(&object.Texto{Value: node.Nome.Value}))
	return nil
}

func (c *Compiler) compileTretaLiteral(node *ast.TretaLiteral) error {
	if err := c.compile(node.Tipo); err != nil {
		return err
	}
	desc := &object.DescLiteral{Tipo: textoDoTipo(node.Tipo), Posicional: node.Posicional(), N: len(node.Valores)}
	for _, n := range node.Nomes {
		desc.Nomes = append(desc.Nomes, n.Value)
	}
	for _, v := range node.Valores {
		if err := c.compile(v); err != nil {
			return err
		}
	}
	c.linhaAtual = node.Token.Line
	c.emit(code.OpInstancia, c.addConstant(desc))
	return nil
}
