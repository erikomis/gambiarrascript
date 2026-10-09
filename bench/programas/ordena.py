# 300 mil inteiros pseudo-aleatorios (MINSTD, semente 42 — o mesmo gerador
# nas 3 linguagens, pra ordenar exatamente a mesma lista) e um sort
x = 42
xs = []
for i in range(300000):
    x = (x * 48271) % 2147483647
    xs.append(x % 1000000)
xs.sort()
print(xs[0])
print(xs[150000])
print(xs[299999])
