# Programa interativo: le do teclado com pergunta()
mostra "Bem-vindo a GambiarraScript"
bota nome = pergunta("Qual teu nome? ")
mostra "Eai " + nome + ", bora tirar uma duvida"

bota chute = numero(pergunta("Quanto e 6 * 7? "))
se_colar chute == 42
    mostra "Acertou, " + nome + "!"
se_nao_colar
    mostra "Quase! era 42, parca"
acabou_finalmente