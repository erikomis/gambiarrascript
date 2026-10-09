# Ajudantes de HTTP sem efeito colateral: nao mexem em banco nem em rede,
# entao da pra testar direto (veja http_test.gs).

# rotas que nao pedem token: [metodo, caminho]. O resto passa pelo JWT.
crava ROTAS_PUBLICAS = [
    ["GET", "/"],
    ["GET", "/saude"],
    ["POST", "/usuarios"],
    ["POST", "/login"]
]

# deu_bom quando a rota nao pede login (HEAD conta como GET)
gambiarra eh_publica(metodo, caminho)
    bota m = se_colar metodo == "HEAD" entao "GET" se_nao_colar metodo
    pra_cada r em ROTAS_PUBLICAS
        se_colar r[0] == m e r[1] == caminho
            funciona deu_bom
        acabou_finalmente
    acabou_finalmente
    funciona deu_ruim
acabou_finalmente

# tira o token do cabecalho "Authorization: Bearer <token>"; nada se nao veio
gambiarra extrai_token(cabecalho)
    se_colar tipo(cabecalho) != "texto"
        funciona nada
    acabou_finalmente
    bota partes = separa(tira_espaco(cabecalho), " ")
    se_colar tamanho(partes) != 2 ou minusculo(partes[0]) != "bearer" ou partes[1] == ""
        funciona nada
    acabou_finalmente
    funciona partes[1]
acabou_finalmente

# o :id do caminho chega como texto; devolve o numero ou nada se nao for id
gambiarra id_valido(t)
    se_colar tipo(t) != "texto" ou nao busca_regex("^[0-9]{1,15}$", t)
        funciona nada
    acabou_finalmente
    funciona numero(t)
acabou_finalmente

gambiarra erro_json(status, msg)
    funciona responde_json({"erro": msg}, status)
acabou_finalmente

# 400 com a lista que o valida() devolveu
gambiarra erros_json(erros)
    funciona responde_json({"erros": erros}, 400)
acabou_finalmente
