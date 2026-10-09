# Testes das regras de tarefa (sem banco e sem servidor). Rode: gs testa
importa "rotas/tarefas.gs" como tarefas

# criar: titulo obrigatorio, feita opcional, campo desconhecido reprova
espera(valida({"titulo": "lavar a louca"}, tarefas.ESQUEMA_NOVA), [])
espera(valida({"titulo": "x", "feita": deu_bom}, tarefas.ESQUEMA_NOVA), [])
espera(valida({"titulo": 3}, tarefas.ESQUEMA_NOVA), ["titulo: tem que ser texto, veio numero"])
espera(valida({"titlo": "x"}, tarefas.ESQUEMA_NOVA), ["titulo: obrigatorio", "titlo: campo nao esperado"])

# mudar: tudo opcional, mas com o tipo certo
espera(valida({}, tarefas.ESQUEMA_MUDA), [])
espera(valida({"feita": "sim"}, tarefas.ESQUEMA_MUDA), ["feita: tem que ser booleano, veio texto"])

# o sqlite devolve 0/1; a API devolve booleano
bota linha = {"id": 3, "titulo": "pagar boleto", "feita": 1, "criada_em": "2026-01-01T00:00:00Z", "usuario_id": 9}
bota saida = tarefas.tarefa_saida(linha)
espera(saida.feita, deu_bom)
afirma(nao tem(saida, "usuario_id"), "nao vaza o dono")
espera(tarefas.booleano_sql(deu_bom), 1)
espera(tarefas.booleano_sql(deu_ruim), 0)
