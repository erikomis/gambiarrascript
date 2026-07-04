# exemplos das novas libs do Tier 1

# --- regex ---
mostra busca_regex("\\d{2}", "azeite23litros")   # deu_bom
bota grupos = combina_regex("(\\w+)@(\\w+)", "Ze@xx; Rita@yy")
pra_cada g em grupos
    mostra g
acabou_finalmente
mostra substitui_regex("(\\w+)@(\\w+)", "$2/$1", "ze@xx")
mostra separa_regex("\\s+", "oi   tropa  do  bem")

# --- tempo ---
mostra agora()
bota t = parse_tempo("2006-01-02", "2024-06-15")
mostra formata_tempo("02/01/2006", t)
mostra duracao({"h": 1, "m": 30})

# --- crypto ---
mostra sha256("gambiarra")
mostra md5("gambiarra")
mostra base64_codifica("tropa")
mostra base64_decodifica(base64_codifica("tropa"))
mostra hex_codifica("AB")

# --- lista extras ---
mostra unicos([1, 2, 1, 3, 2, 3, 4])
mostra achatada([[1, 2], [3], [4, 5]])

# --- conjunto (Set) ---
bota s = conjunto([1, 2, 3, 2, 1])
mostra s
mostra contem_conjunto(s, 2)
mostra uniao(conjunto([1, 2]), conjunto([2, 3]))
mostra intersecao(conjunto([1, 2, 3]), conjunto([2, 3, 4]))
mostra diferenca(conjunto([1, 2, 3]), conjunto([2]))