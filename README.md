# 🇧🇷 GambiarraScript

> A linguagem de programação do jeitinho brasileiro. Escrita em Go.

GambiarraScript é uma linguagem onde você não fecha bloco com `}` nem com `end` — você fecha com **`acabou_finalmente`**, porque programar no Brasil é isso: deu trabalho, mas graças a Deus acabou.

**Testa sem instalar nada:** [playground no navegador](https://erikomis.github.io/gambiarrascript/playground/) · [documentação](https://erikomis.github.io/gambiarrascript/docs/)

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
| `crava`             | declara constante   |
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

`rota(metodo, caminho, handler)` registra uma rota (com parametro e curinga:
`/usuarios/:id` → `pedido["params"]["id"]`, `/arquivos/*resto`); o `handler`
recebe o dicionario-pedido (`metodo`, `caminho`, `corpo`, `cabecalhos`,
`query`, `params`, `json` ja parseado, `ip`, `cookies`) e devolve texto, lista
ou dicionario (viram JSON), `responde_json(valor, status)` ou
`{"status", "corpo", "cabecalhos"}`. Tem middleware (`antes`/`depois`),
`cors()`, `serve_pasta(prefixo, pasta)` e WebSocket (`rota_ws` no servidor,
`conecta_ws` no cliente, com `envia`/`recebe`/`fecha`). `escuta(porta)` sobe
o servidor e desliga com calma no ctrl+c. Erro no handler vira `500`
generico pro cliente e o detalhe vai pro stderr. Cada requisicao roda na
propria goroutine. Exemplos: `examples/api_rest.gs` e `examples/chat_ws.gs`;
doc completa em [Servidor HTTP](https://erikomis.github.io/gambiarrascript/docs/servidor/).

## Rede crua (TCP/UDP)

```
gambiarra eco(c)
    enquanto deu_bom
        bota linha = recebe(c)      # nada = o cliente desligou
        se_colar linha == nada
            vaza
        acabou_finalmente
        envia(c, "eco: " + linha)
    acabou_finalmente
acabou_finalmente

escuta_tcp(9000, eco)               # teste com: nc 127.0.0.1 9000
```

Conexao de rede se usa igual a cano: `envia`, `recebe` e `fecha`.
`conecta_tcp("host:porta", {"timeout": 5})` abre o cliente; por padrao cada
`recebe` devolve uma linha (sem o `\n`) e cada `envia` manda o texto + `\n` —
`{"modo": "bruto"}` troca por bytes crus. `escuta_tcp` roda cada conexao na
propria goroutine e para com calma no ctrl+c. UDP: `escuta_udp(porta,
gambiarra(msg, remetente) ...)`, `envia_udp` e `conecta_udp`. Exemplos em
`examples/eco_tcp.gs`, `examples/cliente_tcp.gs` e `examples/udp.gs`.

## Concorrencia e colecoes

Handler de `rota`/`rota_ws`/`escuta_tcp`, `bora` e `paralelo` rodam em
goroutines de verdade, e lista, dicionario e conjunto aguentam isso: cada
operacao (`d["k"]`, `bota d["k"] = v`, `adiciona`, `remove`, `tamanho`,
`pra_cada`...) e atomica e nunca derruba o processo, igual o GIL do Python.
Operacao COMPOSTA nao e — `bota d["n"] = d["n"] + 1` em duas goroutines pode
perder incremento. Pra isso tem trava:

```
bota t = trava()
bota visitas = {"n": 0}
rota("GET", "/", gambiarra(pedido)
    bota n = com_trava(t, gambiarra()
        bota visitas["n"] = visitas["n"] + 1
        funciona visitas["n"]
    acabou_finalmente)
    funciona "visita numero " + n
acabou_finalmente)
```

`com_trava(t, gambiarra)` roda a gambiarra segurando a trava, devolve o que
ela devolver e solta sempre — erro dentro sobe normal pra quem chamou. A
trava nao e reentrante: pedir a mesma trava de dentro do proprio `com_trava`
da erro (em vez de travar pra sempre). `pra_cada` numa colecao que outra
goroutine esta mexendo percorre um retrato tirado no comeco do laco.

## Segurança e configuração

```
bota cfg = opcoes({"porta": 8080, "segredo": env("JWT_SEGREDO", "troca-isso")})
bota hash = hash_senha("hunter2")               # bcrypt
mostra confere_senha("hunter2", hash)          # deu_bom
bota token = jwt_assina({"usuario": "erik"}, cfg["segredo"], {"expira_em": 3600})
mostra jwt_confere(token, cfg["segredo"])["usuario"]   # erik
bota chave = gera_chave()
mostra decripta(encripta("cartao 1234", chave), chave) # AES-256-GCM
log_info("subiu", {"porta": cfg["porta"]})     # stderr; GS_LOG_FORMATO=json
```

`opcoes` lê `--porta 9090` da linha de comando (com `--ajuda` gerado),
`carrega_env()` lê um `.env`, e `jwt_confere` recusa token adulterado,
expirado ou com `alg` diferente de HS256 com um erro do tipo `"jwt"` — dá pra
responder 401 num `antes(...)`. Veja `examples/seguranca.gs`,
`examples/config.gs` e `examples/api_rest.gs`.

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

## Constantes e matemática

```
crava TAXA = 0.1               # constante: `bota TAXA = 0.2` nem roda
bota r = 3
mostra pi * r ** 2             # ** é potência (2 ** 3 ** 2 == 512)
mostra seno(pi / 2)            # 1 — também tem cosseno, tangente, log, log10, exp
```

`crava` fixa o nome, não o conteúdo: uma lista cravada ainda muda por dentro
(igual `const` do JS). A checagem é estática — `gs check` e o editor acusam a
reatribuição antes de rodar. Veja `examples/constantes.gs` e
`examples/matematica2.gs`.

## Quando deu ruim, a gente arruma

```
arruma
    bota resultado = 10 / 0
quebrou erro
    mostra "deu ruim, parca: " + erro
acabou_finalmente
```

## Instalação e uso

### Jeito rápido — binário pronto (macOS / Linux)

```bash
# Homebrew
brew install erikomis/tap/gambiarrascript

# ou o instalador (sem Homebrew)
curl -fsSL https://raw.githubusercontent.com/erikomis/gambiarrascript/main/install.sh | sh

gs roda examples/fizzbuzz.gs
```

O `gs` do Ghostscript tem o mesmo nome — se você usa o Ghostscript pelo
Homebrew, a fórmula avisa do conflito; prefira o `install.sh` com
`GS_DIR=~/.local/bin`. A extensão do VSCode vem como `.vsix` em cada
[release](https://github.com/erikomis/gambiarrascript/releases)
(Extensions → `...` → Install from VSIX).

Baixa o binário da última release, confere o sha256 e instala em
`/usr/local/bin` (ou `~/.local/bin` se não tiver permissão — nunca usa sudo).
`GS_VERSAO=0.2.0` fixa uma versão, `GS_DIR=~/bin` escolhe a pasta. No Windows,
baixa o `.zip` direto da [página de releases](https://github.com/erikomis/gambiarrascript/releases).

Quer compilar você mesmo? Dois caminhos:

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

## Site, playground e releases

- `./scripts/build-web` gera o runtime WASM do playground (`web/public/gs.wasm`);
  depois `cd web && npm run dev`. Com `--site` builda o site estático em `web/out/`.
- Todo push na `main` publica docs + playground no GitHub Pages
  (`.github/workflows/pages.yml`).
- Toda tag `v*` gera binários mac/linux/windows + checksums numa GitHub Release
  (`.github/workflows/release.yml`): `git tag v0.2.0 && git push origin v0.2.0`.

## Extensão do VSCode

Highlight, snippets, comando de rodar (F5) e language server com erros sublinhados.
Veja [editors/vscode/README.md](editors/vscode/README.md) — em resumo:
`./scripts/build-extension`, abra `editors/vscode` no VSCode e aperte F5.

Licença [MIT](LICENSE). Feito na gambiarra, com carinho. 🛠️
