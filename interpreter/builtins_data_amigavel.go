package interpreter

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gambiarrascript/object"
)

// Datas com formato amigavel (sem o layout "2006-01-02" do Go):
//
//	formata_data(tempo, "dd/mm/aaaa hh:mi:ss")       → "08/10/2026 14:05:09"
//	formata_data(tempo, "dddd, dd 'de' mmmm", fuso)  → "quinta-feira, 08 de outubro"
//	le_data("08/10/2026", "dd/mm/aaaa")              → "2026-10-08T00:00:00Z"
//	le_data("08/10/2026 14:05", "dd/mm/aaaa hh:mi", "America/Sao_Paulo")
//
// Marcadores (minusculos; data e hora sem ambiguidade — mm e SEMPRE mes,
// minuto e `mi`):
//
//	aaaa ano (2026)        aa  ano com 2 digitos (26)
//	mm   mes (10)          mmm mes curto (out)    mmmm mes (outubro)
//	dd   dia (08)          ddd dia da semana curto (qui)
//	dddd dia da semana (quinta-feira)
//	hh   hora 00-23        mi  minuto             ss   segundo
//
// Qualquer outro caractere passa reto. Texto entre aspas simples e literal
// ('de', 'as'); duas aspas simples ('') viram uma. O tempo e o mesmo que
// formata_tempo aceita (ISO 8601 ou segundos Unix); sem o 3o argumento o
// formata_data usa o fuso que veio no instante (agora() e UTC) e o le_data
// le em UTC. Fuso: nome IANA ("America/Sao_Paulo"), "UTC" ou "local".

