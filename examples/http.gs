# cliente HTTP: a GambiarraScript falando com o mundo
# (precisa de internet)

bota r = busca("https://httpbin.org/get")

mostra "status: " + texto(r["status"])
se_colar r["ok"]
    mostra "deu bom! resposta:"
    mostra r["corpo"]
se_nao_colar
    mostra "deu ruim, parca"
acabou_finalmente

# um POST mandando um corpo
bota envio = busca("https://httpbin.org/post", {"metodo": "POST", "corpo": "salve do gambiarra"})
mostra "post status: " + texto(envio["status"])
