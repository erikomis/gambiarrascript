import * as vscode from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

function caminhoDoGs(): string {
  return vscode.workspace
    .getConfiguration('gambiarrascript')
    .get<string>('caminhoDoGs', 'gs');
}

// Coloca o texto entre aspas simples de forma segura pro shell POSIX (zsh/bash),
// pra caminhos com espaco ou metacaracteres nao quebrarem o comando.
function aspas(s: string): string {
  return `'${s.replace(/'/g, `'\\''`)}'`;
}

// Roda o gs DENTRO de um terminal normal (o shell continua vivo depois que o gs
// termina), pra saida nao sumir. Reaproveita o mesmo terminal entre execucoes.
function rodarGs(args: string[]): void {
  const term =
    vscode.window.terminals.find((t) => t.name === 'GambiarraScript') ??
    vscode.window.createTerminal('GambiarraScript');
  term.show();
  const cmd = [caminhoDoGs(), ...args].map(aspas).join(' ');
  term.sendText(cmd, true);
}

export function activate(context: vscode.ExtensionContext): void {
  context.subscriptions.push(
    vscode.commands.registerCommand('gambiarrascript.rodar', async () => {
      const ed = vscode.window.activeTextEditor;
      if (!ed || ed.document.languageId !== 'gambiarrascript') {
        vscode.window.showWarningMessage('Abre um arquivo .gs primeiro, parca.');
        return;
      }
      await ed.document.save();
      rodarGs(['roda', ed.document.fileName]);
    }),
    vscode.commands.registerCommand('gambiarrascript.repl', () => {
      rodarGs(['repl']);
    })
  );

  const server: ServerOptions = {
    command: caminhoDoGs(),
    args: ['lsp'],
    transport: TransportKind.stdio,
  };
  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: 'file', language: 'gambiarrascript' }],
  };
  client = new LanguageClient(
    'gambiarrascript',
    'GambiarraScript LSP',
    server,
    clientOptions
  );
  client.start();

  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration('gambiarrascript.caminhoDoGs')) {
        vscode.window.showInformationMessage(
          'Mudou o caminho do gs — recarrega a janela (Developer: Reload Window) pra valer pro language server.'
        );
      }
    })
  );
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}
