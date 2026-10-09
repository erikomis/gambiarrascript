# 200 mil chaves texto: insere tudo, depois le tudo de volta
bota d = {}
pra_cada i de 0 ate 199999
    bota d["k" + texto(i)] = i
acabou_finalmente
bota s = 0
pra_cada i de 0 ate 199999
    s += d["k" + texto(i)]
acabou_finalmente
mostra tamanho(d)
mostra s
