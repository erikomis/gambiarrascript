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
		if erro.Message == "deu ruim: "+msgCicloJson {
			return erroBuiltin("pra_json(): %s", msgCicloJson)
		}
		return erro
	}
	return &object.Texto{Value: buf.String()}
}

// msgCicloJson: lista/dicionario que contem ele mesmo (`adiciona(xs, xs)`) nao
// tem JSON — descer nele estourava a pilha do Go e derrubava o processo.
const msgCicloJson = "estrutura que contem ela mesma nao vira JSON, parca"

// escreveJson serializa o valor direto no buffer. E escrito na mao (em vez de
// montar um map[string]interface{} e chamar json.Marshal) porque o Marshal
// ordena as chaves de map alfabeticamente — o dicionario tem que sair na ordem
// de insercao, igual JSON.stringify do JS e json.dumps do Python.
func escreveJson(buf *bytes.Buffer, o object.Object) *object.Erro {
	return escreveJsonRec(buf, o, nil)
}

// escreveJsonRec carrega emCurso: as listas/dicionarios abertos no caminho
// ate aqui (por ponteiro). Achar de novo um deles e ciclo; a mesma lista em
// dois lugares lado a lado (sem ciclo) sai normal nos dois. O map so nasce
// quando aparece colecao dentro de colecao.
func escreveJsonRec(buf *bytes.Buffer, o object.Object, emCurso map[object.Object]bool) *object.Erro {
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
		if emCurso[val] {
			return erroBuiltin("%s", msgCicloJson)
		}
		buf.WriteByte('[')
		for i, e := range val.Visao() {
			if i > 0 {
				buf.WriteByte(',')
			}
			if ehColecaoJson(e) {
				emCurso = marcaEmCurso(emCurso, val)
			}
			if erro := escreveJsonRec(buf, e, emCurso); erro != nil {
				return erro
			}
		}
		delete(emCurso, val)
		buf.WriteByte(']')
	case *object.Dicionario:
		if emCurso[val] {
			return erroBuiltin("%s", msgCicloJson)
		}
		buf.WriteByte('{')
		var erro *object.Erro
		primeiro := true
		val.Itera(func(par object.ParDic) {
			if erro != nil {
				return
			}
			if !primeiro {
				buf.WriteByte(',')
			}
			primeiro = false
			escreveTextoJson(buf, chaveJson(par.Chave))
			buf.WriteByte(':')
			if ehColecaoJson(par.Valor) {
				emCurso = marcaEmCurso(emCurso, val)
			}
			erro = escreveJsonRec(buf, par.Valor, emCurso)
		})
		if erro != nil {
			return erro
		}
		delete(emCurso, val)
		buf.WriteByte('}')
	case *object.Instancia:
		// treta vira objeto com os campos (puxadinho achatado, igual Go)
		if emCurso[val] {
			return erroBuiltin("%s", msgCicloJson)
		}
		buf.WriteByte('{')
		for k, par := range val.CamposJson() {
			if k > 0 {
				buf.WriteByte(',')
			}
			escreveTextoJson(buf, par.Nome)
			buf.WriteByte(':')
			if ehColecaoJson(par.Valor) {
				emCurso = marcaEmCurso(emCurso, val)
			}
			if erro := escreveJsonRec(buf, par.Valor, emCurso); erro != nil {
				return erro
			}
		}
		delete(emCurso, val)
		buf.WriteByte('}')
	case *object.Opcao:
		// opcao de cardapio vira o nome dela ("vermelho")
		escreveTextoJson(buf, val.Nome)
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
	case *object.Opcao:
		return k.Nome
	}
	return o.Inspect()
}

func ehColecaoJson(o object.Object) bool {
	switch o.(type) {
	case *object.Lista, *object.Dicionario, *object.Instancia:
		return true
	}
	return false
}

// marcaEmCurso poe a colecao no caminho (criando o map se ainda nao tinha).
func marcaEmCurso(emCurso map[object.Object]bool, c object.Object) map[object.Object]bool {
	if emCurso == nil {
		emCurso = map[object.Object]bool{}
	}
	emCurso[c] = true
	return emCurso
}
