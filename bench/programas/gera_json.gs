# gera o documento de ~1 MB que o bench de JSON le (o roda.sh chama isso uma
# vez antes de medir). Uso: gs roda gera_json.gs saida.json
bota itens = []
pra_cada i de 0 ate 7999
    adiciona(itens, {
        "id": i,
        "nome": "item numero ${i}",
        "preco": i + 0.5,
        "ativo": i % 2 == 0,
        "tags": ["promo", "estoque", "cat${i % 17}"],
        "dono": {"id": i % 100, "nome": "usuario ${i % 100}", "nota": nada},
    })
acabou_finalmente
escreve_arquivo(argumentos()[0], pra_json({"versao": 1, "itens": itens}))
