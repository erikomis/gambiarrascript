package interpreter

import (
	"fmt"
	"time"

	"gambiarrascript/object"
)

// Lib padrão — datas parte 2.
//
//	soma_tempo(instante, nanos)           → novo instante (ISO 8601) com a duracao somada
//	sub_tempo(instante, nanos)            → novo instante (ISO 8601) com a duracao subtraida
//	dia_da_semana(instante)               → texto (segunda, terca, quarta, ... domingo)
//	diferenca_dias(inst1, inst2)          → numero (dias entre os instantes, pode ser negativo)
//	diferenca_horas(inst1, inst2)         → numero (horas entre os instantes, pode ser negativo)
//	converte_tz(instante, timezone)       → texto ISO 8601 no timezone informado (ex: "America/Sao_Paulo")

// builtinSomaTempo soma uma duracao (em nanossegundos, como devolvido por
// duracao()) a um instante e devolve o novo instante em ISO 8601.
func builtinSomaTempo(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("soma_tempo() quer 2 args (instante, nanos), veio %d", len(args))
	}
	t, e := objParaTempo(args[0])
	if e != nil {
		return e
	}
	d, ok := args[1].(*object.Numero)
	if !ok {
		return erroBuiltin("soma_tempo: 2o arg (duracao em ns) tem que ser numero, veio %s", args[1].Type())
	}
	var dur time.Duration
	if d.EhInt {
		dur = time.Duration(d.Int)
	} else {
		dur = time.Duration(d.Value)
	}
	return &object.Texto{Value: t.Add(dur).Format(time.RFC3339)}
}

// builtinSubTempo subtrai uma duracao (em nanossegundos) de um instante e
// devolve o novo instante em ISO 8601.
func builtinSubTempo(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("sub_tempo() quer 2 args (instante, nanos), veio %d", len(args))
	}
	t, e := objParaTempo(args[0])
	if e != nil {
		return e
	}
	d, ok := args[1].(*object.Numero)
	if !ok {
		return erroBuiltin("sub_tempo: 2o arg (duracao em ns) tem que ser numero, veio %s", args[1].Type())
	}
	var dur time.Duration
	if d.EhInt {
		dur = time.Duration(d.Int)
	} else {
		dur = time.Duration(d.Value)
	}
	return &object.Texto{Value: t.Add(-dur).Format(time.RFC3339)}
}

// builtinDiaDaSemana devolve o nome do dia da semana em portugues.
func builtinDiaDaSemana(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("dia_da_semana() quer 1 arg (instante), veio %d", len(args))
	}
	t, e := objParaTempo(args[0])
	if e != nil {
		return e
	}
	dias := []string{
		"domingo", "segunda", "terca", "quarta",
		"quinta", "sexta", "sabado",
	}
	return &object.Texto{Value: dias[t.Weekday()]}
}

// builtinDiferencaDias devolve o numero de dias (inteiro) entre dois
// instantes. Pode ser negativo se inst2 for anterior a inst1.
func builtinDiferencaDias(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("diferenca_dias() quer 2 args (inst1, inst2), veio %d", len(args))
	}
	t1, e := objParaTempo(args[0])
	if e != nil {
		return e
	}
	t2, e := objParaTempo(args[1])
	if e != nil {
		return e
	}
	diferenca := t2.Sub(t1).Hours() / 24
	return object.NumInt(int64(diferenca))
}

// builtinDiferencaHoras devolve o numero de horas entre dois instantes.
// Pode ser negativo se inst2 for anterior a inst1.
func builtinDiferencaHoras(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("diferenca_horas() quer 2 args (inst1, inst2), veio %d", len(args))
	}
	t1, e := objParaTempo(args[0])
	if e != nil {
		return e
	}
	t2, e := objParaTempo(args[1])
	if e != nil {
		return e
	}
	return object.NumInt(int64(t2.Sub(t1).Hours()))
}

// builtinConverteTZ converte um instante para o timezone informado (ex:
// "America/Sao_Paulo", "Europe/London", "UTC"). Devolve ISO 8601 COM offset
// do timezone (nao mais Z de UTC).
func builtinConverteTZ(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("converte_tz() quer 2 args (instante, timezone), veio %d", len(args))
	}
	t, e := objParaTempo(args[0])
	if e != nil {
		return e
	}
	tzNome, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("converte_tz: 2o arg (timezone) tem que ser texto, veio %s", args[1].Type())
	}
	loc, err := time.LoadLocation(tzNome.Value)
	if err != nil {
		return erroBuiltin("converte_tz: timezone %q nao encontrado: %v", tzNome.Value, err)
	}
	return &object.Texto{Value: t.In(loc).Format(time.RFC3339)}
}

// usado pra evitar import "fmt" sem uso se o build for incremental.
var _ = fmt.Sprintf