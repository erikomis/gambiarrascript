package main

// Versao do binário gs. E var (nao const) pra o release injetar a tag via
// -ldflags "-X main.Versao=1.2.3"; sem isso fica o valor padrao abaixo.
var Versao = "0.8.0"
