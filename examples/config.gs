# Config de app de verdade: .env + flags de linha de comando + log no stderr.
# Rode com: gs roda examples/config.gs --porta 9090 --verboso extra.txt
#           gs roda examples/config.gs --ajuda
#           GS_LOG_FORMATO=json gs roda examples/config.gs
# (os logs vao pro stderr; o stdout fica so com o que o script mostra)

# --- .env: cria um de exemplo, carrega e apaga ---
bota arq = "exemplo_config.env"
bota linhas = [
    "# config local",
    "EXEMPLO_DB_URL=sqlite::memory:",
    "export EXEMPLO_NOME=\"api da tropa\"",
    "EXEMPLO_SEGREDO='nao-commita-isso' # comentario some"
]
escreve_arquivo(arq, junta(linhas, "\n") + "\n")
bota carregou = carrega_env(arq)
deleta(arq)
mostra "carregou do .env: ${junta(chaves(carregou), ", ")}"
mostra "nome: ${env("EXEMPLO_NOME")}"

# env com padrao: se a variavel nao existe, vem o 2o argumento
mostra "modo: ${env("EXEMPLO_MODO_QUE_NAO_TEM", "dev")}"

# arquivo que nao existe vira erro do tipo "io" (da pra pegar)
arruma
    carrega_env("nao_tem_esse.env")
quebrou err
    mostra "sem .env: erro do tipo ${erro_tipo(err)}"
acabou_finalmente

# --- flags: o tipo vem do padrao, --ajuda sai sozinho ---
bota cfg = opcoes({
    "porta": 8080,
    "verboso": deu_ruim,
    "nome": env("EXEMPLO_NOME", "api"),
    "tag": []
}, {
    "porta": "porta do servidor HTTP",
    "verboso": "loga em nivel debug",
    "tag": "etiqueta (pode repetir)"
})
mostra "porta ${cfg.porta} (+1 = ${cfg.porta + 1}), verboso ${cfg.verboso}, nome ${cfg.nome}"
mostra "tags: ${cfg.tag}, posicionais: ${cfg._}"

# --- log: stderr, nivel por GS_LOG_NIVEL, JSON com GS_LOG_FORMATO=json ---
log_info("subiu", {"porta": cfg.porta, "nome": cfg.nome})
log_debug("so aparece com GS_LOG_NIVEL=debug", {"tags": cfg.tag})
log_aviso("segredo carregado do .env", {"tamanho": tamanho(env("EXEMPLO_SEGREDO"))})
log_erro("exemplo de erro", {"motivo": "nenhum, e so exemplo"})
mostra "fim"
