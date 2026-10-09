# map + filter sobre 1 milhao de itens (list comprehension, o jeito idiomatico)
xs = list(range(1000000))
ys = [y for y in [x * 2 for x in xs] if y % 3 == 0]
print(len(ys))
print(sum(ys))
