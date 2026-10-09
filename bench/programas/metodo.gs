# 1 milhao de chamadas de metodo numa treta (struct + receiver)
treta Contador
    n = 0
acabou_finalmente

gambiarra (c Contador) soma(k)
    c.n += k
acabou_finalmente

bota c = Contador{}
pra_cada i de 0 ate 999999
    c.soma(i % 3)
acabou_finalmente
mostra c.n
