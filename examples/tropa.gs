bota tropa = ["Erik", "Ana", "Joao", "Bia"]

mostra "chamando a tropa:"
pra_cada nome em tropa
    mostra "  salve, " + nome
acabou_finalmente

gambiarra conta(lista)
    bota total = 0
    pra_cada item em lista
        bota total = total + 1
    acabou_finalmente
    funciona total
acabou_finalmente

mostra "tem " + conta(tropa) + " na call"
