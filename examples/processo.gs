# processos: roda_comando (roda comando externo) + sai (encerra o script)

# roda um comando e inspeciona saida/codigo
bota r = roda_comando("echo", ["salve, tropa"])
mostra "saida: " + tira_espaco(r.saida)
mostra "codigo: " + r.codigo

# codigo != 0 NAO e erro de GS — vem no dict pra voce decidir
bota f = roda_comando("sh", ["-c", "exit 2"])
se_colar f.codigo != 0
    mostra "o comando falhou (codigo " + f.codigo + ")"
acabou_finalmente

# sai encerra o script na hora com o codigo de saida do processo
mostra "vou sair com codigo 0"
sai(0)
mostra "isso nunca aparece"
