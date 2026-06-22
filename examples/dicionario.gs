# dicionario: o jeito gambiarra de guardar coisas com nome
bota pessoa = {"nome": "Erik", "idade": 25, "dev": deu_bom}

mostra "nome: " + pessoa["nome"]
mostra "idade: " + texto(pessoa["idade"])

# mexendo no dicionario
bota pessoa["cidade"] = "Sao Paulo"
mostra "tem cidade? " + texto(tem(pessoa, "cidade"))
mostra "quantos campos: " + texto(tamanho(pessoa))

# percorrendo as chaves
mostra "campos:"
pra_cada chave em pessoa
    mostra "  " + chave + " = " + texto(pessoa[chave])
acabou_finalmente

# chave que nao existe vem nada
mostra pessoa["sobrenome"]
