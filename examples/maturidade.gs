# Tier "maturidade" — escolhe/caso, lambdas, destructuring, dot access,
# atribuicao composta e formata() (printf).

# --- atribuicao composta (sem bota) ---
bota contador = 10
contador += 5
contador *= 2
mostra "contador: ${contador}"

# --- dot access: dict como record, com escrita e metodo ---
bota jogador = {"nome": "Erik", "pontos": 0}
jogador.pontos += 100
bota jogador.nivel = "chefe"
mostra "${jogador.nome} tem ${jogador.pontos} pontos (nivel ${jogador.nivel})"

# --- lambda anonima + builtins de ordem superior ---
bota nums = [1, 2, 3, 4, 5]
mostra "dobrados: ${mapeia(nums, gambiarra(n) funciona n * 2 acabou_finalmente)}"
mostra "pares: ${filtra(nums, gambiarra(n) funciona n % 2 == 0 acabou_finalmente)}"
mostra "soma: ${reduz(nums, gambiarra(acc, n) funciona acc + n acabou_finalmente, 0)}"

# --- destructuring ---
bota [primeiro, segundo] = nums
mostra "primeiro=${primeiro} segundo=${segundo}"
bota {nome, pontos} = jogador
mostra "desestruturado: ${nome} / ${pontos}"

# --- escolhe/caso (switch sem fallthrough) ---
gambiarra clima(condicao)
    escolhe condicao
    caso "sol", "calor"
        funciona "bora pra praia"
    caso "chuva"
        funciona "fica em casa"
    se_nao_colar
        funciona "sei la, olha pela janela"
    acabou_finalmente
acabou_finalmente
mostra clima("sol")
mostra clima("chuva")
mostra clima("neblina")

# --- formata: printf com verbos do Go ---
mostra formata("saldo: R$ %8.2f", 1234.5)
mostra formata("id: %05d | nome: %-10s|", 42, "gambiarra")

# --- async/await: bora + espera ---
gambiarra pesada(n)
    funciona n * n
acabou_finalmente
bota fut = bora pesada(12)
mostra "quadrado assincrono: ${espera(fut)}"
