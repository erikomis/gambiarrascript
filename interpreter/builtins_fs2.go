package interpreter

import (
	"os"
	"path/filepath"

	"gambiarrascript/object"
)

// builtinCopia copia o arquivo `de` pra `pra` (preserva o modo da origem).
func builtinCopia(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("copia() quer 2 args (de, pra), veio %d", len(args))
	}
	de, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("copia: 1o arg (de) tem que ser texto, veio %s", args[0].Type())
	}
	pra, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("copia: 2o arg (pra) tem que ser texto, veio %s", args[1].Type())
	}
	dados, err := os.ReadFile(de.Value)
	if err != nil {
		return erroBuiltinKind(KindIO, "copia %q: %v", de.Value, err)
	}
	modo := os.FileMode(0o644)
	if info, err := os.Stat(de.Value); err == nil {
		modo = info.Mode()
	}
	if err := os.WriteFile(pra.Value, dados, modo); err != nil {
		return erroBuiltinKind(KindIO, "copia pra %q: %v", pra.Value, err)
	}
	return NADA
}

// builtinMove renomeia/move `de` pra `pra`.
func builtinMove(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("move() quer 2 args (de, pra), veio %d", len(args))
	}
	de, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("move: 1o arg (de) tem que ser texto, veio %s", args[0].Type())
	}
	pra, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("move: 2o arg (pra) tem que ser texto, veio %s", args[1].Type())
	}
	if err := os.Rename(de.Value, pra.Value); err != nil {
		return erroBuiltinKind(KindIO, "move %q -> %q: %v", de.Value, pra.Value, err)
	}
	return NADA
}

// builtinTamanhoArquivo devolve o tamanho do arquivo em bytes.
func builtinTamanhoArquivo(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("tamanho_arquivo() quer 1 arg (caminho), veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("tamanho_arquivo: texto esperado, veio %s", args[0].Type())
	}
	info, err := os.Stat(c.Value)
	if err != nil {
		return erroBuiltinKind(KindIO, "tamanho_arquivo %q: %v", c.Value, err)
	}
	return object.NumInt(info.Size())
}

// builtinModificadoEm devolve a hora da ultima modificacao em unix-segundos
// (mesma unidade de agora_num — da pra passar direto pro formata_tempo).
func builtinModificadoEm(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("modificado_em() quer 1 arg (caminho), veio %d", len(args))
	}
	c, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("modificado_em: texto esperado, veio %s", args[0].Type())
	}
	info, err := os.Stat(c.Value)
	if err != nil {
		return erroBuiltinKind(KindIO, "modificado_em %q: %v", c.Value, err)
	}
	return object.NumInt(info.ModTime().Unix())
}

// builtinGlob devolve a lista de caminhos que casam com o padrao (estilo shell:
// *, ?, [...]). Sem match = lista vazia; padrao malformado = erro.
func builtinGlob(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("glob() quer 1 arg (padrao), veio %d", len(args))
	}
	p, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("glob: texto esperado, veio %s", args[0].Type())
	}
	matches, err := filepath.Glob(p.Value)
	if err != nil {
		return erroBuiltin("glob: padrao invalido %q: %v", p.Value, err)
	}
	elems := make([]object.Object, len(matches))
	for i, m := range matches {
		elems[i] = &object.Texto{Value: m}
	}
	return &object.Lista{Elements: elems}
}
