package interpreter

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"gambiarrascript/object"
)

// Parser de JSON proprio. Existe por dois motivos: (1) json.Unmarshal devolve
// map[string]interface{}, que PERDE a ordem das chaves — e dicionario aqui tem
// ordem de insercao; (2) o json.Decoder por token preserva a ordem mas aloca
// uma interface por token e ficou 66% mais lento que o Unmarshal. Esse parser
// le direto da string e monta os objetos da linguagem numa passada.

type leitorJson struct {
	s   string
	pos int
}

func parseJson(txt string) (object.Object, error) {
	l := &leitorJson{s: txt}
	l.pulaEspaco()
	v, err := l.valor()
	if err != nil {
		return nil, err
	}
	l.pulaEspaco()
	if l.pos != len(l.s) {
		return nil, fmt.Errorf("sobrou coisa depois do valor (posicao %d)", l.pos)
	}
	return v, nil
}

func (l *leitorJson) pulaEspaco() {
	for l.pos < len(l.s) {
		switch l.s[l.pos] {
		case ' ', '\t', '\n', '\r':
			l.pos++
		default:
			return
		}
	}
}

func (l *leitorJson) erro(msg string) error {
	return fmt.Errorf("%s (posicao %d)", msg, l.pos)
}

func (l *leitorJson) valor() (object.Object, error) {
	if l.pos >= len(l.s) {
		return nil, l.erro("acabou o texto no meio do valor")
	}
	switch c := l.s[l.pos]; {
	case c == '{':
		return l.objeto()
	case c == '[':
		return l.lista()
	case c == '"':
		t, err := l.texto()
		if err != nil {
			return nil, err
		}
		return &object.Texto{Value: t}, nil
	case c == 't':
		if strings.HasPrefix(l.s[l.pos:], "true") {
			l.pos += 4
			return boolDoNativo(true), nil
		}
		return nil, l.erro("valor invalido")
	case c == 'f':
		if strings.HasPrefix(l.s[l.pos:], "false") {
			l.pos += 5
			return boolDoNativo(false), nil
		}
		return nil, l.erro("valor invalido")
	case c == 'n':
		if strings.HasPrefix(l.s[l.pos:], "null") {
			l.pos += 4
			return NADA, nil
		}
		return nil, l.erro("valor invalido")
	case c == '-' || (c >= '0' && c <= '9'):
		return l.numero()
	}
	return nil, l.erro("valor invalido")
}

func (l *leitorJson) objeto() (object.Object, error) {
	l.pos++ // consome '{'
	d := object.NovoDicionario()
	l.pulaEspaco()
	if l.pos < len(l.s) && l.s[l.pos] == '}' {
		l.pos++
		return d, nil
	}
	for {
		l.pulaEspaco()
		if l.pos >= len(l.s) || l.s[l.pos] != '"' {
			return nil, l.erro("chave de objeto tem que ser texto entre aspas")
		}
		nome, err := l.texto()
		if err != nil {
			return nil, err
		}
		l.pulaEspaco()
		if l.pos >= len(l.s) || l.s[l.pos] != ':' {
			return nil, l.erro("faltou o `:` depois da chave")
		}
		l.pos++
		l.pulaEspaco()
		val, err := l.valor()
		if err != nil {
			return nil, err
		}
		chave := &object.Texto{Value: nome}
		d.Bota(chave.ChaveHash(), object.ParDic{Chave: chave, Valor: val})
		l.pulaEspaco()
		if l.pos >= len(l.s) {
			return nil, l.erro("acabou o texto com o objeto aberto")
		}
		switch l.s[l.pos] {
		case ',':
			l.pos++
		case '}':
			l.pos++
			return d, nil
		default:
			return nil, l.erro("esperava `,` ou `}`")
		}
	}
}

func (l *leitorJson) lista() (object.Object, error) {
	l.pos++ // consome '['
	elems := []object.Object{}
	l.pulaEspaco()
	if l.pos < len(l.s) && l.s[l.pos] == ']' {
		l.pos++
		return object.NovaLista(elems), nil
	}
	for {
		l.pulaEspaco()
		v, err := l.valor()
		if err != nil {
			return nil, err
		}
		elems = append(elems, v)
		l.pulaEspaco()
		if l.pos >= len(l.s) {
			return nil, l.erro("acabou o texto com a lista aberta")
		}
		switch l.s[l.pos] {
		case ',':
			l.pos++
		case ']':
			l.pos++
			return object.NovaLista(elems), nil
		default:
			return nil, l.erro("esperava `,` ou `]`")
		}
	}
}

// texto le uma string JSON. Caminho rapido: sem escape nenhum, devolve a fatia
// direto (o Go compartilha o array de bytes, sem copia).
func (l *leitorJson) texto() (string, error) {
	l.pos++ // consome a aspa de abertura
	ini := l.pos
	for l.pos < len(l.s) {
		c := l.s[l.pos]
		switch {
		case c == '"':
			s := l.s[ini:l.pos]
			l.pos++
			return s, nil
		case c == '\\':
			return l.textoComEscape(ini)
		case c < 0x20:
			return "", l.erro("caractere de controle solto dentro do texto")
		}
		l.pos++
	}
	return "", l.erro("texto sem aspa de fechamento")
}

