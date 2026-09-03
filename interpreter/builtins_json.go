package interpreter

import (
	"bytes"
	"encoding/json"
	"strconv"

	"gambiarrascript/object"
)

func builtinDeJson(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("de_json() quer 1 argumento (texto), veio %d", len(args))
	}
	t, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("de_json() espera texto, veio %s", args[0].Type())
	}
	// parser proprio (json_parser.go): preserva a ordem das chaves do documento
	// — coisa que json.Unmarshal num map perde — e sem o custo do json.Decoder
	// por token, que ficou 66% mais lento que o Unmarshal.
	v, err := parseJson(t.Value)
	if err != nil {
		return erroBuiltin("esse json ta quebrado, parca: %v", err)
	}
	return v
}

func builtinPraJson(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("pra_json() quer 1 argumento, veio %d", len(args))
	}
	var buf bytes.Buffer
	if erro := escreveJson(&buf, args[0]); erro != nil {
		return erro
	}
	return &object.Texto{Value: buf.String()}
}

// escreveJson serializa o valor direto no buffer. E escrito na mao (em vez de
// montar um map[string]interface{} e chamar json.Marshal) porque o Marshal
// ordena as chaves de map alfabeticamente — o dicionario tem que sair na ordem
// de insercao, igual JSON.stringify do JS e json.dumps do Python.
func escreveJson(buf *bytes.Buffer, o object.Object) *object.Erro {
	switch val := o.(type) {
	case *object.Nada:
		buf.WriteString("null")
	case *object.Booleano:
		if val.Value {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case *object.Numero:
		// inteiro exato sai como inteiro: float64 perde precisao acima de 2^53,
		// e de_json ja devolve inteiro exato pra numero sem ponto.
		if val.EhInt {
			buf.WriteString(strconv.FormatInt(val.Int, 10))
			break
		}
		bs, err := json.Marshal(val.Value)
		if err != nil {
			return erroBuiltin("nao consegui virar json: %v", err)
		}
		buf.Write(bs)
	case *object.Texto:
		escreveTextoJson(buf, val.Value)
	case *object.Lista:
		buf.WriteByte('[')
		for i, e := range val.Elements {
			if i > 0 {
				buf.WriteByte(',')
			}
			if erro := escreveJson(buf, e); erro != nil {
				return erro
			}
		}
		buf.WriteByte(']')
	case *object.Dicionario:
		buf.WriteByte('{')
		for i, k := range val.Chaves() {
			par := val.Pares[k]
			if i > 0 {
				buf.WriteByte(',')
			}
			escreveTextoJson(buf, chaveJson(par.Chave))
			buf.WriteByte(':')
			if erro := escreveJson(buf, par.Valor); erro != nil {
				return erro
			}
		}
		buf.WriteByte('}')
	default:
		return erroBuiltin("nao da pra virar json: %s", o.Type())
	}
	return nil
}

// escreveTextoJson escreve uma string ja escapada (aspas incluidas), usando o
// mesmo escaping do encoding/json.
func escreveTextoJson(buf *bytes.Buffer, s string) {
	bs, err := json.Marshal(s)
	if err != nil { // string sempre serializa; fallback defensivo
		buf.WriteString(`""`)
		return
	}
	buf.Write(bs)
}

// chaveJson devolve a forma textual de uma chave de dicionario (JSON exige string).
func chaveJson(o object.Object) string {
	switch k := o.(type) {
	case *object.Texto:
		return k.Value
	case *object.Numero:
		return object.FormatNumero(k.Value)
	case *object.Booleano:
		return k.Inspect()
	}
	return o.Inspect()
}
