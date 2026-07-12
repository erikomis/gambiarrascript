package repl

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"gambiarrascript/interpreter"
	"gambiarrascript/object"
)

// palavrasChave sao as keywords da linguagem, pro autocomplete do REPL.
var palavrasChave = []string{
	"bota", "mostra", "se_colar", "se_nao_colar", "enquanto", "pra_cada",
	"de", "ate", "em", "gambiarra", "funciona", "arruma", "quebrou",
	"finalmente", "vaza", "continua", "deu_bom", "deu_ruim", "nada",
	"acabou_finalmente", "e", "ou", "nao", "importa", "como", "bora",
	"escolhe", "caso", "entao",
}

// ehIdentChar diz se o byte faz parte de um identificador (ASCII + digitos + _).
func ehIdentChar(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// palavraAntes devolve o identificador que termina em `pos` e o indice onde ele
// comeca (pra saber o que substituir ao completar).
func palavraAntes(linha string, pos int) (string, int) {
	if pos > len(linha) {
		pos = len(linha)
	}
	if pos < 0 {
		pos = 0
	}
	i := pos
	for i > 0 && ehIdentChar(linha[i-1]) {
		i--
	}
	return linha[i:pos], i
}

// candidatosCompletar filtra os nomes que comecam com `prefixo` (excluindo o
// proprio prefixo exato), ordenados.
func candidatosCompletar(prefixo string, nomes []string) []string {
	if prefixo == "" {
		return nil
	}
	var out []string
	for _, n := range nomes {
		if n != prefixo && strings.HasPrefix(n, prefixo) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// prefixoComum devolve o maior prefixo comum a todas as strings.
func prefixoComum(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	p := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, p) {
			p = p[:len(p)-1]
			if p == "" {
				return ""
			}
		}
	}
	return p
}

// autocompleta tenta completar a palavra sob o cursor em `pos`. Devolve a linha
// nova, a posicao nova do cursor e se houve completacao. Com 1 candidato,
// completa ele; com varios, avanca ate o maior prefixo comum.
func autocompleta(linha string, pos int, candidatos []string) (string, int, bool) {
	palavra, inicio := palavraAntes(linha, pos)
	cands := candidatosCompletar(palavra, candidatos)
	if len(cands) == 0 {
		return "", 0, false
	}
	var completa string
	if len(cands) == 1 {
		completa = cands[0]
	} else {
		completa = prefixoComum(cands)
	}
	if completa == "" || completa == palavra {
		return "", 0, false
	}
	nova := linha[:inicio] + completa + linha[pos:]
	return nova, inicio + len(completa), true
}

// nomesCompletaveis junta keywords + builtins + variaveis do escopo atual.
func nomesCompletaveis(interp *interpreter.Interpreter, env *object.Environment) []string {
	nomes := append([]string{}, palavrasChave...)
	for nome := range interp.BuiltinsVisiveis() {
		nomes = append(nomes, nome)
	}
	nomes = append(nomes, env.Locais()...)
	return nomes
}

const textoAjuda = "Comandos do REPL:\n" +
	"  :ajuda        mostra esta ajuda\n" +
	"  :limpa        limpa a tela\n" +
	"  setas ↑/↓     navega o historico\n" +
	"  TAB           completa builtin/keyword/variavel\n" +
	"  ctrl+d        vaza (sai)\n"

// trataComando trata linhas que comecam com `:`. Devolve true se a linha era um
// comando (tratado aqui) — o REPL nao deve avaliar como codigo.
func trataComando(linha string, out io.Writer) bool {
	cmd := strings.TrimSpace(linha)
	if !strings.HasPrefix(cmd, ":") {
		return false
	}
	switch cmd {
	case ":ajuda", ":help":
		fmt.Fprint(out, textoAjuda)
	case ":limpa", ":clear":
		fmt.Fprint(out, "\x1b[2J\x1b[H") // limpa tela + cursor no topo
	default:
		fmt.Fprintf(out, "comando desconhecido: %s (tenta :ajuda)\n", cmd)
	}
	return true
}
