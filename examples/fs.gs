# exemplo filesystem (fs) — sistema de arquivos

bota base = caminho_junta(caminho_dir(caminho_abs("saida.gs")), "fs_demo")
cria_dir(base)

escreve_arquivo(caminho_junta(base, "notas.txt"), "primeira versao")
escreve_arquivo(caminho_junta(base, "script.gs"), "mostra 1")

mostra "existe notas.txt? " + texto(existe(caminho_junta(base, "notas.txt")))
mostra "existe sem_nome.txt? " + texto(existe(caminho_junta(base, "sem_nome.txt")))
mostra "base e diretorio? " + texto(eh_dir(base))
mostra "base do caminho: " + caminho_base(caminho_junta(base, "notas.txt"))
mostra "extensao do .gs: " + caminho_ext(caminho_junta(base, "script.gs"))

mostra "conteudo do diretorio:"
pra_cada nome em le_dir(base)
    mostra "  - " + nome
acabou_finalmente

# limpa tudo
deleta(base)
mostra "deletado, ainda existe? " + texto(existe(base))