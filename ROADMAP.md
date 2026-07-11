# ROADMAP — GambiarraScript

O que falta pra linguagem ficar **realmente usável** no dia a dia. A base
(parser, avaliador, funções, closures, erros, HTTP cliente/servidor, JSON,
REPL multiline, LSP e extensão VSCode) já está pronta — e agora também
**VM completa** (freevars, `importa`, `bora`, builtins de ordem superior),
**libs padrão** (regex / tempo / crypto / banco / set / fs / `formata`),
**sintaxe moderna** (interpolação, range, bitwise, atribuição composta,
lambdas, destructuring, `escolhe`/`caso`, dot access) e **tooling**
(`gs check/init/bench/get/build/testa/formata -w`, cache `.gsc`).

Tiers 1–3 estão entregues; o backlog vivo agora são os **Tiers 4–7**
(qualidade de vida, stdlib, ecossistema e motor) + os itens grandes que
ficaram pra levas próprias (DAP, FFI, multi-catch).

---

## Tier 1 — Essencial (sente falta na hora) ✅ entregue

- [x] **Funções de texto**: separar, juntar, maiúsculo, minúsculo, substituir,
      fatiar, contém, começa_com / termina_com, tira_espaco (trim).
- [x] **Funções de lista**: adiciona, remove, ordena, inverte, mapeia, filtra,
      junta (lista → texto), **reduz, acha, acha_indice, unicos, achatada**.
- [x] **Ler entrada do usuário** (stdin) — ex: `pergunta("teu nome: ")`.

## Tier 2 — Pra projetos de verdade ✅ entregue

- [x] **Ler/escrever arquivo** — `le_arquivo` / `escreve_arquivo` / `anexa_arquivo`.
- [x] **Módulos / import** — `importa "caminho.gs"` (tree-walker **e VM**).
- [x] **Math** — raiz, aleatório, arredonda, teto, chão, abs, min, max.
- [x] **Argumentos de linha de comando** (`argumentos()`).
- [x] **Banco de dados** — `conecta`/`fecha` + **`consulta(conn, sql, [params])`** e
      **`executa(conn, sql, [params])`** com placeholders do driver (sqlite,
      mysql/mariadb, postgres). Veja `examples/banco.gs`.
- [x] **Regex** — `busca_regex`, `acha_regex`, `combina_regex`, `substitui_regex`,
      `separa_regex` (subs suporta `$1`, `$2`).
- [x] **Tempo / datetime** — `agora`, `agora_num`, `agora_ns`, `formata_tempo`
      (layout Go), `parse_tempo`, `duracao` (dict ou entre dois instantes),
      `espera_ms`.
- [x] **Crypto / codificação** — `md5`, `sha1`, `sha256`, `sha512`,
      `hmac_sha256`, `base64_codifica/decodifica`, `base32_*`, `hex_*`.
- [x] **Set (conjunto)** — `conjunto`, `contem_conjunto`, `adiciona_conjunto`,
      `remove_conjunto`, `uniao`, `intersecao`, `diferenca`.
- [x] **VM completa** (fase 6f): freevars em closures (1, 2 e 3 níveis),
      `importa`, `bora`/`OpBoraCall` (concorrência na VM), `pra_cada em`
      lista/dicionário (`OpIterSeq`), atribuição por índice (`OpIndexSet`),
      bitwise e range (`OpRange`). A maioria dos exemplos roda igual no
      tree-walker e no `gs roda --vm` — **falta** só builtins de ordem superior
      (ver Tier 3 abaixo).
- [x] **Typechecker básico no LSP** — warnings pra uso de identificador não
      resolvível (não é builtin, keyword, var `bota`, param ou `quebrou`).

## Tier 3 — Polimento / tooling

- [x] Hover no LSP (docs de keywords e builtins).
- [x] Formatador (`gs formata arquivo.gs`).
- [x] Mais exemplos (`libs.gs`, `banco.gs`, `fs.gs`, `tier3.gs`, `maturidade.gs` cobrem as novas libs, operadores e sintaxe).
- [x] Comentário de bloco `/* ... */`.

## Produção (entregue anteriormente)

