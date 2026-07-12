# indice negativo (estilo Python) e indexacao de texto

bota xs = [10, 20, 30, 40]
mostra xs[-1]        # 40 (ultimo)
mostra xs[-2]        # 30 (penultimo)

# atribuicao com indice negativo tambem funciona
xs[-1] += 5
mostra xs            # [10, 20, 30, 45]

# indexacao de texto (rune-aware: conta caractere, nao byte)
bota nome = "café"
mostra nome[0]       # c
mostra nome[-1]      # é
mostra nome[3]       # é (4o caractere, mesmo sendo multibyte)
