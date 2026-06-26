gambiarra produtor(c)
    pra_cada i de 1 ate 3
        mostra "produzi " + i
        envia(c, i)
    acabou_finalmente
    fecha(c)
acabou_finalmente

bota c = cano(2)
bora produtor(c)

# consumidor: recebe ate voltar nada (cano fechado e vazio)
enquanto deu_bom
    bota v = recebe(c)
    se_colar v == nada
        vaza
    acabou_finalmente
    mostra "consumi " + v
acabou_finalmente
mostra "fim"