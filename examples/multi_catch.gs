# multi-catch: varios `quebrou NOME se CONDICAO` no mesmo arruma.
# As clausulas sao tentadas em ordem e a primeira cujo filtro colar pega;
# o `quebrou` sem filtro (se tiver) tem que ser o ultimo e pega o resto.
# Erro que nenhuma clausula pegou continua subindo (depois do finalmente).

# levanta um erro do tipo pedido (envolve_erro da o tipo que a gente quiser)
gambiarra falha(tipo)
    se_colar tipo == "conta"
        funciona 1 / 0
    acabou_finalmente
    arruma
        quebra("na origem")
    quebrou origem
        envolve_erro(tipo, "falhou: " + tipo, origem)
    acabou_finalmente
acabou_finalmente

gambiarra trata(tipo)
    arruma
        falha(tipo)
    quebrou erro se erro_tipo(erro) == "rede"
        funciona "rede: tenta de novo depois"
    quebrou erro se erro_tipo(erro) == "jwt" ou erro_tipo(erro) == "auth"
        funciona "401: sem permissao"
    quebrou erro
        funciona "500: " + erro_tipo(erro)  # pega o resto
    finalmente
        mostra "  (trata ${tipo}: fim)"  # roda uma vez so, pegou ou nao
    acabou_finalmente
acabou_finalmente

pra_cada t em ["rede", "jwt", "auth", "conta"]
    mostra trata(t)
acabou_finalmente

# o filtro e qualquer expressao: aqui olha a causa do erro
arruma
    falha("config")
quebrou erro se erro_causa(erro) != nada e contem(erro_msg(erro_causa(erro)), "origem")
    mostra "veio da origem: " + erro_msg(erro_causa(erro))
acabou_finalmente

# sem pega-resto: o que nenhum filtro aceitar sobe pro arruma de fora
gambiarra so_rede()
    arruma
        falha("io")
    quebrou erro se erro_tipo(erro) == "rede"
        mostra "nao chega aqui"
    acabou_finalmente
acabou_finalmente

arruma
    so_rede()
quebrou erro
    mostra "subiu pro de fora: " + erro_tipo(erro)
acabou_finalmente
