package interpreter

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gambiarrascript/object"
)

// Logging estruturado no stderr:
//
//	log_debug(msg, [campos]) / log_info / log_aviso / log_erro
//
// Texto (padrao), uma linha por chamada:
//
//	2026-10-02T12:00:00-03:00 INFO subiu porta=8080 modo=dev
//
// GS_LOG_FORMATO=json → um objeto JSON por linha (pra coletor de log):
//
//	{"time":"2026-10-02T12:00:00.000-03:00","nivel":"info","msg":"subiu","campos":{"porta":8080}}
//
// GS_LOG_NIVEL = debug | info | aviso | erro (padrao info) corta o que for
// abaixo. As duas env sao lidas a cada chamada, entao um carrega_env() no
// comeco do script ja vale. Nao usa log/slog: o formato pedido (nivel em
// portugues, chaves time/nivel/msg/campos, dict na ordem de insercao) daria
// mais trabalho de dobrar o slog do que escrever as 2 saidas na mao.

type nivelLog int

const (
	nivelDebug nivelLog = iota
	nivelInfo
	nivelAviso
	nivelErro
)

var nomesNivel = [...]string{"debug", "info", "aviso", "erro"}

// agoraLog e o relogio do log (variavel pros testes congelarem o tempo).
var agoraLog = time.Now

// nivelMinimo le GS_LOG_NIVEL. Aceita os apelidos em ingles tambem; valor
// desconhecido cai no padrao (info) em vez de calar o log sem querer.
func nivelMinimo() nivelLog {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GS_LOG_NIVEL"))) {
	case "debug":
		return nivelDebug
	case "aviso", "warn", "warning":
		return nivelAviso
	case "erro", "error":
		return nivelErro
	default:
		return nivelInfo
	}
}

func (i *Interpreter) builtinLogDebug(args []object.Object) object.Object {
	return i.loga(nivelDebug, args)
}
func (i *Interpreter) builtinLogInfo(args []object.Object) object.Object {
	return i.loga(nivelInfo, args)
}
func (i *Interpreter) builtinLogAviso(args []object.Object) object.Object {
	return i.loga(nivelAviso, args)
}
func (i *Interpreter) builtinLogErro(args []object.Object) object.Object {
	return i.loga(nivelErro, args)
}

func (i *Interpreter) loga(nivel nivelLog, args []object.Object) object.Object {
	nome := "log_" + nomesNivel[nivel]
	if len(args) < 1 || len(args) > 2 {
		return erroBuiltin("%s() quer (mensagem, [campos]), veio %d", nome, len(args))
	}
	var campos *object.Dicionario
	if len(args) == 2 {
		d, ok := args[1].(*object.Dicionario)
		if !ok {
			return erroBuiltin("%s: campos tem que ser dicionario, veio %s", nome, args[1].Type())
		}
		campos = d
	}
	if nivel < nivelMinimo() {
		return NADA
	}
	msg := textoDe(args[0])
	agora := agoraLog()

	// monta a linha inteira antes e escreve de uma vez so, segurando o
	// muOut: varias goroutines (bora, handlers do rota) logando ao mesmo
	// tempo nao embolam pedaco de uma linha no meio da outra.
	var buf bytes.Buffer
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GS_LOG_FORMATO")), "json") {
		if erro := linhaLogJSON(&buf, agora, nivel, msg, campos); erro != nil {
			return erro
		}
	} else {
		linhaLogTexto(&buf, agora, nivel, msg, campos)
	}

	i.muOut.Lock()
	defer i.muOut.Unlock()
	w := i.erroOut
	if w == nil {
		w = os.Stderr
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return erroBuiltinKind(KindIO, "%s: nao consegui escrever no stderr: %v", nome, err)
	}
	return NADA
}

func textoDe(o object.Object) string {
	if t, ok := o.(*object.Texto); ok {
		return t.Value
	}
	return o.Inspect()
}

func linhaLogTexto(buf *bytes.Buffer, agora time.Time, nivel nivelLog, msg string, campos *object.Dicionario) {
	buf.WriteString(agora.Format(time.RFC3339))
	buf.WriteByte(' ')
	buf.WriteString(strings.ToUpper(nomesNivel[nivel]))
	buf.WriteByte(' ')
	// mensagem com quebra de linha viraria duas "entradas" no coletor (e
	// abre porta pra forjar linha de log): escapa os controles.
	buf.WriteString(escapaControles(msg))
	if campos != nil {
		campos.Itera(func(par object.ParDic) {
			buf.WriteByte(' ')
			buf.WriteString(valorLogfmt(textoDe(par.Chave)))
			buf.WriteByte('=')
			buf.WriteString(valorLogfmt(textoDe(par.Valor)))
		})
	}
	buf.WriteByte('\n')
}

// valorLogfmt devolve o valor cru se for "limpo"; com espaco, aspas, '=' ou
// controle vai entre aspas (estilo logfmt) — da pra parsear de volta.
func valorLogfmt(s string) string {
	if s == "" {
		return `""`
	}
	for _, r := range s {
		if r == ' ' || r == '"' || r == '=' || r == '\\' || !unicode.IsPrint(r) {
			return strconv.Quote(s)
		}
	}
	return s
}

func escapaControles(s string) string {
	precisa := false
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			precisa = true
			break
		}
	}
	if !precisa {
		return s
	}
	q := strconv.Quote(s)
	return q[1 : len(q)-1]
}

func linhaLogJSON(buf *bytes.Buffer, agora time.Time, nivel nivelLog, msg string, campos *object.Dicionario) *object.Erro {
	buf.WriteString(`{"time":`)
	escreveTextoJson(buf, agora.Format("2006-01-02T15:04:05.000Z07:00"))
	buf.WriteString(`,"nivel":`)
	escreveTextoJson(buf, nomesNivel[nivel])
	buf.WriteString(`,"msg":`)
	escreveTextoJson(buf, msg)
	buf.WriteString(`,"campos":`)
	if campos == nil {
		buf.WriteString("{}")
	} else {
		// valor que nao vira JSON (gambiarra, cano...) sai como texto em vez
		// de derrubar o log: logar nunca deveria ser o que quebra o programa.
		limpo := object.NovoDicionario()
		campos.Itera(func(par object.ParDic) {
			v := par.Valor
			var tmp bytes.Buffer
			if escreveJson(&tmp, v) != nil {
				v = &object.Texto{Value: v.Inspect()}
			}
			k := &object.Texto{Value: textoDe(par.Chave)}
			limpo.Bota(k.ChaveHash(), object.ParDic{Chave: k, Valor: v})
		})
		if erro := escreveJson(buf, limpo); erro != nil {
			return erro
		}
	}
	buf.WriteString("}\n")
	return nil
}
