// valida(valor, esquema) -> lista de erros (texto). Lista vazia = passou.
//
// O esquema e um dicionario campo -> regra. A regra e um nome de tipo
// ("texto", "numero", "inteiro", "booleano", "lista", "dicionario", "email",
// "data", "qualquer"; com "?" no fim = opcional) ou um dicionario com
// {"tipo", "obrigatorio", "min", "max", "padrao", "opcoes", "itens",
// "campos", "_estrito"}. Um dicionario que tem alguma chave fora dessa lista
// e atalho pra {"tipo": "dicionario", "campos": ele}.
//
// Cada erro vem com o caminho na frente: "idade: tem que ser numero, veio
// texto", "itens[2].preco: obrigatorio". Esquema mal escrito (tipo que nao
// existe, regex quebrada) nao e erro de validacao: quebra na hora.
//
// Pura (nao chama codigo do usuario): vale igual nos dois engines.

package interpreter

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"gambiarrascript/object"
)

var tiposValida = map[string]bool{
	"texto": true, "numero": true, "inteiro": true, "booleano": true,
	"lista": true, "dicionario": true, "email": true, "data": true, "qualquer": true,
}

var chavesRegra = map[string]bool{
	"tipo": true, "obrigatorio": true, "min": true, "max": true, "padrao": true,
	"opcoes": true, "itens": true, "campos": true, "_estrito": true,
}

var reEmail = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// cache das regex do "padrao": o mesmo esquema roda a cada pedido.
var (
	cacheRegexValida   = map[string]*regexp.Regexp{}
	cacheRegexValidaMu sync.Mutex
)

func regexValida(p string) (*regexp.Regexp, error) {
	cacheRegexValidaMu.Lock()
	defer cacheRegexValidaMu.Unlock()
	if re, ok := cacheRegexValida[p]; ok {
		return re, nil
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, err
	}
	if len(cacheRegexValida) < 1000 {
		cacheRegexValida[p] = re
	}
	return re, nil
}

// regraValida e uma regra ja lida do esquema.
type regraValida struct {
	tipo        string // "" = qualquer
	obrigatorio bool
	min, max    *float64
	padrao      *regexp.Regexp
	padraoTexto string
	opcoes      []object.Object
	itens       *regraValida
	campos      *esquemaValida
}

type campoValida struct {
	nome  string
	regra *regraValida
}

type esquemaValida struct {
	campos  []campoValida
	estrito bool
}

// erroEsquema e esquema mal escrito (vira erro de verdade, nao item da lista).
type erroEsquema struct{ msg string }

func (e *erroEsquema) Error() string { return e.msg }

func esqErr(caminho, formato string, args ...interface{}) *erroEsquema {
	onde := ""
	if caminho != "" {
		onde = " em " + strconv.Quote(caminho)
	}
	return &erroEsquema{msg: fmt.Sprintf(formato, args...) + onde}
}

func builtinValida(args []object.Object) object.Object {
	if len(args) != 2 {
		return erroBuiltin("valida() quer 2 argumentos (valor, esquema), veio %d", len(args))
	}
	var regra *regraValida
	var err *erroEsquema
	switch esq := args[1].(type) {
	case *object.Dicionario:
		var campos *esquemaValida
		campos, err = lerCamposValida(esq, "")
		regra = &regraValida{tipo: "dicionario", obrigatorio: true, campos: campos}
	case *object.Texto:
		regra, err = lerRegraValida(esq, "")
	default:
		return erroBuiltin("valida(): o esquema tem que ser dicionario (campo -> regra) ou texto (nome do tipo), veio %s", object.NomeTipo(args[1]))
	}
	if err != nil {
		return erroBuiltin("valida(): %s", err.msg)
	}
	var erros []object.Object
	aplicaRegraValida(regra, args[0], "", &erros)
	if erros == nil {
		erros = []object.Object{}
	}
	return object.NovaLista(erros)
}

