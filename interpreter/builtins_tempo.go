package interpreter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gambiarrascript/object"
)

// Lib padrão — tempo/datetime.
//
//	agora()                          → texto ISO 8601 (RFC3339) com timestamp atual
//	agora_num()                      → numero (Unix epoch em segundos, int)
//	agora_ns()                        → numero (Unix epoch em nanossegundos, int)
//	formata_tempo(formato, isoOuTs)   → texto formatado aceitando layout Go
//	                                  (ex: "2006-01-02 15:04:05")
//	parse_tempo(formato, texto)       → texto ISO 8601 do instante parseado
//	duracao(isoOuDicionario)          → numero (nanos) entre dois instantes,
//	                                    ou converte campos {h, m, s, ms} p/ ns
//	espera_ms(ms)                     → bloqueia por ms milissegundos (nada)
//
// Layout Go referencia: '2006-01-02 15:04:05' (consulte pkg time).

func builtinAgora(args []object.Object) object.Object {
	if len(args) != 0 {
		return erroBuiltin("agora() nao quer argumentos, veio %d", len(args))
	}
	return &object.Texto{Value: time.Now().UTC().Format(time.RFC3339)}
}

func builtinAgoraNum(args []object.Object) object.Object {
	if len(args) != 0 {
		return erroBuiltin("agora_num() nao quer argumentos, veio %d", len(args))
	}
	t := time.Now().UTC()
	return object.NumInt(t.Unix())
}

func builtinAgoraNs(args []object.Object) object.Object {
	if len(args) != 0 {
		return erroBuiltin("agora_ns() nao quer argumentos, veio %d", len(args))
	}
	t := time.Now().UTC()
	return object.NumInt(t.UnixNano())
}

func builtinFormataTempo(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("formata_tempo() quer 2 args (formato, iso/ts), veio %d", len(args))
	}
	layout, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("formata_tempo: formato esperado como texto, veio %s", args[0].Type())
	}
	t, e := objParaTempo(args[1])
	if e != nil {
		return e
	}
	return &object.Texto{Value: t.Format(layout.Value)}
}

func builtinParseTempo(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("parse_tempo() quer 2 args (formato, texto), veio %d", len(args))
	}
	layout, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("parse_tempo: formato esperado como texto, veio %s", args[0].Type())
	}
	t, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("parse_tempo: texto esperado, veio %s", args[1].Type())
	}
	parsed, err := time.Parse(layout.Value, t.Value)
	if err != nil {
		return erroBuiltin("parse_tempo falhou: %v", err)
	}
	return &object.Texto{Value: parsed.UTC().Format(time.RFC3339)}
}

func builtinDuracao(args []object.Object) object.Object {
	if len(args) != 1 && len(args) != 2 {
		return erroBuiltin("duracao() quer 1 (dicionario) ou 2 (inst1, inst2) args, veio %d", len(args))
	}
	if len(args) == 1 {
		// dicionario {h, m, s, ms, ns}
		d, ok := args[0].(*object.Dicionario)
		if !ok {
			return erroBuiltin("duracao(dicionario) espera um dicionario, veio %s", args[0].Type())
		}
		var total time.Duration
		total += tempoCampo(d, "h", time.Hour)
		total += tempoCampo(d, "m", time.Minute)
		total += tempoCampo(d, "s", time.Second)
		total += tempoCampo(d, "ms", time.Millisecond)
		total += tempoCampo(d, "us", time.Microsecond)
		total += tempoCampo(d, "ns", time.Nanosecond)
		return object.NumInt(int64(total))
	}
	t1, e := objParaTempo(args[0])
	if e != nil {
		return e
	}
	t2, e := objParaTempo(args[1])
	if e != nil {
		return e
	}
	return object.NumInt(int64(t2.Sub(t1)))
}

func tempoCampo(d *object.Dicionario, nome string, unidade time.Duration) time.Duration {
	chave := &object.Texto{Value: nome}
	par, existe := d.Pares[chave.ChaveHash()]
	if !existe {
		return 0
	}
	num, ok := par.Valor.(*object.Numero)
	if !ok {
		return 0
	}
	if num.EhInt {
		return time.Duration(num.Int) * unidade
	}
	return time.Duration(num.Value * float64(unidade))
}

func builtinEsperaMs(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("espera_ms() quer 1 (ms), veio %d", len(args))
	}
	n, ok := args[0].(*object.Numero)
	if !ok {
		return erroBuiltin("espera_ms: numero (ms), veio %s", args[0].Type())
	}
	ms := n.Value
	time.Sleep(time.Duration(ms * float64(time.Millisecond)))
	return NADA
}

// objParaTempo aceita: *Texto (ISO 8601 RFC3339) ou *Numero (Unix seconds / ns).
func objParaTempo(o object.Object) (time.Time, *object.Erro) {
	switch v := o.(type) {
	case *object.Texto:
		if strings.Contains(v.Value, "T") || strings.ContainsAny(v.Value, "-:/") {
			t, err := time.Parse(time.RFC3339, v.Value)
			if err != nil {
				// tenta layouts comuns UTC sem tz
				layouts := []string{"2006-01-02 15:04:05", "2006-01-02", "15:04:05"}
				for _, l := range layouts {
					if t2, err := time.Parse(l, v.Value); err == nil {
						return t2, nil
					}
				}
				return time.Time{}, erroBuiltin("tempo invalido: %v", err)
			}
			return t, nil
		}
		return time.Time{}, erroBuiltin("tempo texto sem formato reconhecido: %q", v.Value)
	case *object.Numero:
		if v.EhInt {
			return time.Unix(v.Int, 0).UTC(), nil
		}
		return time.Unix(int64(v.Value), 0).UTC(), nil
	}
	return time.Time{}, erroBuiltin("tempo esperado como texto ou numero, veio %s", o.Type())
}

// usado pra evitar import "fmt" sem uso se o build for incremental.
var _ = fmt.Sprintf
var _ = strconv.Atoi
