# Testes das regras de conta (sem banco e sem servidor). Rode: gs testa
importa "rotas/usuarios.gs" como usuarios

# cadastro: o que passa e o que volta 400
bota ok = {"nome": "Jurandir", "email": "ju@exemplo.com", "senha": "segredo123"}
espera(valida(ok, usuarios.ESQUEMA_CADASTRO), [])
bota ruim = valida({"nome": "", "email": "ju", "senha": "123", "admin": deu_bom}, usuarios.ESQUEMA_CADASTRO)
espera(tamanho(ruim), 4)
afirma(contem(ruim[0], "nome"), "nome vazio reprova")
espera(valida(nada, usuarios.ESQUEMA_LOGIN), ["obrigatorio"])

# email e comparado sem caixa e sem espaco
espera(usuarios.normaliza_email("  Ju@Exemplo.COM "), "ju@exemplo.com")

# o hash da senha nunca sai pro cliente
bota linha = {"id": 1, "nome": "Ju", "email": "ju@exemplo.com", "senha_hash": "$2a$...", "criado_em": "2026-01-01T00:00:00Z"}
bota publico = usuarios.usuario_publico(linha)
afirma(nao tem(publico, "senha_hash"), "sem senha_hash na resposta")
espera(publico.email, "ju@exemplo.com")

# o hash falso tem formato de bcrypt (senao o login de email inexistente responde rapido)
afirma(comeca_com(usuarios.HASH_FALSO, "$2a$12$"), "hash falso e bcrypt custo 12")
afirma(nao confere_senha("qualquer", usuarios.HASH_FALSO), "hash falso nao abre nada")

# o token que o login devolve abre com o mesmo segredo
bota token = jwt_assina({"uid": 7}, "segredo-de-teste", {"expira_em": usuarios.VALIDADE_TOKEN})
espera(jwt_confere(token, "segredo-de-teste").uid, 7)
