# Datas parte 2 — soma/sub de duracoes, timezone, dia da semana, diferencas

bota inst = parse_tempo("2006-01-02 15:04:05", "2024-12-25 10:00:00")
mostra "instante: ${inst}"

# soma 2 horas (duracao devolve ns; soma_tempo aceita esse valor)
bota d2h = duracao({"h": 2})
mostra "+ 2h: ${soma_tempo(inst, d2h)}"

# sub 1 dia
bota d1d = duracao({"h": 24})
mostra "- 1d: ${sub_tempo(inst, d1d)}"

# dia da semana em portugues
mostra "dia da semana: ${dia_da_semana(inst)}"

# diferenca entre datas
bota amanha = parse_tempo("2006-01-02 15:04:05", "2024-12-26 12:00:00")
mostra "diferenca em dias: ${diferenca_dias(inst, amanha)}"
mostra "diferenca em horas: ${diferenca_horas(inst, amanha)}"

# converte pra outro fuso
mostra "Sao Paulo: ${converte_tz(inst, "America/Sao_Paulo")}"
mostra "London:    ${converte_tz(inst, "Europe/London")}"
mostra "Tokyo:     ${converte_tz(inst, "Asia/Tokyo")}"