# le um JSON de ~1 MB e faz 10 voltas de parse + serializa
# (o arquivo nao se chama json.py porque ai o "import json" importaria ele)
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    bruto = f.read()
saida = ""
n = 0
for volta in range(10):
    doc = json.loads(bruto)
    n = len(doc["itens"])
    saida = json.dumps(doc, separators=(",", ":"), ensure_ascii=False)
print(n)
print(len(saida))
print("deu_bom" if saida == bruto else "deu_ruim")
