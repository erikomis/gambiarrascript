# Changelog

Mudanças de cada versão do GambiarraScript que importam pra quem usa a
linguagem. O detalhe (o porquê de cada decisão) está nas mensagens de commit e
no [ROADMAP](ROADMAP.md).

## Não lançado

- **Migrações de banco**: `gs migra [sobe | desce [n] | status | novo nome]`
  aplica os arquivos `migracoes/NNN_nome.sobe.sql` (e desfaz com o
  `.desce.sql`) em SQLite, Postgres e MySQL/MariaDB. Cada migração roda numa
  transação, fica anotada em `gs_migracoes` com o checksum e arquivo já
  aplicado que mudou trava tudo com aviso claro. Banco por `--banco URL` (a
  mesma do `conecta`) ou `GS_BANCO`. A builtin `migra(conexao, [pasta])` faz
  o mesmo de dentro do script (servidor migrando na subida).
- **Validação de entrada**: `valida(valor, esquema)` devolve a lista de erros
  com o caminho de cada um (`"idade: tem que ser numero, veio texto"`,
  `"itens[2].preco: obrigatorio"`). Tipos `texto`, `numero`, `inteiro`,
  `booleano`, `lista`, `dicionario`, `email`, `data`; regras `obrigatorio`,
  `min`, `max`, `padrao`, `opcoes`, `itens`, `campos`; `"texto?"` = opcional e
  `"_estrito"` recusa campo a mais. O `examples/api_rest.gs` responde `400`
  com a lista.
- **Tarefas agendadas**: `a_cada(segundos, f)`, `depois_de(segundos, f)`,
  `agenda("0 9 * * 1-5", f)` (cron de 5 campos com `*`, listas, faixas e
  passos, no fuso local; atalhos `@minuto`, `@hora`, `@diario`, `@semanal`,
  `@mensal`, `@anual`) e `cancela(h)`. Cada agendamento roda na própria
  goroutine; erro na tarefa vai pro stderr e o agendamento segue. O `gs roda`
  fica de pé enquanto houver agendamento e o Ctrl+C (ou SIGTERM) para com
  calma.
- **Datas sem layout do Go**: `formata_data(t, "dd/mm/aaaa hh:mi")` e
  `le_data("08/10/2026", "dd/mm/aaaa")`, com nomes de mês e dia em português
  (`mmmm` → "outubro", `dddd` → "quinta-feira") e fuso opcional. `mm` é
  sempre mês; minuto é `mi`. `formata_tempo`/`parse_tempo` continuam.
- **Imagem Docker oficial**: `ghcr.io/erikomis/gambiarrascript` (linux amd64
  e arm64, distroless, usuário `nonroot`) publicada a cada release, com
  `:latest` só nas estáveis. Doc de deploy de API com
  `FROM ghcr.io/erikomis/gambiarrascript`.
- Cache `.gsc` antigo é descartado sozinho (formato 16: builtins novas).

## v0.7.1 — 2026-10-08

- **Playground com metade do tamanho**: o runtime baixa 1,9 MB em vez de
  4,0 MB (16,3 → 7,2 MB sem compressão). Rede, servidor, TLS e processo saem
  do build do navegador, onde não funcionam; os builtins continuam existindo e
  avisam na hora.
- No playground o `busca` travava até o timeout de 10 s; agora responde na
  hora que não roda no navegador.
- Playground testado no Chromium, Firefox e WebKit (Safari), também no CI.
- Código Go formatado com `gofmt` e o CI passa a barrar código desformatado.
- Teste de broadcast do WebSocket deixou de ser instável (o bug era do teste,
  não do servidor).
- Teste dos instaladores no CI consulta a API do GitHub com token (o limite
  por IP derrubava o job do Windows).

## v0.7.0 — 2026-10-08

- **Depurador**: `gs debug [--tree] arq.gs` no terminal (breakpoints com
  condição e logpoint, passo a passo, pilha, variáveis, avaliar expressão;
  comandos em português com apelidos de gdb) e `gs debug --dap` pro VSCode —
  a extensão registra o debugger `gambiarrascript` (F5, breakpoints na margem).

## v0.6.0 — 2026-10-07

- **POO no modelo do Go**: `treta` (struct), métodos com receiver
  (`gambiarra (p Ponto) distancia()`), `combinado` (interface implícita),
  `puxadinho` (composição), `satisfaz`/`como_tipo` e `tipo(v)` com o nome da
  treta.
