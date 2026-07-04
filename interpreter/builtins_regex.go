package interpreter

import (
	"regexp"
	"strings"

	"gambiarrascript/object"
)

// Lib padrão — regex.
//
//	busca_regex(padrao, texto)             → deu_bom/deu_ruim (tem match?)
//	combina_regex(padrao, texto)           → lista de matches (cada um uma
//	                                        lista com grupos [full, g1, g2, ...])
//	acha_regex(padrao, texto)              → texto do primeiro match ou nada
//	substitui_regex(padrao, repl, texto, [n]) → texto com `n` substituições
//	                                        (default todo). repl suporta $1, $2.
//	separa_regex(padrao, texto, [n])      → lista de pedacos (split com regex)

func builtinBuscaRegex(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("busca_regex() quer 2 args (padrao, texto), veio %d", len(args))
	}
	re, e := compilaRegex(args[0])
	if e != nil {
		return e
	}
	t, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("busca_regex: texto esperado, veio %s", args[1].Type())
	}
	return boolDoNativo(re.MatchString(t.Value))
}

func builtinAchaRegex(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("acha_regex() quer 2 args (padrao, texto), veio %d", len(args))
	}
	re, e := compilaRegex(args[0])
	if e != nil {
		return e
	}
	t, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("acha_regex: texto esperado, veio %s", args[1].Type())
	}
	m := re.FindString(t.Value)
	if m == "" && !re.MatchString(t.Value) {
		return NADA
	}
	return &object.Texto{Value: m}
}

func builtinCombinaRegex(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("combina_regex() quer 2 args (padrao, texto), veio %d", len(args))
	}
	re, e := compilaRegex(args[0])
	if e != nil {
		return e
	}
	t, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("combina_regex: texto esperado, veio %s", args[1].Type())
	}
	matches := re.FindAllStringSubmatch(t.Value, -1)
	out := make([]object.Object, 0, len(matches))
	for _, m := range matches {
		grp := make([]object.Object, 0, len(m))
		for _, g := range m {
			grp = append(grp, &object.Texto{Value: g})
		}
		out = append(out, &object.Lista{Elements: grp})
	}
	return &object.Lista{Elements: out}
}

func builtinSubstituiRegex(args []object.Object) object.Object {
	if len(args) < 3 || len(args) > 4 {
		return erroBuiltin("substitui_regex() quer 3 ou 4 args (padrao, repl, texto, [n]), veio %d", len(args))
	}
	re, e := compilaRegex(args[0])
	if e != nil {
		return e
	}
	repl, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("substitui_regex: repl esperado como texto, veio %s", args[1].Type())
	}
	t, ok := args[2].(*object.Texto)
	if !ok {
		return erroBuiltin("substitui_regex: texto esperado, veio %s", args[2].Type())
	}
	n := -1
	if len(args) == 4 {
		if nn, ok := args[3].(*object.Numero); ok {
			n = int(nn.Value)
		}
	}
	// Go's re.ReplaceAllString expande $1..$9 (e ${name}) nativo em repl.
	out := re.ReplaceAllString(t.Value, repl.Value)
	if n >= 0 {
		// limit manual: aplicamos ate N substituicoes recompilando? Simples:
		// re.ReplaceAllString nao tem limite; precisamos respeitar n. Walgo se
		// substituicao simples: aplicamos na primeira ocorrencia ate n vezes.
		out = limitSubst(re, t.Value, repl.Value, n)
	}
	return &object.Texto{Value: out}
}

// limitSubst aplica `re.ReplaceAllString` ate `n` substituicoes a partir do
// inicio. Mantem o resto do texto inalterado.
func limitSubst(re *regexp.Regexp, texto, repl string, n int) string {
	if n <= 0 {
		return texto
	}
	var b strings.Builder
	idx := 0
	for count := 0; count < n; {
		loc := re.FindStringIndex(texto[idx:])
		if loc == nil {
			break
		}
		start := idx + loc[0]
		end := idx + loc[1]
		match := texto[start:end]
		b.WriteString(texto[idx:start])
		b.WriteString(re.ReplaceAllString(match, repl))
		idx = end
		count++
	}
	b.WriteString(texto[idx:])
	return b.String()
}

func builtinSeparaRegex(args []object.Object) object.Object {
	if len(args) < 2 || len(args) > 3 {
		return erroBuiltin("separa_regex() quer 2 ou 3 args (padrao, texto, [n]), veio %d", len(args))
	}
	re, e := compilaRegex(args[0])
	if e != nil {
		return e
	}
	t, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("separa_regex: texto esperado, veio %s", args[1].Type())
	}
	n := -1
	if len(args) == 3 {
		if nn, ok := args[2].(*object.Numero); ok {
			n = int(nn.Value)
		}
	}
	parts := re.Split(t.Value, n)
	out := make([]object.Object, 0, len(parts))
	for _, p := range parts {
		out = append(out, &object.Texto{Value: p})
	}
	return &object.Lista{Elements: out}
}

// compilaRegex retorna *regexp.Regexp compilado do argumento (Texto).
func compilaRegex(o object.Object) (*regexp.Regexp, *object.Erro) {
	t, ok := o.(*object.Texto)
	if !ok {
		return nil, erroBuiltin("padrao regex esperado como texto, veio %s", o.Type())
	}
	re, err := regexp.Compile(t.Value)
	if err != nil {
		return nil, erroBuiltin("regex invalido: %v", err)
	}
	return re, nil
}
