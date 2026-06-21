gambiarra fizzbuzz(n)
    pra_cada i de 1 ate n
        se_colar i % 15 == 0
            mostra "FizzBuzz"
        se_nao_colar se_colar i % 3 == 0
            mostra "Fizz"
        se_nao_colar se_colar i % 5 == 0
            mostra "Buzz"
        se_nao_colar
            mostra i
        acabou_finalmente
    acabou_finalmente
acabou_finalmente

fizzbuzz(20)
