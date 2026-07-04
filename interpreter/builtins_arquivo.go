package interpreter

import (
	"os"
	"path/filepath"
	"sort"

	"gambiarrascript/object"
)

func builtinLeArquivo(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("le_arquivo() quer 1 argumento (caminho), veio %d", len(args))
	}
	caminho, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("le_arquivo() espera texto (caminho), veio %s", args[0].Type())
	}
	bs, err := os.ReadFile(caminho.Value)
	if err != nil {
		return erroBuiltinKind(KindIO, "nao consegui ler %q: %v", caminho.Value, err)
	}
	return &object.Texto{Value: string(bs)}
}

func builtinEscreveArquivo(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("escreve_arquivo() quer 2 argumentos (caminho, texto), veio %d", len(args))
	}
	caminho, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("escreve_arquivo() espera texto (caminho), veio %s", args[0].Type())
	}
	conteudo, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("escreve_arquivo() espera texto (conteudo), veio %s", args[1].Type())
	}
	if err := os.WriteFile(caminho.Value, []byte(conteudo.Value), 0644); err != nil {
		return erroBuiltinKind(KindIO, "nao consegui escrever em %q: %v", caminho.Value, err)
	}
	return NADA
}

// builtinAnexaArquivo acrescenta texto ao final do arquivo (cria se nao existir).
// Distinto de escreve_arquivo: nao sobrescreve. Pré: fluxo pra logs, appending
// incremental em scripts de CLI processando pipes grandes.
func builtinAnexaArquivo(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("anexa_arquivo() quer 2 argumentos (caminho, texto), veio %d", len(args))
	}
	caminho, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("anexa_arquivo() espera texto (caminho), veio %s", args[0].Type())
	}
	conteudo, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("anexa_arquivo() espera texto (conteudo), veio %s", args[1].Type())
	}
	f, err := os.OpenFile(caminho.Value, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return erroBuiltinKind(KindIO, "nao consegui abrir %q pra anexar: %v", caminho.Value, err)
	}
	defer f.Close()
	if _, err := f.WriteString(conteudo.Value); err != nil {
		return erroBuiltinKind(KindIO, "nao consegui anexar em %q: %v", caminho.Value, err)
	}
	return NADA
}

// builtinExiste devolve deu_bom se o caminho existe (stat ok), deu_ruim caso
// contrario. Erros de permissao viram deu_ruim tambem — pragmatico pra scripts
// que so querem saber "posso abrir?".
func builtinExiste(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("existe() quer 1 arg (caminho), veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("existe: texto esperado, veio %s", args[0].Type())
	}
	_, err := os.Stat(c.Value)
	return boolDoNativo(err == nil)
}

// builtinEhDir devolve deu_bom se o caminho existe e e diretorio.
func builtinEhDir(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("eh_dir() quer 1 arg (caminho), veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("eh_dir: texto esperado, veio %s", args[0].Type())
	}
	info, err := os.Stat(c.Value)
	if err != nil {
		return DEU_RUIM
	}
	return boolDoNativo(info.IsDir())
}

// builtinDeleta apaga arquivo OU diretorio (recursivo). Idempotente: nao
// existe nao vira erro. Permissao/outra falha vira *Erro.
func builtinDeleta(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("deleta() quer 1 arg (caminho), veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("deleta: texto esperado, veio %s", args[0].Type())
	}
	if err := os.RemoveAll(c.Value); err != nil {
		return erroBuiltinKind(KindIO, "deleta %q: %v", c.Value, err)
	}
	return NADA
}

// builtinCriaDir faz mkdir -p (cria todos os pais, nao mascara permissoes).
func builtinCriaDir(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("cria_dir() quer 1 arg (caminho), veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("cria_dir: texto esperado, veio %s", args[0].Type())
	}
	if err := os.MkdirAll(c.Value, 0755); err != nil {
		return erroBuiltinKind(KindIO, "cria_dir %q: %v", c.Value, err)
	}
	return NADA
}

// builtinLeDir lista o conteudo do diretorio (1 nivel — nao recursivo).
// Devolve lista de textos com os nomes (sem o caminho prefixado). Ordem
// alfabetica. Se quiser caminho completo, use caminho_junta(dir, nome).
func builtinLeDir(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("le_dir() quer 1 arg (dir), veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("le_dir: texto esperado, veio %s", args[0].Type())
	}
	entries, err := os.ReadDir(c.Value)
	if err != nil {
		return erroBuiltinKind(KindIO, "le_dir %q: %v", c.Value, err)
	}
	nomes := make([]string, 0, len(entries))
	for _, e := range entries {
		nomes = append(nomes, e.Name())
	}
	sort.Strings(nomes)
	out := make([]object.Object, 0, len(nomes))
	for _, n := range nomes {
		out = append(out, &object.Texto{Value: n})
	}
	return &object.Lista{Elements: out}
}

// builtinCaminhoJunta usa filepath.Join pra juntar N pedacos num caminho
// valido do SO. Caminho absoluto reseta.
func builtinCaminhoJunta(args []object.Object) object.Object {
	if len(args) == 0 {
		return erroBuiltin("caminho_junta() quer 1+ args, veio 0")
	}
	pedacos := make([]string, 0, len(args))
	for i, a := range args {
		t, ok := a.(*object.Texto)
		if !ok {
			return erroBuiltin("caminho_junta: arg %d espera texto, veio %s", i, a.Type())
		}
		pedacos = append(pedacos, t.Value)
	}
	return &object.Texto{Value: filepath.Join(pedacos...)}
}

// builtinCaminhoBase devolve o ultimo componente do camho (nome do arquivo/dir).
func builtinCaminhoBase(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("caminho_base() quer 1 arg, veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("caminho_base: texto esperado, veio %s", args[0].Type())
	}
	return &object.Texto{Value: filepath.Base(c.Value)}
}

// builtinCaminhoDir devolve o diretorio do caminho (sem o nome final).
func builtinCaminhoDir(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("caminho_dir() quer 1 arg, veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("caminho_dir: texto esperado, veio %s", args[0].Type())
	}
	return &object.Texto{Value: filepath.Dir(c.Value)}
}

// builtinCaminhoExt devolve a extensao incluindo o ponto (".gs" ou "" se nao tem).
func builtinCaminhoExt(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("caminho_ext() quer 1 arg, veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("caminho_ext: texto esperado, veio %s", args[0].Type())
	}
	return &object.Texto{Value: filepath.Ext(c.Value)}
}

// builtinCaminhoAbs resolve o caminho absoluto (limpa ./.. e prefixa o cwd).
func builtinCaminhoAbs(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("caminho_abs() quer 1 arg, veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("caminho_abs: texto esperado, veio %s", args[0].Type())
	}
	abs, err := filepath.Abs(c.Value)
	if err != nil {
		return erroBuiltinKind(KindIO, "caminho_abs %q: %v", c.Value, err)
	}
	return &object.Texto{Value: abs}
}
