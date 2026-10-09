# upload de arquivo + sessao em cookie + limite de pedidos + gzip
# rode com: gs roda examples/upload.gs  (e abra http://localhost:8080)
# em producao: bota um gera_chave() de verdade em SEGREDO (ambiente ou .env)

bota SEGREDO = env("SEGREDO", "so-pra-dev-troca-por-um-gera_chave-de-verdade")
bota PASTA = env("PASTA_UPLOADS", "uploads")
bota PORTA = env("PORTA", "8080")

usa_sessao({"segredo": SEGREDO})  # pedido.sessao volta no proximo pedido
limita({"por_minuto": 30})  # 30 pedidos/minuto por ip, senao 429
comprime({"minimo": 256})  # html/json acima de 256 bytes vai em gzip

# a pagina e um template (renderiza_arquivo); rode da raiz do repo ou de examples/
bota PAGINA = "examples/upload.html"
se_colar nao existe(PAGINA)
    bota PAGINA = "upload.html"
acabou_finalmente

rota("GET", "/", gambiarra(pedido)
    bota s = pedido.sessao
    bota s["visitas"] = (s.visitas ?? 0) + 1
    funciona responde_html(renderiza_arquivo(PAGINA, {"visitas": s.visitas, "uploads": s.uploads ?? []}))
acabou_finalmente)

rota("POST", "/upload", gambiarra(pedido)
    bota arquivos = pedido.arquivos.arquivo
    se_colar arquivos == nada
        funciona responde_json({"erro": "cade o arquivo? (campo \"arquivo\")"}, 400)
    acabou_finalmente
    se_colar tipo(arquivos) != "lista"
        bota arquivos = [arquivos]
    acabou_finalmente
    # so o resumo vai pra sessao (cookie tem ~4 KB); o arquivo fica no disco
    bota historico = pedido.sessao.uploads ?? []
    bota salvos = []
    pra_cada arq em arquivos
        bota info = {"nome": arq.nome, "tamanho": arq.tamanho, "descricao": pedido.campos.descricao ?? ""}
        bota info["caminho"] = salva_arquivo(arq, PASTA)
        adiciona(salvos, info)
        adiciona(historico, info)
    acabou_finalmente
    bota pedido.sessao["uploads"] = historico
    funciona responde_json({"salvos": salvos}, 201)
acabou_finalmente)

rota("POST", "/sai", gambiarra(pedido)
    bota pedido["sessao"] = {}
    funciona "sessao apagada, falou!"
acabou_finalmente)

escuta(PORTA)
