# API JSON minima pro teste de carga (bench/carga). Mesmo contrato do api.js
# e do api.py:
#   GET  /ping        -> {"ok":true}
#   GET  /itens/:id   -> o item (404 se nao existe)
#   POST /itens       -> cria a partir do JSON {"nome", "preco"}, 201
# Uso: gs roda bench/http/api.gs 8080
bota itens = {}
pra_cada i de 1 ate 1000
    bota itens[texto(i)] = {"id": i, "nome": "item ${i}", "preco": i * 2}
acabou_finalmente
bota contador = {"proximo": 1001}
bota trava_ids = trava()

rota("GET", "/ping", gambiarra() funciona {"ok": deu_bom} acabou_finalmente)

rota("GET", "/itens/:id", gambiarra(pedido)
    bota it = itens[pedido.params.id]
    se_colar it == nada
        funciona responde_json({"erro": "nao achei"}, 404)
    acabou_finalmente
    funciona it
acabou_finalmente)

rota("POST", "/itens", gambiarra(pedido)
    bota dados = pedido.json
    bota id = com_trava(trava_ids, gambiarra()
        bota id = contador["proximo"]
        bota contador["proximo"] = id + 1
        funciona id
    acabou_finalmente)
    bota it = {"id": id, "nome": dados["nome"], "preco": dados["preco"]}
    bota itens[texto(id)] = it
    funciona responde_json(it, 201)
acabou_finalmente)

escuta(argumentos()[0])
