# monta um texto de 100 mil caracteres concatenando um por vez
bota s = ""
pra_cada i de 0 ate 99999
    s += texto(i % 10)
acabou_finalmente
mostra tamanho(s)
mostra s[-10:]
