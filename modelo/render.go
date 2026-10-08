package modelo

import (
	"fmt"
	"html"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gambiarrascript/object"
)

// Opcoes de uma renderizacao.
type Opcoes struct {
	// Pasta e a raiz dos `inclui`/`usa`: nenhum caminho sai dela. "" = a
	// pasta do arquivo do modelo (ou a pasta atual, pra modelo de texto).
	Pasta string
	// Estrito: variavel que nao existe vira erro em vez de texto vazio.
	Estrito bool
	// SemEscapar desliga o escape de HTML (pra texto puro: e-mail, .txt...).
	SemEscapar bool
	// Json e Formata vem do interpretador (pra_json e formata da linguagem).
	Json    func(object.Object) (string, error)
	Formata func(formato string, v object.Object) (string, error)
}

// profMax limita inclui/usa aninhados (modelo que se inclui em circulo).
const profMax = 32

type escopo struct {
	nome string
	v    object.Object
	pai  *escopo
}

type blocoDono struct {
	b *noBloco
	m *Modelo
}

type render struct {
	op     *Opcoes
	raiz   string
	dados  object.Object
	buf    strings.Builder
	blocos map[string]blocoDono
	prof   int
}

// Renderiza roda o modelo com os dados (dicionario, instancia de treta ou
// nada) e devolve o texto pronto.
func (m *Modelo) Renderiza(dados object.Object, op Opcoes) (string, error) {
	if dados == nil {
		dados = &object.Nada{}
	}
	raiz := op.Pasta
	if raiz == "" {
		if m.Caminho != "" {
			raiz = filepath.Dir(m.Caminho)
		} else {
			raiz = "."
		}
	}
	if abs, err := filepath.Abs(raiz); err == nil {
		raiz = abs
	}
	r := &render{op: &op, raiz: raiz, dados: dados}
	if err := r.modelo(m, nil); err != nil {
		return "", err
	}
	return r.buf.String(), nil
}

func (r *render) erro(m *Modelo, linha int, f string, a ...interface{}) error {
	return &Erro{Modelo: m.Nome, Linha: linha, Msg: fmt.Sprintf(f, a...)}
}

func (r *render) modelo(m *Modelo, esc *escopo) error {
	if m.usa == "" {
		return r.nos(m, m.nos, esc)
	}
	// filho com layout: registra os blocos (o mais de baixo ganha) e
	// renderiza o pai no lugar do proprio corpo.
	if r.blocos == nil {
		r.blocos = map[string]blocoDono{}
	}
	for nome, b := range m.blocos {
		if _, ja := r.blocos[nome]; !ja {
			r.blocos[nome] = blocoDono{b: b, m: m}
		}
	}
	pai, err := r.carrega(m, m.usa, m.usaLinha, "usa")
	if err != nil {
		return err
	}
	r.prof++
	defer func() { r.prof-- }()
	return r.modelo(pai, esc)
}

