# Chat em tempo real por WebSocket: todo mundo conectado recebe tudo
# (broadcast). Serve tambem a pagina examples/chat_ws.html em "/".
# PRECISA DE REDE: sobe um servidor e fica escutando (ctrl+c desliga).
# Rode da raiz do repo: gs roda examples/chat_ws.gs [porta]
# e abra http://localhost:8080 em duas abas. Ou, de outro terminal:
#
#   bota ws = conecta_ws("ws://localhost:8080/chat?nome=ze")
#   envia(ws, "salve")
#   mostra recebe(ws)

bota conexoes = []

# cada conexao roda o handler na propria goroutine: a lista compartilhada
# so muda com a trava na mao (cano de 1 lugar: envia tranca, recebe destranca)
bota trava = cano(1)
gambiarra com_trava(f)
    envia(trava, deu_bom)
    arruma
        funciona f()
    finalmente
        recebe(trava)
    acabou_finalmente
acabou_finalmente

# manda pra todo mundo. Copia a lista com a trava e envia fora dela: um
# cliente lento nao segura os outros. Dicionario vai como JSON; envia pra
# conexao que ja caiu so perde a mensagem (nao estoura).
gambiarra espalha(msg)
    bota alvos = com_trava(gambiarra() funciona conexoes[0:] acabou_finalmente)
    pra_cada c em alvos
        envia(c, msg)
    acabou_finalmente
acabou_finalmente

rota_ws("/chat", gambiarra(ws, pedido)
    bota nome = pedido["query"]["nome"] ?? "anonimo"
    com_trava(gambiarra() adiciona(conexoes, ws) acabou_finalmente)
    mostra "${nome} entrou (${tamanho(conexoes)} na sala)"
    espalha({"tipo": "entrou", "nome": nome})
    enquanto deu_bom
        bota texto_msg = recebe(ws) # bloqueia; nada = o cliente saiu
        se_colar texto_msg == nada
            vaza
        acabou_finalmente
        espalha({"tipo": "msg", "nome": nome, "texto": texto_msg})
    acabou_finalmente
    com_trava(gambiarra() remove(conexoes, ws) acabou_finalmente)
    mostra "${nome} saiu"
    espalha({"tipo": "saiu", "nome": nome})
acabou_finalmente)

# a pagina do chat (o caminho e relativo a pasta de onde o gs foi rodado)
rota("GET", "/", gambiarra()
    pra_cada arq em ["examples/chat_ws.html", "chat_ws.html"]
        se_colar existe(arq)
            funciona {"cabecalhos": {"Content-Type": "text/html; charset=utf-8"}, "corpo": le_arquivo(arq)}
        acabou_finalmente
    acabou_finalmente
    funciona {"status": 404, "corpo": "nao achei o chat_ws.html — roda da raiz do repo"}
acabou_finalmente)

bota porta = 8080
se_colar tamanho(argumentos()) > 0
    bota porta = argumentos()[0]
acabou_finalmente
escuta(porta)
