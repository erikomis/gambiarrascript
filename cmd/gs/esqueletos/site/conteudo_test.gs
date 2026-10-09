# Testes do conteudo e dos modelos. Rode: gs testa
importa "conteudo.gs" como conteudo

# busca de artigo pelo slug
espera(conteudo.acha_artigo("como-funciona").titulo, "Como funciona")
espera(conteudo.acha_artigo("nao-existe"), nada)

# todo artigo tem os campos que os modelos usam
pra_cada a em conteudo.ARTIGOS
    espera(valida(a, {"slug": "texto", "titulo": "texto", "data": "data", "tags": {"tipo": "lista", "itens": "texto"}, "resumo": "texto", "paragrafos": {"tipo": "lista", "min": 1, "itens": "texto"}}), [])
acabou_finalmente

# busca: titulo, resumo e tag, sem caixa; termo vazio nao acha nada
espera(tamanho(conteudo.busca_artigos("TUTORIAL")), 2)
espera(tamanho(conteudo.busca_artigos("forno")), 1)
espera(conteudo.busca_artigos("   "), [])
espera(conteudo.busca_artigos(nada), [])
espera(conteudo.todas_tags(), ["modelos", "novidades", "tutorial"])

# os modelos renderizam e escapam o que vem de fora
bota html = renderiza_arquivo("modelos/busca.html", {"site": {"nome": "teste"}, "q": "<script>", "achados": []})
afirma(contem(html, "&lt;script&gt;"), "o termo da busca sai escapado")
afirma(nao contem(html, "<script>"), "nada de script cru")
bota inicio = renderiza_arquivo("modelos/inicio.html", {"site": {"nome": "teste"}, "artigos": conteudo.ARTIGOS, "tags": conteudo.todas_tags()})
afirma(contem(inicio, "Proximos passos"), "o laco lista os artigos")
afirma(contem(inicio, "/estatico/estilo.css"), "o layout puxa o CSS")