// carrega acha o arquivo de um inclui/usa: relativo a pasta do modelo atual
// e sem sair da raiz.
func (r *render) carrega(m *Modelo, nome string, linha int, oque string) (*Modelo, error) {
	if r.prof >= profMax {
		return nil, r.erro(m, linha, "`%s %q`: modelo demais um dentro do outro (inclui/usa em circulo?)", oque, nome)
	}
	if filepath.IsAbs(nome) || strings.HasPrefix(nome, "/") || strings.HasPrefix(nome, `\`) || filepath.VolumeName(nome) != "" {
		return nil, r.erro(m, linha, "`%s %q`: caminho absoluto nao vale, use relativo a pasta do modelo", oque, nome)
	}
	base := r.raiz
	if m.Caminho != "" {
		base = filepath.Dir(m.Caminho)
	}
	alvo := filepath.Clean(filepath.Join(base, filepath.FromSlash(nome)))
	rel, ok := dentro(r.raiz, alvo)
	if ok {
		// link simbolico apontando pra fora tambem nao passa
		if real, err := filepath.EvalSymlinks(alvo); err == nil {
			raizReal, err2 := filepath.EvalSymlinks(r.raiz)
			if err2 != nil {
				raizReal = r.raiz
			}
			_, ok = dentro(raizReal, real)
		}
	}
	if !ok {
		return nil, r.erro(m, linha, "`%s %q` sai da pasta dos modelos (%s) — nao pode", oque, nome, r.raiz)
	}
	sub, err := CarregaArquivo(alvo, rel)
	if err != nil {
		if e, ok := err.(*Erro); ok && e.IO {
			return nil, r.erro(m, linha, "`%s %q`: %s", oque, nome, e.Msg)
		}
		return nil, err
	}
	return sub, nil
}

// dentro diz se alvo fica dentro de raiz (e devolve o caminho relativo com /).
func dentro(raiz, alvo string) (string, bool) {
	rel, err := filepath.Rel(raiz, alvo)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func (r *render) nos(m *Modelo, nos []no, esc *escopo) error {
	for _, n := range nos {
		if err := r.no(m, n, esc); err != nil {
			return err
		}
	}
	return nil
}

func (r *render) no(m *Modelo, n no, esc *escopo) error {
	switch n := n.(type) {
	case *noTexto:
		r.buf.WriteString(n.s)
	case *noValor:
		v, falta, err := r.avalia(m, n.ex, esc, n.linha)
		if err != nil {
			return err
		}
		if falta != "" && r.op.Estrito {
			return r.erro(m, n.linha, "`%s` nao existe (modo estrito)", falta)
		}
		s := paraTexto(v)
		if n.cru || ehCru(n.ex) || r.op.SemEscapar {
			r.buf.WriteString(s)
		} else {
			r.buf.WriteString(html.EscapeString(s))
		}
	case *noSe:
		for _, rm := range n.ramos {
			v, _, err := r.avalia(m, rm.cond, esc, n.linha)
			if err != nil {
				return err
			}
			if verdade(v) {
				return r.nos(m, rm.corpo, esc)
			}
		}
		return r.nos(m, n.senao, esc)
	case *noPraCada:
		return r.praCada(m, n, esc)
	case *noInclui:
		sub, err := r.carrega(m, n.caminho, n.linha, "inclui")
		if err != nil {
			return err
		}
		velhos := r.blocos
		r.blocos = nil
		r.prof++
		err = r.modelo(sub, esc)
		r.prof--
		r.blocos = velhos
		return err
	case *noBloco:
		if d, ok := r.blocos[n.nome]; ok {
			return r.nos(d.m, d.b.corpo, esc)
		}
		return r.nos(m, n.corpo, esc)
	}
	return nil
}

func (r *render) praCada(m *Modelo, n *noPraCada, esc *escopo) error {
	col, falta, err := r.avalia(m, n.col, esc, n.linha)
	if err != nil {
		return err
	}
	if falta != "" && r.op.Estrito {
		return r.erro(m, n.linha, "`%s` nao existe (modo estrito)", falta)
	}
	var chaves, vals []object.Object
	switch c := col.(type) {
	case *object.Lista:
		vals = c.Copia()
		for i := range vals {
			chaves = append(chaves, object.NumInt(int64(i)))
		}
	case *object.Conjunto:
		vals = c.Valores()
		for i := range vals {
			chaves = append(chaves, object.NumInt(int64(i)))
		}
	case *object.Dicionario:
		for _, p := range c.Pares() {
			chaves = append(chaves, p.Chave)
			if n.chave == "" {
				vals = append(vals, p.Chave) // um nome so: as chaves
			} else {
				vals = append(vals, p.Valor)
			}
		}
	case *object.Nada:
	default:
		return r.erro(m, n.linha, "`pra_cada` quer lista, dicionario ou conjunto, veio %s", object.NomeTipo(col))
	}
	if len(vals) == 0 {
		return r.nos(m, n.vazio, esc)
	}
	for i, v := range vals {
		e := esc
		if n.chave != "" {
			e = &escopo{nome: n.chave, v: chaves[i], pai: e}
		}
		e = &escopo{nome: n.valor, v: v, pai: e}
		if err := r.nos(m, n.corpo, e); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------- avaliacao

var nada = &object.Nada{}

// avalia devolve o valor e, se veio de um caminho que nao existe, o texto do
// caminho (falta != ""). Valor que falta vale nada.
func (r *render) avalia(m *Modelo, x expr, esc *escopo, linha int) (object.Object, string, error) {
	switch x := x.(type) {
	case *exLiteral:
		return x.v, "", nil
	case *exCaminho:
		return r.caminho(m, x, esc, linha)
	case *exNao:
		v, _, err := r.avalia(m, x.x, esc, linha)
		if err != nil {
			return nil, "", err
		}
		return &object.Booleano{Value: !verdade(v)}, "", nil
	case *exBin:
		a, _, err := r.avalia(m, x.a, esc, linha)
		if err != nil {
			return nil, "", err
		}
		switch x.op {
		case "e":
			if !verdade(a) {
				return &object.Booleano{Value: false}, "", nil
			}
			b, _, err := r.avalia(m, x.b, esc, linha)
			if err != nil {
				return nil, "", err
			}
			return &object.Booleano{Value: verdade(b)}, "", nil
		case "ou":
			if verdade(a) {
				return &object.Booleano{Value: true}, "", nil
			}
			b, _, err := r.avalia(m, x.b, esc, linha)
			if err != nil {
				return nil, "", err
			}
			return &object.Booleano{Value: verdade(b)}, "", nil
		}
		b, _, err := r.avalia(m, x.b, esc, linha)
		if err != nil {
			return nil, "", err
		}
		ok, msg := compara(x.op, a, b)
		if msg != "" {
			return nil, "", r.erro(m, linha, "%s", msg)
		}
		return &object.Booleano{Value: ok}, "", nil
	case *exFiltro:
		return r.filtro(m, x, esc, linha)
	}
	return nada, "", nil
}

func (r *render) caminho(m *Modelo, c *exCaminho, esc *escopo, linha int) (object.Object, string, error) {
	var v object.Object
	achou := false
	for e := esc; e != nil; e = e.pai {
		if e.nome == c.raiz {
			v, achou = e.v, true
			break
		}
	}
	if !achou {
		v, achou = membro(r.dados, c.raiz)
	}
	if !achou {
		return nada, c.texto, nil
	}
	for _, p := range c.passos {
		if p.nome != "" {
			v, achou = membro(v, p.nome)
		} else {
			idx, _, err := r.avalia(m, p.idx, esc, linha)
			if err != nil {
				return nil, "", err
			}
			v, achou = indice(v, idx)
		}
		if !achou {
			return nada, c.texto, nil
		}
	}
	return v, "", nil
}

func membro(v object.Object, nome string) (object.Object, bool) {
	switch v := v.(type) {
	case *object.Dicionario:
		return v.PegaTexto(nome)
	case *object.Instancia:
		x, msg := v.Membro(nome)
		if msg != "" {
			return nil, false
		}
		return x, true
	}
	return nil, false
}

func indice(v, idx object.Object) (object.Object, bool) {
	switch v := v.(type) {
	case *object.Lista:
		n, ok := idx.(*object.Numero)
		if !ok || !n.EhInt {
			return nil, false
		}
		return v.Indice(int(n.Int))
	case *object.Dicionario:
		k, ok := idx.(object.Chaveavel)
		if !ok {
			return nil, false
		}
		par, ok := v.Pega(k.ChaveHash())
		return par.Valor, ok
	case *object.Instancia:
		t, ok := idx.(*object.Texto)
		if !ok {
			return nil, false
		}
		return membro(v, t.Value)
	}
	return nil, false
}

func (r *render) filtro(m *Modelo, f *exFiltro, esc *escopo, linha int) (object.Object, string, error) {
	v, falta, err := r.avalia(m, f.base, esc, linha)
	if err != nil {
		return nil, "", err
	}
	var arg object.Object
	if f.arg != nil {
		arg, _, err = r.avalia(m, f.arg, esc, linha)
		if err != nil {
			return nil, "", err
		}
	}
	argTexto := func() (string, error) {
		t, ok := arg.(*object.Texto)
		if !ok {
			return "", r.erro(m, linha, "o filtro `%s` quer texto entre aspas, veio %s", f.nome, object.NomeTipo(arg))
		}
		return t.Value, nil
	}
	switch f.nome {
	case "cru":
		return v, falta, nil
	case "maiusculo":
		return &object.Texto{Value: strings.ToUpper(paraTexto(v))}, falta, nil
	case "minusculo":
		return &object.Texto{Value: strings.ToLower(paraTexto(v))}, falta, nil
	case "tamanho":
		switch v := v.(type) {
		case *object.Texto:
			return object.NumInt(int64(utf8.RuneCountInString(v.Value))), falta, nil
		case *object.Lista:
			return object.NumInt(int64(v.Tamanho())), falta, nil
		case *object.Dicionario:
			return object.NumInt(int64(v.Tamanho())), falta, nil
		case *object.Conjunto:
			return object.NumInt(int64(v.Tamanho())), falta, nil
		case *object.Nada:
			return object.NumInt(0), falta, nil
		}
		return nil, "", r.erro(m, linha, "`| tamanho` quer texto, lista, dicionario ou conjunto, veio %s", object.NomeTipo(v))
	case "json":
		if r.op.Json == nil {
			return nil, "", r.erro(m, linha, "`| json` indisponivel aqui")
		}
		s, err := r.op.Json(v)
		if err != nil {
			return nil, "", r.erro(m, linha, "`| json`: %s", err)
		}
		return &object.Texto{Value: jsonSeguro(s)}, falta, nil
	case "formata":
		fmtx, err := argTexto()
		if err != nil {
			return nil, "", err
		}
		if r.op.Formata == nil {
			return &object.Texto{Value: fmt.Sprintf(fmtx, paraTexto(v))}, falta, nil
		}
		s, err := r.op.Formata(fmtx, v)
		if err != nil {
			return nil, "", r.erro(m, linha, "`| formata`: %s", err)
		}
		return &object.Texto{Value: s}, falta, nil
	case "padrao":
		vazio := falta != ""
		switch t := v.(type) {
		case *object.Nada:
			vazio = true
		case *object.Texto:
			vazio = vazio || t.Value == ""
		}
		if vazio {
			return arg, "", nil
		}
		return v, "", nil
	case "junta":
		sep, err := argTexto()
		if err != nil {
			return nil, "", err
		}
		var itens []object.Object
		switch v := v.(type) {
		case *object.Lista:
			itens = v.Copia()
		case *object.Conjunto:
			itens = v.Valores()
		case *object.Nada:
		default:
			return nil, "", r.erro(m, linha, "`| junta` quer lista ou conjunto, veio %s", object.NomeTipo(v))
		}
		partes := make([]string, len(itens))
		for i, it := range itens {
			partes[i] = paraTexto(it)
		}
		return &object.Texto{Value: strings.Join(partes, sep)}, falta, nil
	}
	return v, falta, nil
}

// jsonSeguro troca < > & e os separadores de linha do JS por \uXXXX (igual
// o encoding/json do Go): `{{ x | json | cru }}` dentro de <script> nao fecha
// a tag nem quebra o JS. So aparecem dentro de string JSON, entao e seguro.
func jsonSeguro(s string) string {
	return strings.NewReplacer("<", `<`, ">", `>`, "&", `&`,
		" ", ` `, " ", ` `).Replace(s)
}

func ehCru(x expr) bool {
	for f, ok := x.(*exFiltro); ok; f, ok = f.base.(*exFiltro) {
		if f.nome == "cru" {
			return true
		}
	}
	return false
}

// paraTexto: texto sai como e, nada sai vazio, o resto igual o `mostra`.
func paraTexto(v object.Object) string {
	switch v := v.(type) {
	case nil, *object.Nada:
		return ""
	case *object.Texto:
		return v.Value
	}
	return v.Inspect()
}

// verdade: igual a linguagem — so nada e deu_ruim sao falsos.
func verdade(v object.Object) bool {
	switch v := v.(type) {
	case nil, *object.Nada:
		return false
	case *object.Booleano:
		return v.Value
	}
	return true
}

func compara(op string, a, b object.Object) (bool, string) {
	if a == nil {
		a = nada
	}
	if b == nil {
		b = nada
	}
	switch op {
	case "==":
		return iguais(a, b), ""
	case "!=":
		return !iguais(a, b), ""
	}
	var c int
	switch x := a.(type) {
	case *object.Numero:
		y, ok := b.(*object.Numero)
		if !ok {
			return false, fmt.Sprintf("nao da pra comparar %s com %s usando `%s`", object.NomeTipo(a), object.NomeTipo(b), op)
		}
		c = comparaNum(x, y)
	case *object.Texto:
		y, ok := b.(*object.Texto)
		if !ok {
			return false, fmt.Sprintf("nao da pra comparar %s com %s usando `%s`", object.NomeTipo(a), object.NomeTipo(b), op)
		}
		c = strings.Compare(x.Value, y.Value)
	default:
		return false, fmt.Sprintf("`%s` so compara numero com numero ou texto com texto, veio %s", op, object.NomeTipo(a))
	}
	switch op {
	case ">":
		return c > 0, ""
	case "<":
		return c < 0, ""
	case ">=":
		return c >= 0, ""
	case "<=":
		return c <= 0, ""
	}
	return false, "operador estranho " + op
}

func comparaNum(x, y *object.Numero) int {
	if x.EhInt && y.EhInt {
		switch {
		case x.Int < y.Int:
			return -1
		case x.Int > y.Int:
			return 1
		}
		return 0
	}
	switch {
	case x.Value < y.Value:
		return -1
	case x.Value > y.Value:
		return 1
	}
	return 0
}

func iguais(a, b object.Object) bool {
	switch x := a.(type) {
	case *object.Numero:
		y, ok := b.(*object.Numero)
		return ok && comparaNum(x, y) == 0
	case *object.Texto:
		y, ok := b.(*object.Texto)
		return ok && x.Value == y.Value
	case *object.Booleano:
		y, ok := b.(*object.Booleano)
		return ok && x.Value == y.Value
	case *object.Nada:
		_, ok := b.(*object.Nada)
		return ok
	}
	return a.Type() == b.Type() && a.Inspect() == b.Inspect()
}
