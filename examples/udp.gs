# UDP: servidor + cliente no mesmo script (precisa de rede — nao roda no playground)
# rode com: gs roda examples/udp.gs
# cada datagrama e uma mensagem inteira: sem \n, sem conexao, sem garantia.

# o handler recebe (mensagem, remetente); o que ele devolver volta pro remetente
gambiarra grita(msg, remetente)
    mostra "servidor: chegou \"" + msg + "\""
    se_colar msg == "psiu"
        funciona nada  # nada = nao responde
    acabou_finalmente
    funciona maiusculo(msg) + "!"
acabou_finalmente

# porta 0 = o sistema escolhe; o endereco de verdade chega pelo cano `pronto`.
# o cano `para` derruba o servidor quando alguem fecha ele.
bota pronto = cano(1)
bota para = cano()
bota servidor = bora escuta_udp("127.0.0.1:0", grita, {"pronto": pronto, "para": para})
bota end = recebe(pronto)

# conecta_udp: um "cano" de datagramas com aquele endereco
bota u = conecta_udp(end, {"timeout": 0.5})
envia(u, "salve")
mostra "cliente: " + recebe(u)

# envia_udp: manda e esquece
envia_udp(end, "psiu")
espera_ms(100)

# ninguem responde "psiu" de volta pro u: o timeout estoura com erro "rede"
envia(u, "psiu")
arruma
    recebe(u)
quebrou erro
    mostra "cliente: " + erro_tipo(erro) + " — " + erro_msg(erro)
acabou_finalmente

fecha(u)
fecha(para)
espera(servidor)
mostra "fim"