func lerCamposValida(d *object.Dicionario, caminho string) (*esquemaValida, *erroEsquema) {
	esq := &esquemaValida{}
	for _, par := range d.Pares() {
		k, ok := par.Chave.(*object.Texto)
		if !ok {
			return nil, esqErr(caminho, "nome de campo tem que ser texto, veio %s", object.NomeTipo(par.Chave))
		}
		if k.Value == "_estrito" {
			b, ok := par.Valor.(*object.Booleano)
			if !ok {
				return nil, esqErr(caminho, "_estrito tem que ser booleano")
			}
			esq.estrito = b.Value
			continue
		}
		r, err := lerRegraValida(par.Valor, juntaCaminho(caminho, k.Value))
		if err != nil {
			return nil, err
		}
		esq.campos = append(esq.campos, campoValida{nome: k.Value, regra: r})
	}
	return esq, nil
}

func ehDicRegra(d *object.Dicionario) bool {
	for _, par := range d.Pares() {
		k, ok := par.Chave.(*object.Texto)
		if !ok || !chavesRegra[k.Value] {
			return false
		}
	}
	return true
}

func lerTipoValida(nome, caminho string) (string, bool, *erroEsquema) {
	opcional := strings.HasSuffix(nome, "?")
	nome = strings.TrimSuffix(nome, "?")
	if !tiposValida[nome] {
		return "", false, esqErr(caminho, "tipo desconhecido %q (use texto, numero, inteiro, booleano, lista, dicionario, email, data ou qualquer; ? no fim = opcional)", nome)
	}
	if nome == "qualquer" {
		nome = ""
	}
	return nome, opcional, nil
}

func lerRegraValida(o object.Object, caminho string) (*regraValida, *erroEsquema) {
	switch v := o.(type) {
	case *object.Texto:
		tipo, opcional, err := lerTipoValida(v.Value, caminho)
		if err != nil {
			return nil, err
		}
		return &regraValida{tipo: tipo, obrigatorio: !opcional}, nil
	case *object.Dicionario:
		if !ehDicRegra(v) {
			campos, err := lerCamposValida(v, caminho)
			if err != nil {
				return nil, err
			}
			return &regraValida{tipo: "dicionario", obrigatorio: true, campos: campos}, nil
		}
		return lerDicRegraValida(v, caminho)
	}
	return nil, esqErr(caminho, "regra tem que ser texto (nome do tipo) ou dicionario, veio %s", object.NomeTipo(o))
}

