# __NOME__ — script de linha de comando feito com GambiarraScript.
#
#   gs roda principal.gs --ajuda
#   gs roda principal.gs --nome tropa --vezes 2 --grita
#   gs roda principal.gs -- arquivo1 arquivo2      (o resto vai pra cfg._)

# flag -> valor padrao (o tipo do padrao e o tipo da flag) e a ajuda de cada uma
bota cfg = opcoes({"nome": "mundo", "vezes": 1, "grita": deu_ruim}, {
    "nome": "quem vai ser saudado",
    "vezes": "quantas vezes repetir",
    "grita": "sai tudo em maiusculo"
})

gambiarra saudacao(nome, grita)
    bota msg = "salve, ${nome}!"
    se_colar grita
        funciona maiusculo(msg)
    acabou_finalmente
    funciona msg
acabou_finalmente

se_colar cfg.vezes < 1
    escreve_erro("--vezes tem que ser pelo menos 1\n")
    sai(2)
acabou_finalmente

pra_cada i de 1 ate cfg.vezes
    mostra saudacao(cfg.nome, cfg.grita)
acabou_finalmente

# argumentos soltos (que nao sao flag) chegam em cfg._
pra_cada extra em cfg._
    mostra "recebi: ${extra}"
acabou_finalmente
