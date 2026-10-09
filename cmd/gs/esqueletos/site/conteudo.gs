# O conteudo do site. Aqui e uma lista fixa; troque por um banco (conecta +
# consulta) ou por arquivos quando crescer. Sem efeito colateral: da pra
# testar direto (veja conteudo_test.gs).

crava ARTIGOS = [
    {
        "slug": "salve-tropa",
        "titulo": "Salve, tropa!",
        "data": "2026-01-10",
        "tags": ["novidades"],
        "resumo": "O site saiu do forno: modelos, layout e arquivos estaticos.",
        "paragrafos": [
            "Esse site e servido pelo GambiarraScript: cada pagina e um modelo HTML renderizado no servidor.",
            "O layout fica em modelos/base.html; as paginas so preenchem os blocos."
        ]
    },
    {
        "slug": "como-funciona",
        "titulo": "Como funciona",
        "data": "2026-01-12",
        "tags": ["tutorial", "modelos"],
        "resumo": "rota + renderiza_arquivo + responde_html, e o resto e HTML.",
        "paragrafos": [
            "Cada rota chama renderiza_arquivo com os dados da pagina e devolve responde_html.",
            "Todo valor sai escapado: texto que veio do usuario pode ir direto pro modelo.",
            "O CSS mora em estatico/ e sai pelo serve_pasta."
        ]
    },
    {
        "slug": "proximos-passos",
        "titulo": "Proximos passos",
        "data": "2026-01-15",
        "tags": ["tutorial"],
        "resumo": "Ideias pra crescer: banco, formulario, cache.",
        "paragrafos": [
            "Troque a lista de ARTIGOS por um banco com conecta e consulta.",
            "Formulario? rota POST, pedido.corpo e valida resolvem."
        ]
    }
]

# o artigo do slug, ou nada
gambiarra acha_artigo(slug)
    funciona acha(ARTIGOS, gambiarra(a) funciona a.slug == slug acabou_finalmente)
acabou_finalmente

# artigos cujo titulo, resumo ou tag tem o termo (sem diferenciar caixa)
gambiarra busca_artigos(termo)
    bota t = minusculo(tira_espaco(termo ?? ""))
    se_colar t == ""
        funciona []
    acabou_finalmente
    funciona filtra(ARTIGOS, gambiarra(a)
        funciona contem(minusculo(a.titulo), t) ou contem(minusculo(a.resumo), t) ou contem(junta(a.tags, " "), t)
    acabou_finalmente)
acabou_finalmente

# todas as tags, sem repetir, em ordem alfabetica
gambiarra todas_tags()
    bota tags = []
    pra_cada a em ARTIGOS
        pra_cada tag em a.tags
            adiciona(tags, tag)
        acabou_finalmente
    acabou_finalmente
    bota sem_repetir = unicos(tags)
    ordena(sem_repetir)  # ordena no lugar
    funciona sem_repetir
acabou_finalmente