- **Multi-catch**: `quebrou erro se erro_tipo(erro) == "rede"`, várias
  cláusulas tentadas em ordem.
- **`gs testa --cobertura`**: % de linhas por arquivo, perfil e relatório HTML.
- **Linter** avisa quando uma gambiarra atribui num nome que também é global
  (a global não muda — Python-style).
- **Instalador pro Windows** (`irm .../install.ps1 | iex`, sem admin) e doc
  de instalação com uma seção por sistema. Os instaladores agora sobrevivem
  ao limite da API do GitHub e são testados no CI em Windows, Linux e macOS.
- VM calcula o teto da pilha de cada função (sem ganho de velocidade
  mensurável; ganhou uma checagem de debug que pega pilha desbalanceada).
- Decidido: sem `//` (use `chao(a / b)`) e sem FFI/cgo (quebraria os
  binários estáticos).

## v0.5.0 — 2026-10-07

- **TLS**: `escuta(porta, {"tls": ...})` (HTTPS/wss), TLS no `conecta_tcp` e
  `escuta_tcp`, `ca`/`inseguro` no `busca`/`conecta_ws`, `gera_certificado()`
  pra desenvolvimento.
- **Pacotes**: `gs get` com versão (`github.com/u/r/mod.gs@v1.0.0`),
  `gambiarra.lock` com sha256, `gs instala` (recusa conteúdo adulterado) e
  `gs build --alvo linux/amd64` (cross-compile a partir da release).
- **`importa` de verdade**: o módulo roda uma vez só, `como m` não vaza nomes,
  import circular dá erro claro e o `gs build` embute os módulos.
- **Os dois motores iguais**: escopo de função estilo Python, inteiro que
  estoura vira real, valor de erro é só valor (mostrar não relança), traço de
  pilha da VM igual ao do interpretador.
- `remove(dicionario, chave)` apaga chave; conjunto com ordem de inserção;
  estrutura que contém a si mesma não derruba mais o processo.
- Playground com entrada (`pergunta`, `le_linhas`, `le_tudo`).
- Documentação em inglês completa.
- Segurança: o `gs instala` baixa da fonte do `gambiarra.json`, não da URL
  gravada no lock (um PR mexendo só no lock podia redirecionar o download).

## v0.4.0 — 2026-10-07

- **API HTTP**: rotas com parâmetro (`/usuarios/:id`) e curinga, 404/405
  automáticos, `pedido["json"]`/`params`/`cookies`/`ip`, `responde_json`,
  middleware `antes`/`depois`, `cors()`, `serve_pasta`, erro 500 sem derrubar
  o servidor, desligamento com calma no Ctrl+C.
- **WebSocket**: `rota_ws` no servidor e `conecta_ws` no cliente, usados com
  `envia`/`recebe`/`fecha` igual um `cano`.
- **Sockets**: TCP (`conecta_tcp`/`escuta_tcp`, por linha ou bruto, timeout)
  e UDP.
- **Segurança e config**: `hash_senha`/`confere_senha` (bcrypt),
  `jwt_assina`/`jwt_confere`, `encripta`/`decripta` (AES-256-GCM),
  `token_aleatorio`, `log_info`/`log_erro` (texto ou JSON), `opcoes()` (flags
  com `--ajuda`), `carrega_env()`.
- **Concorrência**: lista, dicionário e conjunto seguros pra uso paralelo —
  o servidor não morre mais com escrita concorrente; `trava()`/`com_trava()`
  pra operação composta.
- Segurança: `cors()` com credenciais exige lista de origens e o `rota_ws`
  só aceita a mesma origem por padrão.

## v0.3.0 — 2026-10-02

- `tipo(x)`, spread na chamada (`f(...lista)`) e formato na interpolação
  (`"${preco:.2f}"`).
- **LSP**: ir pra definição, referências, renomear, outline, ajuda de
  assinatura e formatar documento.
- `gs formata` parou de apagar comentários e linhas em branco, e só grava se
  o resultado tiver o mesmo código e os mesmos comentários.
- Botão "Rodar no playground" em todo exemplo da doc.
- A release atualiza o tap do Homebrew sozinha.

## v0.2.0 — 2026-10-02

Primeira release com binários prontos (macOS, Linux, Windows), `install.sh`,
tap do Homebrew, playground e documentação no GitHub Pages, operador `**`,
`seno`/`cosseno`/`log`/`pi`, constantes com `crava` e uma leva de correções
na VM.
