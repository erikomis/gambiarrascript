# Testes dos ajudantes de HTTP. Rode: gs testa
importa "lib/http.gs" como http

# rotas publicas x protegidas
afirma(http.eh_publica("POST", "/login"), "login e publico")
afirma(http.eh_publica("POST", "/usuarios"), "cadastro e publico")
afirma(http.eh_publica("HEAD", "/saude"), "HEAD conta como GET")
afirma(nao http.eh_publica("GET", "/eu"), "/eu pede token")
afirma(nao http.eh_publica("GET", "/usuarios"), "so o POST de /usuarios e publico")
afirma(nao http.eh_publica("DELETE", "/tarefas/1"), "tarefas pedem token")

# token do cabecalho Authorization
espera(http.extrai_token("Bearer abc.def.ghi"), "abc.def.ghi")
espera(http.extrai_token("bearer   abc"), nada)
espera(http.extrai_token("bearer abc"), "abc")
espera(http.extrai_token("Basic dXNlcjpzZW5oYQ=="), nada)
espera(http.extrai_token("Bearer"), nada)
espera(http.extrai_token(nada), nada)

# :id do caminho
espera(http.id_valido("42"), 42)
espera(http.id_valido("abc"), nada)
espera(http.id_valido("1.5"), nada)
espera(http.id_valido("-1"), nada)
espera(http.id_valido(""), nada)

# respostas de erro
bota r = http.erro_json(404, "nao achei")
espera(r.status, 404)
espera(de_json(r.corpo), {"erro": "nao achei"})
espera(http.erros_json(["titulo: obrigatorio"]).status, 400)
