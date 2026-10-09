# API JSON minima pro teste de carga (bench/carga), em aiohttp (um processo,
# asyncio). http.server seria injusto: e sincrono e de brinquedo.
# Mesmo contrato do api.gs e do api.js. Uso: python3 bench/http/api.py 8080
import sys

from aiohttp import web

itens = {str(i): {"id": i, "nome": f"item {i}", "preco": i * 2} for i in range(1, 1001)}
estado = {"proximo": 1001}


async def ping(_):
    return web.json_response({"ok": True})


async def pega(req):
    it = itens.get(req.match_info["id"])
    if it is None:
        return web.json_response({"erro": "nao achei"}, status=404)
    return web.json_response(it)


async def cria(req):
    dados = await req.json()
    i = estado["proximo"]
    estado["proximo"] = i + 1
    it = {"id": i, "nome": dados["nome"], "preco": dados["preco"]}
    itens[str(i)] = it
    return web.json_response(it, status=201)


app = web.Application()
app.router.add_get("/ping", ping)
app.router.add_get("/itens/{id}", pega)
app.router.add_post("/itens", cria)
web.run_app(app, host="127.0.0.1", port=int(sys.argv[1]), print=None, access_log=None)
