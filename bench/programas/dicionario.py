# 200 mil chaves texto: insere tudo, depois le tudo de volta
d = {}
for i in range(200000):
    d["k" + str(i)] = i
s = 0
for i in range(200000):
    s += d["k" + str(i)]
print(len(d))
print(s)