- [x] Erros robustos (`object.Erro` com `Line/Kind/Stack/Cause/Handled`),
      `quebra`, `erro_msg`, `erro_linha`, `erro_tipo`, `erro_pilha`,
      `erro_causa`, `envolve_erro`.
- [x] Streams / stdin pesado — `le_tudo`, `le_linhas`, `escreve`,
      `escreve_erro`, `anexa_arquivo`, `env`.
- [x] `gs testa`/`gs disasm`/REPL com `=> <valor>`.
- [x] Concorrência real — `Environment` thread-safe, handler do `escuta`
      roda em paralelo, `paralelo(lista, fn)` em goroutines.

---

## O que AINDA falta (próximos tiers, não bloqueiam uso)

### Tier 2 — Desejável

- [x] **`fs` completo** — `existe`, `eh_dir`, `deleta`, `cria_dir` (mkdir -p),
      `le_dir`, `caminho_junta/base/dir/ext/abs`. Veja `examples/fs.gs`.
- [x] **Interpolação de strings** — `"${expr}"` com `\${` pra escapar; roda no
      tree-walker e na VM.
- [x] **`finally`** — bloco `finalmente` no `arruma` (roda sempre; o `quebrou`
      virou opcional). Veja `examples/tier3.gs`.
- [x] **`ordena` com comparator** — builtin `ordena_com(lista, fn)` (fn devolve
      booleano menor-que OU número <0/0/>0).
- [x] **Unicode first-class no lexer** — lexer baseado em runes; identificadores
      aceitam letras Unicode; colunas contadas em runes.
- [x] **`gs formata -w`** — sobrescreve o arquivo (só quando algo mudou).
- [x] **`printf`** — builtin `formata(modelo, valores...)` com os verbos do Go
      (`%v %s %d %f`, padding `%05d`, casas `%.2f`). Booleano/nada saem na cara
      da linguagem (`deu_bom`, `nada`).
- [x] **Profiler básico** — `gs bench [--vm] arquivo.gs [n]` roda N vezes e
      reporta min/mediana/média/max.
- [x] **Package manager mínimo** — `gs init` (cria `gambiarra.json` +
      `principal.gs`), `gs get <url> [nome]` (baixa .gs validado pra
      `gs_modulos/` e registra a dependência no `gambiarra.json`). Sem
      versionamento/lockfile ainda.
- [x] **Enums de `erro_tipo` padronizados** — constantes `runtime`, `builtin`,
      `io`, `rede`, `parse`, `usuario` (ver `interpreter/errors.go`).
- [ ] Debug com breakpoints (DAP no LSP + `gs debug`) — grande; fica pra uma
      leva própria.
- [ ] multi-catch (vários `quebrou` filtrando por `erro_tipo`) deixar para depois esse.

### Tier 3 — Maturidade

- [x] **Operadores bitwise** — `& | ^ ~ << >>` (binários + prefixo `~`), com
      **literais hex/oct/bin** (`0xFF`, `0o17`, `0b1010`). Tree-walker e VM
      (`OpBAnd/OpBOr/OpBXor/OpBNot/OpLShift/OpRShift`). Veja `examples/tier3.gs`.
- [x] **Range `..`** — `1..5` vira lista inclusiva (cresce ou decresce, guarda de
      `RangeMax`); tree-walker e VM (`OpRange`).
- [x] **Atribuição composta** — `+= -= *= /= %= &= |= ^= <<= >>=`, sem `bota`,
      em variável, índice (`xs[i] += 1`) e campo (`obj.n += 1`). Desugar no
      parser (os engines veem um `bota` normal; o formatter preserva a forma).
- [x] **Builtins de ordem superior na VM** — gancho `ChamaCompilada`
      (interpreter → `vm.chamaCompilada`): `mapeia`, `filtra`, `reduz`, `acha`,
      `acha_indice`, `ordena_com`, `paralelo` chamam gambiarras do usuário nos
      DOIS engines. (De quebra: `reduz`/`acha`/`acha_indice` só aceitavam
      builtin como fn — corrigido.)