func lerDicRegraValida(d *object.Dicionario, caminho string) (*regraValida, *erroEsquema) {
	r := &regraValida{obrigatorio: true}
	opcionalPeloTipo := false
	if t, ok := d.PegaTexto("tipo"); ok {
		tt, ok := t.(*object.Texto)
		if !ok {
			return nil, esqErr(caminho, "\"tipo\" tem que ser texto")
		}
		var err *erroEsquema
		r.tipo, opcionalPeloTipo, err = lerTipoValida(tt.Value, caminho)
		if err != nil {
			return nil, err
		}
		r.obrigatorio = !opcionalPeloTipo
	}
	if o, ok := d.PegaTexto("obrigatorio"); ok {
		b, ok := o.(*object.Booleano)
		if !ok {
			return nil, esqErr(caminho, "\"obrigatorio\" tem que ser booleano")
		}
		r.obrigatorio = b.Value
	}
	for _, lim := range []struct {
		nome string
		dst  **float64
	}{{"min", &r.min}, {"max", &r.max}} {
		if o, ok := d.PegaTexto(lim.nome); ok {
			n, ok := o.(*object.Numero)
			if !ok {
				return nil, esqErr(caminho, "%q tem que ser numero", lim.nome)
			}
			v := n.Value
			*lim.dst = &v
		}
	}
	if o, ok := d.PegaTexto("padrao"); ok {
		t, ok := o.(*object.Texto)
		if !ok {
			return nil, esqErr(caminho, "\"padrao\" tem que ser texto (regex)")
		}
		re, err := regexValida(t.Value)
		if err != nil {
			return nil, esqErr(caminho, "regex do \"padrao\" nao compila: %v", err)
		}
		r.padrao, r.padraoTexto = re, t.Value
	}
	if o, ok := d.PegaTexto("opcoes"); ok {
		l, ok := o.(*object.Lista)
		if !ok {
			return nil, esqErr(caminho, "\"opcoes\" tem que ser lista")
		}
		r.opcoes = l.Copia()
	}
	if o, ok := d.PegaTexto("itens"); ok {
		it, err := lerRegraValida(o, caminho+"[]")
		if err != nil {
			return nil, err
		}
		r.itens = it
		if r.tipo == "" {
			r.tipo = "lista"
		}
	}
	if o, ok := d.PegaTexto("campos"); ok {
		cd, ok := o.(*object.Dicionario)
		if !ok {
			return nil, esqErr(caminho, "\"campos\" tem que ser dicionario")
		}
		c, err := lerCamposValida(cd, caminho)
		if err != nil {
			return nil, err
		}
		r.campos = c
		if r.tipo == "" {
			r.tipo = "dicionario"
		}
	}
	if o, ok := d.PegaTexto("_estrito"); ok {
		b, ok := o.(*object.Booleano)
		if !ok {
			return nil, esqErr(caminho, "_estrito tem que ser booleano")
		}
		if r.campos == nil {
			r.campos = &esquemaValida{}
			if r.tipo == "" {
				r.tipo = "dicionario"
			}
		}
		r.campos.estrito = b.Value
	}
	return r, nil
}

func juntaCaminho(base, campo string) string {
	if base == "" {
		return campo
	}
	return base + "." + campo
}

func addErroValida(erros *[]object.Object, caminho, formato string, args ...interface{}) {
	msg := fmt.Sprintf(formato, args...)
	if caminho != "" {
		msg = caminho + ": " + msg
	}
	*erros = append(*erros, &object.Texto{Value: msg})
}

func mostraValorValida(o object.Object) string {
	if t, ok := o.(*object.Texto); ok {
		return strconv.Quote(t.Value)
	}
	return o.Inspect()
}

// confereTipoValida: "" = ok; senao o motivo.
func confereTipoValida(tipo string, v object.Object) string {
	ok := true
	switch tipo {
	case "", "qualquer":
		return ""
	case "texto":
		_, ok = v.(*object.Texto)
	case "numero":
		_, ok = v.(*object.Numero)
	case "inteiro":
		n, eh := v.(*object.Numero)
		ok = eh && (n.EhInt || (n.Value == math.Trunc(n.Value) && !math.IsInf(n.Value, 0)))
		if eh && !ok {
			return "tem que ser inteiro, veio " + n.Inspect()
		}
	case "booleano":
		_, ok = v.(*object.Booleano)
	case "lista":
		_, ok = v.(*object.Lista)
	case "dicionario":
		_, ok = v.(*object.Dicionario)
	case "email":
		t, eh := v.(*object.Texto)
		ok = eh
		if eh && !reEmail.MatchString(t.Value) {
			return "tem que ser email valido, veio " + strconv.Quote(t.Value)
		}
	case "data":
		t, eh := v.(*object.Texto)
		ok = eh
		if eh && !ehDataISO(t.Value) {
			return "tem que ser data ISO (AAAA-MM-DD), veio " + strconv.Quote(t.Value)
		}
	}
	if !ok {
		return "tem que ser " + tipo + ", veio " + object.NomeTipo(v)
	}
	return ""
}

func ehDataISO(s string) bool {
	for _, f := range []string{"2006-01-02", time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if _, err := time.Parse(f, s); err == nil {
			return true
		}
	}
	return false
}

func plural(n float64, um, varios string) string {
	if n == 1 {
		return um
	}
	return varios
}

