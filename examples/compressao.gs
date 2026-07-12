# Compressao — gzip_comprime / gzip_descomprime (texto <-> base64 comprimido)

bota msg = "salve salve salve salve salve salve salve salve salve salve salve salve salve salve salve salve salve salve salve salve salve salve salve"
mostra "tamanho original: ${tamanho(msg)} chars"

bota comprimido = gzip_comprime(msg)
mostra "comprimido: ${tamanho(comprimido)} chars (base64)"
mostra "  -> ${comprimido}"

bota restaurado = gzip_descomprime(comprimido)
mostra "restaurado igual ao original? ${msg == restaurado}"

# round-trip com texto maior e repetitivo (gzip brilha aqui)
bota gigante = ""
pra_cada i em 1..100
    bota gigante = gigante + "linha ${i}\n"
acabou_finalmente
mostra "gigante original: ${tamanho(gigante)} chars"
bota g_comp = gzip_comprime(gigante)
mostra "gigante comprimido: ${tamanho(g_comp)} chars"
mostra "round-trip gigante OK? ${gzip_descomprime(g_comp) == gigante}"