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

# ordena_por: ordena lista de dicts por campo (lista nova, nao mexe na original)
bota gente = [{"n": "Ana", "idade": 30}, {"n": "Ze", "idade": 20}, {"n": "Rita", "idade": 25}]
pra_cada p em ordena_por(gente, "idade")
    mostra p.n + " (" + p.idade + ")"
acabou_finalmente

# agrupa_por: agrupa por chave calculada por uma gambiarra
bota grupos = agrupa_por([1, 2, 3, 4, 5, 6], gambiarra(n) funciona n % 2 acabou_finalmente)
mostra "pares: " + grupos[0]
mostra "impares: " + grupos[1]
