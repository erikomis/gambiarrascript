# Funcoes de texto da GambiarraScript
bota frase = "  GambiarraScript e 10/10  "
bota limpa = tira_espaco(frase)

mostra limpa
mostra maiusculo(limpa)
mostra minusculo(limpa)
mostra contem(limpa, "10/10")
mostra comeca_com(limpa, "Gambiarra")
mostra termina_com(limpa, "10/10")

bota partes = separa("salve,tropa,da,gambiarra", ",")
mostra partes
mostra junta(partes, " | ")
mostra substitui("banana", "a", "o")
mostra fatia("gambiarra", 0, 4)