# 🇧🇷 GambiarraScript

> A linguagem de programação do jeitinho brasileiro. Escrita em Go.

GambiarraScript é uma linguagem onde você não fecha bloco com `}` nem com `end` — você fecha com **`acabou_finalmente`**, porque programar no Brasil é isso: deu trabalho, mas graças a Deus acabou.

**Testa sem instalar nada:** [playground no navegador](https://erikomis.github.io/gambiarrascript/playground/) · [documentação](https://erikomis.github.io/gambiarrascript/docs/)

## Salve, tropa

```
mostra "Salve, tropa!"

bota nome = "Jurandir"
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
| `rende`             | yield (gerador)     |
| `arruma` / `quebrou`| try / catch         |
| `vaza` / `continua` | break / continue    |
| `deu_bom` / `deu_ruim` | true / false     |
| `nada`              | null                |
| `treta` / `combinado` | struct / interface (POO estilo Go) |
| `escolhe` / `caso`  | switch / match (com pattern matching) |
| `cardapio`          | enum                |
| `acabou_finalmente` | fecha o bloco       |

## POO no estilo Go (treta e combinado)

Sem `class`, `this`, `new` nem herança: struct + método com receiver +
interface implícita + composição, copiado do Go.

```
treta Animal
    nome = "anonimo"          # campo com valor padrao
acabou_finalmente
gambiarra (a Animal) fala()   # metodo: o receiver vem antes do nome
    funciona a.nome + " faz barulho"
acabou_finalmente

treta Cachorro
    Animal                    # puxadinho (embedding): nome e fala() sobem
    raca
acabou_finalmente

combinado Falante             # interface: quem tem fala(), satisfaz
    fala()
acabou_finalmente

bota rex = Cachorro{Animal: Animal{nome: "rex"}, raca: "vira-lata"}
mostra rex.fala()             # rex faz barulho
mostra satisfaz(rex, Falante) # deu_bom
mostra tipo(rex)              # Cachorro (type switch: escolhe tipo(v) / caso "Cachorro")
```

- Instancia: `Ponto{x: 1, y: 2}` (por nome) ou `Ponto{1, 2}` (na ordem); campo
  que faltou fica com o padrão ou `nada`. Nome de treta começa com maiúscula.
- Tudo por referência: o método muda a treta. `==` compara campo a campo.
- "Construtor" é convenção: `gambiarra nova_ponto(x, y) funciona Ponto{x, y} acabou_finalmente`.
- `satisfaz(v, Tipo)` e `como_tipo(v, Tipo)` (o `v.(T)` do Go); `pra_json`
  achata o puxadinho igual o `encoding/json`.

Guia completo em `web/content/docs/poo.mdx`; exemplo rodando em `examples/poo.gs`.

## Pattern matching e cardapio (enum)

O `caso` do `escolhe` casa formato e amarra os pedaços; `cardapio` é o enum.

```
cardapio Forma
    circulo
    quadrado
acabou_finalmente

gambiarra trata(v)
    escolhe v
    caso [Forma.circulo, r]                # cardapio dentro de padrao
        funciona 3 * r * r
    caso [primeiro, ...resto] se primeiro > 0   # lista + guarda
        funciona "comeca com ${primeiro}, sobram ${tamanho(resto)}"
    caso {"tipo": "erro", msg}             # dicionario: so as chaves listadas
        funciona "erro: " + msg
    caso Ponto{x: 0, y}                    # treta: confere o tipo e amarra
        funciona "no eixo y em ${y}"
    se_nao_colar
        funciona "sei la"
    acabou_finalmente
acabou_finalmente
```

- Nome solto só amarra **dentro** de `[ ]`, `{ }` e `Tipo{ }`; `caso x`
  continua comparando com a variável `x`. `_` é o curinga.
- `Forma.circulo` imprime `Forma.circulo`, `tipo()` dá `"Forma"`, compara por
  identidade e tem `.nome`/`.indice`; `pra_cada f em Forma` anda nas opções.
- `gs check` avisa `escolhe` sobre um cardapio que esquece opção sem
  `se_nao_colar`. Guia em `web/content/docs/estruturas.mdx` e `poo.mdx`;
  exemplo em `examples/padroes.gs`.

## Geradores (`rende`)

Gambiarra com `rende` vira gerador: entrega um valor por vez, só quando
alguém pede — infinito sem drama.

```
gambiarra naturais()
    bota n = 0
    enquanto deu_bom
        rende n
        n += 1
    acabou_finalmente
acabou_finalmente

mostra pega(naturais(), 5)    # [0, 1, 2, 3, 4]
pra_cada x em naturais()
    se_colar x > 2
        vaza
    acabou_finalmente
    mostra x
acabou_finalmente
```

`proximo(g, [padrao])`, `acabou(g)`, `lista(g)`; treta com `itera()` funciona no
`pra_cada`. Exemplo em `examples/geradores.gs`.

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
o servidor e desliga com calma no ctrl+c; `escuta(porta, {"tls": {"cert":
"cert.pem", "chave": "chave.pem"}})` sobe em HTTPS (e `wss://`). Erro no handler vira `500`
generico pro cliente e o detalhe vai pro stderr. Cada requisicao roda na
propria goroutine. O corpo se confere com `valida(pedido.json, esquema)`, que
devolve a lista de erros com o caminho de cada um (`"itens[2].preco:
obrigatorio"`) pra responder `400`; o banco fica em dia na subida com
`migra(conexao)` (ou `gs migra` na linha de comando). Exemplos: `examples/api_rest.gs` e `examples/chat_ws.gs`;
doc completa em [Servidor HTTP](https://erikomis.github.io/gambiarrascript/docs/servidor/).

## Páginas HTML (modelos)

```
rota("GET", "/", gambiarra(pedido)
    funciona responde_html(renderiza_arquivo("modelos/inicio.html", {
        "titulo": "Cardapio",
        "produtos": [{"nome": "Cafe <forte>", "preco": 6.5}]
    }))
acabou_finalmente)
```

```html
{{ usa "base.html" }}
{{ bloco "conteudo" }}
<h1>{{ titulo }}</h1>
<ul>
  {{ pra_cada p em produtos }}
  <li>{{ p.nome }} — R$ {{ p.preco | formata "%.2f" }}</li>
  {{ acabou }}
</ul>
{{ acabou }}
```

`renderiza(texto, dados)` e `renderiza_arquivo(caminho, dados)` preenchem o
modelo; todo valor sai **escapado** pra HTML (`Cafe &lt;forte&gt;`), e
`{{{ x }}}` ou `| cru` mandam cru. Tem filtros (`maiusculo`, `tamanho`,
`json`, `formata "%.2f"`, `padrao "x"`, `junta ", "`), `se_colar` com
comparação, `{{# comentário }}`, parcial (`inclui "parcial.html"`, sem sair
da pasta dos modelos) e layout (`usa` + `bloco`). Variável que falta sai
vazia (`{"estrito": deu_bom}` vira erro). `responde_html` monta a resposta
com `Content-Type: text/html`. Exemplo: `examples/site.gs`; doc em
[Modelos](https://erikomis.github.io/gambiarrascript/docs/modelos/).

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

## Agenda e datas

```
a_cada(60, gambiarra() log_info("ainda de pe") acabou_finalmente)
agenda("0 9 * * 1-5", gambiarra() mostra "bom dia, tropa" acabou_finalmente)
bota h = depois_de(5, gambiarra() mostra "5s depois" acabou_finalmente)
cancela(h)

mostra formata_data(agora(), "dddd, dd 'de' mmmm 'as' hh:mi", "America/Sao_Paulo")
# quinta-feira, 08 de outubro as 14:05
mostra le_data("25/12/2026", "dd/mm/aaaa")   # 2026-12-25T00:00:00Z
```

Cada agendamento roda na propria goroutine (cron de 5 campos no fuso local,
com `*/5`, listas, faixas e atalhos tipo `@diario`); erro na tarefa vai pro
stderr e o agendamento segue. O `gs roda` fica de pe enquanto tiver
agendamento e o Ctrl+C para com calma. Nas datas, `mm` e mes e `mi` e minuto.

## Segurança e configuração

```
bota cfg = opcoes({"porta": 8080, "segredo": env("JWT_SEGREDO", "troca-isso")})
bota hash = hash_senha("hunter2")               # bcrypt
mostra confere_senha("hunter2", hash)          # deu_bom
bota token = jwt_assina({"usuario": "jurandir"}, cfg["segredo"], {"expira_em": 3600})
mostra jwt_confere(token, cfg["segredo"])["usuario"]   # jurandir
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
bota dados = de_json("{\"nome\": \"Jurandir\"}")
mostra dados["nome"]                       # Jurandir
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
bota j = `{"nome": "Jurandir", "tags": ["go", "gs"]}`
mostra de_json(j)["nome"]   # Jurandir
```

Dentro de crases nada é escapado (`\n` é barra-n literal) e a string pode ocupar
várias linhas. Pra escapes (`\n`, `\t`, `\"`) use aspas duplas `"..."`.

## Pegadinhas / Semântica

- **Escopo de função no estilo Python**: a variável do `pra_cada` e a da cláusula `quebrou` continuam existindo depois que o bloco fecha — elas vazam pro escopo da função que as contém.
- **Atribuir dentro de gambiarra cria local**: `bota x = ...`/`x += ...` dentro de uma função cria um `x` local (na função inteira) e a global fica igual — sem `global`/`nonlocal`. Ler antes de botar enxerga o valor de fora; closure enxerga a variável (não uma cópia). Estado compartilhado vai num dicionário/lista (`estado.total += 1`).
- **Inteiro que estoura 64 bits vira real**: `9223372036854775807 + 1` dá `9223372036854776000`, nunca volta pro negativo.
- **Erro pego é só um valor**: o que o `quebrou` pega dá pra mostrar, guardar e passar adiante sem relançar; só `quebra(...)` levanta de novo.
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

Dá pra ter vários `quebrou`, cada um com filtro (`se` + qualquer condição);
o primeiro que colar pega, e o que nenhum pegar continua subindo:

```
arruma
    bota cfg = de_json(le_arquivo("config.json"))
quebrou erro se erro_tipo(erro) == "io"
    bota cfg = {}                  # sem arquivo: config padrão
quebrou erro
    quebra("config quebrada", erro)
finalmente
    mostra "pronto"
acabou_finalmente
```

## Desempenho

Na faixa do CPython: empata ou ganha em recursão, laço, texto, ordenação e
JSON, e perde pro Node.js (que tem JIT) em laço apertado. Servidor HTTP usa
todos os núcleos. Números medidos (com checksum conferido entre as
linguagens), metodologia e onde o gs perde em
[Desempenho](https://erikomis.github.io/gambiarrascript/docs/desempenho/);
pra rodar na sua máquina: `sh bench/roda.sh` e `sh bench/http.sh`.

## Instalação e uso

### Jeito rápido — binário pronto (macOS / Linux)

```bash
# Homebrew
brew install erikomis/tap/gambiarrascript

# ou o instalador (macOS e Linux, sem Homebrew)
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
`GS_VERSAO=0.2.0` fixa uma versão, `GS_DIR=~/bin` escolhe a pasta. No Windows (PowerShell, sem admin): `irm https://raw.githubusercontent.com/erikomis/gambiarrascript/main/install.ps1 | iex`
— instala em `%LOCALAPPDATA%\Programs\gambiarrascript` e poe no PATH do
usuario. Passo a passo de cada sistema (PATH, WSL, Raspberry Pi, `.zip` na mao)
na [doc de instalacao](https://erikomis.github.io/gambiarrascript/docs/instalacao/).

### Docker — imagem oficial

```bash
docker run --rm ghcr.io/erikomis/gambiarrascript versao
docker run --rm -v "$PWD:/app" ghcr.io/erikomis/gambiarrascript roda app.gs
```

Pra subir uma API (`docker build` em cima da imagem oficial):

```dockerfile
FROM ghcr.io/erikomis/gambiarrascript:latest
COPY --chown=nonroot:nonroot . /app
EXPOSE 8080
CMD ["roda", "api.gs"]
```

Linux amd64 e arm64, distroless, roda como `nonroot`; `:latest` so anda nas
releases estaveis. Detalhes no [deploy com Docker](https://erikomis.github.io/gambiarrascript/docs/servidor/#deploy-com-docker).

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
| `gs testa --cobertura [<dir>]` | idem + % de linhas que rodaram por arquivo (`--cobertura-perfil arq` grava `arquivo:linha contagem`, `--cobertura-html arq` gera o relatório colorido) |
| `gs init [nome]`         | cria o esqueleto do projeto (`gambiarra.json` + `principal.gs`) |
| `gs bench [--vm] <arq.gs> [n]` | mede o tempo de execução em `n` rodadas |
| `gs debug [--tree] <arq.gs> [args]` | depurador no terminal: breakpoints (com condição), passo a passo, pilha e variáveis (`gs debug --dap` é o adapter que a extensão do VSCode usa) |
| `gs get <url \| github.com/u/repo/mod.gs@tag> [nome.gs]` | baixa um módulo `.gs` pra `gs_modulos/` e fixa no `gambiarra.lock` (URL + sha256) |
| `gs instala [--atualiza]` | instala tudo do `gambiarra.json`; com lock, recusa conteúdo que não bate (`--atualiza` re-resolve) |
| `gs build <arq.gs> [-o saida]` | gera um binário standalone com o script embutido |
| `gs build <arq.gs> --alvo linux/amd64` | standalone pra outra plataforma (baixa o `gs` da release e confere o checksum; `--gs-base` pra usar um local) |
| `gs migra [sobe \| desce [n] \| status \| novo nome]` | migrações SQL de `migracoes/NNN_nome.sobe.sql` (e `.desce.sql`), anotadas em `gs_migracoes` com checksum; banco por `--banco URL` ou `GS_BANCO`, pasta por `--pasta` |

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
