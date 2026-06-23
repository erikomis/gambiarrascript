# JSON na GambiarraScript: parsear e serializar

bota json_texto = `{"nome": "Erik", "idade": 25, "langs": ["go", "gs"]}`
bota dados = de_json(json_texto)

mostra "nome: " + dados["nome"]
mostra "primeira lang: " + dados["langs"][0]
mostra "idade + 1: " + texto(dados["idade"] + 1)

# montando um valor e virando JSON
bota resposta = {"ok": deu_bom, "total": 2, "itens": ["a", "b"]}
mostra pra_json(resposta)
