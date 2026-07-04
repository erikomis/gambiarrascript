# exemplo de banco: conecta, cria tabela, insere e consulta

bota c = conecta("sqlite::memory:")

# estrutura
executa(c, "CREATE TABLE gente (id INTEGER PRIMARY KEY, nome TEXT, idade INTEGER)")
executa(c, "INSERT INTO gente (nome, idade) VALUES (?, ?)", "Ze", 30)
executa(c, "INSERT INTO gente (nome, idade) VALUES (?, ?)", "Rita", 25)
executa(c, "INSERT INTO gente (nome, idade) VALUES (?, ?)", "Bia", 41)

#SELECT com placeholder
bota jovens = consulta(c, "SELECT nome, idade FROM gente WHERE idade < ? ORDER BY idade", 40)
pra_cada linha em jovens
    mostra linha["nome"] + " tem " + texto(linha["idade"]) + " anos"
acabou_finalmente

# quantas linhas afetadas por um update
bota n = executa(c, "UPDATE gente SET idade = idade + 1 WHERE idade < 30")
mostra "linhas afetadas: " + texto(n)

fecha(c)