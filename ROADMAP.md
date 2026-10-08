# ROADMAP — GambiarraScript

O que falta pra linguagem ficar **realmente usável** no dia a dia. A base
(parser, avaliador, funções, closures, erros, HTTP cliente/servidor, JSON,
REPL multiline, LSP e extensão VSCode) já está pronta — e agora também
**VM como motor padrão** (freevars, `importa`, `bora`, builtins de ordem
superior, REPL incremental), **libs padrão** (regex / tempo / crypto / banco /
set / fs / csv / gzip / processos / `formata` / matemática com `pi`),
**sintaxe moderna** (interpolação, range, bitwise, atribuição composta, `**`,
`crava`, lambdas, destructuring, `escolhe`/`caso`, ternário, `?.`/`??`,
fatias, dot access), **tooling** (`gs check/init/bench/get/build/testa/doc/
formata -w`, cache `.gsc`) e **distribuição** (release com binários,
`install.sh`, playground + doc no GitHub Pages).

Todos os tiers estão entregues (1–9, incluindo POO estilo Go, depurador,
TLS, pacotes com lock e instaladores pros três sistemas). O que sobra:

| Onde | O que sobra |
|---|---|
| Precisa de conta do mantenedor | publicar a extensão no VS Marketplace / Open VSX (hoje instala pelo `.vsix` de cada release) |
| Site / playground | testar Firefox e Safari; wasm menor |
| Fora de escopo (decidido) | método em tipo não-struct, visibilidade, `//`, FFI/cgo, keywords em inglês |

Histórico por versão no [CHANGELOG](CHANGELOG.md).

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
      bitwise e range (`OpRange`). Builtins de ordem superior vieram no Tier 3;
      hoje a VM é o motor padrão (Tier 7).
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
- [x] **Debug com breakpoints** — `gs debug [--tree] arq.gs [args]` (terminal,
      comandos em português com apelido de gdb: `para`/`b`, `segue`/`c`,
      `proximo`/`n`, `entra`/`s`, `sai`/`o`, `pilha`/`bt`, `ve`/`p`, `vars`,
      `lista`/`l`, `fluxos`, `quadro`, `tira`, `ajuda`; Ctrl+C pausa) e
      `gs debug --dap` (Debug Adapter Protocol no stdio; a extensão do VSCode
      registra o debugger `gambiarrascript` e sobe ele respeitando o
      `gambiarrascript.caminhoDoGs`). Decisões: o adapter é um processo
      separado do `gs lsp` (DAP e LSP são protocolos distintos; o VSCode sobe
      um por sessão de debug); a API de inspeção (`object.Depurador`/`Fluxo`:
      quadros com nome+linha, locais por quadro, globais do arquivo, avaliar
      no quadro) mora nos dois engines, em cima do gancho de linha da
      cobertura — sem depurador o caminho quente não muda (VM só paga com
      bytecode instrumentado); cada fluxo (principal, `bora`, handler do
      servidor, tarefa do paralelo) é uma thread do DAP e para sozinho
      (`allThreadsStopped: false`); breakpoint anda pra próxima linha
      executável (sem nenhuma: `verified: false`), com condição e logpoint nos
      dois engines; stopOnEntry, next/stepIn/stepOut/pause; lista, dicionário,
      conjunto e instância de treta expandem. Limitações: na VM o avaliar é só
      expressão e os locais vão por valor (statement/atribuir só no
      `--tree`); no `--dap` o programa lê stdin vazio; pausa só pega o
      próximo statement (fluxo preso em builtin espera ele voltar); sem
      breakpoint de exceção/função/hit count. Teste de ponta a ponta sobe o
      `gs debug --dap` de verdade e roda o cenário completo nos dois engines.
- [x] **multi-catch** — vários `quebrou NOME [se CONDICAO]` no mesmo `arruma`,
      tentados em ordem; o primeiro cujo filtro colar pega. Nenhum colou: o
      erro sobe depois do `finalmente`. `quebrou` sem filtro só como o último
      (erro de parse). Erro dentro do filtro sobe no lugar do original (igual
      Python). `se` é palavra-chave só ali (contextual, mesma linha). Os 2
      engines, formatter, LSP e gramática do VSCode. Veja
      `examples/multi_catch.gs`.

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
- [x] **FFI / integração com Go (cgo `importa_go`)** — decidido: **não vai
      ter**. cgo quebraria os binários estáticos (`CGO_ENABLED=0`) que rodam em
      qualquer máquina — release, `install.sh` e `gs build --alvo` dependem
      disso. O caminho pra estender é builtin em Go dentro do repo, ou
      conversar com outro programa via `roda_comando`/HTTP/sockets.

### Tier 4 — Qualidade de vida ✅ entregue

Ergonomia de sintaxe e correções que se sente falta no dia a dia:

- [x] **Linha nos erros da VM** — tabela esparsa pc→linha por função
      (`object.LinhaPC`, gravada pelo compiler no `emit`), resolvida no
      recover da VM. Mensagem idêntica ao tree-walker ("deu ruim na linha N:
      ..."), `erro_linha()` funciona após `quebrou` nos 2 engines, e a tabela
      sobrevive ao cache `.gsc`. De quebra: mensagens de divisão por zero e
      índice fora alinhadas byte a byte (teste de paridade garante). O stack
      trace na VM também já bate com o tree-walker (Tier 7).
- [x] **Índice negativo** — `xs[-1]` pega o último (estilo Python), na leitura
      e na atribuição (`xs[-1] += 1`). De quebra entrou **indexação de texto**
      (que não existia): `"café"[0]`/`[-1]`, rune-aware (conta caractere, não
      byte). Helper `object.IndiceNormalizado` compartilhado pelos 2 engines
      (tree-walker `evalIndex`/`evalAtribuiIndice` e VM `vmIndex`/`vmIndexSet`);
      teste de paridade. `examples/indice.gs`.
- [x] **Fatia sintática** — `xs[1:3]`, `xs[:2]`, `xs[2:]` pra lista e texto
      (o builtin `fatia` existe, mas a sintaxe é mais gostosa).
- [x] **`pra_cada` com índice/chave+valor** — `pra_cada i, v em lista` e
      `pra_cada chave, valor em dict`.
- [x] **Parâmetros com valor padrão** — `gambiarra f(x, y = 10)`.
- [x] **Varargs** — `gambiarra f(primeiro, ...resto)` (resto vira lista).
- [x] **Ternário / `se_colar` como expressão** —
      `bota x = se_colar cond entao a se_nao_colar b` (sem `acabou_finalmente`).
- [x] **Navegação segura** — `obj?.campo` (nada se obj for nada) e coalescing
      `x ?? padrao`. Roda nos 2 engines; corrigido bug de underflow de pilha
      na VM (OpPop espúrio no ramo não-nada do `?.`).
- [x] **`importa ... como`** — `importa "util.gs" como util` →
      `util.funcao()`, em vez de despejar tudo no escopo global (colisão de
      nome silenciosa). O `como` amarra so o alias nos dois engines.
- [x] **Constantes** — `crava NOME = valor`. Checagem estática compartilhada
      (`ast.ChecaCravadas`, pela ordem do fonte) roda antes da execução nos 2
      engines e no linter (`gs check`/LSP acusam como erro): `bota`, compostas,
      destructuring, `pra_cada`, `gambiarra`/`quebrou`/`importa como` com o nome
      cravado dão "`X` foi cravada, nao da pra mudar". Escopo de função igual
      ao `bota` (gambiarra pode sombrear); o conteúdo de lista/dict cravado
      ainda muda (igual const do JS). `examples/constantes.gs`.
- [x] **Potência e matemática** — operador `**` (associa à direita, prende
      mais que o menos unário: `-2 ** 2 == -4`) e `**=`; inteiro elevado a
      inteiro continua inteiro (`object.Potencia`, compartilhado pelos 2
      engines, `OpPow` + `OpBinConst` na VM, `.gsc` formato 2). Builtins
      `seno`/`cosseno`/`tangente`/`log`/`log10`/`exp` e o valor `pi`
      (`object.Predefinidas`). Sem `potencia`/`dorme`: `**` e `espera_ms`
      já cobrem (uma pegada por conceito). `examples/matematica2.gs`.

### Tier 5 — Stdlib ✅ entregue

- [x] **Processos** — `roda_comando(cmd, [args])` devolvendo
      `{saida, erro, codigo}` (código != 0 é dado, não erro; só não-iniciar é
      erro) e `sai([codigo])` pra encerrar o script. O `sai` é um objeto de
      controle `object.Sair` que desenrola blocos/loops/funções nos DOIS engines
      (tree-walker via propagação; VM via panic→`SaiRequisicao`), e o `cmd/gs`
      traduz pra `os.Exit(codigo)`. `examples/processo.gs`.
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
- [x] **fs parte 2** — `copia(de, pra)`, `move(de, pra)`, `tamanho_arquivo`
      (bytes), `modificado_em` (unix-segundos, encaixa no `formata_tempo`) e
      `glob("*.gs")` (sem match = lista vazia). Builtins puros, tree-walker **e**
      VM (teste de paridade); `examples/fs2.gs`.
- [x] **Datas parte 2** — `soma_tempo`, `sub_tempo`, `dia_da_semana`,
      `diferenca_dias`, `diferenca_horas`, `converte_tz` (timezone IANA, ex:
      "America/Sao_Paulo"). Veja `examples/datas.gs`.
- [x] **CSV** — `le_csv(caminho)` (1a linha vira cabeçalho, resto vira
      dicionários) e `escreve_csv(caminho, lista, [cabecalhos])` (cabeçalho
      custom opcional reordena colunas). Veja `examples/csv.gs`.
- [x] **Compressão** — `gzip_comprime(texto)` → base64 dos bytes gzipped,
      `gzip_descomprime(base64)` → texto original. Veja `examples/compressao.gs`.

### Tier 5b — Libs que faltavam ✅ entregue

- [x] **HTTP cliente turbinado** — `busca(url, {metodo, corpo, json,
      corpo_base64, cabecalhos, timeout})` com GET/POST/PUT/DELETE/PATCH/
      HEAD/OPTIONS; resposta não-texto ganha `corpo_base64`.
- [x] **Rede baixo nível** — TCP (`conecta_tcp`, `escuta_tcp`,
      `endereco`; modo linha ou bruto, timeout, parada graciosa), UDP
      (`escuta_udp`, `envia_udp`, `conecta_udp`) e WebSocket (`rota_ws` no
      servidor, `conecta_ws` no cliente), tudo via `envia`/`recebe`/`fecha`
      (object.Conexao).
- [x] **TLS no TCP cru** (e HTTPS) — `escuta(porta, {"tls": {"cert",
      "chave"}})` sobe HTTPS (e `wss://`), TLS 1.2+; `conecta_tcp(end,
      {"tls": deu_bom | {"servidor", "inseguro", "ca"}})` e `escuta_tcp(...,
      {"tls": {"cert", "chave"}})`; `busca`/`conecta_ws` ganham `ca` e
      `inseguro`; `gera_certificado([hosts])` (autoassinado, só pra dev).
      Cert/chave/CA aceitam caminho ou o PEM em si.
- [x] **Crypto parte 2** — `hash_senha`/`confere_senha` (bcrypt, custo 12),
      `encripta`/`decripta` (AES-256-GCM; chave crua ou frase via scrypt),
      `gera_chave`, `token_aleatorio` e JWT HS256 (`jwt_assina`/`jwt_confere`,
      recusa `alg` != HS256, erro do tipo `"jwt"`). Veja `examples/seguranca.gs`.
- [x] **Logging** — `log_debug`/`log_info`/`log_aviso`/`log_erro` no stderr,
      `GS_LOG_NIVEL` e `GS_LOG_FORMATO=json`, uma linha inteira por chamada.
- [x] **Parser de flags** — `opcoes(padroes, [ajudas])` com tipo vindo do
      padrao, `--ajuda` gerado e posicionais em `"_"`; mais `carrega_env` (.env)
      e `env(nome, padrao)`. Veja `examples/config.gs`.
- [x] **Servidor parte 2** — rota com `:param`/`*curinga` (404/405 com
      `Allow`), `pedido` com `params`/`json`/`ip`/`cookies`, `responde_json`
      e lista/dicionário virando JSON, `antes`/`depois` (middleware), `cors()`,
      `serve_pasta`, `escuta` com endereço texto + desligamento com calma
      (SIGINT/SIGTERM) + timeouts, erro no handler = 500 genérico + log.

### Bugs de motor — corrigidos e abertos

Achados rodando todo exemplo da doc nos 2 engines; testes em
`vm/correcoes_paridade_test.go` (saída exata nos dois, não só paridade).

- [x] `??` derrubava a VM (panic) quando o lado esquerdo não era `nada`.
- [x] `arruma`/`quebrou`/`finalmente` dentro de gambiarra não enxergavam os
      locais da função na VM ("freevar fora do range"); `funciona`/`vaza`/
      `continua` saíam sem rodar o `finalmente`; tail call dentro de `arruma`
      escapava do try.
- [x] `finalmente` sem `quebrou` engolia o erro (2 engines) — agora relança.
- [x] Fatia `xs[a:b]` compartilhava memória com a lista original.
- [x] `pra_cada de..ate` deixava a variável em fim+1 na VM; `pra_cada ... em`
      aninhado quebrava o laço de fora na VM.
- [x] Linter: blocos agora dividem o escopo da função (fim dos falsos
      "nunca usada"/"pode estar indefinido" em reatribuição dentro de bloco).
- [x] Escrita concorrente no mesmo dicionário (`bora`, handlers do `rota`/
      `rota_ws`/`escuta_tcp`) matava o processo com o "concurrent map writes"
      do Go. Lista/dicionário/conjunto agora têm trava própria (cada operação
      atômica, igual o GIL do Python), ligada só depois que o programa cria a
      primeira goroutine de usuário — programa de uma goroutine só paga uma
      leitura atômica (0–2% nas cargas de coleção, +5% no laço só de global).
      Operação composta usa `trava()` + `com_trava(t, fn)`. Testado com 300
      pedidos paralelos num servidor com estado compartilhado: o binário
      antigo morre, o novo fecha a conta exata.

Ainda abertos (pedem decisão de semântica):

- [x] Overflow de inteiro: a VM caía no real em parte das contas e dava a
      volta em outras (`-min`, `min / -1`), o tree-walker dava a volta em todas
      (`max + 1` virava o mínimo negativo). **Decisão:** conta inteira que
      estoura o int64 (`+`, `-`, `*`, `-x`, `/` exata) vira **real**
      (float64), nos dois engines e no constant folding — helpers
      compartilhados `object.SomaInt/SubInt/MulInt/NegInt/DivInt/RestoInt`
      (testes de sinal clássicos + `math/bits.Mul64`, sem divisão; conferidos
      contra `math/big`). Bit (`& | ^ ~ << >>`) em real continua erro
      (`& bitwise so faz sentido com inteiros`); `<<` desloca os bits sem
      checar estouro (é operação de bit, não de conta). `de_json` já lia
      inteiro gigante como real. De brinde: a VM fazia `%` e `/` exata entre
      inteiros em float (`(7 % 3) & 1` dava erro só na VM) e comparava
      inteiro com inteiro pelo float64 (`9007199254740993 == 9007199254740992`
      dava `deu_bom` só na VM). Bench (mediana de 7, main x novo intercalados):
      fib 1,512 → 1,511 ms, laço global 9,14 → 8,90 ms, laço local (novo
      `BenchmarkLoopLocal`) 8,35 → 8,59 ms (+3%: a checagem de "já botou?"
      da leitura de variável, abaixo).
- [x] `importa` com semântica de módulo de verdade, igual nos dois engines:
      cada módulo roda **uma vez por processo** (cache pelo caminho absoluto,
      `object.Modulos`); `como m` amarra só o `m` (namespace = dicionário em
      ordem alfabética, módulo com escopo próprio); import circular vira
      `importa circular: a.gs -> b.gs -> a.gs` (tipo `"parse"`, também entre
      goroutines, via grafo de espera); relativo ao arquivo que importa, com
      fallback em `gs_modulos/` subindo diretórios; goroutines importando
      juntas esperam a mesma execução. Na VM o módulo compila à parte
      (`OpImporta` + descritor `object.Modulo`); o `.gsc` confere o hash dos
      módulos e o `gs build` embute os módulos importados.
- [x] Valor de erro diferia entre engines: o tree-walker relançava o erro
      num `mostra erro`/`mostra e1` (o valor de um statement era confundido
      com desvio de fluxo no `evalBlock`/`evalProgram`), a VM relançava o que
      `erro_causa(e)` devolvia (erro pego não ficava marcado `Handled`).
      **Decisão:** erro pego pelo `quebrou` é só um valor — mostrar (imprime
      a mensagem, como a VM já fazia), guardar, passar, devolver de gambiarra,
      pôr em lista, comparar (`==` é identidade) e interpolar nunca relançam;
      só `quebra(...)` (ou um erro de runtime não pego) levanta erro.
      `erro_causa(e)` devolve a causa (valor) ou `nada`; `tipo(e)` é
      `"erro"`; `texto(e)`/`"${e}"` dão a mensagem; `pra_json(e)` é erro
      ("nao da pra virar json: ERRO" — use `erro_msg`). Erro relançado pelo
      `finalmente` volta a ser levantado.
- [x] VM não imprimia traço de pilha em erro de builtin (nem no topo nem
      dentro de função). Agora o builtin entra como frame mais de dentro
      (`em tamanho (linha N)`), o erro relançado pelo `finalmente` ganha os
      frames de fora, a gambiarra chamada por `mapeia` & cia ganha o
      `em <mapeia> (linha 0)` e a goroutine o `em <bora:g> (linha N)` — byte
      a byte igual o tree-walker (`TestParidadePilhaErrosDeBuiltin`).
- [x] Escopo de função divergia (o caso simples batia, o resto não): na VM o
      nome era resolvido pela ordem do texto. `n += i` num laço dentro de
      gambiarra lia sempre a global (somava errado), `enquanto x < 3 / x += 1`
      virava laço infinito, ramo de `se_colar` que não rodou deixava o slot
      com lixo da pilha (`gambiarra<f>` no lugar do valor), global declarada
      depois da função era erro de compilação, closure guardava cópia do
      valor (reatribuição depois de criar a closure não aparecia, e
      gambiarra aninhada recursiva/mútua quebrava). **Decisão (a regra do
      tree-walker, estilo Python sem `global`):** todo nome botado numa
      gambiarra/lambda (`bota`, `+=`, `crava`, desestruturação, variável do
      `pra_cada`, nome do `quebrou`, gambiarra aninhada) é local na função
      inteira; ler enxerga o escopo mais de dentro em que o nome **já tem
      valor** (local → função de fora → global → builtin), senão
      ``cade o `x`? voce nao botou isso ainda``; closure enxerga a
      variável (célula), não uma cópia — closures criadas num laço veem o
      último valor. Mexer pelo índice/ponto (`bota xs[0] = ...`, `d.n += 1`)
      continua mudando o objeto compartilhado. Na VM: varredura do corpo
      antes de compilar (`compiler/escopo.go`), globais de topo declaradas
      antes de rodar, slots de local zerados em toda chamada, leitura em
      cadeia (`OpGet*Ou` / `OpGet*Chk`) e células pros locais capturados.
      Nenhum exemplo nem bloco de código da doc dependia do comportamento
      velho da VM (rodei todos nos dois engines, antes/depois).
- [x] Interpolação engolia lixo calado (`"${3.14159:.2f}"` imprimia
      `3.14159`): um laço "se sobrou algo, ignora" no `parser.interpolar`.
      Agora sobra vira erro de parse — e o `:.2f` virou formato de verdade.
- [x] Não tinha como apagar chave de dicionário. Agora `remove(dic, chave)`
      (um nome só pro conceito: o mesmo `remove` da lista, e também
      `remove(conjunto, item)`). Mesmo contrato do de lista: devolve `nada`,
      tirar o que não está lá é no-op calado; chave que não pode ser chave
      (lista...) é erro, igual o `tem`. Chave que volta entra no fim da ordem.
      `remove_conjunto` continua devolvendo o próprio conjunto.
- [x] Estrutura que contém ela mesma (`adiciona(xs, xs)`) estourava a pilha
      do Go — fatal, derrubava o processo — no `mostra`, no `pra_json` e no
      `==`. Agora `mostra` imprime `[1, 2, [...]]` / `{"eu": {...}}` (igual
      Python), `pra_json` dá erro ("estrutura que contem ela mesma nao vira
      JSON") e `==` termina (dois ciclos do mesmo formato são iguais). Tudo
      por ponteiro, só no caminho atual (`[s, s]` sem ciclo sai inteiro), e
      sem alocar nada em estrutura rasa.
- [x] Miudezas: `tamanho()` aceita conjunto; o conjunto lembra a ordem de
      inserção igual o dicionário (`mostra`, `pra_cada` — que agora também
      aceita conjunto — e `uniao`/`intersecao`/`diferenca`), com
      bota/tira/contém O(1): os dois usam o mesmo miolo `ordenado`, em que
      remover deixa buraco e compacta quando passa da metade (o `Tira` do
      dicionário era O(n)); falha de conexão do `busca` (DNS, recusada,
      timeout, resposta cortada) vem com tipo `"rede"`; REPL na VM não lista
      mais os temporários `__*` no TAB.

### Tier 6 — Tooling / ecossistema

- [x] **`gs testa` parte 2** — flag `--vm` (roda a suíte na VM, com
      contagem de asserts via `vm.NovaComInterp`), filtro por nome
      (`gs testa -so aquele_teste`) e cobertura de linhas:
      `gs testa --cobertura` (resumo por arquivo + total),
      `--cobertura-perfil arq` (`arquivo:linha contagem`) e
      `--cobertura-html arq`. Em cima de um **gancho de linha** comum aos dois
      engines (`object.GanchoLinha`, chamado antes de cada statement
      executável; `ast.StatementsExecutaveis` é a regra única): o tree-walker
      testa um nil por statement; a VM só paga com `compiler.Instrumentar`,
      que emite `OpLinha` — o bytecode normal fica idêntico (sem bump do
      `.gsc`, e instrumentado nunca vai pro cache). Paridade de linhas e
      contagens testada (ramos, laços, funções não chamadas, métodos de
      treta, módulos, bora, exemplos). Base pro depurador (breakpoint/passo).
- [x] **`gs formata -w .`** — aceita diretório e varre recursivamente todos
      os `.gs` (helper `coletaArquivosGs`).
- [x] **`gs doc`** — novo subcomando: extrai a assinatura de cada `gambiarra`
      e os comentários `#` acima dela, gerando markdown de referência no stdout
      (aceita arquivo ou diretório).
- [x] **`gs instala`** — baixa todas as dependências do `gambiarra.json` de
      uma vez; `gambiarra.lock` (JSON ordenado: fonte + URL resolvida +
      sha256) pra build reprodutível — com lock, conteúdo que não bate é
      **recusado** (nada é gravado) e `--atualiza` re-resolve e reescreve.
      `gs get github.com/usuario/repo/caminho.gs@tag` fixa tag/commit via
      raw.githubusercontent; só https (http só com `--inseguro`, inclusive
      em redirecionamento).
- [x] **`gs build --alvo`** — cross-compile do standalone (linux/windows a
      partir do mac): baixa o `gs` da release da mesma versão pro os/arch,
      confere com o `checksums.txt`, guarda no cache do usuário
      (`gambiarrascript/<versão>/`) e embute o script; alvo windows sai
      `.exe`. Build de dev (sem release) → `--gs-base <gs-do-alvo>`. Ainda
      não embute os módulos importados (eles são lidos do diretório atual).
- [x] **REPL parte 2** — modo rico via `golang.org/x/term` quando a entrada é
      um TTY: histórico com setas ↑/↓, edição de linha, autocomplete no TAB
      (builtins + keywords + variáveis do escopo) e comandos `:ajuda`/`:limpa`.
      Cai no modo simples linha-a-linha em pipes/testes.
- [x] **Release CI** — `.github/workflows/release.yml` gera binários
      mac/linux/windows (CGO_ENABLED=0, versão via `-X main.Versao`) +
      checksums + `.vsix` numa GitHub Release a cada tag `v*`
      (`scripts/release`); `install.sh` (`curl | sh`, confere sha256, sem
      sudo); CI de `go vet`/`go test` em push/PR. **v0.2.0 publicada.**
      Homebrew: `brew install erikomis/tap/gambiarrascript` (repo
      `erikomis/homebrew-tap`; declara conflito com o ghostscript, que também
      instala `gs`). A fórmula vai pro tap sozinha a cada tag estável.
- [x] **Playground web** — docs + playground estáticos no GitHub Pages
      (`.github/workflows/pages.yml`, `scripts/build-web`). Roda num Web
      Worker (botão Parar + timeout, laço infinito não trava a aba), link de
      compartilhar (código comprimido no `#hash`), highlight próprio no
      CodeMirror e na doc (gramática TextMate da extensão), wasm gzipado.
- [x] **Lint parte 2** — feito no typechecker (`gs check` + LSP): **código
      morto** depois de `funciona`/`vaza`/`continua`, **variável `bota`
      declarada e nunca usada** (top-level isento) e **sombreamento**: avisa
      a gambiarra que lê um nome antes do primeiro `bota` dele quando ela
      atribui no nome em algum lugar (vira local na função inteira) e o nome
      existe fora — `total += 1` ou `bota total = total + 1` querendo mexer na
      global (ou na var da função de fora, numa closure). `bota x = ...` antes
      de ler é local de propósito: não avisa. Os locais vêm da mesma varredura
      do compilador (`compiler.NomesLocais`). Nenhum aviso nos exemplos nem na
      doc, fora os trechos que mostram a pegadinha de propósito.

### Tier 7 — Motor / performance

- [x] **VM como engine padrão** — `gs roda` agora executa na VM por padrão;
      `--tree` volta pro tree-walker (fallback). `--vm` segue aceito por
      compatibilidade. Todos os exemplos rodam idêntico nos dois; paridade de
      erro/linha/stack trace fechada.
- [x] **Stack trace na VM** — já implementado: `handleVMError` monta o `Traço
      de pilha: em f (linha N)` a partir dos frames (`frame.callPos` +
      `fn.Name` + `LinhaDoPC`), byte a byte igual ao tree-walker
      (`TestParidadePilhaErros` garante).
- [x] **Otimizações de bytecode** — **constant folding** (`2 + 3` vira `5` em
      compile time, recursivo, só nos casos byte-a-byte iguais ao runtime) e
      **interning de constantes** (Numero/Texto/Booleano repetidos reusam o
      índice no pool). Peephole (pop+push) ficou de fora — o folding já elimina
      o grosso do compute redundante; jump-fixup seguro fica pra depois.
- [x] **Tail call** — `funciona f(...)` em **auto-recursão** vira `OpTailCall`,
      que reusa o frame atual → recursão em cauda roda em profundidade CONSTANTE
      (testado com 200 mil níveis). Restrito a self-call de propósito: chamadas
      entre funções diferentes seguem empilhando, preservando o traço de pilha.
      De quebra, recursão funda não-cauda agora dá **erro limpo** ("recursao
      funda demais, passou de 1024 chamadas") em vez de panic do Go.
- [x] **Bench de regressão** — suite fixa `go test -bench=. ./vm/`
      (`vm/bench_test.go`): fib (recursão), sort (lista+ordena), json
      (de_json/pra_json). Reporta ns/op + allocs pra comparar commits.
- [x] **Otimizações de runtime** — guiadas por profile (pprof no fib). Duas
      grandes fontes de alocação cortadas: **cache de inteiros pequenos**
      (`object.NumInt` reusa singletons de -256..1024, já que `Numero` é
      imutável) e **reuso de Frame na VM** (frames alocados sob demanda por
      profundidade e reusados a cada chamada, em vez de um `&Frame{}` por
      `OpCall`). Resultado no fib: **~2× mais rápido e 99,9% menos alocações**
      (143k → 132 allocs/op), sem regressão em sort/json e com paridade/`-race`
      intactos.
- [x] **A VM virou o motor de verdade (não só o padrão do `gs roda`)** — a VM
      era o engine padrão só no `gs roda`; `gs build`, `gs testa`, `gs bench` e
      o playground WASM ainda caíam no tree-walker. Efeito colateral feio: o
      binário do `gs build` rodava **10× mais lento** que o `.gs` solto, e a
      suíte validava o motor que não é o de produção. Agora todos rodam na VM
      (`--tree` continua como fallback explícito em todos eles).
- [x] **`argumentos()` e `rota()` estavam quebrados na VM** — dois bugs
      funcionais no engine padrão, não de performance: `argumentos()` voltava
      lista vazia (o caminho da VM montava um `interpreter` próprio e vazio, sem
      os args do script) e `rota()` recusava o handler com "o handler tem que
      ser uma gambiarra, veio FUNCAO" (só aceitava `*object.Funcao`, o tipo do
      tree-walker, e na VM chega `*object.CompiledFunction`) — ou seja, não dava
      pra subir servidor nenhum com `gs roda`. Regressão coberta por teste
      (`vm/servidor_vm_test.go`).
- [x] **Chamada de gambiarra vinda de builtin: 33× mais rápida** — `mapeia`,
      `filtra`, `reduz` e `ordena_com` chamam a função do usuário uma vez por
      elemento, e cada chamada **clonava a VM inteira** (pilha de 16k slots =
      256 KB + 1024 frames). Mapear 200 mil elementos alocava mais de 50 GB e
      o GC afogava — o `mapeia` na VM chegava a ser **150× mais lento que no
      tree-walker**. Agora as VMs de chamada vêm de um `sync.Pool` e a pilha
      nasce com 512 slots crescendo sob demanda (`append` no `push`, mantido
      inlinável de propósito: com o cálculo inline o corpo passava de 81 no
      orçamento de inline do Go e custava ~30% no fib).
- [x] **Memória proporcional ao programa** — a pilha não nasce mais com 16k
      slots e o array de globais deixou de reservar `MaxGlobals` (1 MB zerado em
      toda VM): o compilador agora publica `Bytecode.NumGlobals` e a VM aloca
      exatamente isso. O tamanho é fixado no boot de propósito — o slice de
      globais é compartilhado com os clones do `bora`, então realocar deixaria
      os clones com o array velho.
- [x] **Bench honesto** — `BenchmarkMapeia` entrou na suíte, e `gs bench` passou
      a medir a VM. Medido lado a lado contra o commit anterior (mediana de 5
      rodadas): fib **2,0×**, sort **1,5×**, json **2,1×**, mapeia **32,7×** —
      com 105×, 13×, 5× e 2.628× menos memória, respectivamente.
- [x] **Dicionário com ordem de inserção** — iterar um dicionário usava a ordem
      do `map` do Go, embaralhada de propósito: `pra_cada k em d` saía numa
      ordem **diferente a cada execução**. Agora `object.Dicionario` guarda a
      ordem (`Bota`/`Tira`/`Chaves`/`Itera`), igual Python 3.7+ e JS. Sobrescrever
      chave existente não muda o lugar dela.
- [x] **JSON com parser próprio** — `de_json` usava `json.Unmarshal` num
      `map[string]interface{}`, que **perde a ordem das chaves**; o
      `json.Decoder` por token preserva mas ficou 66% mais lento. Um parser
      escrito à mão (`interpreter/json_parser.go`) resolve os dois: `de_json`
      ficou **1,9× mais rápido** que o original E preserva a ordem do documento;
      `pra_json` serializa direto (sem `map` intermediário), 24% mais rápido e
      respeitando a ordem de inserção. Round-trip `de_json` → `pra_json` agora
      devolve o documento igualzinho. De quebra, inteiro JSON vira inteiro
      exato (`NumInt`), sem passar por `float64`. Validado contra o
      `encoding/json` em ~50 casos, incluindo `\u` com pares surrogate,
      zero à esquerda e números malformados.
- [x] **Erro de compilação fala a língua do usuário** — "VM nao conhece `y`"
      (sem linha, vazando que existe uma VM por baixo) virou "linha 7: nao
      existe nenhum `y` por aqui — confere o nome ou declara com `bota y = ...`".
      Mesmo tratamento pra `vaza`/`continua` fora de laço.

- [x] **Arena de Numeros** — profile do laco apertado mostrou que o gargalo NAO
      e o dispatch: **99,92% das alocacoes vinham de `object.NumInt`**, um
      malloc de 24 bytes por operacao que sai do cache de inteiros (tres por
      iteracao). `object.ArenaNum` aloca em blocos de 32. Sem lock de proposito:
      vive na VM e uma VM roda numa goroutine so (clone do `bora` e sub-VM do
      pool ganham cada uma a sua). Laco 28% mais rapido, 32x menos alocacao.
      Bloco 32 e o meio-termo: um Numero vivo segura o bloco inteiro, entao
      128 daria so +20% de velocidade por 4x mais retencao no pior caso.
- [x] **Superinstrucao `OpBinConst`** — funde `OpConstant K` + operacao binaria
      e aplica a operacao contra o topo da pilha, no lugar: sai um dispatch, o
      push/pop da constante e o pop do operando, nos mesmos 4 bytes. Emitida
      direto pelo compilador, **nao por peephole** — e por isso nao esbarra no
      problema de jump apontando pro meio do par, que tinha deixado o peephole
      de fora. So funde com literal a DIREITA (fundir a esquerda mudaria a
      ordem de avaliacao). Laco e fib 17% mais rapidos cada.

**Acumulado do motor nesta leva**, medido FIM A FIM (`gs roda`, media de 3),
contra o binario do inicio da leva:

| carga | antes | agora | ganho |
|---|---|---|---|
| `mapeia` sobre 200 mil elementos | 1,847s | 0,052s | **35,2x** |
| `de_json`/`pra_json` 20 mil vezes | 0,066s | 0,047s | **1,41x** |
| laco de 5 milhoes | 0,263s | 0,198s | **1,33x** |
| `fib(30)` | 0,116s | 0,099s | **1,17x** |
| `ordena` 200x sobre lista de 500 | 0,059s | 0,059s | 1,00x |

> **Cuidado ao comparar com `go test -bench`.** Os numeros do bench sao bem
> maiores que estes, por dois motivos, e os dois enganam:
>
> 1. **O baseline.** Comparar com o ultimo commit media contra uma arvore SEM o
>    cache de inteiros e o reuso de Frame, que ja estavam aqui sem commitar.
>    Boa parte do "2,5x no fib" era esse trabalho, nao o desta leva.
> 2. **O setup.** `rodaBench` cria uma VM por iteracao (como `gs roda` faz),
>    entao o numero e setup + execucao. Em carga pequena (fib(22) e ~1,5ms) o
>    setup pesa, e ele encolheu muito quando a VM passou a alocar sob demanda —
>    o que aparece como ganho de "execucao" sem ser. `BenchmarkNovaVM` isola
>    esse custo; compare os dois antes de atribuir um ganho.
>
> Regra: bench pra pegar regressao entre commits vizinhos, medicao fim a fim
> pra afirmar ganho.

- [x] **REPL na VM** — era o último caminho no tree-walker, o que é pior que
      lentidão: dava pra uma construção funcionar no REPL e falhar no `gs roda`.
      Exigiu **compilação incremental** (`compiler.NovaEntrada` zera só o buffer
      de instruções e mantém pool de constantes, tabela de símbolos e funções já
      compiladas) mais uma `vm.Sessao` guardando os valores das globais e
      rodando cada entrada numa VM limpa em cima delas — de brinde, erro de
      runtime não deixa `sp`/frames sujos pra próxima linha. Corrigiu uma
      duplicação antiga: `mostra "oi"` saía como `oi` seguido de `=> oi`, porque
      o tree-walker devolvia o valor mostrado e o REPL imprimia de novo.

**O tree-walker agora é só fallback**: sobrou nos ramos `--tree` (que existem de
propósito) e como rede quando a VM não compila algo. Todo caminho padrão —
`gs roda`, `build`, `testa`, `bench`, REPL e o playground WASM — roda na VM.

- [x] **Max stack por funcao** — o compilador publica o `MaxStack` de cada
      gambiarra, do fluxo principal e do corpo de modulo (`compiler.MaxPilha`,
      igual o max_stack da JVM): simula o efeito de cada opcode na pilha por
      TODOS os caminhos (jumps, cadeia `OpGet*Ou`, catch do arruma entrando com
      a pilha zerada + o erro), com maximo nos pontos de juncao. A VM reserva
      `NumLocals + MaxStack` uma vez por frame (OpCall, tail call, metodo,
      `chamaCompilada`, clone do `bora`, `Run`, unwind pos-catch) e o `push`
      virou `stack[sp] = o; sp++` (custo de inline 30 -> 11). Opcode de aridade
      variavel le o operando/descritor; o que o `espalhaArgs`/`abreMetodo`
      abrem a mais eles mesmos reservam. Sem teto garantido (opcode sem efeito
      conhecido, jump torto, pilha negativa, laco que cresce a pilha) o
      `MaxStack` fica 0 e a funcao cai no caminho checado (reserva de 2 slots
      por byte de bytecode, refeita a cada `OpJump` pra tras). Nenhum exemplo
      nem teste cai nele (`TestMaxPilhaExemplos`), e todo opcode novo tem que
      ganhar efeito (`TestEfeitoPilhaCobreTodoOpcode`). Com
      `go test -tags gsdebugpilha ./...` o push confere o teto a cada empilhada
      — a suite inteira passa assim. `.gsc` formato 13.

      **Resultado medido: ganho dentro do ruido.** A/B contra o commit
      anterior, binarios de teste alternados, mediana de 9 rodadas de 1s
      (ns/op, antes -> depois): fib 1.519.933 -> 1.539.523 (+1,3%), loop
      9.319.072 -> 9.327.766 (+0,1%), loop local 8.917.850 -> 8.959.110
      (+0,5%), sort 162.221 -> 163.969 (+1,1%), json 305.477 -> 313.676
      (+2,7%), mapeia 642.268 -> 648.312 (+0,9%), NovaVM 137.704 -> 136.031
      (-1,2%). Fim a fim (`gs roda`, mediana, duas sessoes com a ordem
      trocada): `fib(30)` 0,078s -> 0,078s e 0,082s -> 0,077s; laco de 5
      milhoes 0,239s -> 0,227s e 0,236s -> 0,246s; `mapeia` sobre 200 mil
      0,032s -> 0,033s e 0,033s -> 0,033s. O sinal troca entre sessoes: e
      ruido, nao ganho nem regressao. Faz sentido: o Go continua fazendo o
      bounds check do `stack[sp]`, entao o `if sp >= len` do push antigo
      ja era praticamente o mesmo teste. Ficou mesmo assim porque nao
      regride e deixa a pilha com uma regra so (cresce so na reserva do
      frame) e um invariante conferivel — o modo `gsdebugpilha` pega pilha
      desbalanceada no codegen, que antes passava calada.

### Tier 8 — POO no modelo do Go (structs + métodos + interfaces, SEM herança) ✅ entregue

POO **copiando o jeito do Go** — composição no lugar de herança, interface
satisfeita de forma **implícita**, método como gambiarra **com receiver** e
"construtor" só por convenção (`nova_x`). Nada de `class`, `extends`, `this`,
`new` nem herança. Os dicts continuam existindo; a `treta` é a versão
**nomeada, com campos e métodos**. Guia em `web/content/docs/poo.mdx`,
exemplo em `examples/poo.gs`, paridade em `vm/poo_test.go`.

- [x] **Declarar struct** — `treta Nome` + um campo por linha (`x`,
  `x = padrao`) + `acabou_finalmente`. Tipo de campo ficou de fora (campo é
  dinâmico como tudo). Nome de treta (e de combinado) **começa com
  maiúscula** — é a regra que separa tipo de valor na sintaxe.
- [x] **Instanciar** — `Ponto{x: 1, y: 2}` (nomeado) ou `Ponto{1, 2}`
  (posicional: todos os campos, igual Go; `Ponto{}` = tudo zero). Não mistura
  os dois. Campo desconhecido/repetido = erro. **Ambiguidade com dicionário**:
  só vira literal quando o nome começa com maiúscula (ou `modulo.Nome`) **e** o
  `{` está na mesma linha — `mostra x {...}` e um `{...}` na linha de baixo de
  `mostra LIMITE` continuam dicionário solto como antes. Bloco não usa chave,
  então `se_colar p == Ponto{1, 2}` não tem a ambiguidade do Go.
- [x] **DECISÃO: zero-value** — o padrão declarado (`y = 0`) ou `nada` (sem
  tipo não dá pra ter `0`/`""` do Go). Padrão literal é constante; o resto vira
  thunk que roda **a cada instância** (`itens = []` não é compartilhado).
  Puxadinho nasce com a treta embutida zerada.
- [x] **Métodos com receiver** — `gambiarra (p Ponto) distancia()`; chamada
  `p.distancia()`; `obj.metodo` sem chamar = método ligado (vai pra
  `mapeia`/`rota`/`bora`). Aridade conta sem o receiver ("o metodo Ponto.move
  quer entre 1 e 2 parametro(s)").
- [x] **DECISÃO: receiver valor vs ponteiro** — **tudo referência, método muta
  a treta** (sem cópia no value receiver).
- [x] **DECISÃO: campo × método** — mesmo nome na mesma treta = erro na
  declaração do método (igual Go). Redeclarar método troca (igual gambiarra).
- [x] **DECISÃO: onde declara** — treta, combinado e método só no **topo** do
  arquivo (erro de parse dentro de bloco). Declare a treta antes dos métodos.
- [x] **Interface (`combinado`)** — assinaturas `escreve(texto)`, combinado
  embutido por nome; satisfação **implícita** e confere a aridade.
  `satisfaz(v, Tipo)` → booleano (combinado: tem os métodos; treta: é
  instância dela; combinado vazio = qualquer coisa).
- [x] **Type switch / assertion** — `tipo(instancia)` devolve o nome da treta
  (`object.NomeTipo`), então `escolhe tipo(v)` / `caso "Ponto"` é o type
  switch. `tipo(Ponto)` = `"treta"`, `tipo(Forma)` = `"combinado"`. Assertion
  é builtin (sem sintaxe nova): `como_tipo(v, Tipo)` devolve `v` ou quebra com
  o motivo.
- [x] **Composição / embedding (`puxadinho`)** — conceito, não keyword: linha
  da treta que é só um nome maiúsculo (ou `modulo.Nome`) embute. Campos e
  métodos sobem (promotion); acesso explícito `c.Animal.nome`; método
  promovido recebe a treta de dentro como receiver. Regra do Go: o mais raso
  ganha; empate na mesma profundidade só dá erro quando alguém acessa. No
  literal o campo promovido não vale direto (`Cachorro{Animal: Animal{...}}`).
- [x] **"Construtor" por convenção** — `gambiarra nova_ponto(x, y)`; só idioma
  (doc + snippet).
- [x] **DECISÃO: `==`** — campo a campo (mesma treta + campos iguais), igual
  struct do Go; protegido contra ciclo igual lista/dicionário.
- [x] **mostra / JSON** — `Ponto{x: 1, y: 2}` (texto com aspas, ciclo vira
  `Ponto{...}`); `pra_json` dá objeto com o puxadinho achatado (igual
  `encoding/json`); `bota {x, y} = p` desestrutura pelos campos.
- [x] **Concorrência** — instância segue o desenho das coleções
  (`object/colecoes.go`): trava só depois do modo concorrente, cada
  leitura/escrita de campo atômica, nunca segura duas travas; a tabela de
  métodos da treta também.
- [ ] **DECISÃO: métodos em tipos não-struct** — fora de escopo (foco em
  `treta`).
- [ ] **DECISÃO: visibilidade** — fora de escopo (a maiúscula aqui é sintaxe
  de tipo, não exportação).

Onde mexeu: `token` (TRETA, COMBINADO), `ast/poo.go`, `parser/poo.go`
(lookahead extra pro `gambiarra (p T)`), `object/poo.go` (regra única:
promotion, zero-value, satisfação, JSON), `interpreter/poo.go` e
`compiler/poo.go` + `vm/poo.go` (OpTreta/OpCombinado/OpMetodo/OpInstancia
com descritores no pool; `formatoGSC` 11), formatter (campo por linha com
comentário), LSP (keywords, hover, linter, outline com struct/campos/métodos,
definição/referências), REPL, `gs doc`, extensão VSCode e realce do site.

### Tier 9 — Próxima leva (sugestões novas, conferidas no código)

Coisas que não existem hoje e que a gente sente falta escrevendo exemplo e doc.

**Linguagem / stdlib (curto, alto impacto)**

- [x] **`tipo(x)`** — `"numero"`, `"texto"`, `"booleano"`, `"nada"`,
      `"lista"`, `"dicionario"`, `"conjunto"`, `"erro"`, `"futuro"`, `"cano"`;
      todo chamável (gambiarra, lambda, builtin) é `"funcao"`. Regra única em
      `object.NomeTipo`, compartilhada pelos 2 engines.
- [x] **Spread na chamada** — `f(...lista)`, misturável (`f(1, ...xs, 2)`),
      com builtin, lambda, dot-call, `bora`, varargs e defaults. VM via
      `OpCallEspalha`/`OpBoraEspalha` (o `OpCall` comum não mudou; fib sem
      regressão). Espalhar não-lista: "so da pra espalhar lista, veio X".
- [x] **Divisão inteira `//`** — decidido: **não vai ter**. `chao(7 / 2)`
      já cobre, e a linguagem mantém um nome por conceito (igual ficou sem
      `potencia`/`dorme`).
- [x] **Formato na interpolação** — `"${preco:.2f}"`, `"${n:05d}"`,
      `"${t:-8}"`: o que vem depois do último `:` fora de `()[]{}`/strings é o
      formato, com os verbos do `formata` sem o `%`. Formato inválido é erro
      de parse.
- [x] **multi-catch** — entregue no Tier 2 do jeito sugerido aqui:
      `quebrou erro se erro_tipo(erro) == "rede"`.

**Editor / LSP**

- [x] **Ir pra definição** e **achar referências** — resolve igual ao runtime
      (só gambiarra/lambda abre escopo), inclusive dentro de `${...}` e entre
      arquivos (`importa`, `importa ... como m` + `m.f`). Referências de
      símbolo de topo varrem os `.gs` abertos e o workspace.
- [x] **Renomear** (com prepareRename; recusa keyword, builtin, nome que
      já existe no escopo ou que capturaria outra referência), **outline do
      documento** e **signature help** (`(` e `,`, funciona com a linha pela
      metade, inclusive `m.f(`).
- [x] **Formatar documento** pelo LSP — mesmo formatter do `gs formata`.
      Posições em UTF-16 nas features novas; diagnostics e hover ainda contam
      runa (erra depois de emoji na mesma linha).
- [x] **`gs formata` apagava comentários e linhas em branco** (com `-w`,
      do disco). O lexer guarda comentário como trivia (`lexer.NewComTrivia`,
      só no formatter — `gs roda` não paga) e o formatter devolve cada um pela
      posição. Trava: `formata -w` e o LSP só gravam se o resultado tiver o
      mesmo AST e os mesmos comentários (`formatter.Confere`). Testado com os
      exemplos, todo bloco da doc e 11 mil variações com comentário em toda
      linha.
- [ ] **Publicar a extensão** no VS Marketplace e no Open VSX (hoje
      `"publisher": "local"`, só instala por `.vsix`).

**Projeto / distribuição**

- [x] **LICENSE** — MIT. Vai junto nos tarballs da release, no `.vsix` e na
      fórmula do Homebrew.
- [x] **Tap do Homebrew** — `erikomis/homebrew-tap` no ar com a 0.2.0.
- [x] **Automatizar o tap** — o `release.yml` dá push da fórmula preenchida
      no tap a cada tag estável, com uma deploy key de escrita só no repo do
      tap (secret `TAP_DEPLOY_KEY`), sem token pessoal.
- [x] **gofmt na árvore + gate no CI** — árvore formatada num commit só de
      formatação; o `ci.yml` barra arquivo `.go` desformatado.
- [x] **CHANGELOG** — `CHANGELOG.md` por versão, escrito pra quem usa a
      linguagem (não é a lista crua de commits).

**Site / playground**

- [x] **Botão "rodar no playground"** em todo bloco de código da doc — link
      `#c=` gerado no build (transformer do shiki + zlib), zero JS, funciona
      com clique do meio. ```` ```gambiarrascript sem-playground ```` desliga.
- [x] **`pergunta()` no playground** — caixa "Entrada (stdin)" preenchida
      antes de rodar (funciona no GitHub Pages, sem COOP/COEP): vira o stdin
      do programa, cada `pergunta` consome uma linha (ecoada na saída) e o
      fim devolve texto vazio como no nativo. Vai junto no link (`&e=`);
      exemplo "Quiz" no playground.
- [ ] **Testar Firefox e Safari** — só o Chromium foi exercitado (worker,
      `DecompressionStream`, compartilhar).
- [x] **Doc em inglês completa** — todas as 22 páginas em `docs/en/` (POO incluída).
- [ ] **Wasm menor** — 13,8 MB cru / 3,4 MB gzip; o grosso deve ser
      `net/http` dos builtins de rede, que nem funcionam no navegador. Build
      tag pra tirar rede/banco/processo do `cmd/wasm`.

---

## Migrar as palavras-chave para o INGLÊS (mas continua MEME) — decidido: fica em PT

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