- [x] **`async`/`await`** — via `bora fn(args)` (dispara e devolve Futuro) +
      `espera(futuro)` (aguarda; aceita lista de futuros em paralelo). Builtin,
      não keyword — decidido que não precisa de açúcar sintático extra.
- [x] **Lambdas anônimas** — `gambiarra(x) ... acabou_finalmente` como
      expressão: atribuível, passável pra builtin, valor de dict.
- [x] **Destructuring** — `bota [a, b] = lista` (posição) e `bota {x, y} = dict`
      (chave); faltante vira `nada` (lenient). VM via `OpIndexOuNada`.
- [x] **match/switch** — `escolhe x / caso v1, v2 / se_nao_colar /
      acabou_finalmente`, sem fallthrough, igualdade do `==`.
- [x] **Generics** — N/A: a linguagem é dinâmica, toda gambiarra já é genérica
      por natureza. Fechado sem código.
- [x] **Records + métodos** — via dicts + dot access: `obj.campo` lê,
      `bota obj.campo = v` escreve, `obj.metodo(obj)` chama (açúcar pra
      `obj["campo"]`, funciona nos 2 engines). `struct` formal declarado
      ficou dispensado por ora.
- [x] **REPL multiline** — bloco aberto (se_colar/gambiarra/escolhe/...)
      continua lendo com prompt `.........` até fechar os `acabou_finalmente`.
- [x] **`gs check`** — parse + lint (typechecker do LSP) com linha:coluna;
      exit 1 em erro de parse.
- [x] **`gs init`** — esqueleto `gambiarra.json` + `principal.gs`.
- [x] **`gs build`** — binário standalone (embute a fonte no próprio `gs`;
      re-assina ad-hoc no macOS). `./binario args...` roda o script direto.
- [x] **Cache de bytecode** — `gs roda --vm --cache arquivo.gs` grava/reusa
      `arquivo.gsc` (gob; invalida por hash da fonte, versão e nº de builtins).
- [x] **Decisão PT→EN** — decidido (jul/2026): **mantém PT por enquanto**;
      inglês meme fica pra depois, se rolar, como ALIAS (sem quebrar PT).
      Detalhes na seção abaixo.
- [ ] FFI / integração com Go (cgo `importa_go`) — grande; leva própria.

### Tier 4 — Qualidade de vida (curto prazo, alto impacto)

Ergonomia de sintaxe e correções que se sente falta no dia a dia:

