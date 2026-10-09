# 1 milhao de chamadas de metodo num objeto (class)
class Contador:
    def __init__(self):
        self.n = 0

    def soma(self, k):
        self.n += k


c = Contador()
for i in range(1000000):
    c.soma(i % 3)
print(c.n)
