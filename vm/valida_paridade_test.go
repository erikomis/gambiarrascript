package vm

import "testing"

// valida(valor, esquema): mesma lista de erros nos dois engines.
func TestValidaParidade(t *testing.T) {
	esq := `bota esq = {
    "nome": {"tipo": "texto", "min": 2, "max": 10},
    "idade": "inteiro",
    "apelido": "texto?",
    "email": "email?",
    "nasc": "data?",
    "ativo": "booleano?",
    "status": {"opcoes": ["aberta", "fechada"]},
    "itens": {"tipo": "lista", "min": 1, "itens": {"preco": "numero", "qtd": {"tipo": "inteiro", "min": 1}}},
    "endereco": {"rua": "texto", "cep": {"tipo": "texto", "padrao": "^\\d{5}-\\d{3}$"}, "_estrito": deu_bom},
}
`
	casos := []struct{ src, saida, erro string }{
		{esq + `mostra valida({"nome": "Jurandir", "idade": 30, "status": "aberta", "itens": [{"preco": 1.5, "qtd": 2}], "endereco": {"rua": "a", "cep": "12345-678"}}, esq)`, "[]\n", ""},
		{esq + `pra_cada x em valida({"nome": "Z", "idade": "trinta", "email": "fulano", "nasc": "ontem", "ativo": 1, "status": "xyz", "itens": [{"preco": 1, "qtd": 2}, {"qtd": 0}, {"preco": "1", "qtd": 1.5}], "endereco": {"cep": "1", "extra": 1}}, esq)
    mostra x
acabou_finalmente`, `nome: tem que ter pelo menos 2 caracteres, veio 1
idade: tem que ser inteiro, veio texto
email: tem que ser email valido, veio "fulano"
nasc: tem que ser data ISO (AAAA-MM-DD), veio "ontem"
ativo: tem que ser booleano, veio numero
status: tem que ser um de "aberta", "fechada", veio "xyz"
itens[1].preco: obrigatorio
itens[1].qtd: tem que ser no minimo 1, veio 0
itens[2].preco: tem que ser numero, veio texto
itens[2].qtd: tem que ser inteiro, veio 1.5
endereco.rua: obrigatorio
endereco.cep: nao bate com o padrao ^\d{5}-\d{3}$
endereco.extra: campo nao esperado
`, ""},
		// lista vazia, campo ausente e nada (null do JSON) contam como faltando
		{esq + `mostra valida({"nome": nada, "itens": [], "endereco": {"rua": "r", "cep": "00000-000"}}, esq)`,
			"[nome: obrigatorio, idade: obrigatorio, status: obrigatorio, itens: tem que ter pelo menos 1 item, veio 0]\n", ""},
		{`mostra valida({"tags": ["a", "bb", 3]}, {"tags": {"itens": {"tipo": "texto", "max": 1}}})`,
			"[tags[1]: tem que ter no maximo 1 caractere, veio 2, tags[2]: tem que ser texto, veio numero]\n", ""},
		{`mostra valida({"n": 5, "x": 1}, {"n": {"tipo": "numero", "min": 0, "max": 3}})`, "[n: tem que ser no maximo 3, veio 5]\n", ""},
		{`mostra valida({}, {"a": "texto?", "b": {"tipo": "numero", "obrigatorio": deu_ruim}})`, "[]\n", ""},
		{`mostra valida({"x": 1}, {"_estrito": deu_bom})`, "[x: campo nao esperado]\n", ""},
		{`mostra valida(5, "texto")`, "[tem que ser texto, veio numero]\n", ""},
		{`mostra valida([1], {"a": "texto"})`, "[tem que ser dicionario, veio lista]\n", ""},
		{`mostra valida({"d": "2024-02-30"}, {"d": "data"})`, "[d: tem que ser data ISO (AAAA-MM-DD), veio \"2024-02-30\"]\n", ""},
		{`mostra valida({"d": "2024-02-29T10:00:00Z", "n": 2.0}, {"d": "data", "n": "inteiro"})`, "[]\n", ""},
		// esquema errado quebra (nao e erro de validacao)
		{`valida({}, {"a": "numeros"})`, "", `tipo desconhecido "numeros"`},
		{`valida({}, {"a": {"tipo": "texto", "padrao": "("}})`, "", `regex do "padrao" nao compila`},
		{`valida({}, 3)`, "", "o esquema tem que ser dicionario"},
	}
	for _, c := range casos {
		esperaNosDois(t, c.src, c.saida, c.erro)
	}
}
