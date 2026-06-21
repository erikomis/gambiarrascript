# Extensão GambiarraScript (VSCode)

Highlight, snippets, comandos de rodar/REPL e um language server (erros sublinhados + autocomplete) para arquivos `.gs`.

## Pré-requisito

Tenha o binário `gs` instalado e no PATH (`./scripts/build && ./scripts/install`
na raiz do projeto). Se o `gs` não estiver no PATH, configure o caminho absoluto
em **Settings → `gambiarrascript.caminhoDoGs`**.

## Rodar em modo dev (sem publicar)

1. Compile: na raiz do projeto, `./scripts/build-extension`.
2. Abra a pasta `editors/vscode` no VSCode.
3. Aperte **F5** — abre uma janela "Extension Development Host".
4. Nessa janela, abra qualquer arquivo `.gs`.

## Checklist de verificação manual

- [ ] Highlight: keywords, strings, números e comentários `#` ficam coloridos.
- [ ] Snippets: digitar `gambiarra` + Tab expande o esqueleto da função.
- [ ] Erros: escreva `bota = 5` — o `=` fica sublinhado com a mensagem do parser.
- [ ] Rodar: com um `.gs` aberto, aperte **F5** (ou rode o comando
      "GambiarraScript: Rodar arquivo") — o resultado aparece no terminal.
- [ ] REPL: comando "GambiarraScript: Abrir REPL" abre o `gs repl` no terminal.
- [ ] Autocomplete: Ctrl+Espaço sugere keywords e variáveis já declaradas.

## Empacotar (.vsix), opcional

```bash
cd editors/vscode
docker run --rm -v "$PWD":/ext -w /ext node:20 npx --yes @vscode/vsce package
code --install-extension gambiarrascript-0.1.0.vsix
```
