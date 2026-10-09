# monta um texto de 100 mil caracteres concatenando um por vez
s = ""
for i in range(100000):
    s += str(i % 10)
print(len(s))
print(s[-10:])
