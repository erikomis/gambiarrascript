# fs parte 2: copia, move, tamanho_arquivo, modificado_em, glob

bota dir = "/tmp/gs_fs2_demo"
cria_dir(dir)                      # mkdir -p (idempotente)

bota a = dir + "/a.txt"
escreve_arquivo(a, "salve tropa")
mostra "tamanho de a.txt: " + tamanho_arquivo(a) + " bytes"

# copia a.txt -> b.txt
bota b = dir + "/b.txt"
copia(a, b)
mostra "depois de copiar, .txt no dir: " + tamanho(glob(dir + "/*.txt"))

# move b.txt -> c.txt
bota c = dir + "/c.txt"
move(b, c)
se_colar existe(c) e nao existe(b)
    mostra "move ok: b virou c"
acabou_finalmente

# modificado_em encaixa no formata_tempo (unix-segundos)
mostra "a.txt modificado em: " + formata_tempo("2006-01-02", modificado_em(a))

deleta(dir)                        # limpa a bagunca
mostra "limpei"
