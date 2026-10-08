# cliente TCP que conversa com o examples/eco_tcp.gs (precisa de rede)
# rode com: gs roda examples/cliente_tcp.gs [host:porta] [mensagens...]
#   ex.: gs roda examples/cliente_tcp.gs 127.0.0.1:9000 oi tudo bem

bota args = argumentos()
bota alvo = "127.0.0.1:9000"
bota mensagens = ["salve", "tudo certo?", "falou"]
se_colar tamanho(args) > 0
    bota alvo = args[0]
acabou_finalmente
se_colar tamanho(args) > 1
    bota mensagens = args[1:]
acabou_finalmente

arruma
    # timeout: se o servidor ficar mudo 5s, o recebe quebra com erro "rede"
    bota c = conecta_tcp(alvo, {"timeout": 5})
    mostra "< " + recebe(c)  # a saudacao do servidor
    pra_cada m em mensagens
        envia(c, m)  # vai com \n no fim (modo linha)
        mostra "> " + m
        mostra "< " + recebe(c)
    acabou_finalmente
    envia(c, "tchau")
    mostra recebe(c) ?? "(servidor desligou)"  # nada = o outro lado fechou
    fecha(c)
quebrou erro se erro_tipo(erro) == "rede"
    mostra "rede zuada: " + erro_msg(erro)
quebrou erro
    mostra "deu ruim: " + erro_msg(erro)
acabou_finalmente
