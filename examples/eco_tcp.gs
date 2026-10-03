# servidor de eco TCP, uma linha por vez (precisa de rede — nao roda no playground)
# rode com: gs roda examples/eco_tcp.gs [porta]   (padrao 9000)
# e fale com ele: nc 127.0.0.1 9000   ou   gs roda examples/cliente_tcp.gs
# cada conexao roda na propria goroutine; ctrl+c fecha tudo com calma.

bota args = argumentos()
bota porta = 9000
se_colar tamanho(args) > 0
    bota porta = numero(args[0])
acabou_finalmente

gambiarra atende(c)
    bota quem = endereco(c)
    mostra "chegou " + quem
    envia(c, "salve, " + quem + "! manda que eu devolvo (\"tchau\" encerra)")
    enquanto deu_bom
        bota linha = recebe(c)  # nada = o outro lado desligou
        se_colar linha == nada ou linha == "tchau"
            vaza
        acabou_finalmente
        envia(c, "eco: " + linha)
    acabou_finalmente
    mostra "saiu " + quem
acabou_finalmente  # quando o handler volta, a conexao fecha sozinha

mostra "eco de pe na porta ${porta} (ctrl+c pra parar)"
escuta_tcp(porta, atende)
mostra "servidor fechado, valeu"