- [x] **Linha nos erros da VM** — tabela esparsa pc→linha por função
      (`object.LinhaPC`, gravada pelo compiler no `emit`), resolvida no
      recover da VM. Mensagem idêntica ao tree-walker ("deu ruim na linha N:
      ..."), `erro_linha()` funciona após `quebrou` nos 2 engines, e a tabela
      sobrevive ao cache `.gsc`. De quebra: mensagens de divisão por zero e
      índice fora alinhadas byte a byte (teste de paridade garante). Falta só
      o stack trace na VM (Tier 7).
- [ ] **Índice negativo** — `xs[-1]` pega o último (estilo Python). Hoje dá
      "indice fora da lista". Vale pra lista e texto.
- [ ] **Fatia sintática** — `xs[1:3]`, `xs[:2]`, `xs[2:]` pra lista e texto
      (o builtin `fatia` existe, mas a sintaxe é mais gostosa).
- [ ] **`pra_cada` com índice/chave+valor** — `pra_cada i, v em lista` e
      `pra_cada chave, valor em dict`. Hoje só itera um nome.
- [ ] **Parâmetros com valor padrão** — `gambiarra f(x, y = 10)`.
- [ ] **Varargs** — `gambiarra f(primeiro, ...resto)` (resto vira lista).
- [ ] **Ternário / `se_colar` como expressão** — algo tipo
      `bota x = se_colar cond entao a se_nao_colar b` (sintaxe a decidir).
- [ ] **Navegação segura** — `obj?.campo` (nada se obj for nada) e coalescing
      `x ?? padrao`. Nomes/símbolos a decidir pra manter a zoeira.
- [ ] **`importa ... como`** — `importa "util.gs" como util` →
      `util.funcao()`. Hoje o importa despeja tudo no escopo global (colisão
      de nome é silenciosa).
- [ ] **Constantes** — declaração que não pode ser reatribuída
      (`crava PI = 3.14`? nome a decidir).

### Tier 5 — Stdlib que ainda falta

- [ ] **Processos** — `roda_comando(cmd, [args])` devolvendo
      `{saida, erro, codigo}`, e `sai(codigo)` pra encerrar o script.
      Essencial pra scripts de automação (a praia da linguagem).
- [x] **Lista estatística/agrupamento** — `soma`, `media`, `zip(a, b)`,
      `enumera(lista)`, `ordena_por(lista, "campo")` (lista nova, não muta) e
      `agrupa_por(lista, fn)` (higher-order via `ChamaCompilada`). Tree-walker
      **e** VM (teste de paridade); `examples/stats.gs`. De quebra, corrigido bug
      de paridade: binding do usuário (`bota`/`gambiarra`) agora sombreia builtin
      na VM igual ao tree-walker.
- [x] **Aleatório de verdade** — `semente(n)` (reprodutível), `embaralha(lista)`
      (Fisher-Yates, não muta), `escolhe_um(lista)`, `uuid()` (v4). Gerador
      compartilhado thread-safe (mutex); `semente` afeta `aleatorio` também.
      `examples/aleatorio.gs`.
- [ ] **fs parte 2** — `copia(de, pra)`, `move(de, pra)`, `tamanho_arquivo`,
      `modificado_em`, `glob("*.gs")`.
- [ ] **Datas parte 2** — soma/subtração de durações a instantes, timezone,
      `dia_da_semana`, diferença entre datas em dias/horas.
- [ ] **CSV** — `le_csv` / `escreve_csv` (lista de dicts ↔ arquivo).
- [ ] **Compressão** — `gzip_comprime` / `gzip_descomprime` (e talvez zip).

## Deve libs para essa linguagem
      - [ ] **HTTP cliente turbinado** — `busca` com verbo custom (PUT/DELETE/PATCH),
            headers, timeout e body binário. Hoje cobre o básico.
      - [ ] **Rede baixo nível** — TCP/UDP (`conecta_tcp`, `escuta_tcp`) e
            WebSocket (cliente e servidor).
      - [ ] **Crypto parte 2** — AES (`encripta`/`decripta`) e hash de senha
            (bcrypt/argon2) — md5/sha são pra checksum, não pra senha.
      - [ ] **Logging** — `log_info` / `log_aviso` / `log_erro` com timestamp,
            nível configurável por env e saída em stderr.
      - [ ] **Parser de flags** — `opcoes({"porta": 8080, "verboso": deu_ruim})`
            lendo `--porta 9090 --verboso` dos argumentos.

### Tier 6 — Tooling / ecossistema

- [ ] **`gs testa` parte 2** — flag `--vm`, filtro por nome
      (`gs testa -so aquele_teste`), e cobertura simples (% de linhas
      visitadas).
- [ ] **`gs formata -w .`** — recursivo em diretório (hoje é arquivo a
      arquivo).
- [ ] **`gs doc`** — extrai os comentários `#` acima de cada `gambiarra` e
      gera markdown de referência do projeto.
- [ ] **`gs instala`** — baixa todas as dependências do `gambiarra.json` de
      uma vez; `gambiarra.lock` com hash pra build reprodutível; `gs get`
      com versão/tag na URL.
- [ ] **`gs build --alvo`** — cross-compile do standalone (linux/windows a
      partir do mac): precisa de binários `gs` pré-compilados por plataforma
      embutidos ou baixáveis.
- [ ] **REPL parte 2** — histórico com setas (readline), autocomplete de
      builtins/variáveis, `:ajuda` / `:limpa`.
- [ ] **Release CI** — GitHub Actions gerando binários mac/linux/windows a
      cada tag + fórmula do Homebrew (`brew install gambiarrascript`).
- [ ] **Playground web** — o build wasm já existe (`cmd/wasm`); falta o
      playground no site com editor, saída ao vivo e botão de compartilhar.
- [ ] **Lint parte 2** — variável declarada e não usada, sombreamento
      (redeclarar nome de fora), código morto depois de `funciona`.

### Tier 7 — Motor / performance

- [ ] **VM como engine padrão** — flip do `gs roda` pra VM quando fechar a
      paridade de mensagens de erro (linha/stack trace, ver Tier 4). O
      tree-walker vira fallback (`--tree`).
- [ ] **Stack trace na VM** — igual o tree-walker (`Traço de pilha: em f
      (linha N)`), guardando call-sites nos frames.
- [ ] **Otimizações de bytecode** — constant folding (`2 + 3` vira `5` em
      compile time), peephole (pop+push redundantes), interning de strings
      repetidas na pool de constantes.
- [ ] **Tail call** — recursão em cauda sem estourar frame (hoje MaxFrames
      = 1024).
- [ ] **Bench de regressão** — suite fixa de benchmarks (fib, sort, json,
      http) rodando no CI pra pegar regressão de performance entre commits.

---

## Migrar as palavras-chave para o INGLÊS (mas continua MEME)

> **DECISÃO (jul/2026): fica em PORTUGUÊS por enquanto.** A zoeira BR é a
> identidade da linguagem. Se um dia rolar inglês, será como **alias meme**
> (LookupIdent mapeando ambos pro mesmo token), nunca substituindo o PT.
> O material abaixo fica como referência pra esse futuro talvez.

Ideia original: **trocar as keywords (e os builtins) do
português/gambiarra para o inglês** pra ficar acessível pra fora do Brasil —
**MAS sem perder a zoeira**. Nada de `let`/`print`/`if` chatos: tem que ser
gíria/meme em inglês (estilo "no cap", "yeet", "lowkey", "dip"). A graça da
linguagem é o humor, então o inglês também tem que ser internetês.

Pontos de atenção dessa migração:

1. **Fonte da verdade** das keywords: `token/token.go` (mapa `keywords`).
2. Ao mudar, sincronizar **3 lugares**:
   - `token/token.go` — o mapa de keywords.
   - `lsp/server.go` — as listas `keywords` e `builtinsCompletion` (autocomplete).
   - `editors/vscode/syntaxes/gambiarrascript.tmLanguage.json` — as regex de cor.
   - (e `editors/vscode/snippets/gambiarrascript.json` — os snippets.)
3. Atualizar todos os `examples/*.gs` e os testes (`*_test.go`) que usam as
   keywords antigas.
4. **Sugestão:** suportar os dois (alias PT + EN) por um tempo, pra não quebrar
   os scripts existentes — `LookupIdent` pode mapear ambos pro mesmo token.

### Tabela de tradução sugerida (keywords)

| GambiarraScript (atual) | Inglês sugerido |
|-------------------------|-----------------|
| `bota`                  | `let`           |
| `mostra`                | `print`         |
| `se_colar`              | `if`            |
| `se_nao_colar`          | `else`          |
| `enquanto`              | `while`         |
| `pra_cada`              | `for`           |
| `de`                    | `from`          |
| `ate`                   | `to`            |
| `em`                    | `in`            |
| `gambiarra`             | `func`          |
| `funciona`              | `return`        |
| `arruma`                | `try`           |
| `quebrou`               | `catch`         |
| `vaza`                  | `break`         |
| `continua`              | `continue`      |
| `deu_bom`               | `true`          |
| `deu_ruim`              | `false`         |
| `nada`                  | `nil`           |
| `acabou_finalmente`     | `end`           |
| `e`                     | `and`           |
| `ou`                    | `or`            |
| `nao`                   | `not`           |

### Tabela de tradução sugerida (builtins)

| Atual      | Inglês sugerido |
|------------|-----------------|
| `tamanho`  | `length`        |
| `chaves`   | `keys`          |
| `tem`      | `has`           |
| `texto`    | `string`        |
| `numero`   | `number`        |
| `busca`    | `fetch`         |
| `rota`     | `route`         |
| `escuta`   | `listen`        |
| `de_json`  | `parse_json`    |
| `pra_json` | `to_json`       |

> Obs: trocar pro inglês descaracteriza o tema "gambiarra/zoeira BR". Avaliar se
> a ideia é **substituir** de vez ou **adicionar inglês como alias** mantendo o
> português como identidade da linguagem.
