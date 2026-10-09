# le um JSON de ~1 MB e faz 10 voltas de parse + serializa
bota bruto = le_arquivo(argumentos()[0])
bota saida = ""
bota n = 0
pra_cada volta de 1 ate 10
    bota doc = de_json(bruto)
    bota n = tamanho(doc["itens"])
    bota saida = pra_json(doc)
acabou_finalmente
mostra n
mostra tamanho(saida)
mostra saida == bruto
