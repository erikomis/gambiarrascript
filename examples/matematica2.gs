# Matematica parte 2: potencia, trigonometria, log e o pi

# ** e potencia: inteiro elevado a inteiro continua inteiro
mostra 2 ** 10                 # 1024
mostra 2 ** 3 ** 2             # 512 (associa pela direita: 2 ** 9)
mostra -2 ** 2                 # -4 (o ** prende mais que o menos)
mostra (-2) ** 2               # 4
mostra 2 ** -1                 # 0.5 (expoente negativo vira quebrado)
mostra 2 ** 0.5                # raiz de 2

# composto tambem rola
bota lado = 4
lado **= 2
mostra lado                    # 16
bota capital = 1000
capital *= 1.1 ** 2            # dois anos a 10%
mostra "capital: " + formata("%.2f", capital)

# pi e um valor pronto (nao e gambiarra: sem parenteses)
bota r = 3
bota area = pi * r ** 2
mostra "area do circulo de raio 3: " + formata("%.4f", area)

# trigonometria em radianos
bota graus = 60
bota rad = graus * pi / 180
mostra "seno de 60: " + formata("%.4f", seno(rad))
mostra "cosseno de 60: " + formata("%.4f", cosseno(rad))
mostra "tangente de 45: " + arredonda(tangente(pi / 4))

# log natural, log10 e exp (exp desfaz o log)
mostra log10(1000)             # 3
mostra exp(0)                  # 1
mostra arredonda(log(exp(5)))  # 5

# log de zero ou negativo nao existe
arruma
    mostra log(0)
quebrou err
    mostra "eita: " + erro_msg(err)
acabou_finalmente
