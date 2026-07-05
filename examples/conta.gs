# conta quantos itens tem num array

bota tropa = ["Erik", "Ana", "Joao", "Bia"]

# jeito nativo, ja vem de fabrica na linguagem
mostra "nativo: " + tamanho(tropa)

# jeito na raca: sua propria funca contando na mao
gambiarra conta(lista)
    bota total = 0
    pra_cada item em lista
        bota total = total + 1
    acabou_finalmente
    funciona total
acabou_finalmente

mostra "na raca: " + conta(tropa)
mostra "lista vazia: " + conta([])
mostra "numeros: " + conta([10, 20, 30, 40, 50])
