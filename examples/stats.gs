# munging de dados: soma, media, zip, enumera

bota notas = [7, 8, 10, 6, 9]
mostra "soma: " + soma(notas)      # soma: 40
mostra "media: " + media(notas)    # media: 8

# soma preserva inteiro; com float vira float
mostra soma([1, 2, 0.5])           # 3.5

# zip casa duas listas em pares (para no menor)
bota nomes = ["Ze", "Rita", "Ana"]
bota idades = [30, 25, 40]
pra_cada par em zip(nomes, idades)
    mostra par[0] + " tem " + par[1]
acabou_finalmente

# enumera dá [indice, valor] pra iterar com o indice
pra_cada iv em enumera(nomes)
    mostra iv[0] + ": " + iv[1]
acabou_finalmente
