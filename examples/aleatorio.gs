# aleatoriedade: semente (reprodutivel), embaralha, escolhe_um, uuid

# fixando a semente, a sequencia sai igual toda vez
semente(42)
bota baralho = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
mostra embaralha(baralho)          # sempre o mesmo com semente 42
mostra "original intacto: " + baralho

# escolhe um item aleatorio
bota cores = ["vermelho", "verde", "azul", "amarelo"]
mostra "sorteada: " + escolhe_um(cores)

# uuid v4
mostra uuid()
