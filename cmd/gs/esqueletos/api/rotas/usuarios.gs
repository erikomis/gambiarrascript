# Rotas de conta: cadastro, login (devolve um JWT) e GET /eu (protegida).
# O principal.gs chama registra(db, segredo) uma vez, na subida.
importa "../lib/http.gs" como http

# quanto tempo o token vale (segundos)
crava VALIDADE_TOKEN = 24 * 60 * 60

crava ESQUEMA_CADASTRO = {
    "nome": {"tipo": "texto", "min": 1, "max": 80},
    "email": {"tipo": "email", "max": 200},
    "senha": {"tipo": "texto", "min": 8, "max": 72},
    "_estrito": deu_bom
}

crava ESQUEMA_LOGIN = {
    "email": {"tipo": "texto", "min": 1, "max": 200},
    "senha": {"tipo": "texto", "min": 1, "max": 72},
    "_estrito": deu_bom
}

# hash de mentira: login com email que nao existe confere a senha contra ele
# e gasta o mesmo tempo (sem isso o cronometro entrega quais emails tem conta)
crava HASH_FALSO = "$2a$12$oKYRiYaRSJVZ0LXFXKF9E.lkd6XLKYSTEmZs2aI/6ZoL9WbiUoM9u"

gambiarra normaliza_email(email)
    funciona minusculo(tira_espaco(email))
acabou_finalmente

# o que pode sair pro cliente: nunca o hash da senha
gambiarra usuario_publico(linha)
    funciona {"id": linha.id, "nome": linha.nome, "email": linha.email, "criado_em": linha.criado_em}
acabou_finalmente

gambiarra registra(db, segredo)
    # cadastro: 201 com o usuario, 400 se o corpo nao passar, 409 se o email ja existe
    rota("POST", "/usuarios", gambiarra(pedido)
        bota erros = valida(pedido.json, ESQUEMA_CADASTRO)
        se_colar tamanho(erros) > 0
            funciona http.erros_json(erros)
        acabou_finalmente
        bota dados = pedido.json
        arruma
            bota linhas = consulta(db, "INSERT INTO usuarios (nome, email, senha_hash) VALUES (?, ?, ?) RETURNING id, nome, email, criado_em", tira_espaco(dados.nome), normaliza_email(dados.email), hash_senha(dados.senha))
        quebrou err se contem(erro_msg(err), "UNIQUE")
            funciona http.erro_json(409, "esse email ja tem conta")
        acabou_finalmente
        funciona responde_json(usuario_publico(linhas[0]), 201)
    acabou_finalmente)

    # login: {"token": ...} pra mandar em Authorization: Bearer <token>
    rota("POST", "/login", gambiarra(pedido)
        bota erros = valida(pedido.json, ESQUEMA_LOGIN)
        se_colar tamanho(erros) > 0
            funciona http.erros_json(erros)
        acabou_finalmente
        bota achados = consulta(db, "SELECT id, senha_hash FROM usuarios WHERE email = ?", normaliza_email(pedido.json.email))
        bota hash = HASH_FALSO
        se_colar tamanho(achados) > 0
            bota hash = achados[0].senha_hash
        acabou_finalmente
        bota confere = confere_senha(pedido.json.senha, hash)
        se_colar tamanho(achados) == 0 ou nao confere
            funciona http.erro_json(401, "email ou senha errados")
        acabou_finalmente
        bota token = jwt_assina({"uid": achados[0].id}, segredo, {"expira_em": VALIDADE_TOKEN})
        funciona {"token": token, "tipo": "Bearer", "expira_em": VALIDADE_TOKEN}
    acabou_finalmente)

    # quem sou eu: precisa de token (o middleware do principal.gs bota o usuario_id)
    rota("GET", "/eu", gambiarra(pedido)
        bota achados = consulta(db, "SELECT id, nome, email, criado_em FROM usuarios WHERE id = ?", pedido.usuario_id)
        se_colar tamanho(achados) == 0
            funciona http.erro_json(404, "esse usuario nao existe mais")
        acabou_finalmente
        funciona usuario_publico(achados[0])
    acabou_finalmente)
acabou_finalmente
