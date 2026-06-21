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

function rodarNoTerminal(args: string): void {
  const nome = 'GambiarraScript';
  const term =
    vscode.window.terminals.find((t) => t.name === nome) ??
    vscode.window.createTerminal(nome);
  term.show();
  term.sendText(`"${caminhoDoGs()}" ${args}`);
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
      rodarNoTerminal(`roda "${ed.document.fileName}"`);
    }),
    vscode.commands.registerCommand('gambiarrascript.repl', () => {
      rodarNoTerminal('repl');
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
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}
