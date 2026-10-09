# geradores: `rende` faz a gambiarra entregar valores um por um, sob demanda

# infinito sem drama: so roda quando alguem pede
gambiarra naturais()
    bota n = 0
    enquanto deu_bom
        rende n
        n += 1
    acabou_finalmente
acabou_finalmente

# gerador que filtra outro (pipeline preguicoso)
gambiarra pares(origem)
    pra_cada x em origem
        se_colar x % 2 == 0
            rende x
        acabou_finalmente
    acabou_finalmente
acabou_finalmente

mostra pega(pares(naturais()), 5)  # [0, 2, 4, 6, 8]

# fibonacci ate passar de um limite
gambiarra fibonacci(limite)
    bota [a, b] = [0, 1]
    enquanto a <= limite
        rende a
        bota [a, b] = [b, a + b]
    acabou_finalmente
acabou_finalmente

pra_cada i, f em fibonacci(50)
    mostra "fib ${i} = ${f}"
acabou_finalmente

# pedindo na mao
bota g = fibonacci(1)
mostra proximo(g)          # 0
mostra proximo(g)          # 1
mostra proximo(g)          # 1
mostra acabou(g)           # deu_bom
mostra proximo(g, "fim")   # fim

# uma treta percorrivel: o itera() pode ser um gerador
treta Intervalo
    inicio
    fim
    passo = 1
acabou_finalmente

gambiarra (r Intervalo) itera()
    bota i = r.inicio
    enquanto i <= r.fim
        rende i
        i += r.passo
    acabou_finalmente
acabou_finalmente

pra_cada x em Intervalo{inicio: 0, fim: 10, passo: 5}
    mostra x               # 0, 5, 10
acabou_finalmente
mostra soma(lista(Intervalo{1, 4, 1}))  # 10

# erro dentro do gerador estoura onde o valor foi pedido
gambiarra inversos(xs)
    pra_cada x em xs
        rende 1 / x
    acabou_finalmente
acabou_finalmente

arruma
    pra_cada v em inversos([4, 2, 0, 1])
        mostra v
    acabou_finalmente
quebrou erro
    mostra "parou: " + erro_msg(erro)
acabou_finalmente
