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

### Strings com crase (sem escapar aspas)

Pra escrever JSON, caminhos ou regex sem escapar cada `"`, use crase — string
crua, igual ao Go e ao Node:

```
bota j = `{"nome": "Erik", "tags": ["go", "gs"]}`
mostra de_json(j)["nome"]   # Erik
```

Dentro de crases nada é escapado (`\n` é barra-n literal) e a string pode ocupar
várias linhas. Pra escapes (`\n`, `\t`, `\"`) use aspas duplas `"..."`.

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

## Instalação e uso

Dois caminhos, escolhe o que rolar na sua máquina:

- **Tem Go?** roda direto, sem Docker.
- **Não tem Go, mas tem Docker?** roda tudo num container, sem instalar nada.

### Caminho A — com Go instalado (sem Docker)

Precisa do **Go 1.23 ou mais novo** (`go version` pra conferir):

```bash
# rodar um arquivo direto (sem compilar nada)
go run ./cmd/gs roda examples/fizzbuzz.gs

# abrir o REPL interativo
go run ./cmd/gs repl

# instalar o binário `gs` no PATH e usar de qualquer lugar
go install ./cmd/gs        # joga o `gs` em $(go env GOPATH)/bin
gs roda examples/fizzbuzz.gs
```

Se depois do `go install` o `gs` não for encontrado, garanta que
`$(go env GOPATH)/bin` está no seu `PATH`. Se preferir só gerar o binário sem
instalar: `go build -o dist/gs ./cmd/gs`.

### Caminho B — com Docker (sem instalar Go)

O helper `scripts/dgo` roda tudo num container `golang:1.23`:

```bash
# rodar um arquivo
./scripts/dgo run ./cmd/gs roda examples/fizzbuzz.gs

# abrir o REPL interativo
docker run --rm -it -v "$PWD":/app -w /app golang:1.23 go run ./cmd/gs repl
```

Pra ter o binário nativo no PATH e rodar sem Docker daí pra frente:

```bash
./scripts/build            # compila via Docker -> dist/gs nativo do seu sistema
./scripts/install          # copia pra /usr/local/bin (use --user p/ ~/.local/bin)
gs roda examples/fizzbuzz.gs
```

`./scripts/build --all` gera binários de todas as plataformas (saída em `dist/`).
No macOS (Apple Silicon) o `build`/`install` já reassinam o binário com
`codesign` — o cross-compile via Docker sai sem assinatura e o macOS mata o
binário com "Killed: 9".

## Comandos do `gs`

Além de `roda`, `repl` e `lsp`, o CLI tem:

| Comando | O que faz |
|---------|-----------|
| `gs roda [--vm] [--cache] <arq.gs>` | executa o arquivo (`--vm` usa a máquina virtual, `--cache` reaproveita o bytecode `.gsc`) |
| `gs check <arq.gs>...`   | parse + lint (erros e avisos) sem rodar |
| `gs formata [-w] <arq.gs>...` | formata o código (`-w` sobrescreve no disco) |
| `gs testa [<dir>]`       | roda os `*_test.gs` e soma os asserts |
| `gs init [nome]`         | cria o esqueleto do projeto (`gambiarra.json` + `principal.gs`) |
| `gs bench [--vm] <arq.gs> [n]` | mede o tempo de execução em `n` rodadas |
| `gs get <url> [nome.gs]` | baixa um módulo `.gs` pra `gs_modulos/` |
| `gs build <arq.gs> [-o saida]` | gera um binário standalone com o script embutido |

Roda `gs` sem argumentos (ou `gs --help`) pra ver a ajuda completa.

## Rodando os testes

```bash
go test ./...                 # com Go instalado
./scripts/dgo test ./...      # via Docker
```

## Extensão do VSCode

Highlight, snippets, comando de rodar (F5) e language server com erros sublinhados.
Veja [editors/vscode/README.md](editors/vscode/README.md) — em resumo:
`./scripts/build-extension`, abra `editors/vscode` no VSCode e aperte F5.

Feito na gambiarra, com carinho. 🛠️
