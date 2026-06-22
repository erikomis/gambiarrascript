# 🇧🇷 GambiarraScript

> A linguagem de programação do jeitinho brasileiro. Escrita em Go.

GambiarraScript é uma linguagem onde você não fecha bloco com `}` nem com `end` — você fecha com **`acabou_finalmente`**, porque programar no Brasil é isso: deu trabalho, mas graças a Deus acabou.

## Salve, tropa

```
mostra "Salve, tropa!"

bota nome = "Erik"
bota idade = 25

se_colar idade >= 18
    mostra nome + " pode entrar"
se_nao_colar
    mostra "volta daqui a pouco"
acabou_finalmente
```

## Vocabulário

| GambiarraScript     | O que faz           |
|---------------------|---------------------|
| `bota`              | declara variável    |
| `mostra`            | imprime na tela     |
| `se_colar` / `se_nao_colar` | if / else / else-if |
| `enquanto`          | while               |
| `pra_cada i de 1 ate 10` | for numérico   |
| `pra_cada x em lista`    | for-each       |
| `gambiarra`         | declara função      |
| `funciona`          | return              |
| `arruma` / `quebrou`| try / catch         |
| `vaza` / `continua` | break / continue    |
| `deu_bom` / `deu_ruim` | true / false     |
| `nada`              | null                |
| `acabou_finalmente` | fecha o bloco       |

## Falando com o mundo (HTTP)

```
bota r = busca("https://httpbin.org/get")
se_colar r["ok"]
    mostra r["corpo"]
acabou_finalmente
```

`busca(url)` faz um GET; `busca(url, {"metodo": "POST", "corpo": "...", "cabecalhos": {...}, "timeout": 10})` cobre o resto. A resposta é um dicionário `{"status", "ok", "corpo", "cabecalhos"}`. É bloqueante (sem async): o resultado já vem pronto.

## Servindo HTTP (servidor)

```
gambiarra ola(pedido)
    funciona "salve, " + pedido["caminho"]
acabou_finalmente

rota("GET", "/", ola)
escuta(8080)
```

`rota(metodo, caminho, handler)` registra uma rota; o `handler` é uma `gambiarra`
que recebe um dicionário-pedido (`pedido["metodo"]`, `["caminho"]`, `["corpo"]`,
`["cabecalhos"]`, `["query"]`) e devolve um texto (corpo, status 200) ou um
dicionário `{"status", "corpo", "cabecalhos"}`. `escuta(porta)` sobe o servidor.
É serializado (uma requisição por vez) — concorrência de verdade vem depois.
Cabeçalhos e query usam a forma canônica nas chaves (ex.: `pedido["cabecalhos"]["X-Teste"]`, com maiúscula); quando um cabeçalho ou parâmetro de query vem com múltiplos valores, eles chegam unidos por `", "`.

## JSON

```
bota dados = de_json("{\"nome\": \"Erik\"}")
mostra dados["nome"]                       # Erik
mostra pra_json({"ok": deu_bom, "n": 42})  # {"ok":true,"n":42}
```

`de_json(texto)` transforma JSON em dicionário/lista/texto/número/booleano/`nada`;
`pra_json(valor)` faz o caminho inverso (compacto). Junto com `busca`, `rota` e
`escuta`, dá pra escrever uma API REST inteira: parsear o corpo do pedido com
`de_json(pedido["corpo"])` e responder com `pra_json(...)` e o cabeçalho
`Content-Type: application/json`.

## Pegadinhas / Semântica

- **Escopo de função no estilo Python**: a variável do `pra_cada` e a da cláusula `quebrou` continuam existindo depois que o bloco fecha — elas vazam pro escopo da função que as contém.
- **`e` / `ou` sempre devolvem booleano**: ao contrário de JS ou Python, `deu_bom e deu_bom` retorna `deu_bom` (booleano normalizado), nunca o operando original.
- **Escapes em textos**: as sequências `\"` (aspas), `\\` (barra invertida), `\n` (quebra de linha) e `\t` (tab) funcionam dentro das aspas — qualquer outro `\x` é mantido literal, barra e tudo.

## Quando deu ruim, a gente arruma

```
arruma
    bota resultado = 10 / 0
quebrou erro
    mostra "deu ruim, parca: " + erro
acabou_finalmente
```

## Como rodar (só precisa de Docker)

Sem instalar Go. O helper `scripts/dgo` roda tudo num container.

```bash
# rodar um arquivo
./scripts/dgo run ./cmd/gs roda examples/fizzbuzz.gs

# abrir o REPL interativo
docker run --rm -it -v "$PWD":/app -w /app golang:1.23 go run ./cmd/gs repl
```

## Rodando os testes

```bash
./scripts/dgo test ./...
```

Feito na gambiarra, com carinho. 🛠️

## Instalando o `gs` (binário nativo)

Compile uma vez via Docker e instale no PATH — depois roda sem Docker:

```bash
./scripts/build            # gera dist/gs nativo do seu sistema
./scripts/install          # copia pra /usr/local/bin (use --user p/ ~/.local/bin)
gs roda examples/fizzbuzz.gs
```

Pra gerar binários de todas as plataformas: `./scripts/build --all` (saída em `dist/`).

## Extensão do VSCode

Highlight, snippets, comando de rodar (F5) e language server com erros sublinhados.
Veja [editors/vscode/README.md](editors/vscode/README.md) — em resumo:
`./scripts/build-extension`, abra `editors/vscode` no VSCode e aperte F5.
