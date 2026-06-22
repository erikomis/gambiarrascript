package interpreter

import (
	"encoding/json"

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
	var v interface{}
	if err := json.Unmarshal([]byte(t.Value), &v); err != nil {
		return erroBuiltin("esse json ta quebrado, parca: %v", err)
	}
	return deGo(v)
}

// deGo converte a arvore interface{} do encoding/json em valores GambiarraScript.
func deGo(v interface{}) object.Object {
	switch val := v.(type) {
	case nil:
		return NADA
	case bool:
		return boolDoNativo(val)
	case float64:
		return &object.Numero{Value: val}
	case string:
		return &object.Texto{Value: val}
	case []interface{}:
		elems := make([]object.Object, len(val))
		for i, e := range val {
			elems[i] = deGo(e)
		}
		return &object.Lista{Elements: elems}
	case map[string]interface{}:
		pares := map[object.HashKey]object.ParDic{}
		for k, e := range val {
			chave := &object.Texto{Value: k}
			pares[chave.ChaveHash()] = object.ParDic{Chave: chave, Valor: deGo(e)}
		}
		return &object.Dicionario{Pares: pares}
	}
	return NADA
}

func builtinPraJson(args []object.Object) object.Object {
	if len(args) != 1 {
		return erroBuiltin("pra_json() quer 1 argumento, veio %d", len(args))
	}
	v, erro := paraGo(args[0])
	if erro != nil {
		return erro
	}
	bs, err := json.Marshal(v)
	if err != nil {
		return erroBuiltin("nao consegui virar json: %v", err)
	}
	return &object.Texto{Value: string(bs)}
}

// paraGo converte um valor GambiarraScript numa arvore interface{} serializavel;
// devolve um *object.Erro se algo nao for serializavel.
func paraGo(o object.Object) (interface{}, *object.Erro) {
	switch val := o.(type) {
	case *object.Nada:
		return nil, nil
	case *object.Booleano:
		return val.Value, nil
	case *object.Numero:
		return val.Value, nil
	case *object.Texto:
		return val.Value, nil
	case *object.Lista:
		arr := make([]interface{}, len(val.Elements))
		for i, e := range val.Elements {
			conv, erro := paraGo(e)
			if erro != nil {
				return nil, erro
			}
			arr[i] = conv
		}
		return arr, nil
	case *object.Dicionario:
		obj := map[string]interface{}{}
		for _, par := range val.Pares {
			conv, erro := paraGo(par.Valor)
			if erro != nil {
				return nil, erro
			}
			obj[chaveJson(par.Chave)] = conv
		}
		return obj, nil
	default:
		return nil, erroBuiltin("nao da pra virar json: %s", o.Type())
	}
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
