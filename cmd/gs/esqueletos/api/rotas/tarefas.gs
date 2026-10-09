# CRUD de tarefas. Toda rota aqui pede token: cada usuario so enxerga (e
# mexe) nas proprias tarefas — a de outro usuario responde 404, igual a que
# nao existe, pra nao entregar quais ids existem.
importa "../lib/http.gs" como http

crava ESQUEMA_NOVA = {
    "titulo": {"tipo": "texto", "min": 1, "max": 200},
    "feita": "booleano?",
    "_estrito": deu_bom
}

crava ESQUEMA_MUDA = {
    "titulo": {"tipo": "texto?", "min": 1, "max": 200},
    "feita": "booleano?",
    "_estrito": deu_bom
}

crava COLUNAS = "id, titulo, feita, criada_em"

# linha do banco -> JSON (o sqlite guarda booleano como 0/1)
gambiarra tarefa_saida(linha)
    funciona {"id": linha.id, "titulo": linha.titulo, "feita": linha.feita == 1, "criada_em": linha.criada_em}
acabou_finalmente

gambiarra booleano_sql(b)
    funciona se_colar b entao 1 se_nao_colar 0
acabou_finalmente

# a tarefa do :id, se for do usuario logado; nada se nao existir ou for de outro
gambiarra acha_tarefa(db, pedido)
    bota id = http.id_valido(pedido.params.id)
    se_colar id == nada
        funciona nada
    acabou_finalmente
    bota linhas = consulta(db, "SELECT ${COLUNAS} FROM tarefas WHERE id = ? AND usuario_id = ?", id, pedido.usuario_id)
    se_colar tamanho(linhas) == 0
        funciona nada
    acabou_finalmente
    funciona linhas[0]
acabou_finalmente

gambiarra nao_achei(pedido)
    funciona http.erro_json(404, "tarefa ${pedido.params.id} nao existe")
acabou_finalmente

gambiarra registra(db)
    rota("GET", "/tarefas", gambiarra(pedido)
        bota linhas = consulta(db, "SELECT ${COLUNAS} FROM tarefas WHERE usuario_id = ? ORDER BY id", pedido.usuario_id)
        funciona mapeia(linhas, tarefa_saida)
    acabou_finalmente)

    rota("POST", "/tarefas", gambiarra(pedido)
        bota erros = valida(pedido.json, ESQUEMA_NOVA)
        se_colar tamanho(erros) > 0
            funciona http.erros_json(erros)
        acabou_finalmente
        bota feita = booleano_sql(pedido.json.feita ?? deu_ruim)
        bota linhas = consulta(db, "INSERT INTO tarefas (usuario_id, titulo, feita) VALUES (?, ?, ?) RETURNING ${COLUNAS}", pedido.usuario_id, pedido.json.titulo, feita)
        bota t = tarefa_saida(linhas[0])
        bota resp = responde_json(t, 201)
        bota resp["cabecalhos"]["Location"] = "/tarefas/${t.id}"
        funciona resp
    acabou_finalmente)

    rota("GET", "/tarefas/:id", gambiarra(pedido)
        bota t = acha_tarefa(db, pedido)
        se_colar t == nada
            funciona nao_achei(pedido)
        acabou_finalmente
        funciona tarefa_saida(t)
    acabou_finalmente)

    # PUT parcial: manda so o que muda ({"feita": true})
    rota("PUT", "/tarefas/:id", gambiarra(pedido)
        bota t = acha_tarefa(db, pedido)
        se_colar t == nada
            funciona nao_achei(pedido)
        acabou_finalmente
        bota dados = pedido.json ?? {}
        bota erros = valida(dados, ESQUEMA_MUDA)
        se_colar tamanho(erros) > 0
            funciona http.erros_json(erros)
        acabou_finalmente
        bota titulo = dados.titulo ?? t.titulo
        bota feita = booleano_sql(dados.feita ?? t.feita == 1)
        bota linhas = consulta(db, "UPDATE tarefas SET titulo = ?, feita = ? WHERE id = ? AND usuario_id = ? RETURNING ${COLUNAS}", titulo, feita, t.id, pedido.usuario_id)
        funciona tarefa_saida(linhas[0])
    acabou_finalmente)

    rota("DELETE", "/tarefas/:id", gambiarra(pedido)
        bota t = acha_tarefa(db, pedido)
        se_colar t == nada
            funciona nao_achei(pedido)
        acabou_finalmente
        executa(db, "DELETE FROM tarefas WHERE id = ? AND usuario_id = ?", t.id, pedido.usuario_id)
        funciona {"status": 204}
    acabou_finalmente)
acabou_finalmente
