package vm

import "testing"

// Conta inteira que estoura o int64 vira real (float64) — a mesma regra nos
// dois motores e no constant folding (object.SomaInt & cia). Antes a VM
// caia no float em parte das contas e dava a volta em outras (-min), e o
// tree-walker dava a volta em todas (max + 1 virava o minimo negativo).
func TestEstouroDeInteiroViraReal(t *testing.T) {
	const pre = `bota max = 9223372036854775807
bota min = -9223372036854775807 - 1
`
	casos := []struct{ nome, src, saida, erro string }{
		{"soma", pre + `mostra max + 1`, "9223372036854776000\n", ""},
		{"subtracao", pre + `mostra min - 1`, "-9223372036854776000\n", ""},
		{"multiplicacao", pre + `mostra max * 2
mostra 3037000500 * 3037000500
mostra -3037000500 * 3037000500
mostra 3037000499 * 3037000499`, "18446744073709552000\n9223372037000250000\n-9223372037000250000\n9223372030926249001\n", ""},
		{"menos na frente", pre + `mostra -min
mostra -max`, "9223372036854776000\n-9223372036854775807\n", ""},
		{"divisao e resto", pre + `mostra min / -1
mostra min % -1
mostra (4 / 2) & 1
mostra (7 % 3) & 1
mostra 7 / 2`, "9223372036854776000\n0\n0\n1\n3.5\n", ""},
		{"composta e laco", pre + `bota x = max
x += 1
mostra x
gambiarra soma_tudo(xs)
    bota t = 0
    pra_cada v em xs
        t += v
    acabou_finalmente
    funciona t
acabou_finalmente
mostra soma_tudo([max, 1, -1])`, "9223372036854776000\n9223372036854776000\n", ""},
		{"real continua real", pre + `mostra max + 1 - 1
mostra "${max + 1}"
mostra texto(min - 1)`, "9223372036854776000\n9223372036854776000\n-9223372036854776000\n", ""},
		{"bit em real e erro", pre + `arruma
    mostra (max + 1) & 1
quebrou e1
    mostra erro_msg(e1)
acabou_finalmente`, "deu ruim na linha 4: & bitwise so faz sentido com inteiros\n", ""},
		{"constante dobrada", `mostra 9223372036854775807 + 1
mostra 4611686018427387904 * 2
mostra -9223372036854775807 - 2
mostra 4611686018427387904 * 2 - 1`, "9223372036854776000\n9223372036854776000\n-9223372036854776000\n9223372036854776000\n", ""},
		{"comparacao de inteiro grande e exata", `mostra 9007199254740993 == 9007199254740992
mostra 9007199254740993 > 9007199254740992
mostra 9007199254740993 != 9007199254740992`, "deu_ruim\ndeu_bom\ndeu_bom\n", ""},
		{"de_json gigante vira real", `bota v = de_json("[99999999999999999999, 9223372036854775807, -9223372036854775809]")
mostra v
mostra v[1] - 1`, "[100000000000000000000, 9223372036854775807, -9223372036854776000]\n9223372036854775806\n", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			esperaNosDois(t, c.src, c.saida, c.erro)
		})
	}
}
