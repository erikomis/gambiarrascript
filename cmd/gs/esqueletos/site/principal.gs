# __NOME__ — site com paginas renderizadas no servidor (GambiarraScript).
#
# Rode da pasta do projeto: gs roda principal.gs [--porta 8080]
# e abra http://localhost:8080
importa "conteudo.gs" como conteudo

bota cfg = opcoes({"porta": numero(env("PORTA", "8080"))}, {"porta": "porta HTTP (0 = qualquer uma livre)"})

# dados que todo modelo enxerga (o cabecalho e o rodape usam)
crava SITE = {"nome": "__NOME__", "descricao": "feito com GambiarraScript"}

# renderiza modelos/<arquivo> (compilado uma vez, ate o .html mudar) e
# embrulha na resposta HTML
gambiarra pagina(arquivo, dados, status = 200)
    bota dados["site"] = SITE
    funciona responde_html(renderiza_arquivo("modelos/" + arquivo, dados), status)
acabou_finalmente

# CSS, imagens e afins: /estatico/estilo.css -> estatico/estilo.css
serve_pasta("/estatico", "estatico")

# log de cada pedido no stderr
depois(gambiarra(pedido, resposta)
    log_info("pedido", {"metodo": pedido.metodo, "caminho": pedido.caminho, "status": resposta.status})
acabou_finalmente)

rota("GET", "/", gambiarra()
    funciona pagina("inicio.html", {"artigos": conteudo.ARTIGOS, "tags": conteudo.todas_tags()})
acabou_finalmente)

rota("GET", "/artigos/:slug", gambiarra(pedido)
    bota artigo = conteudo.acha_artigo(pedido.params.slug)
    se_colar artigo == nada
        funciona pagina("nao_achei.html", {"caminho": pedido.caminho}, 404)
    acabou_finalmente
    funciona pagina("artigo.html", {"artigo": artigo})
acabou_finalmente)

# /busca?q=termo — o termo volta pra pagina e sai escapado no HTML
rota("GET", "/busca", gambiarra(pedido)
    bota q = pedido.query.q ?? ""
    funciona pagina("busca.html", {"q": q, "achados": conteudo.busca_artigos(q)})
acabou_finalmente)

rota("GET", "/sobre", gambiarra()
    funciona pagina("sobre.html", {})
acabou_finalmente)

log_info("subindo __NOME__", {"porta": cfg.porta})
escuta(cfg.porta)
