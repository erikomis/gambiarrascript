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

// Depurador: o VSCode sobe `gs debug --dap` (Debug Adapter Protocol no
// stdio) pra cada sessao. Respeita o gambiarrascript.caminhoDoGs.
class FabricaAdapter implements vscode.DebugAdapterDescriptorFactory {
  createDebugAdapterDescriptor(
    _sessao: vscode.DebugSession
  ): vscode.ProviderResult<vscode.DebugAdapterDescriptor> {
    return new vscode.DebugAdapterExecutable(caminhoDoGs(), ['debug', '--dap']);
  }
}

// Sem launch.json (F5 direto num .gs): depura o arquivo aberto.
class ProvedorConfig implements vscode.DebugConfigurationProvider {
  resolveDebugConfiguration(
    _pasta: vscode.WorkspaceFolder | undefined,
    config: vscode.DebugConfiguration
  ): vscode.ProviderResult<vscode.DebugConfiguration> {
    if (!config.type && !config.request && !config.name) {
      const ed = vscode.window.activeTextEditor;
      if (ed && ed.document.languageId === 'gambiarrascript') {
        config.type = 'gambiarrascript';
        config.name = 'Depurar arquivo atual';
        config.request = 'launch';
        config.program = '${file}';
      }
    }
    if (!config.program) {
      vscode.window.showWarningMessage('Abre um arquivo .gs (ou poe o `program` no launch.json), parca.');
      return undefined;
    }
    return config;
  }
}

export function activate(context: vscode.ExtensionContext): void {
  context.subscriptions.push(
    vscode.debug.registerDebugAdapterDescriptorFactory('gambiarrascript', new FabricaAdapter()),
    vscode.debug.registerDebugConfigurationProvider('gambiarrascript', new ProvedorConfig()),
    vscode.commands.registerCommand('gambiarrascript.depurar', async () => {
      const ed = vscode.window.activeTextEditor;
      if (!ed || ed.document.languageId !== 'gambiarrascript') {
        vscode.window.showWarningMessage('Abre um arquivo .gs primeiro, parca.');
        return;
      }
      await ed.document.save();
      await vscode.debug.startDebugging(
        vscode.workspace.getWorkspaceFolder(ed.document.uri),
        {
          type: 'gambiarrascript',
          name: 'Depurar arquivo atual',
          request: 'launch',
          program: ed.document.fileName,
        }
      );
    }),
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
