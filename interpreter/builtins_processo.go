package interpreter

import (
	"bytes"
	"os/exec"

	"gambiarrascript/object"
)

// builtinRodaComando roda um comando externo e devolve um dicionario
// {saida, erro, codigo}: stdout, stderr e o codigo de saida. Sair com codigo
// != 0 NAO e erro de GS (vem no `codigo` pra voce inspecionar); so vira erro
// quando o comando nem consegue iniciar (nao encontrado, sem permissao...).
//
//	roda_comando("ls")
//	roda_comando("git", ["status", "--short"])
func builtinRodaComando(args []object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("roda_comando() quer 1 ou 2 args (comando, [lista de args]), veio %d", len(args))
	}
	nome, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("roda_comando: 1o arg (comando) tem que ser texto, veio %s", args[0].Type())
	}
	var cmdArgs []string
	if len(args) == 2 {
		lst, ok := args[1].(*object.Lista)
		if !ok {
			return erroBuiltin("roda_comando: 2o arg (args) tem que ser lista, veio %s", args[1].Type())
		}
		for idx, e := range lst.Elements {
			t, ok := e.(*object.Texto)
			if !ok {
				return erroBuiltin("roda_comando: arg %d nao e texto, veio %s", idx, e.Type())
			}
			cmdArgs = append(cmdArgs, t.Value)
		}
	}

	cmd := exec.Command(nome.Value, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	codigo := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			codigo = ee.ExitCode() // rodou mas saiu != 0: e dado, nao erro
		} else {
			return erroBuiltinKind(KindIO, "roda_comando %q: %v", nome.Value, err)
		}
	}

	pares := map[object.HashKey]object.ParDic{}
	set := func(chave string, valor object.Object) {
		k := &object.Texto{Value: chave}
		pares[k.ChaveHash()] = object.ParDic{Chave: k, Valor: valor}
	}
	set("saida", &object.Texto{Value: stdout.String()})
	set("erro", &object.Texto{Value: stderr.String()})
	set("codigo", object.NumInt(int64(codigo)))
	return &object.Dicionario{Pares: pares}
}

// builtinSai encerra o script com um codigo de saida (default 0). Devolve o
// objeto de controle *Sair, que desenrola blocos/loops/funcoes ate o runner.
func builtinSai(args []object.Object) object.Object {
	if len(args) > 1 {
		return erroBuiltin("sai() quer 0 ou 1 arg (codigo), veio %d", len(args))
	}
	codigo := 0
	if len(args) == 1 {
		n, ok := args[0].(*object.Numero)
		if !ok {
			return erroBuiltin("sai() espera numero (codigo), veio %s", args[0].Type())
		}
		codigo = int(n.Value)
	}
	return &object.Sair{Codigo: codigo}
}
