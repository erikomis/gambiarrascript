# Seguranca de API: senha com bcrypt, JWT HS256, AES-256-GCM e token aleatorio.
# O que e aleatorio (hash, nonce, token) sai como booleano/tamanho, entao a
# saida e a mesma toda vez — e nos 2 engines (gs roda / gs roda --tree).

# --- senha: guarda o hash, nunca a senha ---
# custo 12 e o padrao (~250ms); aqui 4 pro exemplo rodar ligeiro
bota hash = hash_senha("hunter2", 4)
mostra "hash bcrypt com ${tamanho(hash)} chars, comeca com $2a$? ${comeca_com(hash, "$2a$")}"
mostra "senha certa:  ${confere_senha("hunter2", hash)}"
mostra "senha errada: ${confere_senha("Hunter2", hash)}"
mostra "hash lixo:    ${confere_senha("hunter2", "isso nem e hash")}"

# --- JWT: login devolve token, rota protegida confere ---
crava SEGREDO = "troca-isso-por-gera_chave()"

gambiarra login(usuario, senha)
    se_colar nao confere_senha(senha, hash)
        quebra("usuario ou senha errados")
    acabou_finalmente
    funciona jwt_assina({"sub": usuario, "papel": "admin"}, SEGREDO, {"expira_em": 3600})
acabou_finalmente

gambiarra quem_e(token)
    arruma
        bota claims = jwt_confere(token, SEGREDO)
        funciona "${claims.sub} (${claims.papel})"
    quebrou err
        # erro de token vem com tipo "jwt": vira 401, nao 500
        funciona "401 — ${erro_tipo(err)}: ${erro_msg(err)}"
    acabou_finalmente
acabou_finalmente

bota token = login("ze", "hunter2")
mostra "token tem 3 partes? ${tamanho(separa(token, ".")) == 3}"
mostra "quem e: ${quem_e(token)}"
mostra "segredo errado: ${quem_e(jwt_assina({"sub": "ze"}, "outro segredo"))}"
mostra "expirado: ${quem_e(jwt_assina({"sub": "ze"}, SEGREDO, {"expira_em": -1}))}"
mostra "alg none: ${quem_e("eyJhbGciOiJub25lIn0.eyJzdWIiOiJ6ZSJ9.")}"

# o exemplo do jwt.io sai identico, byte a byte
bota exemplo = jwt_assina({"sub": "1234567890", "name": "John Doe", "iat": 1516239022}, "your-256-bit-secret")
mostra "bate com o jwt.io? ${termina_com(exemplo, "SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c")}"

# --- AES-256-GCM: chave do gera_chave() (rapido) ou frase (scrypt, lento) ---
bota chave = gera_chave()
bota cifrado = encripta("cpf 123.456.789-00", chave)
mostra "chave com ${tamanho(chave)} chars; cifrado de novo da outro texto? ${cifrado != encripta("cpf 123.456.789-00", chave)}"
mostra "decripta: ${decripta(cifrado, chave)}"
mostra "com frase: ${decripta(encripta("so com frase", "minha frase longa"), "minha frase longa")}"
arruma
    decripta(cifrado, gera_chave())
quebrou err
    mostra "chave errada -> ${erro_msg(err)}"
acabou_finalmente

# --- token aleatorio (crypto/rand) pra sessao, reset de senha, API key ---
bota sessao = token_aleatorio()
mostra "token de sessao: ${tamanho(sessao)} chars, url-safe? ${busca_regex("^[A-Za-z0-9_-]+$", sessao)}"
mostra "dois tokens iguais? ${token_aleatorio(16) == token_aleatorio(16)}"
