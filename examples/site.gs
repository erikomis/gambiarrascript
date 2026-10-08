# Site com templates: layout (usa/bloco) + parcial (inclui) + laco, servido
# com rota + responde_html. Os modelos HTML ficam em examples/site/.
# PRECISA DE REDE: sobe um servidor e fica escutando (ctrl+c desliga).
# Rode da raiz do repo: gs roda examples/site.gs [porta]
# e abra http://localhost:8080, /produto/1 ou /busca?q=pao
# (tente /busca?q=<script>alert(1)</script>: sai escapado, nada roda).

bota MODELOS = "examples/site"
se_colar nao existe(MODELOS)
    bota MODELOS = "site"
acabou_finalmente

bota produtos = [
    {"id": 1, "nome": "Cafe coado", "preco": 6.5, "tags": ["quente", "classico"]},
    {"id": 2, "nome": "Pao de queijo", "preco": 4, "tags": ["mineiro"]},
    {"id": 3, "nome": "Coxinha <crocante>", "preco": 7.25, "tags": []}
]

# renderiza o arquivo (compilado uma vez so, ate o .html mudar) e embrulha
# na resposta com Content-Type de HTML
gambiarra pagina(arquivo, dados, status)
    funciona responde_html(renderiza_arquivo(MODELOS + "/" + arquivo, dados), status)
acabou_finalmente

rota("GET", "/", gambiarra(pedido)
    funciona pagina("inicio.html", {
        "titulo": "Cardapio",
        "visitante": pedido.query.nome,
        "produtos": produtos
    }, 200)
acabou_finalmente)

rota("GET", "/produto/:id", gambiarra(pedido)
    pra_cada p em produtos
        se_colar texto(p.id) == pedido.params.id
            funciona pagina("produto.html", {"produto": p}, 200)
        acabou_finalmente
    acabou_finalmente
    funciona pagina("nao_achei.html", {"motivo": "Nao tem produto " + pedido.params.id + "."}, 404)
acabou_finalmente)

rota("GET", "/busca", gambiarra(pedido)
    bota q = pedido.query.q ?? ""
    bota achados = filtra(produtos, gambiarra(p)
        funciona contem(minusculo(p.nome), minusculo(q))
    acabou_finalmente)
    funciona pagina("busca.html", {"q": q, "achados": achados}, 200)
acabou_finalmente)

bota porta = 8080
se_colar tamanho(argumentos()) > 0
    bota porta = argumentos()[0]
acabou_finalmente
mostra "site no ar em http://localhost:${porta} (ctrl+c pra parar)"
escuta(porta)
