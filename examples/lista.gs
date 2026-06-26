# Funcoes de lista: mapeia, filtra, adiciona, ordena...
bota nums = [5, 2, 8, 1, 9]

adiciona(nums, 3)
ordena(nums)
mostra "ordenados: " + junta(nums, ", ")
inverte(nums)
mostra "invertidos: " + junta(nums, ", ")

remove(nums, 9)
mostra "sem o 9: " + junta(nums, ", ")

gambiarra dobra(n)
    funciona n * 2
acabou_finalmente
mostra mapeia(nums, dobra)

gambiarra par(n)
    funciona n % 2 == 0
acabou_finalmente
mostra filtra(nums, par)