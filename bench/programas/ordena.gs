# 300 mil inteiros pseudo-aleatorios (MINSTD, semente 42 — o mesmo gerador
# nas 3 linguagens, pra ordenar exatamente a mesma lista) e um ordena
bota x = 42
bota xs = []
pra_cada i de 1 ate 300000
    bota x = (x * 48271) % 2147483647
    adiciona(xs, x % 1000000)
acabou_finalmente
ordena(xs)
mostra xs[0]
mostra xs[150000]
mostra xs[299999]
