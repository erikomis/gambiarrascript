# Pattern matching no escolhe/caso e cardapio (enum).
# Roda com: gs roda examples/padroes.gs

cardapio Forma
    circulo
    retangulo
    triangulo
acabou_finalmente

treta Ponto
    x
    y
acabou_finalmente

# padrao de lista: tamanho exato, resto e curinga
gambiarra resume(xs)
    escolhe xs
    caso []
        funciona "nada na lista"
    caso [unico]
        funciona "so o ${unico}"
    caso [primeiro, _]
        funciona "dois, comecando no ${primeiro}"
    caso [primeiro, ...resto]
        funciona "${primeiro} e mais ${tamanho(resto)}"
    acabou_finalmente
acabou_finalmente

# padrao de dicionario (subconjunto das chaves) e guarda
gambiarra trata(evento)
    escolhe evento
    caso {"tipo": "erro", "codigo": c} se c >= 500
        funciona "erro do servidor (${c})"
    caso {"tipo": "erro", msg}
        funciona "erro: " + msg
    caso {"tipo": "clique", "em": Ponto{x, y}}
        funciona "clique em ${x},${y}"
    se_nao_colar
        funciona "evento estranho"
    acabou_finalmente
acabou_finalmente

# cardapio dentro de padrao
gambiarra area(f)
    escolhe f
    caso [Forma.circulo, r]
        funciona 3 * r * r
    caso [Forma.retangulo, l, a]
        funciona l * a
    caso [Forma.triangulo, b, a]
        funciona b * a / 2
    acabou_finalmente
acabou_finalmente

mostra resume([])
mostra resume([7])
mostra resume([1, 2])
mostra resume([1, 2, 3, 4])

mostra trata({"tipo": "erro", "codigo": 503, "msg": "caiu"})
mostra trata({"tipo": "erro", "codigo": 404, "msg": "sumiu"})
mostra trata({"tipo": "clique", "em": Ponto{3, 4}})
mostra trata(42)

pra_cada f em Forma
    mostra "${f.indice}: ${f.nome}"
acabou_finalmente
mostra area([Forma.circulo, 2])
mostra area([Forma.retangulo, 2, 5])
mostra area([Forma.triangulo, 3, 4])
mostra tipo(Forma.circulo)
