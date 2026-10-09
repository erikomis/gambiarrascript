# __NOME__ — API REST feita com GambiarraScript.
#
# Rode:  gs roda principal.gs              (porta do PORTA ou 8080)
#        gs roda principal.gs --porta 3000
#        gs roda principal.gs --ajuda
# Config: .env (copie do .env.exemplo) ou variaveis de ambiente.
importa "lib/http.gs" como http
importa "rotas/usuarios.gs" como usuarios
importa "rotas/tarefas.gs" como tarefas

# --- config ---

# o .env e opcional; variavel que ja ta no ambiente ganha do arquivo
se_colar existe(".env")
    carrega_env()
acabou_finalmente

bota cfg = opcoes({
    "porta": numero(env("PORTA", "8080")),
    "banco": env("GS_BANCO", "sqlite:dados/app.db")
}, {
    "porta": "porta HTTP (0 = qualquer uma livre)",
    "banco": "url do banco: sqlite:arquivo.db, postgres://..., mysql://..."
})

bota segredo = env("JWT_SEGREDO", "")
se_colar segredo == ""
    # sem segredo fixo, cada subida sorteia um: os tokens morrem no restart
    log_aviso("JWT_SEGREDO vazio: sorteei um so pra essa execucao (configure no .env)")
    bota segredo = token_aleatorio()
acabou_finalmente

# --- banco: conecta e aplica as migracoes pendentes de migracoes/ ---

bota db = conecta(cfg.banco)
bota novas = migra(db)
se_colar tamanho(novas) > 0
    log_info("migracoes aplicadas", {"versoes": novas})
acabou_finalmente

# --- middleware ---

# marca a hora que o pedido chegou (o log la embaixo calcula quanto demorou)
antes(gambiarra(pedido)
    bota pedido["inicio"] = agora_ns()
acabou_finalmente)

# autenticacao: rota publica passa direto; o resto pede um JWT valido
antes(gambiarra(pedido)
    se_colar http.eh_publica(pedido.metodo, pedido.caminho)
        funciona nada
    acabou_finalmente
    bota token = http.extrai_token(pedido.cabecalhos["Authorization"])
    se_colar token == nada
        funciona http.erro_json(401, "faltou o token: Authorization: Bearer <token> (pega no POST /login)")
    acabou_finalmente
    arruma
        bota claims = jwt_confere(token, segredo)
    quebrou err se erro_tipo(err) == "jwt"
        funciona http.erro_json(401, "token invalido ou vencido, faz login de novo")
    acabou_finalmente
    bota pedido["usuario_id"] = claims.uid
acabou_finalmente)

# log de cada pedido (no stderr; GS_LOG_FORMATO=json pra sair em JSON)
depois(gambiarra(pedido, resposta)
    bota ms = (agora_ns() - (pedido.inicio ?? agora_ns())) / 1000000
    log_info("pedido", {
        "metodo": pedido.metodo,
        "caminho": pedido.caminho,
        "status": resposta.status,
        "ms": arredonda(ms * 10) / 10
    })
acabou_finalmente)

cors()

# --- rotas ---

rota("GET", "/", gambiarra()
    funciona {"nome": "__NOME__", "rotas": ["POST /usuarios", "POST /login", "GET /eu", "GET|POST /tarefas", "GET|PUT|DELETE /tarefas/:id"]}
acabou_finalmente)

rota("GET", "/saude", gambiarra()
    consulta(db, "SELECT 1")
    funciona {"ok": deu_bom}
acabou_finalmente)

usuarios.registra(db, segredo)
tarefas.registra(db)

# --- sobe (ctrl+c / SIGTERM desliga com calma e o escuta volta) ---

log_info("subindo __NOME__", {"porta": cfg.porta, "banco": separa(cfg.banco, ":")[0]})
escuta(cfg.porta)
fecha(db)
log_info("tchau")
