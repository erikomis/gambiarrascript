# crava: constante — depois de cravar, ninguem muda o nome

crava TAXA = 0.1
crava DIAS = ["seg", "ter", "qua", "qui", "sex"]

mostra "taxa: " + TAXA

# `bota TAXA = 0.2` ou `TAXA += 1` aqui embaixo nem roda:
# "`TAXA` foi cravada, nao da pra mudar" (o `gs check` tambem acusa)

# o nome e fixo, mas a lista cravada ainda muda por dentro (igual const do JS)
adiciona(DIAS, "sab")
bota DIAS[0] = "SEG"
mostra DIAS

# dentro de uma gambiarra o escopo e outro: bota cria um local que sombreia
gambiarra com_desconto(preco)
    bota TAXA = 0.5
    funciona preco * (1 - TAXA)
acabou_finalmente
mostra com_desconto(100)       # 50
mostra TAXA                    # 0.1 (a de fora continua cravada)

# crava local de gambiarra: vale so la dentro, a cada chamada
gambiarra juros(valor, meses)
    crava FATOR = (1 + TAXA) ** meses
    funciona valor * FATOR
acabou_finalmente
mostra formata("%.2f", juros(1000, 12))

# crava dentro de laco tambem rola: e o mesmo crava a cada volta
pra_cada i de 1 ate 3
    crava QUADRADO = i ** 2
    mostra QUADRADO
acabou_finalmente
