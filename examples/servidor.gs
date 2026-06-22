# um servidorzinho HTTP em GambiarraScript
# rode com: gs roda examples/servidor.gs  (e abra http://localhost:8080 no navegador)

gambiarra inicio(pedido)
    funciona "Salve! Voce pediu " + pedido["caminho"]
acabou_finalmente

gambiarra eco(pedido)
    funciona {"status": 201, "corpo": "voce mandou: " + pedido["corpo"]}
acabou_finalmente

gambiarra ola(pedido)
    bota nome = pedido["query"]["nome"]
    se_colar nome == nada
        bota nome = "estranho"
    acabou_finalmente
    funciona "e ai, " + nome + "!"
acabou_finalmente

rota("GET", "/", inicio)
rota("POST", "/eco", eco)
rota("GET", "/ola", ola)

mostra "servidor de pe na porta 8080 (ctrl+c pra parar)"
escuta(8080)
