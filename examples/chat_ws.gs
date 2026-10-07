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

# cada conexao roda o handler na propria goroutine. Cada operacao na lista
# (adiciona, remove, fatia, tamanho) ja e atomica sozinha — duas goroutines
# mexendo nela ao mesmo tempo nunca quebram nada. A trava e pra quando VARIAS
# operacoes tem que rodar juntas, sem ninguem se meter no meio.
bota sala = trava()

# manda pra todo mundo. `conexoes[0:]` tira um retrato da lista (atomico) e o
# envio roda fora de qualquer trava: um cliente lento nao segura os outros.
# Dicionario vai como JSON; envia pra conexao que ja caiu so perde a mensagem
# (nao estoura).
gambiarra espalha(msg)
    pra_cada c em conexoes[0:]
        envia(c, msg)
    acabou_finalmente
acabou_finalmente

rota_ws("/chat", gambiarra(ws, pedido)
    bota nome = pedido["query"]["nome"] ?? "anonimo"
    # entrar e contar quem ta na sala e uma operacao so: com a trava, ninguem
    # entra nem sai entre o adiciona e o tamanho
    bota na_sala = com_trava(sala, gambiarra()
        adiciona(conexoes, ws)
        funciona tamanho(conexoes)
    acabou_finalmente)
    mostra "${nome} entrou (${na_sala} na sala)"
    espalha({"tipo": "entrou", "nome": nome})
    enquanto deu_bom
        bota texto_msg = recebe(ws) # bloqueia; nada = o cliente saiu
        se_colar texto_msg == nada
            vaza
        acabou_finalmente
        espalha({"tipo": "msg", "nome": nome, "texto": texto_msg})
    acabou_finalmente
    bota ficaram = com_trava(sala, gambiarra()
        remove(conexoes, ws)
        funciona tamanho(conexoes)
    acabou_finalmente)
    mostra "${nome} saiu (${ficaram} na sala)"
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
