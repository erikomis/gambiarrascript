package object

// ---- Gancho de linha (API Go, compartilhada pelos dois engines) ----
//
// E a infraestrutura que a cobertura do `gs testa --cobertura` usa hoje e que
// um depurador (breakpoint, passo a passo) vai usar depois. Contrato:
//
//   - O gancho e chamado ANTES de cada statement executavel do usuario rodar
//     (a lista sai de ast.StatementsExecutaveis: o que conta e o mesmo nos
//     dois engines, e cada execucao do statement dispara uma vez — volta de
//     laco dispara de novo, ramo que nao rodou nao dispara).
//   - Recebe um *SitioLinha estavel: o mesmo ponteiro toda vez que o mesmo
//     statement roda (da pra usar como chave de mapa). Arquivo e o caminho
//     absoluto do .gs ("" quando o programa nao veio de arquivo); Linha e a
//     linha onde o statement comeca. Nao altere o sitio.
//   - Roda SINCRONO, na goroutine que esta executando o script: bloquear
//     dentro do gancho pausa so aquele fluxo (e o que um breakpoint faz).
//     Com `bora`/paralelo/servidor o gancho e chamado de varias goroutines
//     ao mesmo tempo — quem implementa sincroniza o proprio estado.
//   - Gancho nil = desligado. Tree-walker: Interpreter.DefinirGancho(g) antes
//     do Eval (custo desligado: um teste de nil por statement). VM: compila
//     com compiler.Instrumentar = true (o compilador emite um OpLinha antes de
//     cada statement executavel; sem instrumentar o bytecode e identico ao de
//     sempre, custo zero) e chama vm.DefinirGancho(g) antes do Run. Bytecode
//     instrumentado nunca vai pro cache .gsc.
//
// Pro depurador: o sitio da o ponto de parada (arquivo:linha). Estado extra
// (pilha de chamadas, variaveis) entra depois como campo novo do sitio ou
// como um segundo argumento — mudar a assinatura aqui muda os dois engines
// num lugar so.

// SitioLinha identifica um statement executavel: arquivo e linha.
type SitioLinha struct {
	Arquivo string
	Linha   int
}

// GanchoLinha e o callback chamado antes de cada statement executavel.
type GanchoLinha func(s *SitioLinha)
