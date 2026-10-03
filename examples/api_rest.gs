# API REST de tarefas, em memoria: CRUD com parametro no caminho, JSON,
# middleware de autenticacao (antes), log (depois) e CORS.
# PRECISA DE REDE: sobe um servidor HTTP e fica escutando (ctrl+c desliga).
# Rode: gs roda examples/api_rest.gs [porta]      (porta 0 = qualquer livre)
#
#   curl localhost:8080/tarefas
#   curl -X POST localhost:8080/tarefas -H "Authorization: Bearer segredo" \
#        -H "Content-Type: application/json" -d '{"titulo": "lavar a louca"}'
#   curl localhost:8080/tarefas/1
#   curl -X PUT localhost:8080/tarefas/1 -H "Authorization: Bearer segredo" \
#        -H "Content-Type: application/json" -d '{"feita": true}'
#   curl -X DELETE localhost:8080/tarefas/1 -H "Authorization: Bearer segredo"

crava TOKEN = env("API_TOKEN") ?? "segredo"

# estado em lista/dicionario: gambiarra nao reatribui variavel de fora, mas
# mexe por dentro do que e compartilhado
bota tarefas = []
bota contador = {"proximo": 1}

# os handlers rodam em paralelo (um por pedido): um cano de 1 lugar vira
# trava — envia tranca, recebe destranca — pra dois POST nao pegarem o mesmo id
bota trava = cano(1)
gambiarra com_trava(f)
    envia(trava, deu_bom)
    arruma
        funciona f()
    finalmente
        recebe(trava)
    acabou_finalmente
acabou_finalmente

gambiarra erro_json(status, msg)
    funciona responde_json({"erro": msg}, status)
acabou_finalmente

# o :id do caminho chega como texto
gambiarra acha_tarefa(id)
    funciona acha(tarefas, gambiarra(t) funciona texto(t["id"]) == id acabou_finalmente)
acabou_finalmente

# --- middlewares ---

# so leitura e liberada; o resto pede token. Devolver uma resposta corta o
# pedido aqui; devolver nada deixa seguir (e o handler ve o pedido mexido)
antes(gambiarra(pedido)
    se_colar pedido.metodo == "GET"
        funciona nada
    acabou_finalmente
    se_colar pedido.cabecalhos["Authorization"] != "Bearer " + TOKEN
        funciona erro_json(401, "sem token valido, parca")
    acabou_finalmente
    bota pedido["usuario"] = "admin"
acabou_finalmente)

depois(gambiarra(pedido, resposta)
    mostra "${pedido.metodo} ${pedido.caminho} -> ${resposta.status}"
acabou_finalmente)

cors()

# --- rotas ---

rota("GET", "/", gambiarra()
    funciona "API de tarefas no ar. Tenta GET /tarefas"
acabou_finalmente)

# lista/dicionario devolvido direto vira JSON 200
rota("GET", "/tarefas", gambiarra() funciona tarefas acabou_finalmente)

rota("GET", "/tarefas/:id", gambiarra(pedido)
    bota t = acha_tarefa(pedido.params.id)
    se_colar t == nada
        funciona erro_json(404, "tarefa ${pedido.params.id} nao existe")
    acabou_finalmente
    funciona t
acabou_finalmente)

rota("POST", "/tarefas", gambiarra(pedido)
    bota dados = pedido.json # ja vem parseado (json quebrado nem chega aqui: 400)
    se_colar tipo(dados) != "dicionario" ou tipo(dados["titulo"]) != "texto"
        funciona erro_json(422, "manda um json tipo {\"titulo\": \"...\"}")
    acabou_finalmente
    bota t = com_trava(gambiarra()
        bota nova = {"id": contador["proximo"], "titulo": dados["titulo"], "feita": deu_ruim, "dono": pedido.usuario}
        bota contador["proximo"] = contador["proximo"] + 1
        adiciona(tarefas, nova)
        funciona nova
    acabou_finalmente)
    bota resp = responde_json(t, 201)
    bota resp["cabecalhos"]["Location"] = "/tarefas/${t["id"]}"
    funciona resp
acabou_finalmente)

rota("PUT", "/tarefas/:id", gambiarra(pedido)
    bota t = acha_tarefa(pedido.params.id)
    se_colar t == nada
        funciona erro_json(404, "tarefa ${pedido.params.id} nao existe")
    acabou_finalmente
    bota dados = pedido.json ?? {}
    se_colar tem(dados, "titulo")
        bota t["titulo"] = dados["titulo"]
    acabou_finalmente
    se_colar tem(dados, "feita")
        bota t["feita"] = dados["feita"]
    acabou_finalmente
    funciona t
acabou_finalmente)

rota("DELETE", "/tarefas/:id", gambiarra(pedido)
    bota t = acha_tarefa(pedido.params.id)
    se_colar t == nada
        funciona erro_json(404, "tarefa ${pedido.params.id} nao existe")
    acabou_finalmente
    com_trava(gambiarra() remove(tarefas, t) acabou_finalmente)
    funciona {"status": 204}
acabou_finalmente)

bota porta = 8080
se_colar tamanho(argumentos()) > 0
    bota porta = argumentos()[0]
acabou_finalmente
escuta(porta)
