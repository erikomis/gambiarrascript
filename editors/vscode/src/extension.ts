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

// Cria um terminal cujo processo E o gs, passando os args como argv (sem shell),
// evitando interpretacao de metacaracteres no caminho do arquivo.
function rodarGs(args: string[]): void {
  const term = vscode.window.createTerminal({
    name: 'GambiarraScript',
    shellPath: caminhoDoGs(),
    shellArgs: args,
  });
  term.show();
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
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}
