# CSV — le e escreve arquivos .csv (lista de dicionarios <-> arquivo)

bota pessoas = [
    {"nome": "joao", "idade": "30", "cidade": "Sao Paulo"},
    {"nome": "maria", "idade": "25", "cidade": "Rio de Janeiro"},
    {"nome": "ana", "idade": "40", "cidade": "Belo Horizonte"}
]

# escreve: cabecalho vem das chaves do primeiro dict (ordem nao deterministica)
escreve_csv("/tmp/pessoas.csv", pessoas)

# le de volta: primeira linha vira cabecalho, resto vira dicts
bota lidos = le_csv("/tmp/pessoas.csv")
mostra "linhas: ${tamanho(lidos)}"
pra_cada p em lidos
    mostra "  ${p["nome"]} tem ${p["idade"]} anos e mora em ${p["cidade"]}"
acabou_finalmente

# com cabecalho custom (reordena as colunas)
escreve_csv("/tmp/pessoas2.csv", pessoas, ["cidade", "nome", "idade"])
bota lidos2 = le_csv("/tmp/pessoas2.csv")
mostra "cabecalho custom, segunda linha: ${lidos2[1]["nome"]} de ${lidos2[1]["cidade"]}"

# escreve CSV com virgula no valor (encoding automatico)
bota mistureba = [
    {"fruta": "banana, nanica", "preco": "1,99"},
    {"fruta": "uva", "preco": "5,50"}
]
escreve_csv("/tmp/frutas.csv", mistureba, ["fruta", "preco"])
bota lidos3 = le_csv("/tmp/frutas.csv")
mostra "fruta[0]: ${lidos3[1]["fruta"]} (preco: ${lidos3[1]["preco"]})"