// textoComEscape termina de ler uma string que tem pelo menos um `\`. `ini` e
// onde o conteudo comecou; l.pos esta na primeira barra.
func (l *leitorJson) textoComEscape(ini int) (string, error) {
	var b strings.Builder
	b.WriteString(l.s[ini:l.pos])
	for l.pos < len(l.s) {
		c := l.s[l.pos]
		if c == '"' {
			l.pos++
			return b.String(), nil
		}
		if c < 0x20 {
			return "", l.erro("caractere de controle solto dentro do texto")
		}
		if c != '\\' {
			b.WriteByte(c)
			l.pos++
			continue
		}
		l.pos++
		if l.pos >= len(l.s) {
			return "", l.erro("escape sem nada depois")
		}
		switch l.s[l.pos] {
		case '"':
			b.WriteByte('"')
		case '\\':
			b.WriteByte('\\')
		case '/':
			b.WriteByte('/')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			r, err := l.escapeUnicode()
			if err != nil {
				return "", err
			}
			b.WriteRune(r)
			continue // escapeUnicode ja andou com o pos
		default:
			return "", l.erro("escape desconhecido")
		}
		l.pos++
	}
	return "", l.erro("texto sem aspa de fechamento")
}

// escapeUnicode le `\uXXXX` (l.pos esta no 'u'), juntando o par surrogate
// quando vier um \uD800-\uDBFF seguido do par de baixo.
func (l *leitorJson) escapeUnicode() (rune, error) {
	r, err := l.hex4()
	if err != nil {
		return 0, err
	}
	if !utf16.IsSurrogate(r) {
		return r, nil
	}
	// tenta o par de baixo: \uXXXX logo em seguida
	if l.pos+1 < len(l.s) && l.s[l.pos] == '\\' && l.s[l.pos+1] == 'u' {
		marca := l.pos
		l.pos++ // fica no 'u'
		r2, err := l.hex4()
		if err != nil {
			return 0, err
		}
		if junto := utf16.DecodeRune(r, r2); junto != utf8.RuneError {
			return junto, nil
		}
		l.pos = marca // nao formou par: volta e devolve o replacement
	}
	return utf8.RuneError, nil
}

// hex4 le os 4 digitos hex depois do 'u' e deixa o pos logo apos eles.
func (l *leitorJson) hex4() (rune, error) {
	if l.pos+4 >= len(l.s) {
		return 0, l.erro("escape \\u incompleto")
	}
	n, err := strconv.ParseUint(l.s[l.pos+1:l.pos+5], 16, 32)
	if err != nil {
		return 0, l.erro("escape \\u com digito invalido")
	}
	l.pos += 5
	return rune(n), nil
}

// numero le um numero JSON seguindo a gramatica a risca:
// `-`? ( `0` | [1-9][0-9]* ) ( `.` [0-9]+ )? ( [eE] [+-]? [0-9]+ )?
// Inteiro que cabe em int64 vira Numero exato (EhInt), igual ao resto da
// linguagem; o resto vira float.
func (l *leitorJson) numero() (object.Object, error) {
	ini := l.pos
	if l.pos < len(l.s) && l.s[l.pos] == '-' {
		l.pos++
	}
	// parte inteira: um unico `0`, ou digito 1-9 seguido de mais digitos
	if l.pos >= len(l.s) {
		return nil, l.erro("numero invalido")
	}
	if l.s[l.pos] == '0' {
		l.pos++
	} else if l.s[l.pos] >= '1' && l.s[l.pos] <= '9' {
		l.pulaDigitos()
	} else {
		return nil, l.erro("numero invalido")
	}
	// JSON nao aceita zero a esquerda (`01`) nem digito colado depois do zero
	if l.pos < len(l.s) && l.s[l.pos] >= '0' && l.s[l.pos] <= '9' {
		return nil, l.erro("numero nao pode comecar com zero a esquerda")
	}

	inteiro := true
	if l.pos < len(l.s) && l.s[l.pos] == '.' {
		inteiro = false
		l.pos++
		if !l.temDigito() {
			return nil, l.erro("faltou digito depois do ponto")
		}
		l.pulaDigitos()
	}
	if l.pos < len(l.s) && (l.s[l.pos] == 'e' || l.s[l.pos] == 'E') {
		inteiro = false
		l.pos++
		if l.pos < len(l.s) && (l.s[l.pos] == '+' || l.s[l.pos] == '-') {
			l.pos++
		}
		if !l.temDigito() {
			return nil, l.erro("faltou digito no expoente")
		}
		l.pulaDigitos()
	}

	bruto := l.s[ini:l.pos]
	if inteiro {
		if n, err := strconv.ParseInt(bruto, 10, 64); err == nil {
			return object.NumInt(n), nil
		}
	}
	f, err := strconv.ParseFloat(bruto, 64)
	if err != nil {
		return nil, l.erro("numero invalido: " + bruto)
	}
	return &object.Numero{Value: f}, nil
}

func (l *leitorJson) temDigito() bool {
	return l.pos < len(l.s) && l.s[l.pos] >= '0' && l.s[l.pos] <= '9'
}

func (l *leitorJson) pulaDigitos() {
	for l.temDigito() {
		l.pos++
	}
}