func formataLimite(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func aplicaRegraValida(r *regraValida, v object.Object, caminho string, erros *[]object.Object) {
	if v == nil || v.Type() == object.NADA_OBJ {
		if r.obrigatorio {
			addErroValida(erros, caminho, "obrigatorio")
		}
		return
	}
	if motivo := confereTipoValida(r.tipo, v); motivo != "" {
		addErroValida(erros, caminho, "%s", motivo)
		return
	}
	confereLimitesValida(r, v, caminho, erros)
	if r.padrao != nil {
		if t, ok := v.(*object.Texto); ok && !r.padrao.MatchString(t.Value) {
			addErroValida(erros, caminho, "nao bate com o padrao %s", r.padraoTexto)
		}
	}
	if r.opcoes != nil {
		achou := false
		for _, op := range r.opcoes {
			if iguais(op, v) {
				achou = true
				break
			}
		}
		if !achou {
			partes := make([]string, len(r.opcoes))
			for i, op := range r.opcoes {
				partes[i] = mostraValorValida(op)
			}
			addErroValida(erros, caminho, "tem que ser um de %s, veio %s", strings.Join(partes, ", "), mostraValorValida(v))
		}
	}
	if r.itens != nil {
		if l, ok := v.(*object.Lista); ok {
			for i, item := range l.Copia() {
				aplicaRegraValida(r.itens, item, fmt.Sprintf("%s[%d]", caminho, i), erros)
			}
		}
	}
	if r.campos != nil {
		if d, ok := v.(*object.Dicionario); ok {
			aplicaCamposValida(r.campos, d, caminho, erros)
		}
	}
}

func confereLimitesValida(r *regraValida, v object.Object, caminho string, erros *[]object.Object) {
	if r.min == nil && r.max == nil {
		return
	}
	var medida float64
	unidade := ""
	switch x := v.(type) {
	case *object.Numero:
		if r.min != nil && x.Value < *r.min {
			addErroValida(erros, caminho, "tem que ser no minimo %s, veio %s", formataLimite(*r.min), x.Inspect())
		}
		if r.max != nil && x.Value > *r.max {
			addErroValida(erros, caminho, "tem que ser no maximo %s, veio %s", formataLimite(*r.max), x.Inspect())
		}
		return
	case *object.Texto:
		medida, unidade = float64(utf8.RuneCountInString(x.Value)), "caractere"
	case *object.Lista:
		medida, unidade = float64(x.Tamanho()), "item"
	case *object.Dicionario:
		medida, unidade = float64(x.Tamanho()), "campo"
	default:
		return
	}
	nomeUnidade := func(n float64) string {
		if unidade == "item" {
			return plural(n, "item", "itens")
		}
		return plural(n, unidade, unidade+"s")
	}
	if r.min != nil && medida < *r.min {
		addErroValida(erros, caminho, "tem que ter pelo menos %s %s, veio %s", formataLimite(*r.min), nomeUnidade(*r.min), formataLimite(medida))
	}
	if r.max != nil && medida > *r.max {
		addErroValida(erros, caminho, "tem que ter no maximo %s %s, veio %s", formataLimite(*r.max), nomeUnidade(*r.max), formataLimite(medida))
	}
}

func aplicaCamposValida(esq *esquemaValida, d *object.Dicionario, caminho string, erros *[]object.Object) {
	conhecidos := make(map[string]bool, len(esq.campos))
	for _, c := range esq.campos {
		conhecidos[c.nome] = true
		v, _ := d.PegaTexto(c.nome)
		aplicaRegraValida(c.regra, v, juntaCaminho(caminho, c.nome), erros)
	}
	if !esq.estrito {
		return
	}
	for _, par := range d.Pares() {
		nome := par.Chave.Inspect()
		if t, ok := par.Chave.(*object.Texto); ok {
			nome = t.Value
			if conhecidos[nome] {
				continue
			}
		}
		addErroValida(erros, juntaCaminho(caminho, nome), "campo nao esperado")
	}
}
