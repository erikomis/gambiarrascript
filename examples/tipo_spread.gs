# tipo(x), spread na chamada f(...lista) e formato na interpolacao ${x:fmt}.

# --- tipo(x): pergunta o tipo em runtime ---
gambiarra dobra(n) funciona n * 2 acabou_finalmente
bota coisas = [42, 3.5, "oi", deu_bom, nada, [1, 2], {"a": 1}, conjunto([1])]
pra_cada c em coisas
    mostra "${texto(c)} -> ${tipo(c)}"
acabou_finalmente
# gambiarra, lambda e builtin: tudo "funcao"
mostra "${tipo(dobra)} ${tipo(gambiarra(x) funciona x acabou_finalmente)} ${tipo(tamanho)}"

# type switch: valida o que veio de fora (de_json devolve qualquer coisa)
gambiarra descreve(v)
    escolhe tipo(v)
    caso "numero"
        funciona "numero ${v}"
    caso "texto"
        funciona "texto de ${tamanho(v)} letra(s)"
    caso "lista", "dicionario"
        funciona "colecao com ${tamanho(v)} item(ns)"
    se_nao_colar
        funciona "sei la: ${tipo(v)}"
    acabou_finalmente
acabou_finalmente
pra_cada v em de_json(`[1, "abc", [1, 2, 3], {"x": 1}, null]`)
    mostra descreve(v)
acabou_finalmente

# --- spread: f(...lista) abre a lista em argumentos ---
gambiarra volume(largura, altura, fundo) funciona largura * altura * fundo acabou_finalmente
bota medidas = [2, 3, 4]
mostra "volume: ${volume(...medidas)}"
mostra "misturado: ${volume(10, ...[1, 2])}"

bota notas = [7, 9.5, 6, 8]
mostra "maior nota: ${max(...notas)} / menor: ${min(...notas)}"
bota par = [3, 14]
mostra formata("%d/%d", ...par)

# com varargs: os extras viram o ...resto
gambiarra loga(nivel, ...partes) funciona "[${nivel}] ${junta(partes, " ")}" acabou_finalmente
bota msg = ["deu", "tudo", "certo"]
mostra loga("info", ...msg)

# default completa o que faltar
gambiarra saudacao(nome, cumprimento = "e ai") funciona "${cumprimento}, ${nome}!" acabou_finalmente
mostra saudacao(...["Jurandir"])
mostra saudacao(...["Jurandir", "salve"])

# bora tambem espalha
bota fu = bora volume(...medidas)
mostra "bora: ${espera(fu)}"

# espalhar coisa que nao e lista da erro
arruma
    volume(...42)
quebrou err
    mostra "peguei: ${erro_msg(err)}"
acabou_finalmente

# --- formato na interpolacao: os verbos do formata, sem o % ---
bota preco = 19.9
bota qtd = 3
mostra "preco: R$ ${preco:.2f}"
mostra "total: R$ ${preco * qtd:.2f}"
mostra "pedido #${qtd:05d}"
mostra "hex: ${255:x} / bin: ${5:b}"
mostra "[${"esq":-6}] [${"dir":6}]"
# `:` dentro da expressao (dicionario, fatia, texto) nao e formato
bota precos = {"cafe": 4.5}
mostra "cafe: ${precos["cafe"]:.2f} / fatia: ${notas[1:3]} / texto: ${"a:b"}"
