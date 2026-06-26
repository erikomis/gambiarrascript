# Argumentos de linha de comando
# Rode com: gs roda examples/argumentos.gs um dois treis
bota args = argumentos()
mostra "recebi " + tamanho(args) + " argumentos"
pra_cada a em args
    mostra "- " + a
acabou_finalmente