var (
	mesesPt = []string{"janeiro", "fevereiro", "março", "abril", "maio", "junho",
		"julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}
	mesesCurtosPt = []string{"jan", "fev", "mar", "abr", "mai", "jun",
		"jul", "ago", "set", "out", "nov", "dez"}
	diasPt = []string{"domingo", "segunda-feira", "terça-feira", "quarta-feira",
		"quinta-feira", "sexta-feira", "sábado"}
	diasCurtosPt = []string{"dom", "seg", "ter", "qua", "qui", "sex", "sáb"}
)

// marcadores em ordem de tentativa (o mais comprido primeiro)
var marcadoresData = []string{"aaaa", "mmmm", "dddd", "mmm", "ddd", "aa", "mm", "dd", "hh", "mi", "ss"}

type pecaData struct {
	marcador string // "" = literal
	literal  string
}

func pecasData(formato string) ([]pecaData, error) {
	var pecas []pecaData
	var lit strings.Builder
	solta := func() {
		if lit.Len() > 0 {
			pecas = append(pecas, pecaData{literal: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(formato); {
		if formato[i] == '\'' {
			if strings.HasPrefix(formato[i:], "''") { // '' = aspa literal
				lit.WriteByte('\'')
				i += 2
				continue
			}
			j := strings.IndexByte(formato[i+1:], '\'')
			if j < 0 {
				return nil, fmt.Errorf("aspas simples sem fechar no formato %q", formato)
			}
			lit.WriteString(formato[i+1 : i+1+j])
			i += j + 2
			continue
		}
		achou := ""
		for _, m := range marcadoresData {
			if strings.HasPrefix(formato[i:], m) {
				achou = m
				break
			}
		}
		if achou == "" {
			lit.WriteByte(formato[i])
			i++
			continue
		}
		solta()
		pecas = append(pecas, pecaData{marcador: achou})
		i += len(achou)
	}
	solta()
	return pecas, nil
}

func fusoData(nome string, o object.Object) (*time.Location, *object.Erro) {
	t, ok := o.(*object.Texto)
	if !ok {
		return nil, erroBuiltin("%s: o fuso tem que ser texto (tipo \"America/Sao_Paulo\"), veio %s", nome, object.NomeTipo(o))
	}
	if strings.EqualFold(t.Value, "local") {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(t.Value)
	if err != nil {
		return nil, erroBuiltin("%s: fuso %q nao encontrado", nome, t.Value)
	}
	return loc, nil
}

func builtinFormataData(args []object.Object) object.Object {
	if len(args) != 2 && len(args) != 3 {
		return erroBuiltin("formata_data() quer 2 ou 3 args (tempo, formato[, fuso]), veio %d", len(args))
	}
	t, e := objParaTempo(args[0])
	if e != nil {
		return e
	}
	formato, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("formata_data: o formato tem que ser texto (tipo \"dd/mm/aaaa\"), veio %s", object.NomeTipo(args[1]))
	}
	if len(args) == 3 {
		loc, e := fusoData("formata_data", args[2])
		if e != nil {
			return e
		}
		t = t.In(loc)
	}
	pecas, err := pecasData(formato.Value)
	if err != nil {
		return erroBuiltin("formata_data: %v", err)
	}
	return &object.Texto{Value: formataData(t, pecas)}
}

func formataData(t time.Time, pecas []pecaData) string {
	var b strings.Builder
	for _, p := range pecas {
		switch p.marcador {
		case "":
			b.WriteString(p.literal)
		case "aaaa":
			fmt.Fprintf(&b, "%04d", t.Year())
		case "aa":
			fmt.Fprintf(&b, "%02d", t.Year()%100)
		case "mmmm":
			b.WriteString(mesesPt[t.Month()-1])
		case "mmm":
			b.WriteString(mesesCurtosPt[t.Month()-1])
		case "mm":
			fmt.Fprintf(&b, "%02d", int(t.Month()))
		case "dddd":
			b.WriteString(diasPt[t.Weekday()])
		case "ddd":
			b.WriteString(diasCurtosPt[t.Weekday()])
		case "dd":
			fmt.Fprintf(&b, "%02d", t.Day())
		case "hh":
			fmt.Fprintf(&b, "%02d", t.Hour())
		case "mi":
			fmt.Fprintf(&b, "%02d", t.Minute())
		case "ss":
			fmt.Fprintf(&b, "%02d", t.Second())
		}
	}
	return b.String()
}

func builtinLeData(args []object.Object) object.Object {
	if len(args) != 2 && len(args) != 3 {
		return erroBuiltin("le_data() quer 2 ou 3 args (texto, formato[, fuso]), veio %d", len(args))
	}
	txt, ok := args[0].(*object.Texto)
	if !ok {
		return erroBuiltin("le_data: o 1o argumento tem que ser texto, veio %s", object.NomeTipo(args[0]))
	}
	formato, ok := args[1].(*object.Texto)
	if !ok {
		return erroBuiltin("le_data: o formato tem que ser texto (tipo \"dd/mm/aaaa\"), veio %s", object.NomeTipo(args[1]))
	}
	loc := time.UTC
	if len(args) == 3 {
		var e *object.Erro
		if loc, e = fusoData("le_data", args[2]); e != nil {
			return e
		}
	}
	pecas, err := pecasData(formato.Value)
	if err != nil {
		return erroBuiltin("le_data: %v", err)
	}
	t, err := leData(txt.Value, pecas, loc)
	if err != nil {
		return erroBuiltin("le_data: %q nao bate com %q: %v", txt.Value, formato.Value, err)
	}
	return &object.Texto{Value: t.Format(time.RFC3339)}
}

func leData(s string, pecas []pecaData, loc *time.Location) (time.Time, error) {
	ano, mes, dia, hora, min, seg := 0, 1, 1, 0, 0, 0
	pos := 0
	for _, p := range pecas {
		resto := s[pos:]
		switch p.marcador {
		case "":
			if !strings.HasPrefix(resto, p.literal) {
				return time.Time{}, fmt.Errorf("esperava %q na posicao %d", p.literal, pos+1)
			}
			pos += len(p.literal)
			continue
		case "mmmm", "mmm", "dddd", "ddd":
			nomes := map[string][]string{"mmmm": mesesPt, "mmm": mesesCurtosPt,
				"dddd": diasPt, "ddd": diasCurtosPt}[p.marcador]
			idx, n := casaNome(resto, nomes)
			if idx < 0 {
				return time.Time{}, fmt.Errorf("nome de %s nao reconhecido na posicao %d", map[bool]string{true: "mes", false: "dia da semana"}[p.marcador[0] == 'm'], pos+1)
			}
			if p.marcador[0] == 'm' {
				mes = idx + 1
			}
			pos += n
			continue
		}
		max := 2
		if p.marcador == "aaaa" {
			max = 4
		}
		n, v := 0, 0
		for n < max && n < len(resto) && resto[n] >= '0' && resto[n] <= '9' {
			v = v*10 + int(resto[n]-'0')
			n++
		}
		if n == 0 || (p.marcador == "aaaa" && n != 4) || (p.marcador == "aa" && n != 2) {
			return time.Time{}, fmt.Errorf("esperava numero (%s) na posicao %d", p.marcador, pos+1)
		}
		pos += n
		switch p.marcador {
		case "aaaa":
			ano = v
		case "aa":
			ano = 2000 + v
		case "mm":
			mes = v
		case "dd":
			dia = v
		case "hh":
			hora = v
		case "mi":
			min = v
		case "ss":
			seg = v
		}
	}
	if pos != len(s) {
		return time.Time{}, fmt.Errorf("sobrou %q no fim", s[pos:])
	}
	if mes < 1 || mes > 12 || hora > 23 || min > 59 || seg > 59 {
		return time.Time{}, fmt.Errorf("data ou hora fora do lugar")
	}
	t := time.Date(ano, time.Month(mes), dia, hora, min, seg, 0, loc)
	if t.Day() != dia || int(t.Month()) != mes {
		return time.Time{}, fmt.Errorf("o dia %d nao existe nesse mes", dia)
	}
	return t, nil
}

// casaNome acha qual nome (sem ligar pra maiuscula nem acento) comeca o texto.
// Devolve o indice e quantos bytes do texto consumiu (-1 = nenhum).
func casaNome(texto string, nomes []string) (int, int) {
	melhor, melhorN, melhorTam := -1, 0, 0
	for idx, nome := range nomes {
		n, ok := prefixoSemAcento(texto, nome)
		if ok && utf8.RuneCountInString(nome) > melhorTam {
			melhor, melhorN, melhorTam = idx, n, utf8.RuneCountInString(nome)
		}
	}
	return melhor, melhorN
}

func prefixoSemAcento(texto, nome string) (int, bool) {
	pos := 0
	for _, rn := range nome {
		if pos >= len(texto) {
			return 0, false
		}
		rt, tam := utf8.DecodeRuneInString(texto[pos:])
		if tiraAcento(unicode.ToLower(rt)) != tiraAcento(rn) {
			return 0, false
		}
		pos += tam
	}
	return pos, true
}

func tiraAcento(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ã':
		return 'a'
	case 'é', 'ê':
		return 'e'
	case 'í':
		return 'i'
	case 'ó', 'ô', 'õ':
		return 'o'
	case 'ú':
		return 'u'
	case 'ç':
		return 'c'
	}
	return r
}
