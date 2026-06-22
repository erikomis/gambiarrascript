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

## Pegadinhas / Semântica

- **Escopo de função no estilo Python**: a variável do `pra_cada` e a da cláusula `quebrou` continuam existindo depois que o bloco fecha — elas vazam pro escopo da função que as contém.
- **`e` / `ou` sempre devolvem booleano**: ao contrário de JS ou Python, `deu_bom e deu_bom` retorna `deu_bom` (booleano normalizado), nunca o operando original.
- **Textos são crus**: ainda não existe sequência de escape dentro das aspas — `"linha1\nlinha2"` imprime literalmente `\n`, não uma quebra de linha.

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
