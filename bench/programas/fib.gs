# fib(30) recursivo: custo de chamada de funcao
gambiarra fib(n)
    se_colar n < 2
        funciona n
    acabou_finalmente
    funciona fib(n - 1) + fib(n - 2)
acabou_finalmente
mostra fib(30)
