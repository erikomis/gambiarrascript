# Exemplos de concorrencia: bora + espera + cano
# Rode: gs roda examples/concorrencia_bora.gs

gambiarra demora(n)
    bota out = 0
    pra_cada i de 1 ate n
        bota out = out + i
        espera(10) # simula trabalho
        mostra "soma parcial " + i + " = " + out
    acabou_finalmente
    funciona out
acabou_finalmente

# dispara duas em paralelo
bota f1 = bora demora(100)
bota f2 = bora demora(1000)

mostra "as duas tanque que tanque em paralelo..."
mostra "soma 1..100  = " + espera(f1)
mostra "soma 1..1000 = " + espera(f2)

# dispara varias e espera todas
bota fs = [bora demora(1), bora demora(2), bora demora(3), bora demora(4)]
mostra "juntas = " + espera(fs)