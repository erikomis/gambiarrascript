# __NOME__

Script de linha de comando feito com
[GambiarraScript](https://github.com/erikomis/gambiarrascript).

```bash
gs roda principal.gs --ajuda                       # lista as flags
gs roda principal.gs --nome tropa --vezes 2 --grita
gs roda principal.gs --sem-grita -- a.txt b.txt    # depois do -- tudo vai pra cfg._
```

As flags saem do `opcoes()` no topo do `principal.gs`: o tipo de cada uma
vem do valor padrao (numero, texto, booleano, lista), e flag desconhecida ou
valor zoado vira erro com a lista das que existem.

Pra distribuir sem precisar do `gs` instalado:

```bash
gs build principal.gs -o __NOME__
./__NOME__ --ajuda
```
